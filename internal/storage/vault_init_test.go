package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultConfigAloneIsNotInitialized(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".hylo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hylo", "config.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := IsVaultInitialized(root); err != nil || ok {
		t.Fatalf("config.toml counted as initialized: %v %v", ok, err)
	}
	v, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	if ok, err := IsVaultInitialized(root); err != nil || !ok {
		t.Fatalf("opened vault not initialized: %v %v", ok, err)
	}
}
