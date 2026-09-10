package permissions

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFloorDeniesSensitivePatterns(t *testing.T) {
	assert.True(t, IsForbidden("cat .env"))
	assert.True(t, IsForbidden("ls -la ~/.ssh/"))
}

func TestFloorAllowsNonsensitivePatterns(t *testing.T) {
	assert.False(t, IsForbidden("git status"))
}

func TestRulesResolve(t *testing.T) {
	rules, err := loadRulesFile("testdata/test_rules.json")
	require.NoError(t, err)
	p := &RulePolicy{global: rules}

	cases := []struct {
		input string
		want  Action
	}{
		{`{"command":"git status"}`, ActionAllow},
		{`{"command":"git diff HEAD~1"}`, ActionAllow},
		{`{"command":"rm -rf /tmp/foo"}`, ActionDeny},
		{`{"command":"go build ./..."}`, ActionConfirm},
	}

	for _, c := range cases {
		got := p.Resolve("bash", json.RawMessage(c.input))
		if got != c.want {
			assert.Equal(t, got, c.want)
		}
	}
}

func TestAddRule(t *testing.T) {
	rules, err := loadRulesFile("testdata/test_rules.json")
	require.NoError(t, err)
	p := &RulePolicy{project: rules}

	p.projectPath = fmt.Sprintf("%s/test_rules.json", t.TempDir())

	require.NoError(t, p.AddRule(ScopeProject, "fake_tool", "rm -rf", ActionDeny))

	updatedRules, err := loadRulesFile(p.projectPath)
	require.NoError(t, err)
	assert.Equal(t, "fake_tool", updatedRules[len(updatedRules)-1].Tool)
}
