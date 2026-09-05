package handlers

import (
	"fmt"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/command"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

func ListTasks(store *workerpool.Store) command.Handler {
	return func(a *command.Args) error {
		jobs, err := store.GetJobs()
		if err != nil {
			return err
		}
		for _, j := range jobs {
			status, result := j.Snapshot()
			fmt.Printf("#%d [%s] %s -> %q\n", j.Id, status, j.Instruction, result)
		}
		return nil
	}
}

func SubmitTask(pool *workerpool.WorkerPool) command.Handler {
	return func(a *command.Args) error {
		if len(a.Positional) == 0 {
			return fmt.Errorf("usage: /task <instruction>")
		}

		task, err := pool.Submit(a.Positional[0])
		if err != nil {
			return err
		}

		fmt.Printf("queued job #%d\n", task.Job.Id)
		return nil
	}
}

func Query(agent *agent.Agent) command.Handler {
	return func(a *command.Args) error {
		if len(a.Positional) == 0 {
			return fmt.Errorf("usage: /query \"<instruction>\"")
		}
		if err := agent.Step(a.Positional[0]); err != nil {
			return fmt.Errorf("step error %w", err)
		}
		fmt.Println(agent.Result())
		return nil
	}
}

func PromptHelp(info string) command.Handler {
	return func(a *command.Args) error {
		fmt.Println(info)
		return nil
	}
}
