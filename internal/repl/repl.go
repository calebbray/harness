package repl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
)

type Repl struct {
	ReplConfig
	scanner *bufio.Scanner
}

type ReplConfig struct {
	Logger *slog.Logger
	Agent  *agent.Agent
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
		if line == "exit" {
			break
		}

		if err := r.Agent.Step(line); err != nil {
			r.Logger.Error("step error", "error", err)
		}

	}
	return nil
}

func (r *Repl) confirm(toolName string, input json.RawMessage) bool {
	fmt.Printf("\nrun %s with input %s? [y/N]", toolName, input)
	r.scanner.Scan()
	return strings.ToLower(strings.TrimSpace(r.scanner.Text())) == "y"
}
