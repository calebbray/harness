package mcp

import (
	"testing"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListToolsCaching(t *testing.T) {
	c := NewClient(ClientConfig{})
	err := c.Connect("npx", []string{"-y", "@modelcontextprotocol/server-filesystem", "/Users/caleb.bray/Projects/go"})
	require.NoError(t, err)
	defer func() {
		err := c.Close()
		require.NoError(t, err)
	}()

	_, err = c.Initialize("test-client", "0.1.0")
	require.NoError(t, err)

	tools, err := c.ListTools()
	require.NoError(t, err, "first list tools failed")

	// for _, tool := range tools {
	// 	if tool.Name == "list_directory" {
	// 		t.Log("TOOL", tool, string(tool.InputSchema))
	// 	}
	// }
	//
	// msg, isError, err := c.CallTool("list_directory", map[string]any{
	// 	"path": "./tui",
	// })
	// require.NoError(t, err)
	// assert.False(t, isError)
	// t.Log(string(msg))

	require.True(t, len(tools) > 0, "expected at least one tool from server")
	c.toolsM.Lock()
	cached := c.tools
	c.toolsM.Unlock()
	require.True(t, len(cached) > 0, "expected at least one tool to be cached")

	err = c.stdin.Close()
	require.NoError(t, err)

	tools2, err := c.ListTools()
	require.NoError(t, err, "tools should be cached and not require open connection")

	require.Equal(t, len(tools2), len(tools))

}
