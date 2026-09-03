package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/calebbray/personal-agent/internal/tools"
)

const OpenAIURL = "https://api.openai.com/v1/responses"
const OpenAIDefaultModel = "gpt-4o-mini"

type OpenAIClient struct {
	client *http.Client
	key    string
	model  string
}

func (c *OpenAIClient) Send(msgs []Message, tools []tools.ToolDef) (*Response, error) {
	body := toResponsesRequest(c.model, msgs, tools)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, OpenAIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.key))

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
		var e OpenAIApiError
		if err := json.Unmarshal(data, &e); err == nil {
			return nil, fmt.Errorf("%d %s - %s", res.StatusCode, e.Error.Type, e.Error.Message)
		}
		return nil, fmt.Errorf("%d - %s", res.StatusCode, string(data))
	}

	var r responsesResponse
	if err = json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return fromResponsesResponse(r), nil
}

func (c *OpenAIClient) Provider() string {
	return "openai"
}

func (c *OpenAIClient) Model() string {
	return c.model
}

func toResponsesRequest(model string, messages []Message, tools []tools.ToolDef) responsesRequest {
	var items []responsesInputItem
	for _, msg := range messages {
		for _, block := range msg.Content {
			switch v := block.(type) {
			case TextContent:
				items = append(items, responsesInputItem{Role: msg.Role, Content: v.Text})
			case ToolUseContent:
				items = append(items, responsesInputItem{
					Type: "function_call", CallId: v.Id, Name: v.Name, Arguments: string(v.Input),
				})
			case ToolResultContent:
				output := v.Content
				if v.IsError {
					output = "ERROR: " + output
				}
				items = append(items, responsesInputItem{
					Type: "function_call_output", CallId: v.Id, Output: output,
				})
			}
		}
	}
	return responsesRequest{Model: model, Input: items, Tools: toResponsesTools(tools)}
}

func fromResponsesResponse(res responsesResponse) *Response {
	var content []Content
	hasFunctionCall := false

	for _, item := range res.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					content = append(content, TextContent{Text: c.Text})
				}
			}
		case "function_call":
			hasFunctionCall = true
			content = append(content, ToolUseContent{
				Id: item.CallId, Name: item.Name, Input: json.RawMessage(item.Arguments),
			})
		}
	}

	stopReason := StopReasonEndTurn
	if hasFunctionCall {
		stopReason = StopReasonToolUse
	}
	return &Response{
		Content:    content,
		StopReason: stopReason,
	}
}

func toResponsesTools(tools []tools.ToolDef) []responsesTool {
	out := make([]responsesTool, len(tools))
	for i, t := range tools {
		out[i] = responsesTool{Type: "function", Name: t.Name, Description: t.Description, Parameters: t.Schema}
	}
	return out
}

type responsesInputItem struct {
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	Type      string `json:"type,omitempty"`
	CallId    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type responsesRequest struct {
	Model string               `json:"model"`
	Input []responsesInputItem `json:"input"`
	Tools []responsesTool      `json:"tools,omitempty"`
}

type responsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesOutputItem struct {
	Type      string                   `json:"type"`
	Role      string                   `json:"role,omitempty"`
	Content   []responsesOutputContent `json:"content,omitempty"`
	CallId    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
}

type responsesResponse struct {
	Status string                `json:"status"`
	Output []responsesOutputItem `json:"output"`
}

type OpenAIApiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param,omitempty"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}
