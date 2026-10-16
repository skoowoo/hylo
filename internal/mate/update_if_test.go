package mate

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestUpdateMateIfRejectsStaleRead(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	m := &Mate{Name: "A", AgentID: "claude", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	a, _ := store.GetMate(m.ID)
	b, _ := store.GetMate(m.ID)

	a.Name = "first"
	if err := store.UpdateMateIf(a, a.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	b.Name = "second"
	if err := store.UpdateMateIf(b, b.UpdatedAt); !errors.Is(err, ErrMateConflict) {
		t.Fatalf("stale update err = %v", err)
	}
	got, _ := store.GetMate(m.ID)
	if got.Name != "first" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestBotWorkingDirectories(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	want := map[string]string{"Hylo": "/", "News": "/_bots/_news", "news ": "/_bots/_news-2", "AI: Daily/Weekly": "/_bots/_ai-daily-weekly", "???": "/_bots/_bot"}
	for _, name := range []string{"Hylo", "News", "news ", "AI: Daily/Weekly", "???"} {
		m := &Mate{Name: name, Main: name == "Hylo", Cwd: "/ignored"}
		if err := store.CreateMate(m); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		if m.Cwd != want[name] {
			t.Errorf("%q works in %q, want %q", name, m.Cwd, want[name])
		}
	}
	if err := store.CreateMate(&Mate{Name: "Second main", Main: true}); err == nil {
		t.Fatal("a second main bot was stored")
	}
	long := &Mate{Name: strings.Repeat("新闻", 40)}
	if err := store.CreateMate(long); err != nil || long.Cwd != "/_bots/_"+strings.Repeat("新闻", 24) {
		t.Fatalf("long name works in %q: %v", long.Cwd, err)
	}

	// Bots of one name created at once still get folders of their own.
	var wg sync.WaitGroup
	twins := make([]*Mate, 4)
	for i := range twins {
		twins[i] = &Mate{Name: "Twin"}
		wg.Add(1)
		go func(m *Mate) {
			defer wg.Done()
			if err := store.CreateMate(m); err != nil {
				t.Error(err)
			}
		}(twins[i])
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, m := range twins {
		if seen[m.Cwd] {
			t.Fatalf("two twins work in %q", m.Cwd)
		}
		seen[m.Cwd] = true
	}

	bots, _ := store.ListMates()
	news := bots[1]
	news.Name, news.Cwd = "Renamed", "/elsewhere"
	if err := store.UpdateMate(&news); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetMate(news.ID); got.Cwd != "/_bots/_news" || got.Main {
		t.Fatalf("after update %+v", got)
	}
	if got, _ := store.GetMate(bots[0].ID); !got.Main {
		t.Fatal("main bot lost its role")
	}
}
