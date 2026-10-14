package handler

import (
	"log/slog"
	"testing"

	"github.com/hardhacker/hylo/internal/config"
)

func newMCPTestAPI(enabled bool, port int) *AgentAPI {
	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: port, APIKey: "k"},
		Agent:  config.AgentConfig{MCPEnabled: enabled},
	}
	return &AgentAPI{logger: slog.Default(), cfg: cfg}
}

func TestMCPURLMatchesInjection(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		port    int
		agent   string
		want    bool
	}{
		{"claude enabled", true, 54321, "claude", true},
		{"unknown agent", true, 54321, "unknown", false},
		{"disabled in config", false, 54321, "claude", false},
		{"tcp disabled", true, 0, "claude", false},
	}
	for _, c := range cases {
		a := newMCPTestAPI(c.enabled, c.port)
		has := a.mcpURL(c.agent) != ""
		if has != c.want {
			t.Errorf("%s: mcp=%v, want %v", c.name, has, c.want)
		}
		// The prompt claims MCP exactly when argv/env are actually extended.
		argv, _ := a.injectMCP(c.agent, "", t.TempDir(), nil, nil)
		if (len(argv) > 0) != c.want && c.agent != "opencode" {
			t.Errorf("%s: injected argv=%v inconsistent with mcpURL", c.name, argv)
		}
	}
}
