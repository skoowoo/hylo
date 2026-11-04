package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/storage"
)

func imageClient(t *testing.T) (*client.Client, *storage.Vault) {
	t.Helper()
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/vault/upload-image", NewVault(v).UploadImage)
	return client.NewInProcess(mux), v
}

func TestUploadImageFromLocalFileAndChatAttachment(t *testing.T) {
	c, v := imageClient(t)

	local := filepath.Join(t.TempDir(), "board.png")
	if err := os.WriteFile(local, tinyPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	img, err := c.UploadImage(local)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(img.Src, storage.AssetsDir+"/") || !strings.HasSuffix(img.Name, ".png") {
		t.Fatalf("uploaded %+v", img)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), img.Src)); err != nil {
		t.Fatalf("stored file: %v", err)
	}

	att := "0b8f2a1e-3c4d-4e5f-8a9b-0c1d2e3f4a5b.png"
	if err := os.MkdirAll(filepath.Join(v.Root(), storage.UploadDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v.Root(), storage.UploadDir, att), tinyPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	img, err = c.UploadChatImage(att)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), img.Src)); err != nil {
		t.Fatalf("stored attachment: %v", err)
	}
}

// Only a chat attachment's own file name is accepted, so nothing else on the server is readable.
func TestUploadChatImageRejectsOtherFiles(t *testing.T) {
	c, _ := imageClient(t)
	for _, name := range []string{"../config.toml", "/etc/hosts", "photo.png"} {
		if _, err := c.UploadChatImage(name); err == nil || !strings.Contains(err.Error(), "400") {
			t.Errorf("%q: want a 400, got %v", name, err)
		}
	}
	if _, err := c.UploadChatImage("0b8f2a1e-3c4d-4e5f-8a9b-0c1d2e3f4a5b.png"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("missing attachment: want a 404, got %v", err)
	}
}
