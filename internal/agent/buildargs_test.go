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

func TestBuildImageAttachmentArgs(t *testing.T) {
	paths := []string{"/vault/.hylo/uploads/a.png", "/vault/.hylo/uploads/b.png"}
	codex := BuildInvocationArgs(GetAgentDef("codex"), BuildArgsContext{SessionID: "thread-1", ImagePaths: paths})
	if !containsSeq(codex, "exec", "resume", "thread-1") ||
		!containsSeq(codex, "-i", paths[0]) || !containsSeq(codex, "-i", paths[1]) {
		t.Fatalf("codex args = %v", codex)
	}
	copilot := BuildInvocationArgs(GetAgentDef("copilot"), BuildArgsContext{ImagePaths: paths})
	if !containsSeq(copilot, "--attachment", paths[0]) || !containsSeq(copilot, "--attachment", paths[1]) {
		t.Fatalf("copilot args = %v", copilot)
	}
	for _, id := range []string{"claude", "pi", "opencode", "cursor-agent"} {
		argv := BuildInvocationArgs(GetAgentDef(id), BuildArgsContext{ImagePaths: paths})
		for _, a := range argv {
			if a == "-i" || a == "--attachment" || a == paths[0] {
				t.Fatalf("%s should not receive image flags: %v", id, argv)
			}
		}
	}
}

func TestUserTextImageCaption(t *testing.T) {
	if got := UserText("codex", "", 1); got != "Look at the attached image." {
		t.Fatalf("codex caption = %q", got)
	}
	if got := UserText("copilot", "  ", 2); got != "Look at the attached images." {
		t.Fatalf("copilot caption = %q", got)
	}
	if got := UserText("codex", "look", 1); got != "look" {
		t.Fatalf("caption replaced user text: %q", got)
	}
	if got := UserText("pi", "", 1); got != "" {
		t.Fatalf("pi caption = %q", got)
	}
	if got := UserText("claude", "", 2); got != "" {
		t.Fatalf("claude caption = %q", got)
	}
	if got := UserText("cursor-agent", "", 1); got != "Look at the attached image." {
		t.Fatalf("cursor caption = %q", got)
	}
	if got := UserText("opencode", "  ", 2); got != "Look at the attached images." {
		t.Fatalf("opencode caption = %q", got)
	}
}

func TestTurnTextImagePaths(t *testing.T) {
	paths := []string{"/vault/.hylo/uploads/a.png", "/vault/.hylo/uploads/b.png"}
	got := TurnText("cursor-agent", "look", paths)
	if got != "look\n"+paths[0]+"\n"+paths[1] {
		t.Fatalf("cursor text = %q", got)
	}
	got = TurnText("opencode", "", paths[:1])
	if got != "Look at the attached image.\n"+paths[0] {
		t.Fatalf("opencode text = %q", got)
	}
	for _, id := range []string{"claude", "codex", "pi", "copilot"} {
		if strings.Contains(TurnText(id, "look", paths), paths[0]) {
			t.Fatalf("%s prompt should not contain the path", id)
		}
	}
}

func TestBuildPathImageAccess(t *testing.T) {
	paths := []string{"/vault/.hylo/uploads/a.png", "/vault/.hylo/uploads/b.png"}
	cursor := BuildInvocationArgs(GetAgentDef("cursor-agent"), BuildArgsContext{SessionID: "chat-1", ImagePaths: paths})
	if !containsSeq(cursor, "--resume", "chat-1") || !containsSeq(cursor, "--add-dir", "/vault/.hylo/uploads") {
		t.Fatalf("cursor args = %v", cursor)
	}
	if strings.Count(strings.Join(cursor, "\n"), "--add-dir") != 1 {
		t.Fatalf("cursor add-dir count = %v", cursor)
	}
	for _, a := range cursor {
		if a == "--image" || a == "-f" || a == "--file" || a == paths[0] {
			t.Fatalf("cursor args = %v", cursor)
		}
	}
	opencode := BuildInvocationArgs(GetAgentDef("opencode"), BuildArgsContext{ImagePaths: paths})
	for _, a := range opencode {
		if a == "-f" || a == "--file" || a == "--add-dir" || a == paths[0] {
			t.Fatalf("opencode args = %v", opencode)
		}
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
