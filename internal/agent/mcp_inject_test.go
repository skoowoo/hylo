package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testEP = MCPEndpoint{URL: "http://127.0.0.1:54321/mcp", APIKey: "secret"}

func TestPrepareMCPNeverPutsKeyInArgvOrFiles(t *testing.T) {
	for _, id := range []string{"claude", "codex", "opencode", "copilot", "cursor-agent", "pi"} {
		dir := t.TempDir()
		inj, err := PrepareMCP(id, testEP, dir, nil)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if inj.Env["HYLO_API_KEY"] != "secret" {
			t.Errorf("%s: key not passed via env", id)
		}
		if strings.Contains(strings.Join(inj.Args, " "), "secret") {
			t.Errorf("%s: key leaked into argv: %v", id, inj.Args)
		}
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
			if info != nil && !info.IsDir() {
				if b, _ := os.ReadFile(p); strings.Contains(string(b), "secret") {
					t.Errorf("%s: key leaked into %s", id, p)
				}
			}
			return nil
		})
		for k, v := range inj.Env {
			if k != "HYLO_API_KEY" && strings.Contains(v, "secret") {
				t.Errorf("%s: key leaked into env %s", id, k)
			}
		}
	}
}

func TestPrepareMCPClaude(t *testing.T) {
	inj, err := PrepareMCP("claude", testEP, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(inj.Args) != 2 || inj.Args[0] != "--mcp-config" {
		t.Fatalf("args: %v", inj.Args)
	}
	var cfg struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(inj.Args[1]), &cfg); err != nil {
		t.Fatal(err)
	}
	s := cfg.MCPServers["hylo"]
	if s["type"] != "http" || s["url"] != testEP.URL || s["headers"].(map[string]any)["Authorization"] != "Bearer ${HYLO_API_KEY}" {
		t.Errorf("server: %v", s)
	}
	// Claude drops the whole server if it sees any key beyond these.
	for k := range s {
		if k != "type" && k != "url" && k != "headers" {
			t.Errorf("unexpected key %q in claude server config", k)
		}
	}
}

func TestPrepareMCPCodex(t *testing.T) {
	inj, _ := PrepareMCP("codex", testEP, t.TempDir(), nil)
	want := []string{"-c", `mcp_servers.hylo.url="http://127.0.0.1:54321/mcp"`, "-c", `mcp_servers.hylo.bearer_token_env_var="HYLO_API_KEY"`}
	if strings.Join(inj.Args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("args: %q", inj.Args)
	}

	inj, _ = PrepareMCP("codex", MCPEndpoint{URL: testEP.URL}, t.TempDir(), nil)
	if len(inj.Args) != 2 || len(inj.Env) != 0 {
		t.Errorf("without key: args=%q env=%v", inj.Args, inj.Env)
	}
}

func TestPrepareMCPOpenCodeMergesExistingConfig(t *testing.T) {
	base := []string{`OPENCODE_CONFIG_CONTENT={"theme":"dark","mcp":{"other":{"type":"local","command":["x"]}}}`}
	inj, err := PrepareMCP("opencode", testEP, t.TempDir(), base)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(inj.Env["OPENCODE_CONFIG_CONTENT"]), &cfg); err != nil {
		t.Fatal(err)
	}
	mcp := cfg["mcp"].(map[string]any)
	if cfg["theme"] != "dark" || mcp["other"] == nil {
		t.Errorf("existing config lost: %v", cfg)
	}
	hylo := mcp["hylo"].(map[string]any)
	if hylo["type"] != "remote" || hylo["headers"].(map[string]any)["Authorization"] != "Bearer {env:HYLO_API_KEY}" {
		t.Errorf("hylo entry: %v", hylo)
	}

	if _, err := PrepareMCP("opencode", testEP, t.TempDir(), []string{"OPENCODE_CONFIG_CONTENT=not json"}); err == nil {
		t.Error("invalid existing config should error")
	}
}

func TestPrepareMCPCopilot(t *testing.T) {
	inj, _ := PrepareMCP("copilot", testEP, t.TempDir(), nil)
	if len(inj.Args) != 2 || inj.Args[0] != "--additional-mcp-config" || !strings.Contains(inj.Args[1], `"tools":["*"]`) {
		t.Errorf("args: %q", inj.Args)
	}
}

func TestPrepareMCPFileBasedAgents(t *testing.T) {
	cases := []struct {
		id, file, flag, auth string
	}{
		{"cursor-agent", ".cursor/mcp.json", "--approve-mcps", "Bearer ${env:HYLO_API_KEY}"},
		{"pi", ".pi/mcp.json", "-a", "Bearer ${HYLO_API_KEY}"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, c.file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		existing := `{"mcpServers":{"other":{"url":"http://x"}},"keep":1}`
		if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}

		inj, err := PrepareMCP(c.id, testEP, dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(inj.Args) != 1 || inj.Args[0] != c.flag {
			t.Errorf("%s args: %q", c.id, inj.Args)
		}
		b, _ := os.ReadFile(path)
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		servers := doc["mcpServers"].(map[string]any)
		if servers["other"] == nil || doc["keep"] == nil {
			t.Errorf("%s: existing entries lost: %s", c.id, b)
		}
		h := servers["hylo"].(map[string]any)
		if h["url"] != testEP.URL || h["headers"].(map[string]any)["Authorization"] != c.auth {
			t.Errorf("%s hylo entry: %v", c.id, h)
		}

		// A second run must not rewrite an identical file.
		before, _ := os.Stat(path)
		if _, err := PrepareMCP(c.id, testEP, dir, nil); err != nil {
			t.Fatal(err)
		}
		after, _ := os.Stat(path)
		if !after.ModTime().Equal(before.ModTime()) {
			t.Errorf("%s: unchanged config was rewritten", c.id)
		}
	}
}

func TestPrepareMCPRefusesToClobberInvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cursor", "mcp.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("{not json"), 0o644)

	if _, err := PrepareMCP("cursor-agent", testEP, dir, nil); err == nil {
		t.Fatal("expected error")
	}
	if b, _ := os.ReadFile(path); string(b) != "{not json" {
		t.Errorf("file was modified: %s", b)
	}
}

func TestPrepareMCPUnsupportedAgents(t *testing.T) {
	for _, id := range []string{"hermes", "unknown"} {
		inj, err := PrepareMCP(id, testEP, t.TempDir(), nil)
		if err != nil || len(inj.Args) != 0 {
			t.Errorf("%s: args=%v err=%v", id, inj.Args, err)
		}
	}
}

func TestSupportsMCPMatchesPrepareMCP(t *testing.T) {
	for _, id := range []string{"claude", "codex", "opencode", "copilot", "cursor-agent", "pi", "hermes", "unknown"} {
		inj, err := PrepareMCP(id, testEP, t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		injected := len(inj.Args) > 0 || inj.Env["OPENCODE_CONFIG_CONTENT"] != ""
		if injected != SupportsMCP(id) {
			t.Errorf("%s: SupportsMCP=%v but injected=%v", id, SupportsMCP(id), injected)
		}
	}
}

func TestMCPPromptHintMentionsMapping(t *testing.T) {
	for _, want := range []string{"extract_section", "knowledge_list_indexes", "--help", "Fall back"} {
		if !strings.Contains(MCPPromptHint, want) {
			t.Errorf("hint missing %q", want)
		}
	}
}
