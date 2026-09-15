package plan

import (
	"fmt"
	"slices"

	"github.com/google/uuid"
)

type Status int

const (
	Pending Status = iota
	InProgress
	InReview
	NeedsRework
	Approved
	Failed
)

type TaskNode struct {
	Id          uuid.UUID
	Title       string
	Description string
	DependsOn   []uuid.UUID
	Status      Status
	Attempt     int
}

func NewTaskNode(title, description string, dependsOn ...uuid.UUID) (*TaskNode, error) {
	n := &TaskNode{
		Id:          uuid.New(),
		Title:       title,
		Description: description,
		Status:      Pending,
		Attempt:     0,
		DependsOn:   dependsOn,
	}
	if err := validateNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *TaskNode) addDependency(id uuid.UUID) error {
	if id == n.Id {
		return fmt.Errorf("attempted to add self dependency")
	}
	n.DependsOn = append(n.DependsOn, id)
	return nil
}

type Plan struct {
	Summary string
	Nodes   []*TaskNode
	graph   *graph
}

type graph struct {
	lookup     map[uuid.UUID]*TaskNode
	dependents map[uuid.UUID][]uuid.UUID
}

func NewPlan(summary string, tasks ...*TaskNode) (*Plan, error) {
	p := &Plan{
		Summary: summary,
	}

	if err := p.AddTasks(tasks...); err != nil {
		return nil, fmt.Errorf("plan construction: %w", err)
	}

	return p, nil
}

func (p *Plan) AddTasks(tasks ...*TaskNode) error {
	p.Nodes = append(p.Nodes, tasks...)
	if err := p.makeGraph(); err != nil {
		return fmt.Errorf("adding task: %w", err)
	}

	if err := p.validate(); err != nil {
		return fmt.Errorf("adding task: %w", err)
	}
	return nil
}

func (p *Plan) AddDependency(nodeId, dependsOnId uuid.UUID) error {
	n, ok := p.graph.lookup[nodeId]
	if !ok {
		return fmt.Errorf("nodeId not found: (%s)", nodeId)
	}

	_, ok = p.graph.lookup[dependsOnId]
	if !ok {
		return fmt.Errorf("dependsOnId not found: (%s)", dependsOnId)
	}

	original := slices.Clone(n.DependsOn)

	if err := n.addDependency(dependsOnId); err != nil {
		return fmt.Errorf("error adding dependency to (%s): %w", nodeId, err)
	}

	if err := p.makeGraph(); err != nil {
		n.DependsOn = original
		p.makeGraph()
		return fmt.Errorf("validation failed, restoring original plan: %w", err)

	}

	if err := p.validate(); err != nil {
		n.DependsOn = original
		p.makeGraph()
		return fmt.Errorf("validation failed, restoring original plan: %w", err)
	}

	return nil
}

func validateNode(n *TaskNode) error {
	if slices.Contains(n.DependsOn, n.Id) {
		return fmt.Errorf("task node '%s (%s)' depends on itself", n.Title, n.Id)
	}
	return nil
}

func (p *Plan) Ready() []*TaskNode {
	var ready []*TaskNode
	for _, n := range p.Nodes {
		if n.Status != Pending && n.Status != NeedsRework {
			continue
		}

		isReady := true
		for _, dep := range n.DependsOn {
			if p.graph.lookup[dep].Status != Approved {
				isReady = false
			}
		}
		if isReady {
			ready = append(ready, n)
		}
	}
	return ready
}

func (p *Plan) Blocked() []*TaskNode {
	blocked := make(map[uuid.UUID]bool)
	var queue []uuid.UUID

	for _, n := range p.Nodes {
		if n.Status == Failed {
			queue = append(queue, n.Id)
		}
	}

	for len(queue) > 0 {
		var popped uuid.UUID
		// bypassing error check as the loop condition covers empty queue case
		popped, queue, _ = dequeue(queue)
		deps := p.graph.dependents[popped]
		for _, d := range deps {
			if _, ok := blocked[d]; ok {
				// were already blocked. Skip to avoid reprocessing
				continue
			}

			n := p.graph.lookup[d]
			if n.Status == Approved || n.Status == Failed {
				continue
			}

			blocked[d] = true
			queue = append(queue, d)
		}
	}
	var out []*TaskNode
	for k := range blocked {
		n := p.graph.lookup[k]
		out = append(out, n)
	}
	return out
}

func (p *Plan) makeGraph() error {
	lookup := make(map[uuid.UUID]*TaskNode)
	dependents := make(map[uuid.UUID][]uuid.UUID)

	for _, n := range p.Nodes {
		lookup[n.Id] = n
	}

	for _, n := range p.Nodes {
		for _, depId := range n.DependsOn {
			if _, ok := lookup[depId]; !ok {
				// If its not here... This means a node doesn't exist and this should be an error
				return fmt.Errorf("node '%s (%s)' has a dependency for a node that does not exist", n.Title, n.Id)
			}
			dependents[depId] = append(dependents[depId], n.Id)
		}
	}

	p.graph = &graph{lookup: lookup, dependents: dependents}

	return nil
}

func (p *Plan) validate() error {
	if p.graph == nil {
		return fmt.Errorf("plan is not in a state to validate")
	}
	numDependencies := make(map[uuid.UUID]int)

	for _, n := range p.Nodes {
		numDependencies[n.Id] = len(n.DependsOn)
	}

	var queue []uuid.UUID
	for id, count := range numDependencies {
		if count == 0 {
			queue = append(queue, id)
		}
	}

	popped := 0
	for len(queue) > 0 {
		var done uuid.UUID
		var err error

		done, queue, err = dequeue(queue)
		if err != nil {
			return err
		}

		for _, depId := range p.graph.dependents[done] {
			numDependencies[depId]--
			if count, ok := numDependencies[depId]; ok && count == 0 {
				queue = append(queue, depId)
			}
		}

		popped++
	}

	if popped < len(p.Nodes) {
		return fmt.Errorf("cycle error")
	}

	return nil
}

func dequeue(arr []uuid.UUID) (uuid.UUID, []uuid.UUID, error) {
	if len(arr) <= 0 {
		return uuid.UUID{}, nil, fmt.Errorf("queue empty")
	}

	out := arr[0]
	arr = arr[1:]

	return out, arr, nil
}

func (s Status) String() string {
	switch s {
	case Pending:
		return "pending"
	case InProgress:
		return "in progress"
	case InReview:
		return "in review"
	case NeedsRework:
		return "needs rework"
	case Approved:
		return "approved"
	case Failed:
		return "failed"
	default:
		return "invalid status"
	}
}
