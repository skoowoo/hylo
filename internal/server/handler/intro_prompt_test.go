package handler

import (
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestIntroPromptQuotesTheGreeting(t *testing.T) {
	p := introPrompt("Your Q3 plan keeps asking who it's for.", "- 4 notes")
	if !strings.Contains(p, `"Your Q3 plan keeps asking who it's for."`) || !strings.Contains(p, "editor_type") {
		t.Fatalf("prompt misses the greeting or the editor: %s", p)
	}
	if strings.Contains(introPrompt("", ""), "already opened") {
		t.Fatal("no greeting, nothing to quote")
	}
}

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
	o, err := buildVaultOverview(v)
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
