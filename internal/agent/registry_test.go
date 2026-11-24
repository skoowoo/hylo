package agent

import "testing"

func TestSupportsNativeSession(t *testing.T) {
	want := map[string]bool{
		"claude":       true,
		"codex":        true,
		"opencode":     true,
		"cursor-agent": true,
		"copilot":      true,
		"pi":           true,
		"agy":          true,
	}
	for _, d := range BuiltInAgents() {
		got := d.SupportsNativeSession
		expect, ok := want[d.ID]
		if !ok {
			t.Fatalf("agent %q missing from supportsNativeSession audit table", d.ID)
		}
		if got != expect {
			t.Errorf("agent %q SupportsNativeSession = %v, want %v", d.ID, got, expect)
		}
	}
	if len(want) != len(BuiltInAgents()) {
		t.Fatalf("audit table has %d entries, registry has %d agents", len(want), len(BuiltInAgents()))
	}
}

func TestParseAgyModels(t *testing.T) {
	raw := "gemini-4-neon-high              Gemini 4 Neon (High)\n" +
		"gemini-4-neon-medium            Gemini 4 Neon (Medium)\n" +
		"gemini-4-neon-high              Duplicate Ignored\n" +
		"solo-model\n"
	got := parseAgyModels(raw)
	if len(got) != 4 {
		t.Fatalf("got %+v, want 4 entries", got)
	}
	if got[1].ID != "gemini-4-neon-high" || got[1].Label != "Gemini 4 Neon (High)" {
		t.Errorf("got[1] = %+v", got[1])
	}
	if got[3].ID != "solo-model" || got[3].Label != "solo-model" {
		t.Errorf("got[3] = %+v", got[3])
	}
	if parseAgyModels("\n  \n") != nil {
		t.Error("expected nil for empty output")
	}
}
