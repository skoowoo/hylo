package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func feedAll(t *TurnLog, ms ...map[string]any) {
	for _, m := range ms {
		t.Feed(m)
	}
}

func TestTurnLogAnswerIsLastTextRun(t *testing.T) {
	var l TurnLog
	feedAll(&l,
		map[string]any{"type": "text_delta", "delta": "Writing it now."},
		map[string]any{"type": "tool_use", "name": "editor_type"},
		map[string]any{"type": "tool_result", "content": "ok"},
		map[string]any{"type": "thinking_delta", "delta": "hmm"},
		map[string]any{"type": "text_delta", "delta": "Do"},
		map[string]any{"type": "note_access", "path": "/a.md"},
		map[string]any{"type": "text_delta", "delta": "ne."},
	)
	if l.Answer() != "Done." {
		t.Fatalf("%q", l.Answer())
	}
}

func TestTurnLogAnswerSurvivesTrailingSteps(t *testing.T) {
	var l TurnLog
	feedAll(&l,
		map[string]any{"type": "text_delta", "delta": "All set."},
		map[string]any{"type": "tool_use", "name": "TodoWrite"},
		map[string]any{"type": "tool_result", "content": "ok"},
	)
	if l.Answer() != "All set." {
		t.Fatalf("%q", l.Answer())
	}
}

func TestTurnLogCompactsEvents(t *testing.T) {
	var l TurnLog
	feedAll(&l,
		map[string]any{"type": "status", "label": "thinking"},
		map[string]any{"type": "thinking_delta", "delta": "a"},
		map[string]any{"type": "thinking_delta", "delta": "b"},
		map[string]any{"type": "tool_use", "id": "t1", "name": "Write", "input": map[string]any{
			"file_path": "/v/a.md", "content": strings.Repeat("x", 5000), "n": 3,
		}},
		map[string]any{"type": "tool_result", "toolUseId": "t1", "content": strings.Repeat("y", 5000)},
		map[string]any{"type": "text_delta", "delta": "He"},
		map[string]any{"type": "text_delta", "delta": "llo"},
	)
	var evs []map[string]any
	if err := json.Unmarshal([]byte(l.JSON()), &evs); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 4 {
		t.Fatalf("want 4 events, got %d: %s", len(evs), l.JSON())
	}
	if evs[0]["delta"] != "ab" || evs[3]["delta"] != "Hello" {
		t.Fatalf("deltas not merged: %s", l.JSON())
	}
	if in := evs[1]["input"].(map[string]any); len(in) != 1 || in["file_path"] != "/v/a.md" {
		t.Fatalf("input not compacted: %v", in)
	}
	if n := len([]rune(evs[2]["content"].(string))); n != turnLogResultMax+1 {
		t.Fatalf("result not clipped: %d", n)
	}
	for _, e := range evs {
		if _, ok := e["at"].(float64); !ok {
			t.Fatalf("missing at: %v", e)
		}
	}
}
