package orchestrator

import (
	"fmt"
	"sync"

	"github.com/calebbray/personal-agent/internal/plan"
)

// executor needs to be safe to call concurrently acress different nodes
type Executor func(*plan.TaskNode) error

func Run(p *plan.Plan, exec Executor) error {
	for {
		ready := p.Ready()
		if len(ready) == 0 {
			if blocked := p.Blocked(); len(blocked) > 0 {
				return fmt.Errorf("plan stuck: %d node(s) blocked", len(blocked))
			}
			return nil // done - nothing ready or blocked
		}

		var wg sync.WaitGroup
		for _, n := range ready {
			wg.Add(1)
			go func(n *plan.TaskNode) {
				defer wg.Done()
				n.Status = plan.InProgress

				if err := exec(n); err != nil {
					n.Status = plan.Failed
					return
				}

				n.Status = plan.Approved
			}(n)
		}
		wg.Wait()
	}
}
