package mate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// schemaSQL creates all tables for the mate package (mates, triggers, conversations, messages).
// Uses CREATE TABLE IF NOT EXISTS so it is safe to run on every startup.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS mates (
    id              TEXT    PRIMARY KEY,
    name            TEXT    NOT NULL DEFAULT '',
    description     TEXT    NOT NULL DEFAULT '',
    agent_id        TEXT    NOT NULL DEFAULT '',
    model           TEXT    NOT NULL DEFAULT '',
    color           TEXT    NOT NULL DEFAULT '',
    cwd             TEXT    NOT NULL DEFAULT '',
    main            INTEGER NOT NULL DEFAULT 0,
    system_prompt   TEXT    NOT NULL DEFAULT '',
    enabled         INTEGER NOT NULL DEFAULT 1,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS mate_triggers (
    id            TEXT    PRIMARY KEY,
    mate_id       TEXT    NOT NULL REFERENCES mates(id) ON DELETE CASCADE,
    event_types   TEXT    NOT NULL DEFAULT '',
    prompt        TEXT    NOT NULL DEFAULT '',
    schedule      TEXT    NOT NULL DEFAULT '',
    last_fired_at INTEGER NOT NULL DEFAULT 0,
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS conversations (
    id                     TEXT    PRIMARY KEY,
    mate_id                TEXT    NOT NULL DEFAULT '',
    title                  TEXT    NOT NULL DEFAULT '',
    type                   TEXT    NOT NULL DEFAULT 'chat',
    agent_session_id       TEXT    NOT NULL DEFAULT '',
    agent_session_agent_id TEXT    NOT NULL DEFAULT '',
    agent_session_cwd      TEXT    NOT NULL DEFAULT '',
    created_at             INTEGER NOT NULL,
    updated_at             INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id              TEXT    PRIMARY KEY,
    conversation_id TEXT    NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT    NOT NULL,
    content         TEXT    NOT NULL DEFAULT '',
    agent_id        TEXT    NOT NULL DEFAULT '',
    mate_id         TEXT    NOT NULL DEFAULT '',
    model_id        TEXT    NOT NULL DEFAULT '',
    run_id          TEXT    NOT NULL DEFAULT '',
    status          TEXT    NOT NULL DEFAULT '',
    trigger_event   TEXT    NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_message_note_access (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id  TEXT    NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    path        TEXT    NOT NULL,
    action      TEXT    NOT NULL,
    tool_name   TEXT    NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS mate_triggers_mate_idx  ON mate_triggers(mate_id);
-- A bot's folder, and the skills enabled there, belong to it alone.
CREATE UNIQUE INDEX IF NOT EXISTS mates_cwd_idx ON mates(cwd) WHERE cwd NOT IN ('', '/');
CREATE INDEX IF NOT EXISTS conversations_type_idx  ON conversations(type, updated_at);
CREATE INDEX IF NOT EXISTS conversations_mate_idx  ON conversations(mate_id, updated_at);
CREATE INDEX IF NOT EXISTS chat_messages_conv_idx  ON chat_messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS chat_message_note_access_msg_idx  ON chat_message_note_access(message_id);
CREATE INDEX IF NOT EXISTS chat_message_note_access_path_idx ON chat_message_note_access(path);

CREATE TABLE IF NOT EXISTS bot_channels (
    id            TEXT    PRIMARY KEY,
    mate_id       TEXT    NOT NULL REFERENCES mates(id) ON DELETE CASCADE,
    kind          TEXT    NOT NULL,
    token         TEXT    NOT NULL,
    owner_user_id TEXT    NOT NULL DEFAULT '',
    external_id   TEXT    NOT NULL DEFAULT '',
    display       TEXT    NOT NULL DEFAULT '',
    proxy_url     TEXT    NOT NULL DEFAULT '',
    base_url      TEXT    NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS bot_channels_mate_kind ON bot_channels(mate_id, kind);
CREATE UNIQUE INDEX IF NOT EXISTS bot_channels_identity ON bot_channels(kind, external_id) WHERE external_id != '';
`

// Store is the single data access point for both mate config and chat history.
// All data lives in <vaultRoot>/.hylo/mate.db.
type Store struct {
	db       *sql.DB
	onChange func()
}

// SetOnChange registers fn to run after any change to bots or their triggers.
// Set it before the store is in use.
func (s *Store) SetOnChange(fn func()) { s.onChange = fn }

// changed passes err through, telling the hook first when the change went in.
func (s *Store) changed(err error) error {
	if err == nil && s.onChange != nil {
		s.onChange()
	}
	return err
}

// Open opens (or creates) the combined database at <vaultRoot>/.hylo/mate.db.
func Open(vaultRoot string) (*Store, error) {
	dir := filepath.Join(vaultRoot, ".hylo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mate: create dir: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "mate.db"))
	if err != nil {
		return nil, fmt.Errorf("mate: open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: foreign keys: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: schema: %w", err)
	}
	for _, col := range []string{"path_prefixes", "name"} {
		if _, err := db.Exec(`ALTER TABLE mate_triggers ADD COLUMN ` + col + ` TEXT NOT NULL DEFAULT ''`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				db.Close()
				return nil, fmt.Errorf("mate: add %s column: %w", col, err)
			}
		}
	}
	for _, c := range []struct{ table, col, def string }{
		{"chat_messages", "trigger_id", "TEXT NOT NULL DEFAULT ''"},
		{"chat_messages", "trace", "TEXT NOT NULL DEFAULT ''"},
		{"conversations", "agent_session_cwd", "TEXT NOT NULL DEFAULT ''"},
		{"mates", "main", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if _, err := db.Exec(`ALTER TABLE ` + c.table + ` ADD COLUMN ` + c.col + ` ` + c.def); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				db.Close()
				return nil, fmt.Errorf("mate: add %s column: %w", c.col, err)
			}
		}
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS mates_main_idx ON mates(main) WHERE main = 1`); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: main index: %w", err)
	}
	return &Store{db: db}, nil
}

// DB exposes the underlying connection for read-only diagnostic use.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

// ── shared helpers ────────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
