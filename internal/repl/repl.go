package repl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/cli"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

type Repl struct {
	ReplConfig
	scanner  *bufio.Scanner
	commands Commands
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
		commands:   make(Commands),
	}

	r.Agent.SetConfirmer(r.confirm)

	return r
}

type Commands map[string]cli.Handler

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
			task, err := r.WorkerPool.Submit(instruction)
			if err != nil {
				fmt.Printf("couldn't submit job for processing %s", err)
			}
			fmt.Printf("queued job #%d\n", task.Job.Id)
		case line == "/tasks":
			jobs, err := r.Store.GetJobs()
			if err != nil {
				fmt.Printf("failed to fetch jobs")
				continue
			}
			for _, j := range jobs {
				status, result := j.Snapshot()
				fmt.Printf("#%d [%s] %s -> %q\n", j.Id, status, j.Instruction, result)
			}
		case strings.HasPrefix(line, "/provider"):
			arg := strings.TrimSpace(strings.TrimPrefix(line, "/provider"))
			if arg == "" {
				fmt.Printf("provider: %s, model: %s\n", r.Agent.Client.Provider(), r.Agent.Client.Model())
				continue
			}
			newClient, err := client.New(arg, "")
			if err != nil {
				fmt.Println("failed to switch provider:", err)
				continue
			}
			r.Agent.SetClient(newClient)
			fmt.Printf("switched to %s (model: %s)\n", newClient.Provider(), newClient.Model())
			continue
		case strings.HasPrefix(line, "/model"):
			arg := strings.TrimSpace(strings.TrimPrefix(line, "/model"))
			if arg == "" {
				fmt.Printf("model: %s\n", r.Agent.Client.Model())
				continue
			}
			newClient, err := client.New(r.Agent.Client.Provider(), arg)
			if err != nil {
				fmt.Println("failed to switch model:", err)
				continue
			}
			r.Agent.SetClient(newClient)
			fmt.Printf("switched to model: %s\n", newClient.Model())
			continue
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
