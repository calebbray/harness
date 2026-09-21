package pipeline

import (
	"github.com/calebbray/personal-agent/internal/orchestrator"
	"github.com/calebbray/personal-agent/internal/plan"
)

type ReviewResult struct {
	Approved bool
	Notes    string
}

type IntegrateResult struct {
	Status plan.Status
	Notes  string
}

type Config struct {
	Construct func(*plan.TaskNode) error
	Review    func(*plan.TaskNode) (ReviewResult, error)
	Integrate func(*plan.TaskNode, ReviewResult) (IntegrateResult, error)
}

func Execute(cfg Config) orchestrator.Executor {
	return func(n *plan.TaskNode) error {
		if err := cfg.Construct(n); err != nil {
			return err
		}

		result, err := cfg.Review(n)
		if err != nil {
			return err
		}

		decision, err := cfg.Integrate(n, result)
		if err != nil {
			return err
		}

		n.Status = decision.Status
		return nil
	}
}
