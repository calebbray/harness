package command

import (
	"errors"
)

type Handler func(*Args) error

type Args struct {
	Positional []string
	Flags      map[string]string
	Booleans   map[string]bool
}

func NewArgs() *Args {
	return &Args{
		Positional: []string{},
		Flags:      make(map[string]string),
		Booleans:   make(map[string]bool),
	}
}

type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

func (r *Registry) Register(name string, h Handler) {
	r.handlers[name] = h
}

func (r *Registry) Get(name string) (Handler, bool) {
	h, ok := r.handlers[name]
	if !ok {
		return nil, false
	}
	return h, true
}

var ErrCmdNotFound = errors.New("command not found")

func (r *Registry) Run(cmd string, args *Args) error {
	f, ok := r.handlers[cmd]
	if !ok {
		return ErrCmdNotFound
	}
	return f(args)
}
