package client

import (
	"fmt"
	"net/http"
	"os"

	"github.com/calebbray/personal-agent/internal/tools"
)

type Client interface {
	Send(messages []Message, tools []tools.ToolDef) (*Response, error)
}

func New(upstreamProvider string) (Client, error) {
	switch upstreamProvider {
	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("no api key set")
		}

		return &AnthropicClient{
			key:    key,
			client: &http.Client{},
			model:  AnthropicModel,
		}, nil
	case "mock":
		return &MockClient{}, nil
	default:
		return nil, fmt.Errorf("invalid upstream provider")
	}
}
