package agent

import "testing"

func TestNoteAccessTracker_ClaudeStyleTools(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)

	got := tr.Feed(map[string]any{
		"type": "tool_use", "name": "Read",
		"input": map[string]any{"file_path": "/vault/journal/april.md"},
	})
	want := []NoteAccess{{Path: "/journal/april.md", Action: NoteActionRead, Tool: "Read"}}
	assertPaths(t, got, want)

	// A later Edit on the same note upgrades it from read to write, and is
	// reported again since the action changed.
	got = tr.Feed(map[string]any{
		"type": "tool_use", "name": "Edit",
		"input": map[string]any{"file_path": "/vault/journal/april.md"},
	})
	want = []NoteAccess{{Path: "/journal/april.md", Action: NoteActionWrite, Tool: "Edit"}}
	assertPaths(t, got, want)

	// A repeat read of the already-upgraded path is not reported again.
	got = tr.Feed(map[string]any{
		"type": "tool_use", "name": "Read",
		"input": map[string]any{"file_path": "/vault/journal/april.md"},
	})
	if len(got) != 0 {
		t.Fatalf("expected no new entries for a repeat read, got %+v", got)
	}

	entries := tr.Entries()
	if len(entries) != 1 || entries[0].Action != NoteActionWrite {
		t.Fatalf("expected a single upgraded entry, got %+v", entries)
	}
}

func TestNoteAccessTracker_RelativeToVaultAndCwdSubdir(t *testing.T) {
	tr := NewNoteAccessTracker("/vault/projects", "/vault", nil)
	got := tr.Feed(map[string]any{
		"type": "tool_use", "name": "write_file",
		"input": map[string]any{"path": "notes/todo.md"},
	})
	assertPaths(t, got, []NoteAccess{{Path: "/projects/notes/todo.md", Action: NoteActionWrite, Tool: "write_file"}})
}

func TestNoteAccessTracker_RejectsOutsideVaultAndNonMarkdown(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)

	cases := []map[string]any{
		{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/etc/passwd.md"}},
		{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/vault/.hylo/meta.db"}},
		{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/vault/.hylo/uploads/x.md"}},
		{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/vault/image.png"}},
	}
	for _, c := range cases {
		if got := tr.Feed(c); len(got) != 0 {
			t.Fatalf("expected rejection for %v, got %+v", c["input"], got)
		}
	}
	if len(tr.Entries()) != 0 {
		t.Fatalf("expected no entries, got %+v", tr.Entries())
	}
}

func TestNoteAccessTracker_RejectsGlobPatterns(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)

	cases := []map[string]any{
		{"type": "tool_use", "name": "Glob", "input": map[string]any{"path": "*.md"}},
		{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/vault/**/*.md"}},
		{"type": "tool_use", "name": "Bash", "input": map[string]any{"command": "cat /vault/daily-*.md"}},
	}
	for _, c := range cases {
		if got := tr.Feed(c); len(got) != 0 {
			t.Fatalf("expected glob pattern to be rejected for %v, got %+v", c["input"], got)
		}
	}
	if len(tr.Entries()) != 0 {
		t.Fatalf("expected no entries, got %+v", tr.Entries())
	}
}

func TestNoteAccessTracker_BashHeuristic(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)
	got := tr.Feed(map[string]any{
		"type": "tool_use", "name": "Bash",
		"input": map[string]any{"command": "cat /vault/a.md && echo hi >> /vault/b.md"},
	})
	assertPaths(t, got, []NoteAccess{
		{Path: "/a.md", Action: NoteActionRead, Tool: "Bash"},
		{Path: "/b.md", Action: NoteActionWrite, Tool: "Bash"},
	})
}

func TestNoteAccessTracker_ExplicitNotesOverride(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)
	got := tr.Feed(map[string]any{
		"type": "tool_use", "name": "edit",
		"input": map[string]any{"__notes": []any{
			map[string]any{"path": "/vault/plan.md", "action": "write"},
		}},
	})
	assertPaths(t, got, []NoteAccess{{Path: "/plan.md", Action: NoteActionWrite, Tool: "edit"}})
}

func TestNoteAccessTracker_IgnoresNonToolUseEvents(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)
	if got := tr.Feed(map[string]any{"type": "text_delta", "delta": "hello"}); got != nil {
		t.Fatalf("expected nil for non tool_use event, got %+v", got)
	}
}

