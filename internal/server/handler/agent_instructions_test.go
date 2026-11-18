package handler

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/pelletier/go-toml/v2"
)

func TestNativeInstructionsCLIHelper(t *testing.T) {
	capture := os.Getenv("HYLO_TEST_NATIVE_CAPTURE")
	if capture == "" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if args[0] == "app-server" {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			var req struct {
				ID     int    `json:"id"`
				Method string `json:"method"`
			}
			_ = json.Unmarshal(s.Bytes(), &req)
			if req.Method == "initialize" {
				_, _ = os.Stdout.WriteString("{\"id\":1,\"result\":{}}\n")
			}
			if req.Method == "config/read" {
				_, _ = os.Stdout.WriteString("{\"id\":2,\"result\":{\"config\":{\"developer_instructions\":\"existing rules\"}}}\n")
				os.Exit(0)
			}
		}
	} else {
		message, _ := io.ReadAll(os.Stdin)
		b, _ := json.Marshal(map[string]any{"args": args, "message": string(message)})
		if err := os.WriteFile(capture, b, 0600); err != nil {
			os.Exit(2)
		}
		_, _ = os.Stdout.WriteString("{\"type\":\"thread.started\",\"thread_id\":\"native-test\"}\n")
	}
	os.Exit(0)
}

func TestNativeInstructionsRefreshOnResume(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell launcher")
	}
	a := testAgentAPI(t)
	store, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	a.store, a.hub = store, agent.NewHub()
	conv, err := store.CreateConversation("bot")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin, capture := filepath.Join(dir, "codex"), filepath.Join(dir, "capture.json")
	exe, _ := os.Executable()
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=^TestNativeInstructionsCLIHelper$ -- \"$@\"\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for i, rules := range []string{"first bot rules", "updated bot rules"} {
		run := a.hub.CreateRun(nil)
		a.executeChat(t.Context(), run, chatBody{
			AgentID: "codex", ConversationID: conv.ID, Message: "user request", SystemPrompt: rules,
			AgentCliEnv: map[string]string{"CODEX_BIN": bin, "HYLO_TEST_NATIVE_CAPTURE": capture},
		})
		if status := run.StatusJSON()["status"]; status != "succeeded" {
			t.Fatalf("run status = %v", status)
		}
		var got struct {
			Args    []string `json:"args"`
			Message string   `json:"message"`
		}
		b, err := os.ReadFile(capture)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if got.Message != "user request" {
			t.Fatalf("instructions leaked into user message: %q", got.Message)
		}
		found := false
		for _, arg := range got.Args {
			if !strings.HasPrefix(arg, "developer_instructions") {
				continue
			}
			var cfg map[string]string
			if err := toml.Unmarshal([]byte(arg), &cfg); err != nil {
				t.Fatal(err)
			}
			text := cfg["developer_instructions"]
			if !strings.HasPrefix(text, "existing rules\n\n") || !strings.Contains(text, rules) || !strings.Contains(text, "the vault root") || strings.Contains(text, "# User request") {
				t.Fatalf("incorrect native instructions: %q", text)
			}
			found = true
		}
		if !found {
			t.Fatal("missing native instructions")
		}
		if i == 1 && (len(got.Args) < 3 || got.Args[1] != "resume" || got.Args[2] != "native-test") {
			t.Fatalf("did not resume native session: %v", got.Args)
		}
	}
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	run := a.hub.CreateRun(nil)
	a.logger = slog.New(slog.NewTextHandler(cancelOnSpawnWriter{cancel: func() { a.canceled.Store(run.ID, true) }}, nil))
	a.executeChat(t.Context(), run, chatBody{
		AgentID: "codex", ConversationID: conv.ID, Message: "must not execute",
		AgentCliEnv: map[string]string{"CODEX_BIN": bin, "HYLO_TEST_NATIVE_CAPTURE": capture},
	})
	if status := run.StatusJSON()["status"]; status != "canceled" {
		t.Fatalf("cancel during process handoff = %v", status)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("canceled handoff still delivered the request: %v", err)
	}
}

type cancelOnSpawnWriter struct{ cancel func() }

func (w cancelOnSpawnWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), `msg="agent spawn"`) {
		w.cancel()
	}
	return len(p), nil
}

func TestInstructionFailureDoesNotLeaveAnUnstartedSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses Unix executable and temporary directory semantics")
	}
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "first", true: "resume"}[resume], func(t *testing.T) {
			a := testAgentAPI(t)
			store, err := mate.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			a.store, a.hub = store, agent.NewHub()
			conv, err := store.CreateConversation("bot")
			if err != nil {
				t.Fatal(err)
			}
			if resume {
				if err := store.SetConversationAgentSession(conv.ID, "claude", a.vault.Root(), "existing-session"); err != nil {
					t.Fatal(err)
				}
			}
			dir := t.TempDir()
			bin := filepath.Join(dir, "claude")
			if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
				t.Fatal(err)
			}
			blocked := filepath.Join(dir, "not-a-directory")
			if err := os.WriteFile(blocked, nil, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			t.Setenv("TMPDIR", blocked)
			run := a.hub.CreateRun(nil)
			a.executeChat(t.Context(), run, chatBody{AgentID: "claude", ConversationID: conv.ID, Message: "hello"})
			if run.StatusJSON()["status"] != "failed" {
				t.Fatal(run.StatusJSON())
			}
			current, err := store.GetConversation(conv.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if resume {
				want = "existing-session"
			}
			if current.AgentSessionID != want {
				t.Fatalf("session after preparation failure = %q, want %q", current.AgentSessionID, want)
			}
		})
	}
}
