package handler

import (
	"log/slog"
	"testing"

	"github.com/hardhacker/hylo/internal/config"
)

func newMCPTestAPI(port int) *AgentAPI {
	cfg := &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: port, APIKey: "k"},
	}
	return &AgentAPI{logger: slog.Default(), cfg: cfg}
}

func TestMCPURLMatchesInjection(t *testing.T) {
	cases := []struct {
		name  string
		port  int
		agent string
		want  bool
	}{
		{"claude", 54321, "claude", true},
		{"unknown agent", 54321, "unknown", false},
		{"tcp disabled", 0, "claude", false},
	}
	for _, c := range cases {
		a := newMCPTestAPI(c.port)
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
