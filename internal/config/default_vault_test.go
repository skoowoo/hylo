package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultVaultPathKeepsLegacyRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := defaultVaultPath(); got != DefaultVaultPath {
		t.Fatalf("fresh home: got %q", got)
	}
	if err := os.MkdirAll(filepath.Join(home, ".hylo", "root"), 0o750); err != nil {
		t.Fatal(err)
	}
	if got := defaultVaultPath(); got != legacyVaultPath {
		t.Fatalf("legacy home: got %q", got)
	}
}
