package cli

import (
	"fmt"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/command"
	"github.com/calebbray/personal-agent/internal/handlers"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

type Cli struct {
	Config
	*command.Registry
	agent *agent.Agent
	store *workerpool.Store
}

type Config struct {
	Agent *agent.Agent
	Store *workerpool.Store
}

func New(cfg Config) *Cli {
	reg := command.NewRegistry()
	reg.Register("--help", handlers.PromptHelp("harness: a help guide"))
	reg.Register("-q", handlers.Query(cfg.Agent))
	reg.Register("/tasks", handlers.ListTasks(cfg.Store))
	return &Cli{
		Registry: reg,
		agent:    cfg.Agent,
		store:    cfg.Store,
	}
}

func ParseArgs(args []string) (*command.Args, error) {
	a := command.NewArgs()
	for _, arg := range args {
		if isPositional(arg) {
			a.Positional = append(a.Positional, arg)
			continue
		}

		if isFlag(arg) {
			flag, value, err := parseFlag(arg)
			if err != nil {
				return a, err
			}
			a.Flags[flag] = value
			continue
		}

		if isBool(arg) {
			b, err := parseBool(arg)
			if err != nil {
				return a, err
			}
			a.Booleans[b] = true
			continue
		}
	}
	return a, nil
}

func isPositional(input string) bool {
	return !strings.HasPrefix(input, "--")
}

func isFlag(input string) bool {
	return strings.HasPrefix(input, "--") && strings.Contains(input, "=")
}

func isBool(input string) bool {
	return strings.HasPrefix(input, "--") && !strings.Contains(input, "=")
}

func parseFlag(input string) (flag, value string, err error) {
	parts := strings.Split(input, "=")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid flag: %s", input)
	}
	flag, _ = strings.CutPrefix(parts[0], "--")
	value = parts[1]
	return flag, value, nil
}

func parseBool(input string) (string, error) {
	key, _ := strings.CutPrefix(input, "--")
	return key, nil
}
