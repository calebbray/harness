package repl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

type Repl struct {
	ReplConfig
	scanner *bufio.Scanner
}

type ReplConfig struct {
	Logger     *slog.Logger
	Agent      *agent.Agent
	WorkerPool *workerpool.WorkerPool
	Store      *workerpool.Store
}

func New(cfg ReplConfig) *Repl {
	scanner := bufio.NewScanner(os.Stdin)
	r := &Repl{
		ReplConfig: cfg,
		scanner:    scanner,
	}

	r.Agent.SetConfirmer(r.confirm)

	return r
}

func (r *Repl) Run() error {
	for {
		fmt.Print("> ")
		if !r.scanner.Scan() {
			break
		}
		if r.scanner.Err() != nil {
			r.Logger.Error("scanner error", "error", r.scanner.Err())
		}
		line := strings.TrimSpace(r.scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case line == "exit":
			return nil
		case strings.HasPrefix(line, "/task "):
			instruction := strings.TrimPrefix(line, "/task ")
			task := r.WorkerPool.Submit(instruction)
			r.Store.Add(task.Job)
			fmt.Printf("queued job #%d\n", task.Job.Id)
		case line == "/tasks":
			for _, j := range r.Store.Jobs() {
				status, result := j.Snapshot()
				fmt.Printf("#%d [%s] %s -> %q\n", j.Id, status, j.Instruction, result)
			}
		default:
			if err := r.Agent.Step(line); err != nil {
				r.Logger.Error("step error", "error", err)
				continue
			}

			fmt.Println(r.Agent.Result())
		}

	}
	return nil
}

func (r *Repl) confirm(toolName string, input json.RawMessage) bool {
	fmt.Printf("\nrun %s with input %s? [y/N]", toolName, input)
	r.scanner.Scan()
	return strings.ToLower(strings.TrimSpace(r.scanner.Text())) == "y"
}
