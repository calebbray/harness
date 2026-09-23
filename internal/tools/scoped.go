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

	"github.com/calebbray/personal-agent/internal/permissions"
)

func Scoped(dir string) *Registry {
	r := NewRegistry()

	r.Register("bash", bashDescription, bashSchema, func(input json.RawMessage) (string, error) {
		var in bashInput
		if err := json.Unmarshal(input, &in); err != nil {
			return "", err
		}
		if permissions.IsForbidden(in.Command) {
			return "", fmt.Errorf("refused: command references a protected path")
		}
		return runShell(dir, in.Command)
	}, true)

	r.Register("read_file", readFileDescription, readFileSchema, func(input json.RawMessage) (string, error) {
		var in readFileInput
		if err := json.Unmarshal(input, &in); err != nil {
			return "", err
		}
		path := resolvePath(dir, in.Filepath)
		if permissions.IsForbidden(path) {
			return "", fmt.Errorf("refused: command references a protected path")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		return string(data), nil
	}, false)

	r.Register("write_file", writeFileDescription, writeFileSchema, func(input json.RawMessage) (string, error) {
		var in writeFileInput
		if err := json.Unmarshal(input, &in); err != nil {
			return "", err
		}
		path := resolvePath(dir, in.Path)
		if permissions.IsForbidden(path) {
			return "", fmt.Errorf("refused: command references a protected path")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("could not create parent dirs: %w", err)
		}
		if err := os.WriteFile(path, []byte(in.Contents), 0o644); err != nil {
			return "", fmt.Errorf("failed to write to file: %w", err)
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(in.Contents), path), nil
	}, true)

	r.Register("get_date", getDateDescription, getDateSchema, handleGetDate, false)

	return r
}

func runShell(dir, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
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

	return fmt.Sprintf("exit code: %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout.String(), stderr.String()), nil
}