func assertPaths(t *testing.T, got, want []NoteAccess) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %+v, want %+v", got, want)
	}
	for i := range got {
		if got[i].Path != want[i].Path || got[i].Action != want[i].Action || got[i].Tool != want[i].Tool {
			t.Fatalf("entry %d mismatch: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestNoteAccessTracker_BashOnlyDemonstrableChangesAreWrites(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)
	got := tr.Feed(map[string]any{
		"type": "tool_use", "name": "Bash",
		"input": map[string]any{"command": "grep -n x /vault/a.md 2>/dev/null; sed -n '1,9p' /vault/b.md; awk 'NR>=3' /vault/c.md; cat /vault/d.md > /vault/e.md; cp /vault/f.md /vault/g.md; sed -i s/a/b/ /vault/h.md"},
	})
	assertPaths(t, got, []NoteAccess{
		{Path: "/a.md", Action: NoteActionRead, Tool: "Bash"},
		{Path: "/b.md", Action: NoteActionRead, Tool: "Bash"},
		{Path: "/c.md", Action: NoteActionRead, Tool: "Bash"},
		{Path: "/d.md", Action: NoteActionRead, Tool: "Bash"},
		{Path: "/e.md", Action: NoteActionWrite, Tool: "Bash"},
		{Path: "/f.md", Action: NoteActionRead, Tool: "Bash"},
		{Path: "/g.md", Action: NoteActionWrite, Tool: "Bash"},
		{Path: "/h.md", Action: NoteActionWrite, Tool: "Bash"},
	})
}

type stubKnown struct{}

func (stubKnown) Tool(name string, input map[string]any) ([]NoteAccess, bool) {
	if name != "mcp__hylo__read" {
		return nil, false
	}
	p, _ := input["path"].(string)
	return []NoteAccess{{Path: p, Action: NoteActionRead}}, true
}

func (stubKnown) Command(args []string) ([]NoteAccess, bool) {
	if len(args) < 2 || args[0] != "append" {
		return nil, false
	}
	return []NoteAccess{{Path: args[1], Action: NoteActionWrite}}, true
}

// Known tools name vault paths, which must not be read as filesystem paths.
func TestNoteAccessTracker_KnownToolsTakeVaultPaths(t *testing.T) {
	tr := NewNoteAccessTracker("/vault/_bots/_a", "/vault", stubKnown{})
	tr.Feed(map[string]any{"type": "tool_use", "name": "mcp__hylo__read", "input": map[string]any{"path": "/journal/today.md"}})
	tr.Feed(map[string]any{"type": "tool_use", "name": "mcp__hylo__read", "input": map[string]any{"path": "/.hylo/x.md"}})
	tr.Feed(map[string]any{"type": "tool_use", "name": "Bash", "input": map[string]any{"command": `cat x && /usr/local/bin/hylo append "/b.md" && hylo init`}})
	assertPaths(t, tr.Entries(), []NoteAccess{
		{Path: "/journal/today.md", Action: NoteActionRead, Tool: "mcp__hylo__read"},
		{Path: "/b.md", Action: NoteActionWrite, Tool: "Bash"},
	})
}

// Listing, searching and counting a note shows on the card but is no read of it.
func TestNoteAccessTracker_SkimsAreNotReads(t *testing.T) {
	tr := NewNoteAccessTracker("/vault", "/vault", nil)
	for _, call := range []map[string]any{
		{"name": "Grep", "input": map[string]any{"pattern": "x", "path": "/vault/g.md"}},
		{"name": "Bash", "input": map[string]any{"command": `ls l.md && wc -l w.md; grep -n "a; b" r.md`}},
		{"name": "Bash", "input": map[string]any{"command": "cat c.md"}},
		{"name": "Read", "input": map[string]any{"file_path": "/vault/r.md"}},
		{"name": "Bash", "input": map[string]any{"command": "ls x.md; rm x.md"}},
	} {
		call["type"] = "tool_use"
		tr.Feed(call)
	}
	skim := map[string]bool{}
	for _, e := range tr.Entries() {
		skim[e.Path] = e.Skim
	}
	want := map[string]bool{"/g.md": true, "/l.md": true, "/w.md": true, "/r.md": false, "/c.md": false, "/x.md": false}
	if len(skim) != len(want) {
		t.Fatalf("entries = %+v", tr.Entries())
	}
	for p, w := range want {
		if skim[p] != w {
			t.Errorf("%s skim = %v, want %v", p, skim[p], w)
		}
	}
}
