package agent

import "testing"

func TestTextAccumulatorBreaksAfterToolsAndThinking(t *testing.T) {
	var a TextAccumulator
	for _, m := range []map[string]any{
		{"type": "text_delta", "delta": "Running it now."},
		{"type": "tool_use", "name": "bot_create"},
		{"type": "tool_result", "content": "ok"},
		{"type": "text_delta", "delta": "```receipt\ntitle: Done\n```"},
		{"type": "thinking_delta", "delta": "hmm"},
		{"type": "text_delta", "delta": "Next.\n"},
		{"type": "tool_use", "name": "read"},
		{"type": "text_delta", "delta": "End"},
	} {
		a.Feed(m)
	}
	want := "Running it now.\n\n```receipt\ntitle: Done\n```\n\nNext.\n\nEnd"
	if got := a.String(); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestTextAccumulatorKeepsDeltasOfOneBlockTogether(t *testing.T) {
	var a TextAccumulator
	a.Feed(map[string]any{"type": "tool_use", "name": "read"})
	a.Feed(map[string]any{"type": "text_delta", "delta": "Hel"})
	a.Feed(map[string]any{"type": "note_access", "path": "/a.md"})
	a.Feed(map[string]any{"type": "text_delta", "delta": "lo"})
	if a.String() != "Hello" {
		t.Fatalf("%q", a.String())
	}
	a.Feed(map[string]any{"type": "text_replace", "text": "final"})
	if a.String() != "final" {
		t.Fatalf("%q", a.String())
	}
}
