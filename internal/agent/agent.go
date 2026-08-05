package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/tools"
)

type Agent struct {
	AgentConfig
	history    []client.Message
	confirmer  Confirmer
	lastResult string
}

type AgentConfig struct {
	Client              client.Client
	Tools               *tools.Registry
	EnforceConfirmation bool
	Logger              *slog.Logger
}

func New(cfg AgentConfig) *Agent {
	return &Agent{
		AgentConfig: cfg,
		history:     make([]client.Message, 0),
		confirmer:   DefaultConfirmer(cfg.EnforceConfirmation),
	}
}

func (a *Agent) Result() string {
	return a.lastResult
}

const maxIterations = 10

func (a *Agent) Step(userInput string) error {
	a.appendUserMessage(userInput)
	for range maxIterations {
		var done bool
		var err error
		done, err = a.step()
		if err != nil {
			return err
		}
		if done {
			break
		}
	}
	return nil
}

func (a *Agent) step() (bool, error) {
	a.Logger.Debug("--> sending messages\n", "num_messages", len(a.history))
	res, err := a.Client.Send(a.history, a.Tools.Defs())
	if err != nil {
		return false, fmt.Errorf("failed to get response to message: %s\n", err)
	}

	a.Logger.Debug("<-- stopped", "reason", res.StopReason, "num_content_blocks", len(res.Content))
	for _, c := range res.Content {
		a.Logger.Debug("content block", "block", c)
	}

	if res.StopReason != "tool_use" {
		a.history = append(a.history, client.Message{Role: "assistant", Content: res.Content})
		for _, content := range res.Content {
			if t, ok := content.(client.TextContent); ok {
				// fmt.Println(t.Text)
				a.lastResult = t.Text
			}
		}
		return true, nil
	}

	var results []client.Content
	for _, content := range res.Content {
		var result string
		var isError bool
		v, ok := content.(client.ToolUseContent)
		if !ok {
			continue
		}

		if a.Tools.RequiresConfirmation(v.Name) {
			if a.confirmer(v.Name, v.Input) {
				result, isError = a.Tools.Dispatch(v.Name, v.Input)
			} else {
				results = append(results, client.ToolResultContent{
					ContentType: "tool_result",
					Id:          v.Id,
					Content:     "user declined to run this tool",
					IsError:     true,
				})
				continue
			}
		} else {
			result, isError = a.Tools.Dispatch(v.Name, v.Input)
		}

		a.Logger.Debug("[executed]", "id", v.Id, "content", truncate(result, 200), "isError", isError)

		results = append(results, client.ToolResultContent{
			ContentType: "tool_result",
			Id:          v.Id,
			Content:     result,
			IsError:     isError,
		})
	}
	a.history = append(a.history,
		client.Message{Role: "assistant", Content: res.Content},
		client.Message{Role: "user", Content: results},
	)
	return false, nil
}

func (a *Agent) appendUserMessage(userInput string) {
	a.history = append(a.history, client.Message{
		Role: "user",
		Content: []client.Content{
			client.TextContent{
				ContentType: "text",
				Text:        userInput,
			},
		},
	})
}

func (a *Agent) SetConfirmer(confirmer Confirmer) {
	a.confirmer = confirmer
}

// basic DefaultConfirmer to either allow or disallow all commands
func DefaultConfirmer(allow bool) func(string, json.RawMessage) bool {
	return func(string, json.RawMessage) bool {
		return !allow
	}
}

func (a *Agent) History() []client.Message {
	return a.history
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "...(truncated)"
}
