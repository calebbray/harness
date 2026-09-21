package pipeline

import (
	"testing"

	"github.com/calebbray/personal-agent/internal/orchestrator"
	"github.com/calebbray/personal-agent/internal/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteHappyPath(t *testing.T) {
	n, err := plan.NewTaskNode("solo", "no deps")
	require.NoError(t, err)

	var constructed, reviewed, integrated bool
	cfg := Config{
		Construct: func(tn *plan.TaskNode) error {
			constructed = true
			return nil
		},
		Review: func(tn *plan.TaskNode) (ReviewResult, error) {
			reviewed = true
			return ReviewResult{Approved: true}, nil
		},
		Integrate: func(tn *plan.TaskNode, rr ReviewResult) (IntegrateResult, error) {
			integrated = true
			return IntegrateResult{Status: plan.Approved}, nil
		},
	}

	require.NoError(t, Execute(cfg)(n))
	assert.True(t, constructed && reviewed && integrated)
	assert.Equal(t, plan.Approved, n.Status)
}

func TestExecutionWithReworkNeeded(t *testing.T) {
	n, err := plan.NewTaskNode("solo", "no deps")
	require.NoError(t, err)
	p, err := plan.NewPlan("rework", n)
	require.NoError(t, err)

	constructCalls := 0
	integrateCalls := 0

	cfg := Config{
		Construct: func(tn *plan.TaskNode) error {
			constructCalls++
			return nil
		},
		Review: func(tn *plan.TaskNode) (ReviewResult, error) {
			return ReviewResult{Approved: true}, nil
		},
		Integrate: func(tn *plan.TaskNode, rr ReviewResult) (IntegrateResult, error) {
			integrateCalls++
			if integrateCalls == 1 {
				return IntegrateResult{Status: plan.NeedsRework}, nil
			}
			return IntegrateResult{Status: plan.Approved}, nil
		},
	}

	require.NoError(t, orchestrator.Run(p, Execute(cfg)))

	assert.Equal(t, plan.Approved, n.Status)
	assert.Equal(t, 2, integrateCalls)
	assert.Equal(t, 2, constructCalls)
}
