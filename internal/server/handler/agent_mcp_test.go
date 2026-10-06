package handler

import (
	"log/slog"
	"strings"
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

func TestMCPPromptHintOnlyWhenInjected(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		port    int
		agent   string
		want    bool
	}{
		{"claude enabled", true, 54321, "claude", true},
		{"hermes unsupported", true, 54321, "hermes", false},
		{"disabled in config", false, 54321, "claude", false},
		{"tcp disabled", true, 0, "claude", false},
	}
	for _, c := range cases {
		a := newMCPTestAPI(c.enabled, c.port)
		hint := a.mcpPromptHint(c.agent)
		if (hint != "") != c.want {
			t.Errorf("%s: hint present=%v, want %v", c.name, hint != "", c.want)
		}
		// The prompt claims MCP exactly when argv/env are actually extended.
		argv, _ := a.injectMCP(c.agent, t.TempDir(), nil, nil)
		if (len(argv) > 0) != c.want && c.agent != "opencode" {
			t.Errorf("%s: injected argv=%v inconsistent with hint", c.name, argv)
		}
	}
}

func TestMCPPromptHintJoinsSystemPrompt(t *testing.T) {
	a := newMCPTestAPI(true, 54321)
	got := mergeSystemPrompts("mate prompt", a.mcpPromptHint("claude"))
	if !strings.HasPrefix(got, "mate prompt\n\n---\n\n") || !strings.Contains(got, "Hylo MCP tools") {
		t.Errorf("merged prompt: %q", got)
	}
	if got := mergeSystemPrompts("mate prompt", a.mcpPromptHint("hermes")); got != "mate prompt" {
		t.Errorf("unsupported agent changed prompt: %q", got)
	}
}
