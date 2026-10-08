package inbox

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS inbox_messages (
    id          TEXT    PRIMARY KEY,
    source      TEXT    NOT NULL DEFAULT '',
    title       TEXT    NOT NULL DEFAULT '',
    body        TEXT    NOT NULL DEFAULT '',
    is_read     INTEGER NOT NULL DEFAULT 0,
    read_at     INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS inbox_messages_created_idx ON inbox_messages(created_at DESC);
CREATE INDEX IF NOT EXISTS inbox_messages_unread_idx  ON inbox_messages(is_read, created_at DESC);
`

const defaultListLimit = 50

// retentionWindow bounds how long a message stays in the inbox.
const retentionWindow = 30 * 24 * time.Hour

// Store shares the caller-provided *sql.DB (mate.db); Open only adds its own
// table. The caller owns the connection's lifecycle.
type Store struct {
	db        *sql.DB
	mu        sync.Mutex
	subs      map[chan Message]struct{}
	lastPrune atomic.Int64
}

// Open adds the inbox_messages table (if missing) to db and returns a Store.
// Databases created before the schema was trimmed keep their old level/link/
// metadata columns; those have defaults, so inserts that omit them still work.
func Open(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("inbox: schema: %w", err)
	}
	s := &Store{db: db, subs: make(map[chan Message]struct{})}
	s.pruneOld()
	return s, nil
}

// Create inserts a new inbox message and notifies SSE subscribers.
func (s *Store) Create(m Message) (Message, error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(
		`INSERT INTO inbox_messages(id, source, title, body, is_read, read_at, created_at)
		 VALUES (?,?,?,?,0,0,?)`,
		m.ID, m.Source, m.Title, m.Body, m.CreatedAt.UnixMilli(),
	)
	if err != nil {
		return Message{}, fmt.Errorf("inbox: create message: %w", err)
	}
	s.push(m)
	if time.Since(time.UnixMilli(s.lastPrune.Load())) > 24*time.Hour {
		go s.pruneOld()
	}
	return m, nil
}

// pruneOld is best-effort; a failure is retried on the next daily check.
func (s *Store) pruneOld() {
	s.lastPrune.Store(time.Now().UnixMilli())
	cutoff := time.Now().Add(-retentionWindow).UnixMilli()
	if _, err := s.db.Exec(`DELETE FROM inbox_messages WHERE created_at < ?`, cutoff); err != nil {
		slog.Warn("inbox: prune old messages", "err", err)
	}
}

// List returns messages newest-first, narrowed by f.
func (s *Store) List(f ListFilter) ([]Message, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	query := `SELECT id, source, title, body, is_read, created_at
	          FROM inbox_messages WHERE 1=1`
	var args []any
	if f.UnreadOnly {
		query += ` AND is_read = 0`
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, f.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("inbox: list: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

// UnreadCount returns the number of unread messages.
func (s *Store) UnreadCount() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM inbox_messages WHERE is_read = 0`).Scan(&n); err != nil {
		return 0, fmt.Errorf("inbox: unread count: %w", err)
	}
	return n, nil
}

// MarkRead marks a single message as read.
func (s *Store) MarkRead(id string) error {
	_, err := s.db.Exec(
		`UPDATE inbox_messages SET is_read = 1, read_at = ? WHERE id = ? AND is_read = 0`,
		time.Now().UnixMilli(), id,
	)
	if err != nil {
		return fmt.Errorf("inbox: mark read: %w", err)
	}
	return nil
}

// MarkAllRead marks every unread message as read.
func (s *Store) MarkAllRead() error {
	_, err := s.db.Exec(`UPDATE inbox_messages SET is_read = 1, read_at = ? WHERE is_read = 0`, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("inbox: mark all read: %w", err)
	}
	return nil
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	var out []Message
	for rows.Next() {
		var m Message
		var isRead int
		var createdAtMs int64
		if err := rows.Scan(&m.ID, &m.Source, &m.Title, &m.Body, &isRead, &createdAtMs); err != nil {
			return nil, fmt.Errorf("inbox: scan message: %w", err)
		}
		m.IsRead = isRead == 1
		m.CreatedAt = time.UnixMilli(createdAtMs)
		out = append(out, m)
	}
	return out, rows.Err()
}
