package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJiraCycleTimeHandler(t *testing.T) {
	jt, err := NewJiraTools()
	require.NoError(t, err)

	data, err := jt.HandleJiraCycleTime(json.RawMessage(`{"project": "API"}`))
	require.NoError(t, err)
	t.Log(data)
}
