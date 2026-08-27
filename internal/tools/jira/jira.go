package jira

import (
	"bytes"
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
	SaveIssues([]Issue) (saved, updated int, err error)
	GetTeamIdByName(string) (int, error)
	GetIssuesFinishedInTimeframe(teamId int, start, end int64) ([]Issue, error)
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

func (jc *jiraClient) post(path string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, jiraDomain+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(jc.user, jc.key)
	req.Header.Set("content-type", "application/json")

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

func (jc *jiraClient) fetchSprintIssues(teamBoardName string) ([]jiraIssueKey, error) {
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
		return nil, fmt.Errorf("no active sprint for project")
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

type Issue struct {
	Key    string
	TeamId int
	Title  string
	// Pointer fields since these can be null in the db
	Loe        *int
	StartedAt  *int64
	FinishedAt *int64
}

func (jt *JiraTools) getIssue(key string) (Issue, error) {
	var i Issue
	i.Key = key
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		return i, fmt.Errorf("invalid key format. Expected {team}-{issue number}, got=%s", key)
	}

	data, err := jt.client.get(fmt.Sprintf("/rest/api/2/issue/%s?expand=changelog", key))
	if err != nil {
		return i, err
	}

	var cl changelog
	if err = json.Unmarshal(data, &cl); err != nil {
		return i, err
	}
	i.Title = cl.Fields.Title

	if cl.Fields.Loe != nil {
		v := int(*cl.Fields.Loe)
		i.Loe = &v
	}

	if s, ok := cl.start(jt.categories); ok {
		i.StartedAt = &s
	}

	if f, ok := cl.finish(jt.categories); ok {
		i.FinishedAt = &f
	}

	i.TeamId, err = jt.store.GetTeamIdByName(parts[0])
	if err != nil {
		return i, fmt.Errorf("could not get id for team with name: %s (%s)", parts[0], err)
	}

	return i, nil
}

const JiraCycleTimeSchema = `{
	"type": "object",
	"properties": {
		"project": {"type": "string", "description": "project id to use"},
		"start_date": {"type": "string", "description": "Lower bound, RFC3339, e.g. \"2026-08-08T16:13:05Z\". Defaults to 2 weeks before end_date (or now)."},
		"end_date": {"type": "string", "description": "Upper bound, RFC3339, e.g. \"2026-08-08T16:13:05Z\". Defaults to now, or 2 weeks after start_date if only start_date is given."}
	},
	"required": ["project"]
}`

type jiraCycleTimeInput struct {
	Project   string `json:"project"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// TODO: realistically this belongs in some kind of utility package but for now it lives in jira
func resolveWindow(startStr, endStr string) (start, end time.Time, err error) {
	if startStr != "" && endStr != "" {
		start, err = parseRFCTimestamp(startStr)
		if err != nil {
			return start, end, err
		}
		end, err = parseRFCTimestamp(endStr)
		return start, end, err
	}

	if startStr == "" && endStr == "" {
		end = time.Now()
		start = end.AddDate(0, 0, -14)
		return start, end, nil
	}

	if startStr == "" {
		end, err = parseRFCTimestamp(endStr)
		if err != nil {
			return start, end, err
		}
		start = end.AddDate(0, 0, -14)
		return start, end, nil
	}

	start, err = parseRFCTimestamp(startStr)
	if err != nil {
		return start, end, err
	}
	end = start.AddDate(0, 0, 14)
	return start, end, nil
}

func (jt *JiraTools) HandleCycleTimeStatistics(input json.RawMessage) (string, error) {
	err := jt.getCategories()
	if err != nil {
		return "", err
	}

	var in jiraCycleTimeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}

	start, end, err := resolveWindow(in.StartDate, in.EndDate)
	if err != nil {
		return "", err
	}

	teamId, err := jt.store.GetTeamIdByName(in.Project)
	if err != nil {
		return "", err
	}

	issues, err := jt.store.GetIssuesFinishedInTimeframe(teamId, start.Unix(), end.Unix())
	if err != nil {
		return "", err
	}

	mets := make(cycleData)
	for _, iss := range issues {
		// safety guard around potential nil pointers. Should never encounter this.
		if iss.FinishedAt == nil || iss.StartedAt == nil {
			continue
		}
		mets[iss.Key] = time.Duration(*iss.FinishedAt-*iss.StartedAt) * time.Second
	}

	response := cycleTimeResponse{
		Issues: toIssueOutput(mets),
		Metrics: metrics{
			Average:      mets.Avg().String(),
			Median:       mets.Median().String(),
			Percentile25: mets.Percentile(25).String(),
			Percentile75: mets.Percentile(75).String(),
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
			"description": "identifier of the team that to pull issues for. Might be in stringified number, jira project label, or a stored alias of a team."
		}
	},
	"required": ["project"]
}`

type jiraSyncIssuesInput struct {
	Project string `json:"project"`
}

func (jt *JiraTools) HandleJiraIssueSync(input json.RawMessage) (string, error) {
	err := jt.getCategories()
	if err != nil {
		return "", err
	}

	var in jiraSyncIssuesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	keys, err := jt.client.fetchSprintIssues(in.Project)
	if err != nil {
		return "", err
	}

	var issues []Issue
	var errors []error
	for _, key := range keys {
		iss, err := jt.getIssue(key.Key)
		if err != nil {
			errors = append(errors, fmt.Errorf("could not get issue %s: %s", key.Key, err))
			continue
		}
		issues = append(issues, iss)
	}

	saved, updated, err := jt.store.SaveIssues(issues)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Saved %d new issues, Updated %d existing issues", saved, updated), nil
}

func (jc *jiraClient) searchIssuesJQL(jql string) ([]Issue, error) {
	var allIssues []Issue
	pageToken := ""

	const maxPages = 50
	for range maxPages {
		body := jqlSearchRequest{JQL: jql, Fields: []string{"summary"}, MaxResults: 100}
		if pageToken != "" {
			body.NextPageToken = pageToken
		}

		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		data, err := jc.post("/rest/api/3/search/jql", payload)
		if err != nil {
			return nil, err
		}

		var res jqlSearchResponse
		if err = json.Unmarshal(data, &res); err != nil {
			return nil, err
		}
		allIssues = append(allIssues, res.Issues...)

		if res.IsLast {
			return allIssues, nil
		}

		if res.NextPageToken == "" {
			return allIssues, fmt.Errorf("pagination error: isLast == false but no next token")
		}

		pageToken = res.NextPageToken
	}
	return allIssues, fmt.Errorf("exceeded %d pages, stopping to avoid endless loop", maxPages)
}

type jqlSearchRequest struct {
	JQL           string   `json:"jql"`
	Fields        []string `json:"fields"`
	MaxResults    int      `json:"maxResults,omitempty"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

type jqlSearchResponse struct {
	Issues        []Issue `json:"issues"`
	IsLast        bool    `json:"isLast"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
}
