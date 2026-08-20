package tools

import "github.com/calebbray/personal-agent/internal/mcp"

type mcpServerConfig struct {
	Name    string
	Command string
	Args    []string
}

var knownServers = []mcpServerConfig{
	{Name: "filesystem", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/Users/caleb.bray/Projects/go"}},
}

func connectMCPServers(r *Registry) {
	for _, cfg := range knownServers {
		client := mcp.NewClient(mcp.ClientConfig{})
		if err := client.Connect(cfg.Command, cfg.Args); err != nil {
			// log something here at some point
			continue
		}

		if _, err := client.Initialize("harness", "0.2.1"); err != nil {
			continue
		}
		if err := RegisterMCPTools(r, client, cfg.Name); err != nil {
			continue
		}
	}
}
