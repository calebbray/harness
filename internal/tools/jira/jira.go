package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const jiraDomain = "https://lightcast-io.atlassian.net"

type JiraSaver interface {
	SaveIssue(key, title string, loe int, teamId int, started_at, finished_at int64) error
	GetTeamIdByName(string) (int, error)
}

type statusCategories map[string]string

type JiraTools struct {
	client     *jiraClient
	store      JiraSaver
	once       sync.Once
	categories statusCategories
}

func NewJiraTools(store JiraSaver) (*JiraTools, error) {
	client, err := newClient()
	if err != nil {
		return nil, err
	}
	return &JiraTools{client: client, store: store}, nil
}

func (jt *JiraTools) getCategories() error {
	var err error
	jt.once.Do(func() {
		jt.categories, err = jt.client.fetchStatusCategories()
	})
	return err
}

type jiraClient struct {
	*http.Client
	key  string
	user string
}

func newClient() (*jiraClient, error) {
	key := os.Getenv("JIRA_API_KEY")
	user := os.Getenv("JIRA_USERNAME")
	if key == "" || user == "" {
		return nil, fmt.Errorf("missing jira username or key")
	}
	return &jiraClient{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		key:  key,
		user: user,
	}, nil
}

func (jc *jiraClient) get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, jiraDomain+path, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(jc.user, jc.key)

	res, err := jc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return io.ReadAll(res.Body)
}

func (jc *jiraClient) fetchStatusCategories() (statusCategories, error) {
	data, err := jc.get("/rest/api/2/status")
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Id       string `json:"id"`
		Category struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	categories := make(statusCategories, len(raw))
	for _, s := range raw {
		categories[s.Id] = s.Category.Key
	}
	return categories, nil
}

func (jc *jiraClient) fetchSprintIssues(teamBoardName string) ([]jiraIssue, error) {
	// for now I'm just having a simple switch, but will have a better implementation for this when we have some more caching
	var boardId string
	switch teamBoardName {
	case "API":
		boardId = "467"
	case "ADR":
		boardId = "6988"
	case "APIENG":
		boardId = "478"
	}

	data, err := jc.get(fmt.Sprintf("/rest/agile/1.0/board/%s/sprint?state=active", boardId))
	if err != nil {
		return nil, err
	}

	var b board
	if err = json.Unmarshal(data, &b); err != nil {
		return nil, err
	}

	if len(b.Values) == 0 {
		return nil, fmt.Errorf("no active srint for project")
	}

	data, err = jc.get(strings.TrimPrefix(b.Values[0].SprintUrl, jiraDomain) + "/issue")

	if err != nil {
		return nil, err
	}
	var s sprint
	if err = json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return s.Issues, nil
}

func (jt *JiraTools) issueCycleTime(key string, ch chan cycleReport) error {
	data, err := jt.client.get(fmt.Sprintf("/rest/api/2/issue/%s?expand=changelog", key))
	if err != nil {
		return err
	}

	var cl changelog
	if err = json.Unmarshal(data, &cl); err != nil {
		return err
	}

	s, sok := cl.start(jt.categories)
	f, fok := cl.finish(jt.categories)
	if !sok || !fok {
		return nil
	}

	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid key format. Expected {team}-{issue number}, got=%s", key)
	}

	teamId, err := jt.store.GetTeamIdByName(parts[0])
	if err != nil {
		return fmt.Errorf("could not get id for team with name: %s (%s)", parts[0], err)
	}

	if err := jt.store.SaveIssue(key, cl.Fields.Title, int(cl.Fields.Loe), teamId, s, f); err != nil {
		fmt.Printf("Error saving issue to store %s\n", err)
	}
	ch <- cycleReport{Key: key, CycleTime: time.Duration(f - s)}
	return nil
}

const JiraCycleTimeSchema = `{
	"type": "object",
	"properties": {
		"project": {
			"type": "string",
			"description": "project id to use"
		}
	},
	"required": ["project"]
}`

type jiraCycleTimeInput struct {
	Project string `json:"project"`
}

func (jt *JiraTools) HandleJiraCycleTime(input json.RawMessage) (string, error) {
	err := jt.getCategories()
	if err != nil {
		return "", err
	}

	var in jiraCycleTimeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}

	// for now, we'll just construct the client every time
	jc, err := newClient()
	if err != nil {
		return "", err
	}

	issues, err := jc.fetchSprintIssues(in.Project)
	if err != nil {
		return "", err
	}

	cycleCh := make(chan cycleReport, len(issues))
	var wg sync.WaitGroup
	results := make(cycleData)

	for _, issue := range issues {
		wg.Add(1)
		go func(issue jiraIssue) {
			defer wg.Done()
			if err := jt.issueCycleTime(issue.Key, cycleCh); err != nil {
				fmt.Println(err)
			}
		}(issue)
	}

	go func() {
		wg.Wait()
		close(cycleCh)
	}()

	for report := range cycleCh {
		results[report.Key] = report.CycleTime
	}

	response := cycleTimeResponse{
		Issues: toIssueOutput(results),
		Metrics: metrics{
			Average:      results.Avg().String(),
			Median:       results.Median().String(),
			Percentile25: results.Percentile(25).String(),
			Percentile75: results.Percentile(75).String(),
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(data), nil

}

const SyncIssueSchema = `{
	"type": "object",
	"properties": {
		"project": {
			"type": "string",
			"description": "project id to use"
		}
	},
	"required": ["project"]
}`

func (jt *JiraTools) HandleJiraIssueSync(input json.RawMessage) error
