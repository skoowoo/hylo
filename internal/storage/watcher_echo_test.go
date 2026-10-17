package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

type eventLog struct {
	t      *testing.T
	v      *Vault
	mu     sync.Mutex
	events []string
	flushN int
}

func watchedVault(t *testing.T) *eventLog {
	t.Helper()
	l := &eventLog{t: t, v: newTestVault(t)}
	l.v.SetEventHook(func(e plugin.Event) {
		l.mu.Lock()
		l.events = append(l.events, string(e.Type)+" "+e.Path)
		l.mu.Unlock()
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := l.v.StartWatcher(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return l
}

// take waits until the watcher has handled everything before it (proven by an
// external sentinel note arriving), then returns and clears the events so far.
func (l *eventLog) take() []string {
	l.t.Helper()
	l.flushN++
	name := fmt.Sprintf("zz-flush-%d.md", l.flushN)
	if err := os.WriteFile(filepath.Join(l.v.Root(), name), []byte("x\n"), 0o644); err != nil {
		l.t.Fatal(err)
	}
	want := "fs_create /" + name
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		if slices.Contains(l.events, want) {
			var out []string
			for _, e := range l.events {
				if !strings.Contains(e, "/zz-flush-") {
					out = append(out, e)
				}
			}
			l.events = nil
			l.mu.Unlock()
			return out
		}
		l.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	l.t.Fatal("watcher never reported the flush sentinel")
	return nil
}

func expectEvents(t *testing.T, step string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s: events = %q, want %q", step, got, want)
	}
}

func TestWatcherDropsEchoesOfVaultWrites(t *testing.T) {
	l := watchedVault(t)
	v := l.v
	p := Path("/a.md")

	if err := v.WriteNote(p, []byte("---\ntags: [x, y]\n---\nhello\n"), ""); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "create", l.take(), "vault_create /a.md")

	if err := v.WriteNote(p, []byte("---\ntags: [x, y]\n---\nhello again\n"), ""); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "write", l.take(), "vault_write /a.md")
	if n, _ := v.StatNote(p); !slices.Equal(n.Tags, []string{"x", "y"}) {
		t.Fatalf("tags after the write's echo = %q", n.Tags)
	}

	moved, err := v.MoveNote(p, "/sub")
	if err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "move", l.take(), "vault_delete /a.md", "vault_create /sub/a.md")

	if err := v.DeleteNote(moved); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "delete", l.take(), "vault_delete /sub/a.md")

	u, err := v.CreateUntitledNote("/", []byte("draft\n"))
	if err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "untitled", l.take(), "vault_create "+u.String())
}

// A note deleted and recreated before the watcher catches up must stay tracked,
// and plugins must end on the create.
func TestWatcherKeepsQuicklyRecreatedNote(t *testing.T) {
	l := watchedVault(t)
	v := l.v
	p := Path("/c.md")

	for _, step := range []func() error{
		func() error { return v.WriteNote(p, []byte("one\n"), "") },
		func() error { return v.DeleteNote(p) },
		func() error { return v.WriteNote(p, []byte("two\n"), "") },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	expectEvents(t, "recreate", l.take(), "vault_create /c.md", "vault_delete /c.md", "vault_create /c.md")
	if _, err := v.StatNote(p); err != nil {
		t.Fatalf("recreated note lost its metadata: %v", err)
	}
}

func TestWatcherAnnouncesExternalChanges(t *testing.T) {
	l := watchedVault(t)
	abs := filepath.Join(l.v.Root(), "b.md")

	if err := os.WriteFile(abs, []byte("---\ntags: [ext]\n---\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "external create", l.take(), "fs_create /b.md")
	if n, _ := l.v.StatNote(Path("/b.md")); !slices.Equal(n.Tags, []string{"ext"}) {
		t.Fatalf("tags of an external note = %q", n.Tags)
	}

	if err := os.WriteFile(abs, []byte("b changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "external write", l.take(), "fs_write /b.md")

	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "external delete", l.take(), "fs_delete /b.md")

	// Restored from a backup or the trash: the mtime predates everything seen so far.
	tmp := filepath.Join(t.TempDir(), "b.md")
	if err := os.WriteFile(tmp, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(tmp, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, abs); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "restore", l.take(), "fs_create /b.md")
}

func TestWatcherTracksExternalImages(t *testing.T) {
	l := watchedVault(t)
	v := l.v
	abs := filepath.Join(v.Root(), "pics", "a.png")

	// A fresh directory too, so the kqueue catch-up walk has to find it.
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, testPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "external image", l.take(), "fs_image /pics/a.png")
	if imgs, _ := v.GetImagesByName("a.png"); len(imgs) != 1 || imgs[0].Dir != "/pics" {
		t.Fatalf("external image not registered: %+v", imgs)
	}

	if err := os.WriteFile(abs, append(testPNG, 0), 0o644); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "external image write", l.take(), "fs_image /pics/a.png")

	n := Path("/n.md")
	if err := v.ReplaceNoteAssets(n, AssetKindImage, []NoteAsset{{Kind: AssetKindImage, Filename: "a.png"}}); err != nil {
		t.Fatal(err)
	}
	if err := v.ReplaceNoteAssets(n, AssetKindCover, []NoteAsset{{Kind: AssetKindCover, Filename: "a.png"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "external image delete", l.take(), "fs_image /pics/a.png")
	if imgs, _ := v.GetImagesByName("a.png"); len(imgs) != 0 {
		t.Fatalf("deleted image still registered: %+v", imgs)
	}
	if _, err := v.GetNoteAsset(n, AssetKindCover); err == nil {
		t.Fatal("cover of a deleted image survived")
	}
	if notes, _ := v.CoverlessNotesEmbedding("a.png"); !slices.Equal(notes, []Path{n}) {
		t.Fatalf("embed rows must outlive the image so a restore relinks it, got %q", notes)
	}
}

func TestWatcherDropsEchoesOfVaultImages(t *testing.T) {
	l := watchedVault(t)
	img, err := l.v.SaveImageBytes(testPNG, ".png")
	if err != nil {
		t.Fatal(err)
	}
	p := img.Dir + "/" + img.Name
	expectEvents(t, "save", l.take(), "vault_image "+p)

	if err := l.v.DeleteImage(img.Dir, img.Name); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, "delete", l.take(), "vault_image "+p)
}
