package mate

import (
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestNoteActivityPointsCollapsesIntervals(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "mate.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE note_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL, actor TEXT NOT NULL, action TEXT NOT NULL,
		started_at INTEGER NOT NULL, ended_at INTEGER NOT NULL, ref TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO note_activity(path, actor, action, started_at, ended_at, ref) VALUES ('/a.md','user','write',10,90,'msg')`); err != nil {
		t.Fatal(err)
	}
	if err := noteActivityPoints(db); err != nil {
		t.Fatal(err)
	}
	if err := noteActivityPoints(db); err != nil {
		t.Fatal(err)
	}
	var path, action string
	var at int64
	if err := db.QueryRow(`SELECT path, action, at FROM note_activity`).Scan(&path, &action, &at); err != nil {
		t.Fatal(err)
	}
	if path != "/a.md" || action != NoteAccessWrite || at != 90 {
		t.Fatalf("row = %s %s %d", path, action, at)
	}
}

func TestRecordUserVisitStoresEachReport(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	at := func(min int) time.Time {
		return time.UnixMilli(1_700_000_000_000).Add(time.Duration(min) * time.Minute)
	}
	for _, v := range []struct {
		path, action string
		min          int
	}{
		{"/a.md", NoteAccessRead, 0},
		{"/a.md", NoteAccessRead, 5}, // a later report of the same note
		{"/a.md", NoteAccessWrite, 20},
		{"/a.md", NoteAccessRead, 60},
		{"/b.md", NoteAccessRead, 2},
	} {
		if err := store.RecordUserVisit(v.path, v.action, at(v.min)); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := store.db.Query(`SELECT path, action, at FROM note_activity WHERE actor = ? ORDER BY path, at`, ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		path, action string
		at           int64
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.path, &r.action, &r.at); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	ms := func(min int) int64 { return at(min).UnixMilli() }
	want := []row{
		{"/a.md", NoteAccessRead, ms(0)},
		{"/a.md", NoteAccessRead, ms(5)},
		{"/a.md", NoteAccessWrite, ms(20)},
		{"/a.md", NoteAccessRead, ms(60)},
		{"/b.md", NoteAccessRead, ms(2)},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("visits = %+v, want %+v", got, want)
	}
}

func TestActivityDaysCountsDistinctNotesPerLocalDay(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	day := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }
	add := func(path, actor, action string, at time.Time) {
		if _, err := store.db.Exec(`INSERT INTO note_activity(path, actor, action, at) VALUES (?,?,?,?)`,
			path, actor, action, at.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	add("/a.md", ActorUser, NoteAccessRead, day(10, 9))
	add("/a.md", ActorUser, NoteAccessWrite, day(10, 11)) // write wins
	add("/a.md", "mate-1", NoteAccessRead, day(10, 13))
	add("/b.md", "mate-1", NoteAccessWrite, day(10, 14))
	add("/c.md", ActorUser, NoteAccessRead, day(10, 23))
	add("/c.md", ActorUser, NoteAccessRead, day(11, 1)) // a later report, its own day
	add("/gone.md", ActorUser, NoteAccessWrite, day(10, 8))
	add("/old.md", ActorUser, NoteAccessRead, day(1, 8)) // before the range

	rows, err := store.ListNoteActivity(day(9, 0), day(13, 0))
	if err != nil {
		t.Fatal(err)
	}
	byDay := NoteDays(rows, func(p string) bool { return p != "/gone.md" })
	days := ActivityDays(byDay, day(9, 0), day(12, 0))
	if len(days) != 4 || !days[0].Date.Equal(day(9, 0)) {
		t.Fatalf("days = %+v", days)
	}
	if d := days[0]; d.All.Total() != 0 {
		t.Fatalf("empty day = %+v", d)
	}
	d := days[1]
	if d.All != (ActivityCount{Read: 1, Write: 2}) || d.User != (ActivityCount{Read: 1, Write: 1}) || d.Bots != (ActivityCount{Read: 1, Write: 1}) ||
		d.ByBot["mate-1"] != (ActivityCount{Read: 1, Write: 1}) {
		t.Fatalf("Oct 10 = %+v", d)
	}
	if d := days[2]; d.All != (ActivityCount{Read: 1}) || d.User != (ActivityCount{Read: 1}) {
		t.Fatalf("Oct 11 = %+v", d)
	}

	notes := NotesOn(byDay, day(10, 0))
	var order []string
	for _, n := range notes {
		order = append(order, n.Path)
	}
	if want := []string{"/c.md", "/b.md", "/a.md"}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if a := notes[2]; a.User != NoteAccessWrite || a.ByBot["mate-1"] != NoteAccessRead {
		t.Fatalf("/a.md = %+v", a)
	}
	if c := notes[0]; !c.Last.Equal(day(10, 23)) {
		t.Fatalf("/c.md last on Oct 10 = %v", c.Last)
	}
}
