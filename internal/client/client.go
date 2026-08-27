package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/calebbray/personal-agent/internal/tools"
)

const (
	StopReasonToolUse = "tool_use"
	StopReasonEndTurn = "end_turn"
)

type Client interface {
	Send(messages []Message, tools []tools.ToolDef) (*Response, error)
	Provider() string
	Model() string
}

type Message struct {
	Role    string
	Content []Content
}

type ToolDef struct {
	Name                 string
	Description          string
	Schema               json.RawMessage
	RequiresConfirmation bool
}

type Response struct {
	Content    []Content
	StopReason string
}

func New(provider, model string) (Client, error) {
	switch provider {
	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("no api key set")
		}

		if model == "" {
			model = "claude-haiku-4-5"
		}

		return &AnthropicClient{
			key:    key,
			client: &http.Client{},
			model:  model,
		}, nil
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("no api key set")
		}

		if model == "" {
			model = "gpt-4o-mini"
		}

		return &OpenAIClient{
			key:    key,
			client: &http.Client{},
			model:  model,
		}, nil

	case "mock":
		return &MockClient{}, nil
	default:
		return nil, fmt.Errorf("invalid upstream provider")
	}
}

type Content interface {
	Type() string
}

type TextContent struct {
	ContentType string
	Text        string
}

func (TextContent) Type() string {
	return "text"
}

func (t TextContent) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "text"),
		slog.String("text", t.Text),
	)
}

type ToolUseContent struct {
	ContentType string
	Id          string
	Name        string
	Input       json.RawMessage
}

func (ToolUseContent) Type() string {
	return "tool_use"
}

func (t ToolUseContent) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "tool_use"),
		slog.String("id", t.Id),
		slog.String("name", t.Name),
		slog.String("input", string(t.Input)),
	)
}

type ToolResultContent struct {
	ContentType string
	Id          string
	Content     string
	IsError     bool
}

func (ToolResultContent) Type() string {
	return "tool_result"
}

func (t ToolResultContent) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "tool_result"),
		slog.String("tool_use_id", t.Id),
		slog.Bool("is_error", t.IsError),
	)
}
