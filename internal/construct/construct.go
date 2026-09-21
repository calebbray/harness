package construct

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/orchestrator"
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

		n.Status = plan.Approved

		_, err = worktree.CommitWorktree(wt, fmt.Sprintf("construct: %s", n.Title))
		return err

	}
}

func AgentWork(c client.Client, logger *slog.Logger) func(worktreePath string, n *plan.TaskNode) error {
	return func(worktreePath string, n *plan.TaskNode) error {
		scopedLogger := logger.With("task_id", n.Id, "task_title", n.Title, "attempt", n.Attempt)

		a := agent.New(agent.AgentConfig{
			Client:              c,
			Tools:               tools.Scoped(worktreePath),
			Logger:              scopedLogger,
			EnforceConfirmation: false,
		})

		return a.Step(workerPrompt(n))
	}
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
