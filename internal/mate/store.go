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
    -- Unused: chats are titled by their first message (ListConversations).
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

-- One touch of a note. A day folds its rows; a write counts over a read.
-- Outlives the chat that caused it.
CREATE TABLE IF NOT EXISTS note_activity (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    path        TEXT    NOT NULL,
    actor       TEXT    NOT NULL,
    action      TEXT    NOT NULL,
    at          INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS mate_triggers_mate_idx  ON mate_triggers(mate_id);
-- A bot's folder, and the skills enabled there, belong to it alone.
CREATE UNIQUE INDEX IF NOT EXISTS mates_cwd_idx ON mates(cwd) WHERE cwd NOT IN ('', '/');
CREATE INDEX IF NOT EXISTS conversations_type_idx  ON conversations(type, updated_at);
CREATE INDEX IF NOT EXISTS conversations_mate_idx  ON conversations(mate_id, updated_at);
CREATE INDEX IF NOT EXISTS chat_messages_conv_idx  ON chat_messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS chat_message_note_access_msg_idx  ON chat_message_note_access(message_id);
CREATE INDEX IF NOT EXISTS chat_message_note_access_path_idx ON chat_message_note_access(path);
CREATE INDEX IF NOT EXISTS note_activity_at_idx ON note_activity(at);
CREATE INDEX IF NOT EXISTS note_activity_path_idx ON note_activity(path);

CREATE TABLE IF NOT EXISTS bot_channels (
    id            TEXT    PRIMARY KEY,
    mate_id       TEXT    NOT NULL REFERENCES mates(id) ON DELETE CASCADE,
    kind          TEXT    NOT NULL,
    token         TEXT    NOT NULL,
    owner_user_id TEXT    NOT NULL DEFAULT '',
    external_id   TEXT    NOT NULL DEFAULT '',
    display       TEXT    NOT NULL DEFAULT '',
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
	onChat   func(ids []string)
}

// SetOnChange registers fn to run after any change to bots or their triggers.
// Set it before the store is in use.
func (s *Store) SetOnChange(fn func()) { s.onChange = fn }

// SetOnChat registers fn to run after the transcript changes, with the ids of
// the messages written or deleted, or none when a chat was opened or removed.
// Bot config goes through SetOnChange. Set it before the store is in use. fn
// runs inline and must not call back into the store.
func (s *Store) SetOnChat(fn func(ids []string)) { s.onChat = fn }

// changed passes err through, telling the hook first when the change went in.
func (s *Store) changed(err error) error {
	if err == nil && s.onChange != nil {
		s.onChange()
	}
	return err
}

func (s *Store) chatChanged(ids ...string) {
	if s.onChat != nil {
		s.onChat(ids)
	}
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
	// Activity briefly stored a visit as an interval. Charts only read a point,
	// so keep the end and drop the span.
	if err := noteActivityPoints(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: note activity: %w", err)
	}
	if _, err := db.Exec(schemaSQL + pulseSchemaSQL + desktopSchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: schema: %w", err)
	}
	// proxy_url was a per-channel proxy. Outbound proxy is server.proxy now.
	if _, err := db.Exec(`ALTER TABLE bot_channels DROP COLUMN proxy_url`); err != nil && !strings.Contains(err.Error(), "no such column") {
		db.Close()
		return nil, fmt.Errorf("mate: drop proxy_url: %w", err)
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
		{"chat_messages", "attachments", "TEXT NOT NULL DEFAULT ''"},
		{"chat_messages", "app", "TEXT NOT NULL DEFAULT ''"},
		{"conversations", "agent_session_cwd", "TEXT NOT NULL DEFAULT ''"},
		{"mates", "main", "INTEGER NOT NULL DEFAULT 0"},
		{"mates", "seen_at", "INTEGER NOT NULL DEFAULT 0"},
		{"pulse_runs", "nudged", "TEXT NOT NULL DEFAULT ''"},
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
	// Pulse reads a bot's recent wake replies.
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS chat_messages_wake_idx ON chat_messages(mate_id, updated_at) WHERE role = 'assistant' AND trigger_event != ''`); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: wake index: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS chat_messages_reply_idx ON chat_messages(mate_id, updated_at) WHERE role = 'assistant'`); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: reply index: %w", err)
	}
	// Whether a bot is at work (ListMates): only the few replies still on their way.
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS chat_messages_running_idx ON chat_messages(mate_id) WHERE status = 'running'`); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: running index: %w", err)
	}
	// A bot's latest messages (ListMates, pulse) are read on every pacing minute.
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS chat_messages_mate_idx ON chat_messages(mate_id, role, created_at)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: chat mate index: %w", err)
	}
	// Trigger runs used to land in their own conversation and an inbox; they are chat turns now.
	for _, q := range []string{
		`DELETE FROM conversations WHERE type = 'trigger'`,
		`DELETE FROM mate_triggers WHERE event_types LIKE '%agent_run_completed%'`,
		`DROP TABLE IF EXISTS inbox_messages`,
	} {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, fmt.Errorf("mate: drop trigger runs: %w", err)
		}
	}
	if err := activeConversations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("mate: active conversations: %w", err)
	}
	return &Store{db: db}, nil
}

// activeConversations adds each bot's current-chat pointer. It used to be
// whichever chat changed last, so that one becomes current, and the empty
// chats a new chat used to leave behind go.
func activeConversations(db *sql.DB) error {
	if _, err := db.Exec(`ALTER TABLE mates ADD COLUMN active_conversation_id TEXT NOT NULL DEFAULT ''`); err != nil {
		if strings.Contains(err.Error(), "duplicate column name") {
			return nil
		}
		return err
	}
	_, err := db.Exec(`
		DELETE FROM conversations WHERE NOT EXISTS (SELECT 1 FROM chat_messages WHERE conversation_id = conversations.id);
		UPDATE mates SET active_conversation_id = COALESCE(
			(SELECT id FROM conversations WHERE mate_id = mates.id ORDER BY updated_at DESC LIMIT 1), '');
	`)
	return err
}

// noteActivityPoints collapses the unreleased interval rows (started_at, ended_at)
// into the touch at which they ended. A missing table, or one already on `at`, is left alone.
func noteActivityPoints(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('note_activity') WHERE name = 'started_at'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`
		CREATE TABLE note_activity_new (
			id     INTEGER PRIMARY KEY AUTOINCREMENT,
			path   TEXT    NOT NULL,
			actor  TEXT    NOT NULL,
			action TEXT    NOT NULL,
			at     INTEGER NOT NULL
		);
		INSERT INTO note_activity_new(path, actor, action, at)
			SELECT path, actor, action, ended_at FROM note_activity;
		DROP TABLE note_activity;
		ALTER TABLE note_activity_new RENAME TO note_activity;
	`); err != nil {
		return err
	}
	return tx.Commit()
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
