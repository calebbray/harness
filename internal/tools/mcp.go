package tools

import (
	"encoding/json"
	"fmt"

	"github.com/calebbray/personal-agent/internal/mcp"
)

func mcpHandler(client *mcp.MCPClient, toolName string) Handler {
	return func(input json.RawMessage) (string, error) {
		var args map[string]any
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		text, isError, err := client.CallTool(toolName, args)
		if err != nil {
			return "", err
		}

		if isError {
			return "", fmt.Errorf("%s", text)
		}

		return text, nil
	}
}

func RegisterMCPTools(r *Registry, client *mcp.MCPClient, prefix string) error {
	mcpTools, err := client.ListTools()
	if err != nil {
		return fmt.Errorf("failed to list mcp tools: %w", err)
	}

	for _, tool := range mcpTools {
		name := tool.Name
		if prefix != "" {
			name = prefix + "_" + tool.Name
		}
		r.Register(name, tool.Description, string(tool.InputSchema), mcpHandler(client, tool.Name), needsConfirmation(tool))
	}

	return nil
}

func needsConfirmation(tool mcp.MCPTool) bool {
	if tool.Annotations == nil {
		return true
	}

	if tool.Annotations.ReadOnlyHint != nil && *tool.Annotations.ReadOnlyHint {
		return false
	}

	if tool.Annotations.DestructiveHint != nil && !*tool.Annotations.DestructiveHint {
		return false
	}

	return true
}
