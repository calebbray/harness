package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

type Handler func(*Args) error

type Cli struct {
	Config
	handlers map[string]Handler
	agent    *agent.Agent
	store    *workerpool.Store
}

type Config struct {
	Agent *agent.Agent
	Store *workerpool.Store
}

type Args struct {
	Positional []string
	Flags      map[string]string
	Booleans   map[string]bool
}

func New(cfg Config) *Cli {
	cli := &Cli{
		handlers: make(map[string]Handler),
		agent:    cfg.Agent,
		store:    cfg.Store,
	}
	cli.Register("--help", helpHandler)
	cli.Register("-q", cli.queryHandler)
	cli.Register("/tasks", cli.listTasksHandler)

	return cli
}

func newArgs() *Args {
	return &Args{
		Positional: []string{},
		Flags:      make(map[string]string),
		Booleans:   make(map[string]bool),
	}
}

func (c *Cli) Register(name string, h Handler) {
	c.handlers[name] = h
}

func ParseArgs(args []string) (*Args, error) {
	a := newArgs()
	for _, arg := range args {
		if isPositional(arg) {
			a.Positional = append(a.Positional, arg)
			continue
		}

		if isFlag(arg) {
			if err := a.parseFlag(arg); err != nil {
				return a, err
			}
			continue
		}

		if isBool(arg) {
			if err := a.parseBool(arg); err != nil {
				return a, err
			}
			continue
		}
	}
	return a, nil
}

var ErrCmdNotFound = errors.New("command not found")

func (c *Cli) Run(cmd string, args *Args) error {
	f, ok := c.handlers[cmd]
	if !ok {
		return ErrCmdNotFound
	}
	return f(args)
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

func (a *Args) parseFlag(input string) error {
	parts := strings.Split(input, "=")
	if len(parts) != 2 {
		return fmt.Errorf("invalid flag: %s", input)
	}
	key, _ := strings.CutPrefix(parts[0], "--")
	a.Flags[key] = parts[1]
	return nil
}

func (a *Args) parseBool(input string) error {
	key, _ := strings.CutPrefix(input, "--")
	a.Booleans[key] = true
	return nil
}
