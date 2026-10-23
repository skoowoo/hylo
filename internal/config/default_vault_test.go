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

func TestResolvedUploadDir(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", DefaultUploadDir},
		{"_agent_uploads", DefaultUploadDir},
		{"/_agent_uploads/", DefaultUploadDir},
		{".hylo/uploads", DefaultUploadDir},
		{"attachments", "attachments"},
	}
	for _, c := range cases {
		if got := (AgentConfig{UploadDir: c.in}).ResolvedUploadDir(); got != c.want {
			t.Fatalf("ResolvedUploadDir(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
