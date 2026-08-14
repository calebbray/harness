package workerpool

import (
	"log/slog"
	"sync"

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

type Job struct {
	Id          int64
	Instruction string
	Status      Status
	Result      string
}

func NewJob(id int64, instuction string) *Job {
	return &Job{
		Id:          id,
		Instruction: instuction,
		Status:      Pending,
	}
}

func (j *Job) Snapshot() (Status, string) {
	return j.Status, j.Result
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
	Store  *Store
}

func NewTask(id int64, instructions string, cfg TaskConfig) *Task {
	return &Task{
		Job: NewJob(id, instructions),
		Agent: agent.New(agent.AgentConfig{
			Tools:               cfg.Tools,
			Client:              cfg.Client,
			Logger:              cfg.Logger,
			EnforceConfirmation: false,
		}),
	}
}

func (t *Task) Process(store *Store) {
	store.MarkRunning(t.Job.Id)
	if err := t.Agent.Step(t.Job.Instruction); err != nil {
		store.MarkFailed(t.Job.Id, err.Error())
		return
	}
	store.MarkDone(t.Job.Id, t.Agent.Result())
}

type WorkerPool struct {
	TaskConfig
	taskCh     chan *Task
	concurrent int
	wg         sync.WaitGroup
}

func New(workers int, cfg TaskConfig) *WorkerPool {
	return &WorkerPool{
		taskCh:     make(chan *Task, 100),
		concurrent: workers,
		TaskConfig: cfg,
	}
}

func (wp *WorkerPool) Submit(instructions string) (*Task, error) {
	id, err := wp.Store.InsertJob(instructions)
	if err != nil {
		return nil, err
	}
	t := NewTask(id, instructions, wp.TaskConfig)
	wp.wg.Add(1)
	go func() {
		wp.taskCh <- t
	}()
	return t, nil
}

func (wp *WorkerPool) Run() {
	for range wp.concurrent {
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	for task := range wp.taskCh {
		task.Process(wp.Store)
		wp.wg.Done()
	}
}

func (wp *WorkerPool) Close() error {
	close(wp.taskCh)
	return nil
}
