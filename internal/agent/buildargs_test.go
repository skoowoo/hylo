package agent

import (
	"strings"
	"testing"
)

func TestBuildClaudeSessionArgs(t *testing.T) {
	def := GetAgentDef("claude")
	if def == nil {
		t.Fatal("claude def missing")
	}
	first := BuildInvocationArgs(def, BuildArgsContext{SessionID: "abc-123", FirstSession: true})
	if !containsSeq(first, "--session-id", "abc-123") {
		t.Fatalf("first session args = %v, want --session-id abc-123", first)
	}
	resume := BuildInvocationArgs(def, BuildArgsContext{SessionID: "abc-123"})
	if !containsSeq(resume, "--resume", "abc-123") {
		t.Fatalf("resume args = %v, want --resume abc-123", resume)
	}
}

func TestBuildCodexSessionArgs(t *testing.T) {
	def := GetAgentDef("codex")
	if def == nil {
		t.Fatal("codex def missing")
	}
	resume := BuildInvocationArgs(def, BuildArgsContext{SessionID: "thread-1", Cwd: "/tmp"})
	if !containsSeq(resume, "exec", "resume", "thread-1") {
		t.Fatalf("resume args = %v", resume)
	}
}

func TestBuildPiSessionArgs(t *testing.T) {
	def := GetAgentDef("pi")
	if def == nil {
		t.Fatal("pi def missing")
	}
	first := BuildInvocationArgs(def, BuildArgsContext{SessionID: "abc-123", FirstSession: true, Model: "deepseek/deepseek-flash"})
	if !containsSeq(first, "--mode", "rpc") || !containsSeq(first, "--session-id", "abc-123") {
		t.Fatalf("first session args = %v, want --mode rpc --session-id abc-123", first)
	}
	resume := BuildInvocationArgs(def, BuildArgsContext{SessionID: "abc-123", Model: "deepseek/deepseek-flash"})
	if !containsSeq(resume, "--session-id", "abc-123") {
		t.Fatalf("resume args = %v, want --session-id abc-123", resume)
	}
	for _, argv := range [][]string{first, resume} {
		for i := 0; i < len(argv)-1; i++ {
			if argv[i] == "--session" {
				t.Fatalf("--session exits when the id is missing: %v", argv)
			}
		}
	}
	ex := CLIExample(def)
	if strings.Contains(ex, `echo "Hello"`) || !strings.Contains(ex, "--mode json") || !strings.Contains(ex, "-p") {
		t.Fatalf("cli example = %q, want json one-shot, not raw stdin", ex)
	}
}

func TestBuildOpenCodeSessionArgs(t *testing.T) {
	def := GetAgentDef("opencode")
	if def == nil {
		t.Fatal("opencode def missing")
	}
	args := BuildInvocationArgs(def, BuildArgsContext{SessionID: "sess-9", Cwd: "/tmp"})
	if !containsSeq(args, "-s", "sess-9") {
		t.Fatalf("args = %v", args)
	}
	for _, a := range args {
		if a == "-" {
			t.Fatalf("session resume must not include stdin sentinel '-': %v", args)
		}
	}
}

func containsSeq(argv []string, seq ...string) bool {
outer:
	for i := 0; i+len(seq) <= len(argv); i++ {
		for j, s := range seq {
			if argv[i+j] != s {
				continue outer
			}
		}
		return true
	}
	return false
}
