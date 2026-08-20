package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/calebbray/personal-agent/internal/tools/jira"
)

type Handler func(input json.RawMessage) (string, error)

type Registry struct {
	defs     []ToolDef
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{
		defs:     make([]ToolDef, 0),
		handlers: make(map[string]Handler),
	}
}

func (r *Registry) Register(name, description, schema string, h Handler, requiresConfirmation bool) {
	r.handlers[name] = h
	r.defs = append(r.defs, ToolDef{
		Name:                 name,
		Description:          description,
		Schema:               json.RawMessage(schema),
		requiresConfirmation: requiresConfirmation,
	})
}

func (r *Registry) RequiresConfirmation(name string) bool {
	for _, def := range r.defs {
		if def.Name == name {
			return def.requiresConfirmation
		}
	}
	return false
}

func (r *Registry) Defs() []ToolDef {
	return r.defs
}

func (r *Registry) Dispatch(name string, input json.RawMessage) (result string, isError bool) {
	if h, ok := r.handlers[name]; ok {
		result, err := h(input)
		if err != nil {
			return err.Error(), true
		}
		return result, false
	}

	return "no tool with given name", true
}

func Default(issueSaver jira.JiraSaver) *Registry {
	r := NewRegistry()

	r.Register(
		"bash",
		"Execute a shell command and return its standard out, error, and exit code. Use for things like running programs, listing files, git, and build tools",
		bashSchema,
		handleBashCommand,
		true,
	)

	r.Register(
		"read_file",
		"Read the entire contents of a file at a given path",
		readFileSchema,
		handleReadFile,
		false,
	)

	r.Register(
		"write_file",
		"Write (overwrite) the given contents to a file at the given path. Creates parent directories if needed.",
		writeFileSchema,
		handleWriteFile,
		true,
	)

	// TODO: add a time / calendar tool that doesn't have to run a bunch of bash commands

	jiraTools, err := jira.NewJiraTools(issueSaver)
	if err == nil {
		r.Register(
			"jira_cycle_time",
			"get cycle time statistics for a jira project over a particular time frame. Defaults to the last two weeks",
			jira.JiraCycleTimeSchema,
			jiraTools.HandleCycleTimeStatistics,
			false,
		)

		r.Register(
			"jira_sync_issues",
			"sync jira issues for a team",
			jira.SyncIssueSchema,
			jiraTools.HandleJiraIssueSync,
			false,
		)
	}

	connectMCPServers(r)
	return r
}

type ToolDef struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Schema               json.RawMessage `json:"input_schema"`
	requiresConfirmation bool
}

const bashSchema = `{
	"type": "object",
	"properties": {
		"command": {
			"type": "string", 
			"description": "The shell command to run"
		}
	},
	"required": ["command"]
}`

type bashInput struct {
	Command string `json:"command"`
}

func handleBashCommand(input json.RawMessage) (string, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", in.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after 30 seconds")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("failed to run command: %w", err)
		}
	}

	result := fmt.Sprintf("exit code: %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout.String(), stderr.String())

	return result, nil
}

const readFileSchema = `{
	"type": "object",
	"properties": {
		"filepath": {
			"type": "string",
			"description": "the absolute path to the file to read"
		}
	},
	"required": ["filepath"]
}`

type readFileInput struct {
	Filepath string `json:"filepath"`
}

func handleReadFile(input json.RawMessage) (string, error) {
	var in readFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}

	data, err := os.ReadFile(in.Filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(data), nil
}

type writeFileInput struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
}

const writeFileSchema = `{
	"type": "object",
	"properties": {
		"path": {"type": "string", "description": "Where to write the file"},
		"contents": {"type": "string", "description": "file contents to write"}
	},
	"required": ["path", "contents"]
}`

func handleWriteFile(input json.RawMessage) (string, error) {
	var in writeFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(in.Path), 0o755); err != nil {
		return "", fmt.Errorf("could not create parent dirs: %w", err)
	}

	if err := os.WriteFile(in.Path, []byte(in.Contents), 0o644); err != nil {
		return "", fmt.Errorf("failed to write to file: %w", err)
	}

	return fmt.Sprintf("wrote %d bytes to %s", len(in.Contents), in.Path), nil
}
