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
