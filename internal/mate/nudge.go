package mate

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Nudge is one time a round raised a nudge (internal/pulse).
type Nudge struct {
	At    time.Time
	Items []string // what the raise covered, for nudges that name things
}

// Nudges lists the raises of nudge name since t that reached the user or may
// still, oldest first.
func (s *Store) Nudges(mateID, name string, since time.Time) ([]Nudge, error) {
	rows, err := s.db.Query(`SELECT at, nudged FROM pulse_runs
		WHERE mate_id = ? AND at >= ? AND nudged != '' AND delivery IN ('', 'sent')
		  AND EXISTS (SELECT 1 FROM json_each(nudged) WHERE key = ?)
		ORDER BY at`, mateID, since.UnixMilli(), name)
	if err != nil {
		return nil, fmt.Errorf("mate: nudges: %w", err)
	}
	defer rows.Close()
	var out []Nudge
	for rows.Next() {
		var at int64
		var nudged string
		if err := rows.Scan(&at, &nudged); err != nil {
			return nil, err
		}
		var m map[string][]string
		_ = json.Unmarshal([]byte(nudged), &m)
		out = append(out, Nudge{At: fromMs(at), Items: m[name]})
	}
	return out, rows.Err()
}

// NudgedSince reports whether any nudge went out, or is on its way, since t.
func (s *Store) NudgedSince(mateID string, since time.Time) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pulse_runs WHERE mate_id = ? AND at >= ? AND nudged != '' AND delivery IN ('', 'sent')`,
		mateID, since.UnixMilli()).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("mate: nudged since: %w", err)
	}
	return n > 0, nil
}

// A bot's own folders are its doing, not the user's notes.
const ownFolders = `path NOT LIKE '/\_bots/%' ESCAPE '\' AND path NOT LIKE '/\_memory/%' ESCAPE '\'`

// WriteTimes lists when the user wrote since t: by hand, or through a bot
// they asked. A bot's writes on its own don't count.
func (s *Store) WriteTimes(since time.Time) ([]time.Time, error) {
	return s.times(`SELECT at FROM note_activity WHERE actor = 'user' AND action = 'write' AND at >= ?1
		UNION ALL
		SELECT a.created_at FROM chat_message_note_access a JOIN chat_messages m ON m.id = a.message_id
		WHERE a.action = 'write' AND m.trigger_event = '' AND a.created_at >= ?1 AND a.`+ownFolders, since)
}

// ReadTimes lists when the user read since t: themselves, or through a bot
// they asked (note_activity keeps no bot reads from wakes or skims).
func (s *Store) ReadTimes(since time.Time) ([]time.Time, error) {
	return s.times(`SELECT at FROM note_activity WHERE action = 'read' AND at >= ?1 AND `+ownFolders, since)
}

// AroundDays lists the local days since t the user was around: at the
// computer, or doing anything in hylo.
func (s *Store) AroundDays(since time.Time) (map[string]bool, error) {
	ts, err := s.times(`SELECT started_at FROM desktop_sessions WHERE ended_at >= ?1
		UNION ALL SELECT ended_at FROM desktop_sessions WHERE ended_at >= ?1
		UNION ALL SELECT at FROM note_activity WHERE actor = 'user' AND at >= ?1
		UNION ALL SELECT created_at FROM chat_messages WHERE role = 'user' AND created_at >= ?1`, since)
	if err != nil {
		return nil, err
	}
	days := map[string]bool{}
	for _, t := range ts {
		if !t.Before(since) {
			days[t.Format(time.DateOnly)] = true
		}
	}
	return days, nil
}

func (s *Store) times(query string, since time.Time) ([]time.Time, error) {
	rows, err := s.db.Query(query, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("mate: times: %w", err)
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var ms sql.NullInt64
		if err := rows.Scan(&ms); err != nil {
			return nil, err
		}
		out = append(out, time.UnixMilli(ms.Int64))
	}
	return out, rows.Err()
}
