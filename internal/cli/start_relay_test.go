package cli

import "testing"

func TestRunRelayRequiresJoinKey(t *testing.T) {
	for _, key := range []string{"", "  ", "\t"} {
		err := runRelay("127.0.0.1:0", t.TempDir()+"/state.json", key)
		if err == nil {
			t.Fatalf("join key %q started the relay", key)
		}
	}
}
