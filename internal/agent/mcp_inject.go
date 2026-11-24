package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

const (
	mcpServerName = "hylo"
	mcpRunHeader  = "X-Hylo-Run"
)

// RunEnv carries the run id into every agent process, for the hylo CLI and
// for agents whose MCP config is a shared project file that references it in
// mcpRunHeader (mcp.runHeader), so the file stays the same across runs.
const RunEnv = "HYLO_RUN_ID"

// mcpAgents lists the agent CLIs PrepareMCP knows how to configure.
var mcpAgents = map[string]bool{
	"claude": true, "codex": true, "opencode": true, "copilot": true, "cursor-agent": true, "pi": true, "agy": true,
}

// SupportsMCP reports whether Hylo can inject its MCP endpoint into agentID.
func SupportsMCP(agentID string) bool { return mcpAgents[agentID] }

// MCPEndpoint is where agent CLIs reach Hylo's MCP server.
type MCPEndpoint struct {
	URL string
	// RunID ties tool calls to the run, so cancelling it stops its editor tools.
	RunID string
}

// runScoped sends the run as ?run= (mcp.RunParam), for agents whose config is per process.
func (ep MCPEndpoint) runScoped() MCPEndpoint {
	if ep.RunID == "" {
		return ep
	}
	u, err := url.Parse(ep.URL)
	if err != nil {
		return ep
	}
	q := u.Query()
	q.Set("run", ep.RunID)
	u.RawQuery = q.Encode()
	ep.URL = u.String()
	return ep
}

// MCPInjection is what a run needs so the agent CLI can see Hylo's MCP tools.
type MCPInjection struct {
	Args []string
	Env  map[string]string
}

// PrepareMCP returns the argv/env additions for agentID and writes any project-local
// config file the CLI can only read from disk. Agents without support get a zero value.
// Local /mcp accepts direct loopback requests without an API key, so no agent needs one.
func PrepareMCP(agentID string, ep MCPEndpoint, cwd string, baseEnv []string) (MCPInjection, error) {
	inj := MCPInjection{Env: map[string]string{}}
	switch agentID {
	case "cursor-agent", "pi":
		inj.Env[RunEnv] = ep.RunID
	case "agy":
		// Shared project file; agy does not expand env vars in headers.
	default:
		ep = ep.runScoped()
	}

	switch agentID {
	case "claude":
		// Claude silently drops a server that carries unknown keys such as Copilot's "tools".
		srv := map[string]any{"type": "http", "url": ep.URL}
		b, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{mcpServerName: srv}})
		inj.Args = []string{"--mcp-config", string(b)}

	case "codex":
		inj.Args = []string{"-c", "mcp_servers." + mcpServerName + ".url=" + strconv.Quote(ep.URL)}

	case "opencode":
		v, err := opencodeConfig(ep, baseEnv)
		if err != nil {
			return MCPInjection{}, err
		}
		inj.Env["OPENCODE_CONFIG_CONTENT"] = v

	case "copilot":
		srv := map[string]any{"type": "http", "url": ep.URL, "tools": []string{"*"}}
		b, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{mcpServerName: srv}})
		inj.Args = []string{"--additional-mcp-config", string(b)}

	case "cursor-agent":
		// Without --approve-mcps a project server stays "not loaded (needs approval)".
		inj.Args = []string{"--approve-mcps"}
		srv := map[string]any{
			"url":     ep.URL,
			"headers": map[string]string{mcpRunHeader: "${env:" + RunEnv + "}"},
		}
		if err := mergeMCPFile(filepath.Join(cwd, ".cursor", "mcp.json"), srv); err != nil {
			return MCPInjection{}, err
		}

	case "pi":
		// Pi ignores project-local MCP config unless the project is trusted for this run.
		inj.Args = []string{"-a"}
		srv := map[string]any{
			"url":     ep.URL,
			"headers": map[string]string{mcpRunHeader: "${" + RunEnv + "}"},
		}
		if err := mergeMCPFile(filepath.Join(cwd, ".pi", "mcp.json"), srv); err != nil {
			return MCPInjection{}, err
		}

	case "agy":
		srv := map[string]any{"serverUrl": ep.URL}
		if err := mergeMCPFile(filepath.Join(cwd, ".agents", "mcp_config.json"), srv); err != nil {
			return MCPInjection{}, err
		}
	}
	return inj, nil
}

func opencodeConfig(ep MCPEndpoint, baseEnv []string) (string, error) {
	cfg, err := readOpenCodeConfig(baseEnv)
	if err != nil {
		return "", err
	}
	srv := map[string]any{"type": "remote", "url": ep.URL}
	mcp := map[string]any{}
	if v, ok := cfg["mcp"]; ok && v != nil {
		m, valid := v.(map[string]any)
		if !valid {
			return "", fmt.Errorf("OPENCODE_CONFIG_CONTENT.mcp must be an object")
		}
		mcp = m
	}
	servers := map[string]any{}
	if v, ok := mcp["servers"]; ok && v != nil {
		m, valid := v.(map[string]any)
		if !valid {
			return "", fmt.Errorf("OPENCODE_CONFIG_CONTENT.mcp.servers must be an object")
		}
		servers = m
	}
	servers[mcpServerName] = srv
	mcp["servers"] = servers
	cfg["mcp"] = mcp
	b, err := json.Marshal(cfg)
	return string(b), err
}

func readOpenCodeConfig(baseEnv []string) (map[string]any, error) {
	cfg := map[string]any{}
	if raw := envToMap(baseEnv)["OPENCODE_CONFIG_CONTENT"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return nil, fmt.Errorf("existing OPENCODE_CONFIG_CONTENT is not valid JSON: %w", err)
		}
	}
	if cfg == nil {
		return nil, fmt.Errorf("OPENCODE_CONFIG_CONTENT must be an object")
	}
	return cfg, nil
}

// mergeMCPFile sets mcpServers.hylo in path, leaving every other entry untouched.
// It refuses to overwrite a file it cannot parse.
func mergeMCPFile(path string, srv map[string]any) error {
	doc := map[string]any{}
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(existing, &doc); err != nil {
			return fmt.Errorf("%s is not valid JSON, leaving it alone: %w", path, err)
		}
	case !os.IsNotExist(err):
		return err
	}

	servers, _ := doc["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[mcpServerName] = srv
	doc["mcpServers"] = servers

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if bytes.Equal(out, existing) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
