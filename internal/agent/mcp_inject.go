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
	// mcpKeyEnv carries the API key to the agent process; configs only reference it by name.
	mcpKeyEnv = "HYLO_API_KEY"
	// mcpRunEnv carries the run id to agents whose config is a shared project file; the
	// file references it in mcpRunHeader (mcp.runHeader), so it stays the same across runs.
	mcpRunEnv    = "HYLO_RUN_ID"
	mcpRunHeader = "X-Hylo-Run"
)

// mcpAgents lists the agent CLIs PrepareMCP knows how to configure.
var mcpAgents = map[string]bool{
	"claude": true, "codex": true, "opencode": true, "copilot": true, "cursor-agent": true, "pi": true,
}

// SupportsMCP reports whether Hylo can inject its MCP endpoint into agentID.
func SupportsMCP(agentID string) bool { return mcpAgents[agentID] }

// MCPEndpoint is where agent CLIs reach Hylo's MCP server.
type MCPEndpoint struct {
	URL    string
	APIKey string
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
func PrepareMCP(agentID string, ep MCPEndpoint, cwd string, baseEnv []string) (MCPInjection, error) {
	inj := MCPInjection{Env: map[string]string{}}
	if ep.APIKey != "" {
		inj.Env[mcpKeyEnv] = ep.APIKey
	}
	switch agentID {
	case "cursor-agent", "pi":
		inj.Env[mcpRunEnv] = ep.RunID
	default:
		ep = ep.runScoped()
	}

	switch agentID {
	case "claude":
		// Claude silently drops a server that carries unknown keys such as Copilot's "tools".
		srv := httpServer(ep, "${"+mcpKeyEnv+"}")
		srv["type"] = "http"
		b, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{mcpServerName: srv}})
		inj.Args = []string{"--mcp-config", string(b)}

	case "codex":
		inj.Args = []string{"-c", "mcp_servers." + mcpServerName + ".url=" + strconv.Quote(ep.URL)}
		if ep.APIKey != "" {
			inj.Args = append(inj.Args, "-c", "mcp_servers."+mcpServerName+".bearer_token_env_var="+strconv.Quote(mcpKeyEnv))
		}

	case "opencode":
		v, err := opencodeConfig(ep, baseEnv)
		if err != nil {
			return MCPInjection{}, err
		}
		inj.Env["OPENCODE_CONFIG_CONTENT"] = v

	case "copilot":
		srv := httpServer(ep, "${"+mcpKeyEnv+"}")
		srv["type"] = "http"
		srv["tools"] = []string{"*"}
		b, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{mcpServerName: srv}})
		inj.Args = []string{"--additional-mcp-config", string(b)}

	case "cursor-agent":
		// Without --approve-mcps a project server stays "not loaded (needs approval)".
		inj.Args = []string{"--approve-mcps"}
		srv := httpServer(ep, "${env:"+mcpKeyEnv+"}")
		addHeader(srv, mcpRunHeader, "${env:"+mcpRunEnv+"}")
		if err := mergeMCPFile(filepath.Join(cwd, ".cursor", "mcp.json"), srv); err != nil {
			return MCPInjection{}, err
		}

	case "pi":
		// Pi ignores project-local MCP config unless the project is trusted for this run.
		inj.Args = []string{"-a"}
		srv := httpServer(ep, "${"+mcpKeyEnv+"}")
		addHeader(srv, mcpRunHeader, "${"+mcpRunEnv+"}")
		if err := mergeMCPFile(filepath.Join(cwd, ".pi", "mcp.json"), srv); err != nil {
			return MCPInjection{}, err
		}
	}
	return inj, nil
}

func httpServer(ep MCPEndpoint, keyRef string) map[string]any {
	srv := map[string]any{"url": ep.URL}
	if ep.APIKey != "" {
		addHeader(srv, "Authorization", "Bearer "+keyRef)
	}
	return srv
}

func addHeader(srv map[string]any, k, v string) {
	h, _ := srv["headers"].(map[string]string)
	if h == nil {
		h = map[string]string{}
		srv["headers"] = h
	}
	h[k] = v
}

func opencodeConfig(ep MCPEndpoint, baseEnv []string) (string, error) {
	cfg := map[string]any{}
	if raw := envToMap(baseEnv)["OPENCODE_CONFIG_CONTENT"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return "", fmt.Errorf("existing OPENCODE_CONFIG_CONTENT is not valid JSON: %w", err)
		}
	}
	srv := map[string]any{"type": "remote", "url": ep.URL}
	if ep.APIKey != "" {
		srv["headers"] = map[string]string{"Authorization": "Bearer {env:" + mcpKeyEnv + "}"}
	}
	mcp, _ := cfg["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	mcp[mcpServerName] = srv
	cfg["mcp"] = mcp
	b, err := json.Marshal(cfg)
	return string(b), err
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
