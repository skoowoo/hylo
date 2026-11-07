package storage

import (
	"testing"
	"time"
)

func TestOpenTasksInStaleNotes(t *testing.T) {
	v := newTestVault(t)
	now := time.Now()
	notes := map[Path]string{
		"/plans/trip.md":         "# Trip\n- [ ] book the train\n- [x] pack\n```\n- [ ] not a task\n```\n1. [ ] renew passport\n",
		"/plans/fresh.md":        "- [ ] written today\n",
		"/_knowledge/Travel.md":  "- [ ] compiled, not theirs\n",
		"/_shorts/2026-10-01.md": "- [ ] call mom\n",
		"/plans/done.md":         "- [x] all done\n",
	}
	for p, body := range notes {
		meta := Note{}
		if p.Dir() == "/_shorts" {
			meta.Kind = KindShort
		}
		if err := v.WriteNoteWithMeta(p, []byte(body), meta); err != nil {
			t.Fatal(err)
		}
		if p != "/plans/fresh.md" {
			if _, err := v.db.Exec(`UPDATE notes SET updated_at = ? WHERE dir = ? AND name = ?`, now.AddDate(0, 0, -20).UnixNano(), p.Dir(), p.Base()); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := v.OpenTasks(now.AddDate(0, -6, 0), now.AddDate(0, 0, -14))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"/plans/trip.md:book the train": 2, "/plans/trip.md:renew passport": 7, "/_shorts/2026-10-01.md:call mom": 1}
	if len(got) != len(want) {
		t.Fatalf("tasks = %+v", got)
	}
	for _, task := range got {
		if line, ok := want[task.Path+":"+task.Text]; !ok || line != task.Line {
			t.Errorf("unexpected task %+v", task)
		}
	}
}
