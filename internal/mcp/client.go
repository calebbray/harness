package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type MCPClient struct {
	ClientConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	writeM sync.Mutex

	idM    sync.Mutex
	nextId int

	pendingM sync.Mutex
	pending  map[int]chan *response

	toolsM sync.Mutex
	tools  []MCPTool
	// OnNotification Notifier
}

type ClientConfig struct {
	io.WriteCloser
	OnNotification Notifier
}

func NewClient(cfg ClientConfig) *MCPClient {
	c := &MCPClient{
		ClientConfig: cfg,
		pending:      make(map[int]chan *response),
		stdin:        cfg.WriteCloser,
	}

	if cfg.OnNotification == nil {
		c.OnNotification = c.DefaultNotifier
	}
	return c
}

type Notifier func(method string, result json.RawMessage)

type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Annotations *struct {
		ReadOnlyHint    *bool `json:"readOnlyHint"`
		DestructiveHint *bool `json:"destructiveHint"`
	} `json:"annotations"`
}

func needsConfirmation(tool MCPTool) bool {
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

func (t MCPTool) String() string {
	var sb strings.Builder
	sb.WriteString("MCP Tool: ")
	sb.WriteString(t.Name)
	sb.WriteString("\n")
	// sb.WriteString("------------\n")
	// sb.WriteString("Description:\n")
	// sb.WriteString(t.Description)
	// sb.WriteString("\n\n")
	// sb.WriteString("Schema:\n")
	// sb.WriteString(string(t.InputSchema))
	// sb.WriteString("\n\n")
	return sb.String()
}

func (c *MCPClient) Connect(command string, args []string) error {
	c.cmd = exec.Command(command, args...)
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	c.stdin = stdin

	if err := c.cmd.Start(); err != nil {
		return err
	}

	go c.readLoop(bufio.NewReader(stdout))
	return nil
}

func (c *MCPClient) Initialize(clientName, clientVersion string) (ServerInfo, error) {
	var s ServerInfo
	result, err := c.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": clientName, "version": clientVersion},
	}, 10*time.Second)
	if err != nil {
		return s, err
	}

	if err = json.Unmarshal(result, &s); err != nil {
		return s, err
	}

	if err = c.notify("notifications/initialized", nil); err != nil {
		return s, fmt.Errorf("initialized notification: %w", err)
	}
	return s, nil
}

func (c *MCPClient) ListTools() ([]MCPTool, error) {
	c.toolsM.Lock()
	if c.tools != nil {
		defer c.toolsM.Unlock()
		return c.tools, nil
	}
	c.toolsM.Unlock()

	result, err := c.call("tools/list", nil, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []MCPTool `json:"tools"`
	}
	if err = json.Unmarshal(result, &parsed); err != nil {
		return nil, err
	}

	c.toolsM.Lock()
	c.tools = parsed.Tools
	c.toolsM.Unlock()
	return parsed.Tools, nil
}

func (c *MCPClient) CallTool(name string, args map[string]any) (string, bool, error) {
	result, err := c.call("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	}, 10*time.Second)
	if err != nil {
		return "", false, err
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", false, err
	}

	var sb strings.Builder
	for i, block := range parsed.Content {
		if block.Type != "text" {
			continue
		}

		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(block.Text)
	}
	return sb.String(), parsed.IsError, nil
}

func (c *MCPClient) Close() error {
	return c.cmd.Process.Kill()
}

func (c *MCPClient) readLoop(r *bufio.Reader) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			// process exited or closed, unblock pending responses
			c.pendingM.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.pendingM.Unlock()
			return
		}

		var res response
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			continue
		}

		if res.Id == nil {
			// server/client notification, nothing we expected a response to
			if c.OnNotification != nil {
				c.OnNotification(res.Method, res.Result)
			}
			continue
		}

		c.pendingM.Lock()
		ch, ok := c.pending[*res.Id]
		if ok {
			delete(c.pending, *res.Id)
		}
		c.pendingM.Unlock()

		if ok {
			ch <- &res
			close(ch)
		}
	}
}

func (c *MCPClient) call(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	c.idM.Lock()
	c.nextId++
	id := c.nextId
	c.idM.Unlock()

	ch := make(chan *response, 1)
	c.pendingM.Lock()
	c.pending[id] = ch
	c.pendingM.Unlock()

	req := request{JSONRPC: "2.0", Id: id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	c.writeM.Lock()
	_, err = c.stdin.Write(append(data, '\n'))
	c.writeM.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed while waiting for response to %s", method)
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-time.After(timeout):
		c.pendingM.Lock()
		delete(c.pending, id)
		c.pendingM.Unlock()
		return nil, fmt.Errorf("timed out waiting for response to %s", method)
	}
}

func (c *MCPClient) notify(method string, params any) error {
	n := notification{JSONRPC: "2.0", Method: method, Params: params}
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}

	c.writeM.Lock()
	defer c.writeM.Unlock()
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *MCPClient) DefaultNotifier(method string, result json.RawMessage) {
	switch method {
	case "notifications/tools/list_changed":
		c.toolsM.Lock()
		c.tools = nil
		c.toolsM.Unlock()
	}
}
