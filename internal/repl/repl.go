package repl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/command"
	"github.com/calebbray/personal-agent/internal/handlers"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

type Repl struct {
	ReplConfig
	scanner  *bufio.Scanner
	commands *command.Registry
}

type ReplConfig struct {
	Logger     *slog.Logger
	Agent      *agent.Agent
	WorkerPool *workerpool.WorkerPool
	Store      *workerpool.Store
}

func New(cfg ReplConfig) *Repl {
	r := &Repl{
		ReplConfig: cfg,
		scanner:    bufio.NewScanner(os.Stdin),
	}

	reg := command.NewRegistry()
	reg.Register("/tasks", handlers.ListTasks(cfg.Store))
	reg.Register("/task", handlers.SubmitTask(cfg.WorkerPool))
	reg.Register("/provider", r.handleProvider)
	reg.Register("/model", r.handleModel)
	reg.Register("/exit", r.handleExit)

	r.commands = reg
	r.Agent.SetConfirmer(r.confirm)

	return r
}

func (r *Repl) handleExit(*command.Args) error {
	return nil
}

func isCommand(input string) bool {
	return strings.HasPrefix(input, "/")
}
func (r *Repl) getCommand(input string) (command.Handler, *command.Args, bool) {
	if !strings.HasPrefix(input, "/") {
		return nil, nil, false
	}

	cmd, argString, _ := strings.Cut(input, " ")
	h, ok := r.commands.Get(cmd)
	if !ok {
		return nil, nil, false
	}
	args := &command.Args{Positional: []string{argString}}

	return h, args, true
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

		if isCommand(line) {
			f, args, ok := r.getCommand(line)
			if !ok {
				badCommand, _, _ := strings.Cut(line, " ")
				r.Logger.Error("not a valid command", "invalid command", badCommand)
			}
			if err := f(args); err != nil {
				r.Logger.Error("error executing command", "err", err)
			}
			continue
		}

		if err := r.Agent.Step(line); err != nil {
			r.Logger.Error("step error", "error", err)
			continue
		}

		fmt.Println(r.Agent.Result())
	}
	return nil
}

func (r *Repl) handleProvider(a *command.Args) error {
	newClient, err := client.New(a.Positional[0], "")
	if err != nil {
		return fmt.Errorf("failed to switch provider: %w", err)
	}
	r.Agent.SetClient(newClient)
	fmt.Printf("switched to %s (model: %s)\n", newClient.Provider(), newClient.Model())
	return nil
}

func (r *Repl) handleModel(a *command.Args) error {
	newClient, err := client.New(r.Agent.Client.Provider(), a.Positional[0])
	if err != nil {
		return fmt.Errorf("failed to switch model: %w", err)
	}
	r.Agent.SetClient(newClient)
	fmt.Printf("switched to model: %s\n", newClient.Model())
	return nil
}

func (r *Repl) confirm(toolName string, input json.RawMessage) bool {
	fmt.Printf("\nrun %s with input %s? [y/N]", toolName, input)
	r.scanner.Scan()
	return strings.ToLower(strings.TrimSpace(r.scanner.Text())) == "y"
}
