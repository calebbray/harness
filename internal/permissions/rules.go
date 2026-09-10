package permissions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

type Action string

const (
	ActionAllow   Action = "allow"
	ActionDeny    Action = "deny"
	ActionConfirm Action = "confirm"
)

type Rule struct {
	Tool   string `json:"tool"`
	Match  string `json:"match"`
	Action Action `json:"action"`
}

type RulePolicy struct {
	global      []Rule
	globalPath  string
	project     []Rule
	projectPath string
}

func NewRulePolicy(globalPath, projectPath string) (*RulePolicy, error) {
	global, err := loadRulesFile(globalPath)
	if err != nil {
		return nil, fmt.Errorf("rule policy: %w", err)
	}
	project, err := loadRulesFile(projectPath)
	if err != nil {
		return nil, fmt.Errorf("rule policy: %w", err)
	}
	return &RulePolicy{
		global: global, globalPath: globalPath,
		project: project, projectPath: projectPath,
	}, nil
}

func (p *RulePolicy) Resolve(tool string, input json.RawMessage) Action {
	if a, ok := matchRules(p.project, tool, input); ok {
		return a
	}

	if a, ok := matchRules(p.global, tool, input); ok {
		return a
	}

	return ActionConfirm
}

func (p *RulePolicy) AddRule(scope Scope, tool, match string, action Action) error {
	rule := Rule{Tool: tool, Match: match, Action: action}
	if scope == ScopeGlobal {
		p.global = append(p.global, rule)
		return writeRulesFile(p.globalPath, p.global)
	}
	p.project = append(p.project, rule)
	return writeRulesFile(p.projectPath, p.project)
}

func writeRulesFile(path string, rules []Rule) error {
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func loadRulesFile(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var rules []Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func matchRules(rules []Rule, tool string, input json.RawMessage) (Action, bool) {
	for _, r := range rules {
		if r.Tool == tool && strings.Contains(string(input), r.Match) {
			return r.Action, true
		}
	}
	return "", false
}
