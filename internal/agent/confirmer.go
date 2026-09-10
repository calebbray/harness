package agent

import "encoding/json"

type Confirmer func(toolName string, input json.RawMessage) Decision

type Decision int

const (
	DenyOnce Decision = iota
	AllowOnce
	AlwaysAllowProject
	AlwaysDenyProject
)
