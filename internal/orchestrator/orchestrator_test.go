package orchestrator

import (
	"fmt"
	"testing"

	"github.com/calebbray/personal-agent/internal/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestration(t *testing.T) {
	t.Run("successfully runs a full plan", func(t *testing.T) {
		a, _ := plan.NewTaskNode("a", "root")
		b, _ := plan.NewTaskNode("b", "left", a.Id)
		c, _ := plan.NewTaskNode("c", "right", a.Id)
		d, _ := plan.NewTaskNode("d", "point", b.Id, c.Id)

		p, err := plan.NewPlan("plan", a, b, c, d)
		require.NoError(t, err)

		require.NoError(t, Run(p, executeLogger(t)))

		for _, n := range p.Nodes {
			assert.Equal(t, plan.Approved, n.Status)
		}
	})

	t.Run("stops and reports blocked nodes on failure", func(t *testing.T) {
		a, _ := plan.NewTaskNode("a", "root")
		b, _ := plan.NewTaskNode("b", "child", a.Id)
		c, _ := plan.NewTaskNode("c", "grandchild", b.Id)

		p, err := plan.NewPlan("plan", a, b, c)
		require.NoError(t, err)

		failMe := func(n *plan.TaskNode) error {
			if n.Id == a.Id {
				return fmt.Errorf("simulated failure")
			}
			return nil
		}

		require.Error(t, Run(p, failMe))

		assert.Equal(t, plan.Failed, a.Status)
		assert.Equal(t, plan.Pending, b.Status)
		assert.Equal(t, plan.Pending, c.Status)

		assert.Len(t, p.Blocked(), 2)
	})
}

func executeLogger(t *testing.T) Executor {
	t.Helper()
	return func(n *plan.TaskNode) error {
		t.Logf("node executed %s (%s)", n.Title, n.Id)
		return nil
	}
}
