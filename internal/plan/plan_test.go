package plan

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Nodes get are validated on construction and revalidate on adding dependencies
func TestTaskNodeValidate(t *testing.T) {
	t.Run("self dependency", func(t *testing.T) {
		a, err := NewTaskNode("a", "root")
		require.NoError(t, err)
		require.Error(t, a.addDependency(a.Id))
	})
}

func TestPlanValidate(t *testing.T) {
	t.Run("valid chain", func(t *testing.T) {
		a, _ := NewTaskNode("a", "first")
		b, _ := NewTaskNode("b", "second", a.Id)
		c, _ := NewTaskNode("c", "third", b.Id)

		_, err := NewPlan("plan", a, b, c)
		require.NoError(t, err)
	})
	t.Run("valid diamond", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "left", a.Id)
		c, _ := NewTaskNode("c", "right", a.Id)
		d, _ := NewTaskNode("d", "point", b.Id, c.Id)

		_, err := NewPlan("plan", a, b, c, d)
		require.NoError(t, err)
	})
	t.Run("dangling dependency", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root", uuid.New())

		_, err := NewPlan("plan", a)
		require.Error(t, err)
	})
}

func TestPlanReady(t *testing.T) {
	t.Run("no dependencies is immediately ready", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		p, err := NewPlan("plan", a)
		require.NoError(t, err)

		require.Len(t, p.Ready(), 1)
		require.Equal(t, a.Id, p.Ready()[0].Id)
	})

	t.Run("blocked until dependency is approved", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "child", a.Id)
		p, err := NewPlan("plan", a, b)
		require.NoError(t, err)

		ready := p.Ready()
		require.Len(t, ready, 1)
		assert.Equal(t, a.Id, ready[0].Id)

		a.Status = Approved

		ready = p.Ready()
		require.Len(t, ready, 1)
		assert.Equal(t, b.Id, ready[0].Id)
	})

	t.Run("needs rework is still ready", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "rework")
		c, _ := NewTaskNode("c", "c", b.Id)

		p, err := NewPlan("plan", a, b, c)
		require.NoError(t, err)

		b.Status = NeedsRework
		ready := p.Ready()
		assert.NotContains(t, ready, c)
	})

	t.Run("in progress is never ready", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "b")
		c, _ := NewTaskNode("c", "c", b.Id)

		p, err := NewPlan("plan", a, b, c)
		require.NoError(t, err)

		a.Status = InProgress
		ready := p.Ready()
		require.Len(t, ready, 1)
		assert.Equal(t, b.Id, ready[0].Id)
	})
}

func TestPlanBlocked(t *testing.T) {
	t.Run("direct dependent of a failed node is blocked", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "child", a.Id)
		p, err := NewPlan("plan", a, b)
		require.NoError(t, err)

		a.Status = Failed

		blocked := p.Blocked()
		require.Len(t, blocked, 1)
		assert.Equal(t, b.Id, blocked[0].Id)
	})

	t.Run("blocking cascades transitively", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "child", a.Id)
		c, _ := NewTaskNode("c", "child", b.Id)
		p, err := NewPlan("plan", a, b, c)
		require.NoError(t, err)

		a.Status = Failed

		blocked := p.Blocked()
		require.Len(t, blocked, 2)
		assert.Equal(t, b.Id, blocked[0].Id)
		assert.Equal(t, c.Id, blocked[1].Id)
	})

	t.Run("already-resolved nodes aren't marked blocked", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "child", a.Id)
		c, _ := NewTaskNode("c", "child", b.Id)
		p, err := NewPlan("plan", a, b, c)
		require.NoError(t, err)

		a.Status = Approved

		blocked := p.Blocked()
		require.Len(t, blocked, 0)
	})
}

func TestPlanAddDependency(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "child", a.Id)
		c, _ := NewTaskNode("c", "child")
		p, err := NewPlan("plan", a, b)
		require.NoError(t, err)
		require.NoError(t, p.AddTasks(c))
		require.NoError(t, p.AddDependency(c.Id, b.Id))
	})

	t.Run("nodeId not found", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		p, err := NewPlan("plan", a)
		require.NoError(t, err)

		require.Error(t, p.AddDependency(uuid.New(), a.Id))
	})

	t.Run("dependsOnId not found", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		p, err := NewPlan("plan", a)
		require.NoError(t, err)

		require.Error(t, p.AddDependency(a.Id, uuid.New()))
	})

	t.Run("rejects a cycle and fully restores state", func(t *testing.T) {
		a, _ := NewTaskNode("a", "root")
		b, _ := NewTaskNode("b", "child", a.Id)
		p, err := NewPlan("plan", a, b)
		require.NoError(t, err)

		err = p.AddDependency(a.Id, b.Id) // attempts a -> b, on top of existing b -> a
		require.Error(t, err)

		// DependsOn itself was restored
		assert.NotContains(t, a.DependsOn, b.Id)

		// and p.graph was rebuilt to match - this is exactly what the
		// missing makeGraph() call would have failed to do
		assert.NotContains(t, p.graph.dependents[b.Id], a.Id)

		// belt-and-suspenders: the plan as a whole should still consider
		// itself valid after the failed call, not left in a broken state
		assert.NoError(t, p.validate())
	})
}
