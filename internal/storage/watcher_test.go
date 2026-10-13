package storage

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

// A note written right after its directory is made lands before the watch on the
// directory does; it still has to be indexed and announced.
func TestWatcherCatchesNotesInANewDirectory(t *testing.T) {
	v := newTestVault(t)
	var mu sync.Mutex
	seen := map[string]bool{}
	v.SetEventHook(func(e plugin.Event) {
		if e.Type == plugin.EventFSCreate {
			mu.Lock()
			seen[e.Path] = true
			mu.Unlock()
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := v.StartWatcher(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(v.Root(), "Travel", "Japan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kyoto.md"), []byte("# Kyoto\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := seen["/Travel/Japan/kyoto.md"]
		mu.Unlock()
		if ok {
			if _, err := v.StatNote(Path("/Travel/Japan/kyoto.md")); err != nil {
				t.Fatalf("announced but not indexed: %v", err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no fs_create for a note written into a new directory")
}
