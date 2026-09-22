package construct

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/orchestrator"
	"github.com/calebbray/personal-agent/internal/permissions"
	"github.com/calebbray/personal-agent/internal/plan"
	"github.com/calebbray/personal-agent/internal/tools"
	"github.com/calebbray/personal-agent/internal/worktree"
	"github.com/google/uuid"
)

type Config struct {
	RepoDir      string
	WorktreeRoot string
	BaseRef      string
	Work         func(worktreePath string, n *plan.TaskNode) error
}

func Execute(cfg Config) orchestrator.Executor {
	return func(n *plan.TaskNode) error {
		if err := cfg.construct(n); err != nil {
			return err
		}
		n.Status = plan.Approved
		return nil
	}
}

func (cfg Config) construct(n *plan.TaskNode) error {
	baseRef, err := deriveBaseRef(n, cfg.BaseRef)
	if err != nil {
		return err
	}

	wt, err := worktree.Add(cfg.RepoDir, filepath.Join(cfg.WorktreeRoot, n.Id.String()), labelTaskBranch(n.Id), baseRef)
	if err != nil {
		return err
	}

	if err := cfg.Work(wt.Path, n); err != nil {
		return err
	}

	_, err = worktree.CommitWorktree(wt, fmt.Sprintf("construct: %s", n.Title))
	return err
}

func Step(cfg Config) orchestrator.Executor {
	return cfg.construct
}

func AgentWork(c client.Client, logger *slog.Logger) func(worktreePath string, n *plan.TaskNode) error {
	return func(worktreePath string, n *plan.TaskNode) error {
		rp, err := permissivePolicy(worktreePath)
		if err != nil {
			return err
		}

		a := agent.New(agent.AgentConfig{
			Client:     c,
			Tools:      tools.Scoped(worktreePath),
			Logger:     logger.With("task_id", n.Id, "task_title", n.Title, "attempt", n.Attempt),
			RulePolicy: rp,
		})

		return a.Step(workerPrompt(n))
	}
}

func permissivePolicy(dir string) (*permissions.RulePolicy, error) {
	rp, err := permissions.NewRulePolicy(
		filepath.Join(dir, ".agent-rules-global.json"),
		filepath.Join(dir, ".agent-rules-project.json"),
	)
	if err != nil {
		return nil, err
	}
	for _, tool := range []string{"bash", "write_file"} {
		if err := rp.AddRule(permissions.ScopeProject, tool, "", permissions.ActionAllow); err != nil {
			return nil, err
		}
	}

	return rp, nil
}

const workerPreamble = `You are the implementation stage of a multi-agent coding pipeline. You are working on exactly one task from a larger plan; you do not see the rest of the plan, the planner's reasoning, or any other task. Use the available tools (bash, read_file, write_file) to make the actual change described below, working inside your assigned git worktree. When you are done, reply with a short plain-text summary of what you changed and why. Do not ask clarifying questions - nobody is watching this conversation. Make reasonable assumptions and note them in your summary.`

func workerPrompt(n *plan.TaskNode) string {
	var b strings.Builder
	b.WriteString(workerPreamble)
	fmt.Fprintf(&b, "\n\nTask (%s:\n%s\n)", n.Title, n.Description)
	if n.Attempt > 0 {
		fmt.Fprintf(&b, "\nThis is attempt %d - a previous attempt on this task needed rework.\n", n.Attempt+1)
	}
	return b.String()
}

func deriveBaseRef(n *plan.TaskNode, baseRef string) (string, error) {
	switch len(n.DependsOn) {
	case 0:
		return baseRef, nil
	case 1:
		return labelTaskBranch(n.DependsOn[0]), nil
	default:
		return "", fmt.Errorf("multi dependency is not yet supported")
	}
}

func labelTaskBranch(id uuid.UUID) string {
	return "task/" + id.String()
}
