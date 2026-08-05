package workerpool

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/tools"
)

type Status int

const (
	Pending Status = iota
	Running
	Done
	Failed
)

func (s Status) String() string {
	switch s {
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Done:
		return "done"
	case Failed:
		return "failed"
	default:
		return "invalid status"
	}
}

var idCounter atomic.Int64

type Job struct {
	Id          int64
	Instruction string
	Status      Status
	Result      string
	mu          sync.Mutex
}

func NewJob(instuction string) *Job {
	return &Job{
		Id:          idCounter.Add(1),
		Instruction: instuction,
		Status:      Pending,
	}
}

// get the current status and result
func (j *Job) Snapshot() (Status, string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Status, j.Result
}

func (j *Job) start() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = Running
}

func (j *Job) done(reason string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = Done
	j.Result = reason
}

func (j *Job) fail(e error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = Failed
	j.Result = e.Error()
}

type Task struct {
	TaskConfig
	Agent *agent.Agent
	Job   *Job
}

type TaskConfig struct {
	Client client.Client
	Tools  *tools.Registry
	Logger *slog.Logger
}

func NewTask(instructions string, cfg TaskConfig) *Task {
	return &Task{
		Job: NewJob(instructions),
		Agent: agent.New(agent.AgentConfig{
			Tools:               cfg.Tools,
			Client:              cfg.Client,
			Logger:              cfg.Logger,
			EnforceConfirmation: false,
		}),
	}
}

func (t *Task) Process() {
	t.Job.start()
	if err := t.Agent.Step(t.Job.Instruction); err != nil {
		t.Job.fail(err)
		return
	}
	t.Job.done(t.Agent.Result())
}

type WorkerPool struct {
	taskCh     chan *Task
	concurrent int
	wg         sync.WaitGroup
	taskConfig TaskConfig
}

func New(workers int, cfg TaskConfig) *WorkerPool {
	return &WorkerPool{
		taskCh:     make(chan *Task, 100),
		concurrent: workers,
		taskConfig: cfg,
	}
}

func (wp *WorkerPool) Submit(instructions string) *Task {
	t := NewTask(instructions, wp.taskConfig)
	wp.wg.Add(1)
	go func() {
		wp.taskCh <- t
	}()
	return t
}

func (wp *WorkerPool) Run() {
	for range wp.concurrent {
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	for task := range wp.taskCh {
		task.Process()
		wp.wg.Done()
	}
}

func (wp *WorkerPool) Close() error {
	close(wp.taskCh)
	return nil
}
