package mate

import (
	"database/sql"
	"fmt"
	"slices"
	"time"
)

// ActorUser is note_activity's actor for the person; bots are their mate id.
const ActorUser = "user"

// RecordUserVisit logs the user reading (or, with NoteAccessWrite, editing) a
// note at at. Each report is its own row; a day folds them, write over read.
func (s *Store) RecordUserVisit(path, action string, at time.Time) error {
	if action != NoteAccessWrite {
		action = NoteAccessRead
	}
	if _, err := s.db.Exec(
		`INSERT INTO note_activity(path, actor, action, at) VALUES (?,?,?,?)`,
		path, ActorUser, action, at.UnixMilli(),
	); err != nil {
		return fmt.Errorf("mate: record user visit: %w", err)
	}
	return nil
}

// RenameNotePath moves every record of a note to its new path, so a renamed
// or moved note keeps its chat cards and its activity history.
func (s *Store) RenameNotePath(oldPath, newPath string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("mate: rename note path tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, table := range []string{"chat_message_note_access", "note_activity"} {
		if _, err := tx.Exec(`UPDATE `+table+` SET path = ? WHERE path = ?`, newPath, oldPath); err != nil {
			return fmt.Errorf("mate: rename note path in %s: %w", table, err)
		}
	}
	return tx.Commit()
}

// NoteActivity is one touch of a note.
type NoteActivity struct {
	Path   string
	Actor  string
	Action string
	At     time.Time
}

// ListNoteActivity returns every touch in [from, to).
func (s *Store) ListNoteActivity(from, to time.Time) ([]NoteActivity, error) {
	rows, err := s.db.Query(
		`SELECT path, actor, action, at FROM note_activity
		 WHERE at >= ? AND at < ? ORDER BY at`,
		from.UnixMilli(), to.UnixMilli(),
	)
	if err != nil {
		return nil, fmt.Errorf("mate: list note activity: %w", err)
	}
	defer rows.Close()
	var out []NoteActivity
	for rows.Next() {
		var a NoteActivity
		var at int64
		if err := rows.Scan(&a.Path, &a.Actor, &a.Action, &at); err != nil {
			return nil, err
		}
		a.At = time.UnixMilli(at)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ActivityCount counts distinct notes; a note both read and written that day
// counts as written.
type ActivityCount struct {
	Read  int
	Write int
}

func (c ActivityCount) Total() int { return c.Read + c.Write }

// ActivityDay is one local day of the heatmap.
type ActivityDay struct {
	Date time.Time // local midnight
	// A note touched by several actors counts once in All, and by several bots
	// once in Bots, as its strongest action.
	All   ActivityCount
	User  ActivityCount
	Bots  ActivityCount
	ByBot map[string]ActivityCount
}

// NoteDay is what happened to one note on one day: the user's action and each
// bot's ("" when it didn't touch the note), and when it was last touched.
type NoteDay struct {
	Path  string
	Last  time.Time
	User  string
	ByBot map[string]string
}

func midnight(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func strongest(cur, next string) string {
	if cur == NoteAccessWrite || next == NoteAccessWrite {
		return NoteAccessWrite
	}
	return NoteAccessRead
}

// NoteDays groups rows by local day and note. keep drops notes that no longer
// exist; a deleted note's history stays in the table but leaves the chart.
func NoteDays(rows []NoteActivity, keep func(path string) bool) map[time.Time]map[string]*NoteDay {
	out := map[time.Time]map[string]*NoteDay{}
	for _, r := range rows {
		if !keep(r.Path) {
			continue
		}
		day := midnight(r.At)
		notes := out[day]
		if notes == nil {
			notes = map[string]*NoteDay{}
			out[day] = notes
		}
		n := notes[r.Path]
		if n == nil {
			n = &NoteDay{Path: r.Path, ByBot: map[string]string{}}
			notes[r.Path] = n
		}
		if r.At.After(n.Last) {
			n.Last = r.At
		}
		if r.Actor == ActorUser {
			n.User = strongest(n.User, r.Action)
		} else {
			n.ByBot[r.Actor] = strongest(n.ByBot[r.Actor], r.Action)
		}
	}
	return out
}

// ActivityDays summarizes each local day from from's day through to's, empty
// days included.
func ActivityDays(byDay map[time.Time]map[string]*NoteDay, from, to time.Time) []ActivityDay {
	var out []ActivityDay
	for day := midnight(from); !day.After(to); day = day.AddDate(0, 0, 1) {
		d := ActivityDay{Date: day, ByBot: map[string]ActivityCount{}}
		for _, n := range byDay[day] {
			count(&d.User, n.User)
			bots := ""
			for bot, action := range n.ByBot {
				c := d.ByBot[bot]
				count(&c, action)
				d.ByBot[bot] = c
				bots = strongest(bots, action)
			}
			count(&d.Bots, bots)
			all := bots
			if n.User != "" {
				all = strongest(all, n.User)
			}
			count(&d.All, all)
		}
		out = append(out, d)
	}
	return out
}

func count(c *ActivityCount, action string) {
	switch action {
	case NoteAccessWrite:
		c.Write++
	case NoteAccessRead:
		c.Read++
	}
}

// NotesOn lists the notes touched on day, most recently touched first.
func NotesOn(byDay map[time.Time]map[string]*NoteDay, day time.Time) []NoteDay {
	var out []NoteDay
	for _, n := range byDay[midnight(day)] {
		out = append(out, *n)
	}
	slices.SortFunc(out, func(a, b NoteDay) int { return b.Last.Compare(a.Last) })
	return out
}

// FirstNoteActivity returns when activity was first recorded; ok is false
// before anything has been.
func (s *Store) FirstNoteActivity() (first time.Time, ok bool, err error) {
	var ms sql.NullInt64
	if err := s.db.QueryRow(`SELECT MIN(at) FROM note_activity`).Scan(&ms); err != nil {
		return time.Time{}, false, fmt.Errorf("mate: first note activity: %w", err)
	}
	if !ms.Valid {
		return time.Time{}, false, nil
	}
	return time.UnixMilli(ms.Int64), true, nil
}
