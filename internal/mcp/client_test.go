package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallMatchesResponseToItsOwnRequest(t *testing.T) {
	c, fs := newFakeServer()
	go func() {
		id := fs.nextRequestId(t)
		fs.sendResponse(id, `{"ok":true}`)
	}()
	result, err := c.call("some/method", nil, 2*time.Second)
	require.NoError(t, err)
	assert.Contains(t, string(result), `"ok":true`)
}

func TestCallConcurrentCallsGetTheirOwnResponses(t *testing.T) {
	c, fs := newFakeServer()
	// respond out of order deliberately - proves matching isn't relying on request
	// order matching response order.
	go func() {
		var ids []int
		for range 10 {
			ids = append(ids, fs.nextRequestId(t))
		}
		for i := len(ids) - 1; i >= 0; i-- {
			fs.sendResponse(ids[i], fmt.Sprintf(`{"echo":%d}`, ids[i]))
		}
	}()

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			result, err := c.call("echo", nil, 2*time.Second)
			if err != nil {
				t.Errorf("call failed: %s", err)
				return
			}

			var parsed struct {
				Echo int `json:"echo"`
			}
			require.NoError(t, json.Unmarshal(result, &parsed))
			assert.True(t, parsed.Echo > 0)
		})
	}
	wg.Wait()
}

func TestNotificationInvalidatesToolsCache(t *testing.T) {
	c, fs := newFakeServer()
	c.tools = []MCPTool{{Name: "stale-cached-tool"}}
	fs.sendNotification("notifications/tools/list_changed")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.toolsM.Lock()
		cleared := c.tools == nil
		c.toolsM.Unlock()
		if cleared {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected tools cache to be cleared, but it never was")
}

func TestNewClientWiresUpDefaultNotifier(t *testing.T) {
	c := NewClient(ClientConfig{})
	if c.OnNotification == nil {
		t.Fatal("expected NewClient(nil) to wire up DefaultNotifier, got nil")
	}
}

type fakeServer struct {
	requests *bufio.Reader
	writer   io.Writer
}

func newFakeServer() (*MCPClient, *fakeServer) {
	reqR, reqW := io.Pipe()
	respR, respW := io.Pipe()

	c := NewClient(ClientConfig{WriteCloser: reqW})
	go c.readLoop(bufio.NewReader(respR))
	return c, &fakeServer{requests: bufio.NewReader(reqR), writer: respW}
}

func (f *fakeServer) nextRequestId(t *testing.T) int {
	t.Helper()
	line, err := f.requests.ReadString('\n')
	require.NoError(t, err)

	var req struct {
		Id int `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(line), &req))
	return req.Id
}

func (f *fakeServer) sendResponse(id int, result string) {
	fmt.Fprintf(f.writer, `{"jsonrpc":"2.0","id":%d,"result":%s}`+"\n", id, result)
}

func (f *fakeServer) sendNotification(method string) {
	fmt.Fprintf(f.writer, `{"jsonrpc":"2.0","method":%q}`+"\n", method)
}
