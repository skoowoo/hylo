package storage

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

const shortDayFixture = `---
kind: short
date: 2026-10-29
---

###### Short Note: 2026-10-29 09:00:00

first

---

hand-written, no heading

***

###### Short Note: 2026-10-29 09:00:00

same second

---

###### Short Note: 2026-10-29 10:30:00

last
`

func writeShortFixture(t *testing.T, v *Vault) Path {
	t.Helper()
	p := Path(ShortDir + "/2026-10-29.md")
	if err := v.WriteNoteWithMeta(p, []byte(shortDayFixture), Note{Kind: KindShort}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseShortDayIDs(t *testing.T) {
	p := Path(ShortDir + "/2026-10-29.md")
	var ids []string
	for _, s := range parseShortDay([]byte(shortDayFixture), p) {
		ids = append(ids, s.ID)
	}
	want := []string{"2026-10-29-090000", "", "2026-10-29-090000-2", "2026-10-29-103000"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %q, want %q", ids, want)
	}
	if got, _ := shortPathFromID("2026-10-29-090000-2"); got != p {
		t.Fatalf("path from id = %q", got)
	}
}

func TestUpdateShortSplicesOnlyItsEntry(t *testing.T) {
	v := newTestVault(t)
	p := writeShortFixture(t, v)

	e, err := v.UpdateShort("2026-10-29-090000-2", "", []byte("edited\n---\nstill one entry"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Content != "edited\n\nstill one entry" {
		t.Fatalf("content = %q", e.Content)
	}
	raw, _ := v.ReadNote(p)
	want := strings.Replace(shortDayFixture, "same second", "edited\n\nstill one entry", 1)
	if string(raw) != want {
		t.Fatalf("file =\n%s\nwant\n%s", raw, want)
	}
}

func TestUpdateShortHashConflict(t *testing.T) {
	v := newTestVault(t)
	writeShortFixture(t, v)
	if _, err := v.UpdateShort("2026-10-29-103000", "stale", []byte("x")); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	e, _ := v.GetShort("2026-10-29-103000")
	if _, err := v.UpdateShort(e.ID, e.Hash, []byte("x")); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteShortKeepsNeighbours(t *testing.T) {
	v := newTestVault(t)
	p := writeShortFixture(t, v)

	for _, id := range []string{"2026-10-29-090000", "2026-10-29-103000"} {
		if err := v.DeleteShort(id, ""); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := v.ReadNote(p)
	want := "---\nkind: short\ndate: 2026-10-29\n---\n\nhand-written, no heading\n\n***\n\n###### Short Note: 2026-10-29 09:00:00\n\nsame second\n"
	if string(raw) != want {
		t.Fatalf("file =\n%q\nwant\n%q", raw, want)
	}
	if _, err := v.GetShort("2026-10-29-103000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted entry still found: %v", err)
	}
}

func TestDeleteLastShortRemovesFile(t *testing.T) {
	v := newTestVault(t)
	e, err := v.AppendShort([]byte("only one"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.DeleteShort(e.ID, e.Hash); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadNote(Path(e.Path)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("daily file still there: %v", err)
	}
}

func TestAppendShortUniqueIDs(t *testing.T) {
	v := newTestVault(t)
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		e, err := v.AppendShort([]byte("note"))
		if err != nil {
			t.Fatal(err)
		}
		if seen[e.ID] {
			t.Fatalf("duplicate id %q", e.ID)
		}
		seen[e.ID] = true
	}
	list, err := v.ListShortEntries(ShortListOptions{})
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %d entries, err = %v", len(list), err)
	}
	if _, err := v.AppendShort([]byte(" \n---\n ")); !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty append err = %v", err)
	}
}

func TestAppendShortKeepsHandEditedFrontmatter(t *testing.T) {
	v := newTestVault(t)
	day := shortDayPath(time.Now())
	if err := v.WriteNoteWithMeta(day, []byte("---\ntags: [x]\n---\n\nhand-written\n"), Note{Kind: KindShort}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.AppendShort([]byte("new")); err != nil {
		t.Fatal(err)
	}
	raw, _ := v.ReadNote(day)
	if !strings.HasPrefix(string(raw), "---\nkind: short\ntags: [x]\n---\n\nhand-written\n\n---\n\n") || strings.Count(string(raw), "kind:") != 1 {
		t.Fatalf("file =\n%s", raw)
	}
}

func TestShortWritesNameTheirEntry(t *testing.T) {
	v := newTestVault(t)
	var got []plugin.Event
	v.SetEventHook(func(e plugin.Event) {
		if e.Type != plugin.EventVaultShortAppend {
			got = append(got, e)
		}
	})
	e, err := v.AppendShort([]byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := v.UpdateShort(e.ID, e.Hash, []byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.DeleteShort(e.ID, u.Hash); err != nil {
		t.Fatal(err)
	}
	want := []plugin.Event{
		{Type: plugin.EventVaultCreate, ShortID: e.ID, ShortHash: e.Hash},
		{Type: plugin.EventVaultWrite, ShortID: e.ID, ShortHash: u.Hash},
		{Type: plugin.EventVaultDelete, ShortID: e.ID},
	}
	if len(got) != len(want) {
		t.Fatalf("events = %+v", got)
	}
	for i, w := range want {
		if got[i].Type != w.Type || got[i].ShortID != w.ShortID || got[i].ShortHash != w.ShortHash {
			t.Errorf("event %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestShortTags(t *testing.T) {
	v := newTestVault(t)
	days := map[string]string{
		"2026-10-28": "---\nkind: short\n---\n\n###### Short Note: 2026-10-28 09:00:00\n\nno tags\n",
		"2026-10-29": "---\nkind: short\n---\n\n###### Short Note: 2026-10-29 09:00:00\n\n#读书 one\n\n---\n\n###### Short Note: 2026-10-29 10:00:00\n\n#Idea and #读书\n\n---\n\n###### Short Note: 2026-10-29 11:00:00\n\nplain\n",
		"2026-10-30": "---\nkind: short\ntags: [day]\n---\n\n###### Short Note: 2026-10-30 09:00:00\n\n#idea\n",
	}
	for day, body := range days {
		if err := v.WriteNote(Path(ShortDir+"/"+day+".md"), []byte(body), KindShort); err != nil {
			t.Fatal(err)
		}
	}

	counts, err := v.ShortTagCounts()
	if err != nil {
		t.Fatal(err)
	}
	// By day, not by short; a day file's own front matter tags count too.
	want := []ShortTagCount{{"idea", 2}, {"day", 1}, {"读书", 1}}
	if !slices.Equal(counts, want) {
		t.Errorf("counts = %v, want %v", counts, want)
	}

	entries, err := v.ListShortEntries(ShortListOptions{Tag: "读书"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	if strings.Join(ids, ",") != "2026-10-29-100000,2026-10-29-090000" {
		t.Errorf("ids = %v", ids)
	}
	if entries, _ := v.ListShortEntries(ShortListOptions{Tag: "day"}); len(entries) != 0 {
		t.Errorf("front matter tag matched entries: %v", entries)
	}
}
