package view

import (
	"os"
	"path/filepath"
	"testing"
)

// The desktop shell page is not served by Hylo, so it carries its own copy of
// the tokens. UPDATE_DESKTOP_TOKENS=1 rewrites it (make desktop-ui).
func TestDesktopShellTokens(t *testing.T) {
	want := "/* Generated from internal/server/view/shared_tokens.go by `make desktop-ui`. Do not edit. */\n" +
		appTokensCSS + "\n"
	path := filepath.Join("..", "..", "..", "desktop-tauri", "ui", "tokens.css")
	if os.Getenv("UPDATE_DESKTOP_TOKENS") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("desktop-tauri/ui/tokens.css is stale; run `make desktop-ui`")
	}
}
