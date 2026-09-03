package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJiraCycleTimeHandler(t *testing.T) {
	jt, err := NewJiraTools(MockSaver{})
	require.NoError(t, err)

	_, err = jt.HandleCycleTimeStatistics(json.RawMessage(`{"project": "API"}`))
	require.NoError(t, err)
}

func TestJiraSyncHandler(t *testing.T) {
	jt, err := NewJiraTools(MockSaver{})
	require.NoError(t, err)

	// iss, err := jt.searchIssuesJQL(`project = API AND resolved >= -14d ORDER BY resolved ASC`)
	// require.NoError(t, err)
	//
	// for _, i := range iss {
	// 	t.Log(i.Key)
	// }

	result, err := jt.HandleJiraIssueSync(json.RawMessage(`{"query": "project = API AND resolved >= -14d ORDER BY resolved ASC"}`))
	t.Log(result)
	require.NoError(t, err)
}

type MockSaver struct{}

func (MockSaver) SaveIssue(key, title string, loe int, teamId int, started_at, finished_at int64) error {
	return nil
}
func (MockSaver) SaveIssues([]Issue) (saved, updated int, err error) {
	return 0, 0, nil
}
func (MockSaver) GetTeamIdByName(string) (int, error) {
	return 0, nil
}
func (MockSaver) GetIssuesFinishedInTimeframe(teamId int, start, end int64) ([]Issue, error) {
	return nil, nil
}
