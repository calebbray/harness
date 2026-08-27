package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/calebbray/personal-agent/internal/tools"
)

const MaxTokenRequest = 4046
const AnthropicURL = "https://api.anthropic.com/v1/messages"
const AnthropicVersion = "2023-06-01"
const AnthropicModel = "claude-haiku-4-5"

type AnthropicClient struct {
	client *http.Client
	key    string
	model  string
}

func (c *AnthropicClient) Send(msgs []Message, tools []tools.ToolDef) (*Response, error) {
	body := toAnthropicRequest(c.model, MaxTokenRequest, "", msgs, tools)
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

	var r anthropicResponse
	if err = json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return fromAnthropicResponse(r), nil
}

func (c *AnthropicClient) Provider() string {
	return "anthropic"
}

func (c *AnthropicClient) Model() string {
	return c.model
}

func (m anthropicMessage) String() string {
	var out strings.Builder

	for _, c := range m.Content {
		if c.Text != "" {
			out.WriteString(c.Text)
			out.WriteString("\n")

		}
	}

	return out.String()
}

func toAnthropicRequest(model string, maxTokens int, system string, messages []Message, tools []tools.ToolDef) anthropicRequest {
	var out []anthropicMessage
	for _, m := range messages {
		var ac []anthropicContent
		for _, c := range m.Content {
			switch v := c.(type) {
			case TextContent:
				ac = append(ac, anthropicContent{Type: "text", Text: v.Text})
			case ToolUseContent:
				ac = append(ac, anthropicContent{Type: "tool_use", Id: v.Id, Name: v.Name, Input: v.Input})
			case ToolResultContent:
				ac = append(ac, anthropicContent{Type: "tool_result", ToolUseId: v.Id, Content: v.Content, IsError: v.IsError})
			}
		}
		out = append(out, anthropicMessage{Role: m.Role, Content: ac})
	}

	anthropicTools := make([]anthropicToolDef, len(tools))
	for i, t := range tools {
		anthropicTools[i] = anthropicToolDef{Name: t.Name, Description: t.Description, Schema: t.Schema}
	}
	return anthropicRequest{Model: model, MaxTokens: maxTokens, System: system, Messages: out, Tools: anthropicTools}
}

func fromAnthropicResponse(res anthropicResponse) *Response {
	var content []Content
	for _, c := range res.Content {
		switch c.Type {
		case "text":
			content = append(content, TextContent{Text: c.Text})
		case "tool_use":
			content = append(content, ToolUseContent{Id: c.Id, Name: c.Name, Input: c.Input})
		case "tool_result":
			content = append(content, ToolResultContent{Id: c.ToolUseId, Content: c.Content, IsError: c.IsError})
		}
	}

	return &Response{
		Content:    content,
		StopReason: res.StopReason,
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicToolDef `json:"tools,omitempty"`
	System    string             `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicResponse struct {
	Id         string             `json:"id"`
	Model      string             `json:"model"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	StopReason string             `json:"stop_reason"`
	Content    []anthropicContent `json:"content"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"input_schema"`
}

type anthropicContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Id        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseId string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type ApiError struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
