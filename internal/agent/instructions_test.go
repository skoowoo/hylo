package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestInstructionFilesAreRunScoped(t *testing.T) {
	for _, id := range []string{"pi", "claude", "opencode"} {
		t.Run(id, func(t *testing.T) {
			a, err := PrepareInstructions(t.Context(), id, "", "", "first", nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(a.Close)
			b, err := PrepareInstructions(t.Context(), id, "", "", "updated", nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(b.Close)
			if a.dir == b.dir {
				t.Fatal("runs share an instruction directory")
			}
			for _, c := range []struct {
				inj  InstructionInjection
				want string
			}{{a, "first"}, {b, "updated"}} {
				path := filepath.Join(c.inj.dir, "instructions.md")
				got, err := os.ReadFile(path)
				if err != nil || string(got) != c.want {
					t.Fatalf("instructions = %q, %v", got, err)
				}
				if runtime.GOOS != "windows" {
					st, _ := os.Stat(path)
					if st.Mode().Perm() != 0600 {
						t.Fatalf("instructions permissions = %v", st.Mode())
					}
				}
			}
			if id == "pi" && !containsSeq(a.Args, "--append-system-prompt", filepath.Join(a.dir, "instructions.md")) {
				t.Fatal(a.Args)
			}
			if id == "claude" && (!containsSeq(a.Args, "--append-system-prompt-file", filepath.Join(a.dir, "instructions.md")) || !containsSeq(a.Args, "--system-prompt-snapshot", "off")) {
				t.Fatal(a.Args)
			}
			a.Close()
			if _, err := os.Stat(a.dir); !os.IsNotExist(err) {
				t.Fatalf("instruction directory survived cleanup: %v", err)
			}
		})
	}
}

func TestOpenCodeInstructionsPreserveMCPAndUserConfig(t *testing.T) {
	base := []string{`OPENCODE_CONFIG_CONTENT={"instructions":["/user/rules.md"],"model":"test/model","mcp":{"servers":{"other":{"type":"remote","url":"https://example.com"}}}}`}
	mcp, err := PrepareMCP("opencode", MCPEndpoint{URL: "http://localhost/mcp"}, "", base)
	if err != nil {
		t.Fatal(err)
	}
	inj, err := PrepareInstructions(t.Context(), "opencode", "", "", "hylo rules", mapToEnv(mcp.Env))
	if err != nil {
		t.Fatal(err)
	}
	defer inj.Close()
	cfg, err := readOpenCodeConfig(mapToEnv(inj.Env))
	if err != nil {
		t.Fatal(err)
	}
	files := cfg["instructions"].([]any)
	servers := cfg["mcp"].(map[string]any)["servers"].(map[string]any)
	if len(files) != 2 || files[0] != "/user/rules.md" || cfg["model"] != "test/model" || servers["hylo"] == nil || servers["other"] == nil {
		t.Fatalf("merged config = %#v", cfg)
	}
}

func TestInvalidInstructionConfigFailsAndCleansUp(t *testing.T) {
	for _, raw := range []string{"{", "null", `{"instructions":"wrong type"}`, `{"instructions":[1]}`} {
		inj, err := PrepareInstructions(t.Context(), "opencode", "", "", "rules", []string{"OPENCODE_CONFIG_CONTENT=" + raw})
		if err == nil {
			inj.Close()
			t.Fatalf("accepted invalid config %q", raw)
		}
		if _, statErr := os.Stat(inj.dir); !os.IsNotExist(statErr) {
			t.Fatalf("failed injection leaked %q: %v", inj.dir, statErr)
		}
	}
}

func TestInstructionConfigHelper(t *testing.T) {
	mode := os.Getenv("HYLO_TEST_CODEX_CONFIG")
	if mode == "" {
		return
	}
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(s.Bytes(), &request); err != nil {
			os.Exit(2)
		}
		switch request.Method {
		case "initialize":
			if mode == "notification" {
				_, _ = os.Stdout.WriteString("{\"id\":\"server-request\",\"method\":\"notification\"}\n")
			}
			_, _ = os.Stdout.WriteString("{\"id\":1,\"result\":{}}\n")
		case "config/read":
			if mode == "stall" {
				_ = os.WriteFile(os.Getenv("HYLO_TEST_CODEX_READY"), nil, 0600)
				_, _ = io.Copy(io.Discard, os.Stdin)
				os.Exit(0)
			}
			if mode == "missing-config" {
				_, _ = os.Stdout.WriteString("{\"id\":2,\"result\":{}}\n")
				os.Exit(0)
			}
			_, _ = os.Stdout.WriteString("{\"id\":2,\"result\":{\"config\":{\"developer_instructions\":\"user rules\"}}}\n")
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func TestCodexInstructionsMergeWithoutReplacingBasePrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell launcher")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "codex")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=^TestInstructionConfigHelper$ -- \"$@\"\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	rules := "Hylo 中文\nquotes: \"'\\\t\x01"
	inj, err := PrepareInstructions(t.Context(), "codex", bin, dir, rules, append(os.Environ(), "HYLO_TEST_CODEX_CONFIG=1"))
	if err != nil {
		t.Fatal(err)
	}
	defer inj.Close()
	var cfg map[string]string
	if len(inj.Args) != 2 || inj.Args[0] != "-c" {
		t.Fatal(inj.Args)
	}
	if err := toml.Unmarshal([]byte(inj.Args[1]), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["developer_instructions"] != "user rules\n\n"+rules || len(cfg) != 1 {
		t.Fatalf("config = %#v", cfg)
	}
	if strings.Contains(strings.Join(RedactInstructionArgs(inj.Args), " "), "user rules") || !strings.Contains(inj.Args[1], "user rules") {
		t.Fatal("logging leaks instructions or changes actual invocation")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := PrepareInstructions(ctx, "codex", bin, dir, rules, nil); err == nil {
		t.Fatal("canceled configuration read succeeded")
	}
	for _, mode := range []string{"notification", "missing-config", "stall"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			ready := filepath.Join(t.TempDir(), "ready")
			env := append(os.Environ(), "HYLO_TEST_CODEX_CONFIG="+mode, "HYLO_TEST_CODEX_READY="+ready)
			if mode == "stall" {
				go func() {
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							if _, err := os.Stat(ready); err == nil {
								cancel()
								return
							}
						}
					}
				}()
			}
			_, err := PrepareInstructions(ctx, "codex", bin, dir, rules, env)
			switch mode {
			case "notification":
				if err != nil {
					t.Fatal(err)
				}
			case "missing-config":
				if err == nil {
					t.Fatal("missing config would discard existing user instructions")
				}
			case "stall":
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("stalled config read did not cancel: %v", err)
				}
			}
		})
	}
}

func TestPiExtraDirectoriesAreNotInstructions(t *testing.T) {
	args := BuildInvocationArgs(GetAgentDef("pi"), BuildArgsContext{ExtraAllowedDirs: []string{"/vault"}})
	if strings.Contains(strings.Join(args, " "), "--append-system-prompt") {
		t.Fatal(args)
	}
}
