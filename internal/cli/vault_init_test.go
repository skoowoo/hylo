package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/storage"
)

func TestVaultInitializationRestoresCovers(t *testing.T) {
	for _, mode := range []string{"automatic", "command"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			remoteURL := "http://127.0.0.1:1/cached.png"
			hash := sha256.Sum256([]byte(remoteURL))
			cached := fmt.Sprintf("remote-%x.png", hash[:8])
			files := map[string]string{
				"wiki.md":               "# Wiki\n![[cover.png]]\n",
				"markdown.md":           "# Markdown\n![cover](_assets/cover.png)\n",
				"frontmatter.md":        "---\nimage_url: preferred.png\n---\n![[cover.png]]\n",
				"remote.md":             "---\nimage_url: " + remoteURL + "\n---\n# Remote\n",
				"fallback.md":           "![[missing.png]]\n![[cover.png]]\n",
				"plain.md":              "# No image\n",
				"_shorts/2026-11-16.md": "---\nkind: short\n---\n![[cover.png]]\n",
				"_assets/cover.png":     "image bytes",
				"_assets/preferred.png": "image bytes",
				"_assets/" + cached:     "cached image bytes",
			}
			for name, content := range files {
				dest := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "automatic" {
				vault, err := storage.New(root)
				if err != nil {
					t.Fatal(err)
				}
				cfg := config.MustLoadFor(root)
				err = autoInitVault(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, vault, false)
				closeErr := vault.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			} else {
				cmd := newInitCmd()
				cmd.SetArgs([]string{root})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			}
			vault, err := storage.New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer vault.Close()
			for name, want := range map[string]string{
				"wiki.md": "cover.png", "markdown.md": "cover.png", "frontmatter.md": "preferred.png",
				"remote.md": cached, "fallback.md": "cover.png",
			} {
				cover, err := vault.GetNoteAsset(storage.Path("/"+name), storage.AssetKindCover)
				if err != nil || cover.Filename != want {
					t.Errorf("%s: cover = %+v, err = %v; want %s", name, cover, err, want)
				}
				if name == "remote.md" && cover.SourceURL != remoteURL {
					t.Errorf("remote source = %q", cover.SourceURL)
				}
			}
			for _, name := range []string{"plain.md", "_shorts/2026-11-16.md"} {
				if _, err := vault.GetNoteAsset(storage.Path("/"+name), storage.AssetKindCover); !errors.Is(err, storage.ErrMetadataNotFound) {
					t.Errorf("%s: expected no cover, got %v", name, err)
				}
			}
		})
	}
}
