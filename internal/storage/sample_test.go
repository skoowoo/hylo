package storage

import (
	"math/rand/v2"
	"testing"
	"time"
)

func TestSampleSpreadsHistoryAndDomains(t *testing.T) {
	v := newTestVault(t)
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.Local)
	write := func(p Path, body string, kind Kind, title string) {
		t.Helper()
		if err := v.WriteNoteWithMeta(p, []byte(body), Note{Kind: kind, Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	backdate := func(p Path, at time.Time) {
		t.Helper()
		if _, err := v.db.Exec(`UPDATE notes SET created_at = ? WHERE dir = ? AND name = ?`, at.UnixNano(), p.Dir(), p.Base()); err != nil {
			t.Fatal(err)
		}
	}

	early := Path("/journal/early.md")
	late := Path("/journal/late.md")
	recent := Path("/journal/recent.md")
	memory := Path("/_memory/_state.md")
	source := Path("/journal/river.md")
	rivers := Path("/_knowledge/Rivers.md")
	cooking := Path("/_knowledge/Cooking.md")
	geo := Path("/_knowledge/_indexes/Geography.md")
	cookIdx := Path("/_knowledge/_indexes/Cooking.md")

	write(early, "a note from the early years\n", "", "")
	backdate(early, time.Date(2018, 2, 1, 0, 0, 0, 0, time.Local))
	write(late, "a note from later on\n", "", "")
	backdate(late, time.Date(2023, 8, 1, 0, 0, 0, 0, time.Local))
	write(recent, "written this week\n", "", "")
	backdate(recent, now.AddDate(0, 0, -4))
	write(memory, "system memory\n", "", "")
	backdate(memory, time.Date(2018, 1, 1, 0, 0, 0, 0, time.Local))

	write(source, "the river in 2018\n", "", "")
	backdate(source, time.Date(2018, 1, 15, 0, 0, 0, 0, time.Local))
	write(rivers, "---\nkind: knowledge\nperspective: own\n---\ncompiled rivers\n", KindKnowledge, "Rivers")
	if err := v.SetKnowledgeDeps(rivers, []Path{source}); err != nil {
		t.Fatal(err)
	}
	write(geo, "# Geography\n", KindIndex, "Geography")
	if err := v.SetIndexDeps(geo, []Path{rivers}); err != nil {
		t.Fatal(err)
	}
	write(cooking, "old recipes\n", KindKnowledge, "Cooking")
	backdate(cooking, time.Date(2019, 5, 1, 0, 0, 0, 0, time.Local))
	write(cookIdx, "# Cooking\n", KindIndex, "Cooking")
	if err := v.SetIndexDeps(cookIdx, []Path{cooking}); err != nil {
		t.Fatal(err)
	}

	oldShort := "---\nkind: short\n---\n###### Short Note: 2019-06-01 08:30:00\n\nremember the river\n"
	write(Path("/_shorts/2019-06-01.md"), oldShort, KindShort, "")
	newShort := "---\nkind: short\n---\n###### Short Note: 2026-11-01 09:00:00\n\njust now\n"
	write(Path("/_shorts/2026-11-01.md"), newShort, KindShort, "")

	items, err := v.Sample(SampleOptions{
		Each: 2,
		Now:  now,
		Rand: rand.New(rand.NewPCG(1, 2)),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]SampleItem{}
	var earlyRaw, lateRaw int
	for _, it := range items {
		got[it.Path] = it
		if it.Kind != "raw" {
			continue
		}
		switch it.CreatedAt.Year() {
		case 2018:
			earlyRaw++
		case 2023:
			lateRaw++
		default:
			t.Errorf("raw note %s dated %s", it.Path, it.CreatedAt.Format(time.DateOnly))
		}
	}
	if earlyRaw != 1 || lateRaw != 1 {
		t.Errorf("raw spread = %d early, %d late; got %v", earlyRaw, lateRaw, paths(items))
	}
	if _, ok := got[recent.String()]; ok {
		t.Error("recent note was drawn even though older notes exist")
	}
	if _, ok := got[memory.String()]; ok {
		t.Error("system note was drawn")
	}
	if _, ok := got[geo.String()]; ok {
		t.Error("index file was drawn")
	}
	r, ok := got[rivers.String()]
	if !ok || r.Domain != "Geography" || r.CreatedAt.Year() != 2018 || r.Perspective != "own" {
		t.Errorf("rivers = %+v, want own, Geography, dated 2018", r)
	}
	c, ok := got[cooking.String()]
	if !ok || c.Domain != "Cooking" || c.CreatedAt.Year() != 2019 {
		t.Errorf("cooking = %+v, want Cooking dated 2019", c)
	}
	s, ok := got["/_shorts/2019-06-01.md"]
	if !ok || s.Kind != "short" || s.ID != "2019-06-01-083000" || s.Excerpt != "remember the river" {
		t.Errorf("short = %+v", s)
	}
	if _, ok := got["/_shorts/2026-11-01.md"]; ok {
		t.Error("recent short was drawn")
	}
}

func TestSampleFallsBackWhenNothingIsOld(t *testing.T) {
	v := newTestVault(t)
	if err := v.WriteNote(Path("/only.md"), []byte("brand new\n"), ""); err != nil {
		t.Fatal(err)
	}
	items, err := v.Sample(SampleOptions{Each: 2, OlderThan: 90, Rand: rand.New(rand.NewPCG(1, 2))})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Path != "/only.md" {
		t.Fatalf("items = %+v, want the only note", items)
	}
}

func TestSampleEmpty(t *testing.T) {
	v := newTestVault(t)
	items, err := v.Sample(SampleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, want an empty slice", items)
	}
}

func paths(items []SampleItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Path
	}
	return out
}
