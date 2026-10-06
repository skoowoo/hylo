package handler

import (
	"log/slog"

	"github.com/hardhacker/hylo/internal/agent"
)

// mcpURL returns the endpoint agentID should use, or "" when this run gets no MCP.
func (a *AgentAPI) mcpURL(agentID string) string {
	if !a.cfg.Agent.MCPEnabled || !agent.SupportsMCP(agentID) {
		return ""
	}
	url, _ := a.cfg.Server.MCPURL()
	return url
}

// mcpPromptHint is the extra system prompt for runs that get MCP tools.
func (a *AgentAPI) mcpPromptHint(agentID string) string {
	if a.mcpURL(agentID) == "" {
		return ""
	}
	return agent.MCPPromptHint
}

// injectMCP appends Hylo's MCP endpoint to argv/env for agentID. A failure only
// costs the run its MCP tools, so it is logged and the run proceeds.
func (a *AgentAPI) injectMCP(agentID, cwd string, argv, env []string) ([]string, []string) {
	url := a.mcpURL(agentID)
	if url == "" {
		return argv, env
	}
	inj, err := agent.PrepareMCP(agentID, agent.MCPEndpoint{URL: url, APIKey: a.cfg.Server.APIKey}, cwd, env)
	if err != nil {
		a.logger.Warn("mcp injection skipped", slog.String("agentId", agentID), slog.String("err", err.Error()))
		return argv, env
	}
	for k, v := range inj.Env {
		env = mergeEnvKey(env, k, v)
	}
	return append(argv, inj.Args...), env
}
