package config

import (
	"path/filepath"
	"testing"
)

func TestTelegramPairingTogglesTheBridge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	live := &Config{}
	if err := PersistTelegramPairing(path, live, TelegramPairing{BotToken: "tok", UserID: "42", Username: "hylo_bot"}); err != nil {
		t.Fatal(err)
	}
	saved, err := MergedFromOptionalFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Plugins.Telegram.Enabled || !live.Plugins.Telegram.Enabled || !live.TelegramConnected() {
		t.Fatalf("pairing must enable the bridge: saved %+v, live %+v", saved.Plugins.Telegram, live.Plugins.Telegram)
	}
	if err := PersistTelegramPairing(path, live, TelegramPairing{}); err != nil {
		t.Fatal(err)
	}
	if saved, _ = MergedFromOptionalFile(path); saved.Plugins.Telegram.Enabled || live.Plugins.Telegram.Enabled || live.TelegramConnected() {
		t.Fatal("disconnecting must disable the bridge")
	}
}
