package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestVaultOverviewListsFoldersAndRecentNotes(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if got := vaultOverview(v); !strings.Contains(got, "empty") {
		t.Fatalf("empty vault: %s", got)
	}
	for p, body := range map[string]string{
		"/Projects/Q3 launch plan.md": "# Q3\n\nWho is the first user?",
		"/Projects/Pricing.md":        "# Pricing\n\nFreemium or paid.",
		"/Reading/Dune.md":            "# Dune\n\nThe spice must flow.",
	} {
		if err := v.WriteNote(storage.Path(p), []byte(body), ""); err != nil {
			t.Fatal(err)
		}
	}
	o, err := buildVaultOverview(v, 0)
	if err != nil || o.Notes != 3 || len(o.Recent) != 3 || strings.HasSuffix(o.Recent[0].Title, ".md") {
		t.Fatalf("overview: %+v, %v", o, err)
	}
	got := vaultOverview(v)
	for _, want := range []string{"3 notes", "Projects (2)", "Reading (1)", "/Reading/Dune.md", "spice"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestVaultOverviewSamplesOnlyTheirOwnWriting(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	for _, p := range []string{"/Ideas/a.md", "/_memory/identity.md", "/_setup/progress.md"} {
		if err := v.WriteNote(storage.Path(p), []byte("x"), ""); err != nil {
			t.Fatal(err)
		}
	}
	o, err := buildVaultOverview(v, 0)
	if err != nil || !o.Obsidian || len(o.Recent) != 1 || o.Recent[0].Path != "/Ideas/a.md" {
		t.Fatalf("overview: %+v, %v", o, err)
	}
}

func TestWritingTimeNeedsAHabit(t *testing.T) {
	day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local)
	var late, bulk []storage.Note
	for i := 0; i < 10; i++ {
		late = append(late, storage.Note{UpdatedAt: day.AddDate(0, 0, i).Add(time.Duration(23*60+i) * time.Minute)})
		bulk = append(bulk, storage.Note{UpdatedAt: day.Add(time.Duration(i) * time.Second)})
	}
	if got := writingTime(late); got != "late at night" {
		t.Fatalf("late: %q", got)
	}
	if got := writingTime(bulk); got != "" {
		t.Fatalf("one sync isn't a habit: %q", got)
	}
}

func TestSpreadSpansTheRange(t *testing.T) {
	ns := make([]storage.Note, 10)
	for i := range ns {
		ns[i].Name = string(rune('a' + i))
	}
	got := spread(ns, 4, 1)
	if len(got) != 4 || got[0].Name != "b" || got[3].Name != "h" {
		t.Fatalf("%+v", got)
	}
}
