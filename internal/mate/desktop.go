package mate

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const desktopSchemaSQL = `
-- A stretch of the user at the computer with one app in front. Away is the gaps.
CREATE TABLE IF NOT EXISTS desktop_sessions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    app         TEXT    NOT NULL,
    started_at  INTEGER NOT NULL,
    ended_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS desktop_sessions_end_idx ON desktop_sessions(ended_at);
`

// desktopKeep is how long desktop sessions are kept.
const desktopKeep = 30 * 24 * time.Hour

// DesktopSession is a stretch of the user at the computer with one app in front.
type DesktopSession struct {
	App   string    `json:"app"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// TouchDesktop extends the latest session to at when it is the same app and
// ended within gap, or starts a new one.
func (s *Store) TouchDesktop(app string, at time.Time, gap time.Duration) error {
	var id, ended int64
	var lastApp string
	err := s.db.QueryRow(`SELECT id, app, ended_at FROM desktop_sessions ORDER BY ended_at DESC LIMIT 1`).
		Scan(&id, &lastApp, &ended)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mate: last desktop session: %w", err)
	}
	ms := at.UnixMilli()
	if err == nil && lastApp == app && ms-ended <= gap.Milliseconds() && ms >= ended {
		_, err = s.db.Exec(`UPDATE desktop_sessions SET ended_at = ? WHERE id = ?`, ms, id)
		return err
	}
	if _, err := s.db.Exec(`INSERT INTO desktop_sessions(app, started_at, ended_at) VALUES (?,?,?)`,
		app, ms, ms); err != nil {
		return fmt.Errorf("mate: start desktop session: %w", err)
	}
	_, err = s.db.Exec(`DELETE FROM desktop_sessions WHERE ended_at < ?`, at.Add(-desktopKeep).UnixMilli())
	return err
}

// DesktopSessions lists the sessions overlapping [from, to], oldest first.
func (s *Store) DesktopSessions(from, to time.Time) ([]DesktopSession, error) {
	rows, err := s.db.Query(`SELECT app, started_at, ended_at FROM desktop_sessions
		WHERE ended_at >= ? AND started_at <= ? ORDER BY started_at`, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("mate: desktop sessions: %w", err)
	}
	defer rows.Close()
	var out []DesktopSession
	for rows.Next() {
		var d DesktopSession
		var start, end int64
		if err := rows.Scan(&d.App, &start, &end); err != nil {
			return nil, err
		}
		d.Start, d.End = time.UnixMilli(start), time.UnixMilli(end)
		out = append(out, d)
	}
	return out, rows.Err()
}

// LastDesktop is when the user was last seen at the computer.
func (s *Store) LastDesktop() (time.Time, error) {
	var ms sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(ended_at) FROM desktop_sessions`).Scan(&ms); err != nil {
		return time.Time{}, fmt.Errorf("mate: last desktop: %w", err)
	}
	return fromMs(ms.Int64), nil
}

// ClearDesktop deletes every desktop session.
func (s *Store) ClearDesktop() error {
	_, err := s.db.Exec(`DELETE FROM desktop_sessions`)
	return err
}
