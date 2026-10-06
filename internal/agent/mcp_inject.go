package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	mcpServerName = "hylo"
	// mcpKeyEnv carries the API key to the agent process; configs only reference it by name.
	mcpKeyEnv = "HYLO_API_KEY"
)

// mcpAgents lists the agent CLIs PrepareMCP knows how to configure.
var mcpAgents = map[string]bool{
	"claude": true, "codex": true, "opencode": true, "copilot": true, "cursor-agent": true, "pi": true,
}

// SupportsMCP reports whether Hylo can inject its MCP endpoint into agentID.
func SupportsMCP(agentID string) bool { return mcpAgents[agentID] }

// MCPPromptHint tells an agent that has Hylo MCP tools how they relate to the CLI
// commands that skills are written against.
const MCPPromptHint = `Hylo MCP tools are connected (MCP server "hylo"). They map 1:1 to the hylo CLI: ` +
	"`hylo <command> [<subcommand>]` is the tool `<command>[_<subcommand>]`, for example `hylo read` = `read`, " +
	"`hylo extract section` = `extract_section`, `hylo knowledge list-indexes` = `knowledge_list_indexes`, " +
	"`hylo tag list` = `tag_list`, `hylo short create` = `short_create`. Your client may show the tool with a server prefix " +
	"(such as `mcp__hylo__extract_section` or `hylo_extract_section`). Positional arguments and flags become named parameters; " +
	"see each tool's schema.\n\n" +
	"Whenever a skill or instruction shows a `hylo ...` shell command, call the equivalent MCP tool instead of running the command. " +
	"Fall back to the shell command only if the tool is missing or unavailable. If the tools are not in your tool list yet, " +
	"search your deferred tools for \"hylo\" first. Skip any `hylo --help` environment check.\n\n" +
	"Differences from the CLI: `create` takes `content` only (no --file); `resolve` always returns full metadata; results are JSON."

// MCPEndpoint is where agent CLIs reach Hylo's MCP server.
type MCPEndpoint struct {
	URL    string
	APIKey string
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
		if err := mergeMCPFile(filepath.Join(cwd, ".cursor", "mcp.json"), srv); err != nil {
			return MCPInjection{}, err
		}

	case "pi":
		// Pi ignores project-local MCP config unless the project is trusted for this run.
		inj.Args = []string{"-a"}
		srv := httpServer(ep, "${"+mcpKeyEnv+"}")
		if err := mergeMCPFile(filepath.Join(cwd, ".pi", "mcp.json"), srv); err != nil {
			return MCPInjection{}, err
		}
	}
	return inj, nil
}

func httpServer(ep MCPEndpoint, keyRef string) map[string]any {
	srv := map[string]any{"url": ep.URL}
	if ep.APIKey != "" {
		srv["headers"] = map[string]string{"Authorization": "Bearer " + keyRef}
	}
	return srv
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
