package client

import (
	"sync"

	"github.com/calebbray/personal-agent/internal/tools"
)

type MockClient struct {
	Responses []*Response
	calls     int
	mu        sync.Mutex
}

func (mc *MockClient) Send(messages []Message, tools []tools.ToolDef) (*Response, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.calls >= len(mc.Responses) {
		return &Response{StopReason: "end_turn", Content: []Content{
			TextContent{ContentType: "text", Text: "mock: out of responses"},
		}}, nil
	}

	r := mc.Responses[mc.calls]
	mc.calls++
	return r, nil
}
