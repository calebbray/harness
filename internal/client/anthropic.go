package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/calebbray/personal-agent/internal/tools"
)

const MaxTokenRequest = 4046
const AnthropicURL = "https://api.anthropic.com/v1/messages"
const AnthropicVersion = "2023-06-01"
const Model = "claude-haiku-4-5"

type Client struct {
	client *http.Client
	key    string
	model  string
}

func New() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("no api key set")
	}

	return &Client{
		key:    key,
		client: &http.Client{},
		model:  Model,
	}, nil
}

// temporarily sending tools, maybe later this should be a property on the client. Not sure yet where that lives
func (c *Client) Send(msgs []Message, tools []tools.ToolDef) (*Response, error) {
	body := request{
		Model:     c.model,
		MaxTokens: MaxTokenRequest,
		Messages:  msgs,
		Tools:     tools,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, AnthropicURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("anthropic-version", AnthropicVersion)

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		var e ApiError
		if err := json.Unmarshal(data, &e); err == nil {
			return nil, fmt.Errorf("%d %s - %s", res.StatusCode, e.Error.Type, e.Error.Message)
		}
		return nil, fmt.Errorf("%d - %s", res.StatusCode, string(data))
	}

	var r Response
	if err = json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return &r, nil
}

type request struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []Message       `json:"messages"`
	Tools     []tools.ToolDef `json:"tools,omitempty"`
	System    string          `json:"system,omitempty"`
}

type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

func (m Message) String() string {
	var out strings.Builder

	for _, c := range m.Content {
		switch v := c.(type) {
		case TextContent:
			out.WriteString(v.Text)
			out.WriteString("\n")
		}
	}

	return out.String()
}

type Content interface {
	Type() string
}

type TextContent struct {
	ContentType string `json:"type"`
	Text        string `json:"text"`
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
	ContentType string          `json:"type"`
	Id          string          `json:"id"`
	Name        string          `json:"name"`
	Input       json.RawMessage `json:"input"`
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
	ContentType string `json:"type"`
	Id          string `json:"tool_use_id"`
	Content     string `json:"content"`
	IsError     bool   `json:"is_error"`
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

type Response struct {
	Id         string    `json:"id"`
	Model      string    `json:"model"`
	Type       string    `json:"type"`
	Role       string    `json:"role"`
	StopReason string    `json:"stop_reason"`
	Content    []Content `json:"content"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// custom unmarshaller for responses. Specifically handles the various content types
func (r *Response) UnmarshalJSON(data []byte) error {
	type Alias Response
	a := struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	for _, raw := range a.Content {
		var kind struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(raw, &kind); err != nil {
			return err
		}

		switch kind.Type {
		case "text":
			var t TextContent
			if err := json.Unmarshal(raw, &t); err != nil {
				return err
			}
			r.Content = append(r.Content, t)

		case "tool_use":
			var t ToolUseContent
			if err := json.Unmarshal(raw, &t); err != nil {
				return err
			}
			r.Content = append(r.Content, t)

		case "tool_result":
			var t ToolResultContent
			if err := json.Unmarshal(raw, &t); err != nil {
				return err
			}
			r.Content = append(r.Content, t)

		default:
			return fmt.Errorf("unknown content type %q", kind.Type)
		}
	}

	return nil
}

// type ToolDef struct {
// 	Name        string          `json:"name"`
// 	Description string          `json:"description"`
// 	Schema      json.RawMessage `json:"input_schema"`
// }

type ApiError struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
