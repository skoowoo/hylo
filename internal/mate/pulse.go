package mate

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MateEventPulse marks a wake a bot's own rounds started (internal/pulse).
// Not a trigger event: no trigger can be set on it.
const MateEventPulse MateEventType = "pulse"

const pulseSchemaSQL = `
-- What a bot means to do or follow up on; a due item wakes it once.
CREATE TABLE IF NOT EXISTS bot_agenda (
    id          TEXT    PRIMARY KEY,
    mate_id     TEXT    NOT NULL REFERENCES mates(id) ON DELETE CASCADE,
    text        TEXT    NOT NULL,
    due_at      INTEGER NOT NULL DEFAULT 0,
    fired_at    INTEGER NOT NULL DEFAULT 0,
    done_at     INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS bot_agenda_mate_idx ON bot_agenda(mate_id, done_at);

-- One of a bot's own rounds: why it woke, what it decided, how it ended.
CREATE TABLE IF NOT EXISTS pulse_runs (
    id          TEXT    PRIMARY KEY,
    mate_id     TEXT    NOT NULL REFERENCES mates(id) ON DELETE CASCADE,
    at          INTEGER NOT NULL,
    ended_at    INTEGER NOT NULL DEFAULT 0,
    why         TEXT    NOT NULL DEFAULT '',
    decision    TEXT    NOT NULL DEFAULT '',
    summary     TEXT    NOT NULL DEFAULT '',
    brief       TEXT    NOT NULL DEFAULT '',
    wrote       TEXT    NOT NULL DEFAULT '',
    delivery    TEXT    NOT NULL DEFAULT '',
    delivered_at INTEGER NOT NULL DEFAULT 0,
    run_id      TEXT    NOT NULL DEFAULT '',
    nudged      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS pulse_runs_mate_idx ON pulse_runs(mate_id, at);
`

// AgendaItem is something a bot means to do or follow up on.
type AgendaItem struct {
	ID      string    `json:"id"`
	MateID  string    `json:"mateId"`
	Text    string    `json:"text"`
	Due     time.Time `json:"due,omitzero"`
	FiredAt time.Time `json:"firedAt,omitzero"` // when a round first saw it due
	Created time.Time `json:"created"`
}

// ErrAgendaItemNotFound is a finish of an item the bot doesn't have open.
var ErrAgendaItemNotFound = errors.New("no open agenda item with that id")

// Pulse decisions.
const (
	PulseSilent = "silent" // looked, nothing worth saying
	PulseSpeak  = "speak"  // handed a brief to the chat
	PulseFailed = "failed"
)

// PulseRun is one of a bot's own rounds.
type PulseRun struct {
	ID       string    `json:"id"`
	MateID   string    `json:"mateId"`
	At       time.Time `json:"at"`
	EndedAt  time.Time `json:"endedAt,omitzero"`
	Why      string    `json:"why"`
	Decision string    `json:"decision"`
	Summary  string    `json:"summary"`
	Brief    string    `json:"brief"`
	Wrote    []string  `json:"wrote,omitempty"` // notes the round wrote, though it shouldn't
	// Nudged names the nudges its brief raises, each with the items it covered.
	Nudged map[string][]string `json:"nudged,omitempty"`
	// Delivery is how the chat turn the brief started ended: sent, silent or failed.
	Delivery string `json:"delivery"`
	RunID    string `json:"runId"`
}

// AddAgenda adds an open item; a zero due has no time.
func (s *Store) AddAgenda(mateID, text string, due time.Time) (AgendaItem, error) {
	it := AgendaItem{ID: uuid.NewString(), MateID: mateID, Text: strings.TrimSpace(text), Due: due, Created: time.Now()}
	if it.Text == "" {
		return AgendaItem{}, errors.New("agenda item text is empty")
	}
	_, err := s.db.Exec(`INSERT INTO bot_agenda(id, mate_id, text, due_at, created_at) VALUES (?,?,?,?,?)`,
		it.ID, mateID, it.Text, unixMs(due), it.Created.UnixMilli())
	if err != nil {
		return AgendaItem{}, fmt.Errorf("mate: add agenda: %w", err)
	}
	return it, nil
}

// OpenAgenda lists a bot's open items, timed ones first by due time.
func (s *Store) OpenAgenda(mateID string) ([]AgendaItem, error) {
	return s.queryAgenda(`WHERE mate_id = ? AND done_at = 0 ORDER BY due_at = 0, due_at, created_at`, mateID)
}

// DueAgenda lists open items due by now that no round has seen yet.
func (s *Store) DueAgenda(mateID string, now time.Time) ([]AgendaItem, error) {
	return s.queryAgenda(`WHERE mate_id = ? AND done_at = 0 AND fired_at = 0 AND due_at > 0 AND due_at <= ? ORDER BY due_at`, mateID, now.UnixMilli())
}

func (s *Store) queryAgenda(where string, args ...any) ([]AgendaItem, error) {
	rows, err := s.db.Query(`SELECT id, mate_id, text, due_at, fired_at, created_at FROM bot_agenda `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("mate: list agenda: %w", err)
	}
	defer rows.Close()
	var out []AgendaItem
	for rows.Next() {
		var it AgendaItem
		var due, fired, created int64
		if err := rows.Scan(&it.ID, &it.MateID, &it.Text, &due, &fired, &created); err != nil {
			return nil, err
		}
		it.Due, it.FiredAt, it.Created = fromMs(due), fromMs(fired), fromMs(created)
		out = append(out, it)
	}
	return out, rows.Err()
}

// MarkAgendaFired records that a round woke for these items, so each wakes the bot once.
func (s *Store) MarkAgendaFired(ids []string, at time.Time) error {
	for _, id := range ids {
		if _, err := s.db.Exec(`UPDATE bot_agenda SET fired_at = ? WHERE id = ? AND fired_at = 0`, at.UnixMilli(), id); err != nil {
			return fmt.Errorf("mate: fire agenda: %w", err)
		}
	}
	return nil
}

// FinishAgenda closes one of the bot's open items.
func (s *Store) FinishAgenda(mateID, id string) error {
	res, err := s.db.Exec(`UPDATE bot_agenda SET done_at = ? WHERE id = ? AND mate_id = ? AND done_at = 0`,
		time.Now().UnixMilli(), id, mateID)
	if err != nil {
		return fmt.Errorf("mate: finish agenda: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAgendaItemNotFound
	}
	return nil
}

// StartPulse records a round as it begins.
func (s *Store) StartPulse(mateID, why string, at time.Time) (PulseRun, error) {
	p := PulseRun{ID: uuid.NewString(), MateID: mateID, At: at, Why: why}
	if _, err := s.db.Exec(`INSERT INTO pulse_runs(id, mate_id, at, why) VALUES (?,?,?,?)`, p.ID, mateID, at.UnixMilli(), why); err != nil {
		return PulseRun{}, fmt.Errorf("mate: start pulse: %w", err)
	}
	return p, nil
}

// EndPulse records how a round ended. Notes it wrote count as the bot's writes.
func (s *Store) EndPulse(p PulseRun) error {
	wrote := ""
	if len(p.Wrote) > 0 {
		b, _ := json.Marshal(p.Wrote)
		wrote = string(b)
	}
	nudged := ""
	if len(p.Nudged) > 0 {
		b, _ := json.Marshal(p.Nudged)
		nudged = string(b)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("mate: end pulse tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`UPDATE pulse_runs SET ended_at = ?, decision = ?, summary = ?, brief = ?, wrote = ?, run_id = ?, nudged = ? WHERE id = ?`,
		unixMs(p.EndedAt), p.Decision, p.Summary, p.Brief, wrote, p.RunID, nudged, p.ID); err != nil {
		return fmt.Errorf("mate: end pulse: %w", err)
	}
	for _, path := range p.Wrote {
		if _, err := tx.Exec(`INSERT INTO note_activity(path, actor, action, at) VALUES (?,?,?,?)`,
			path, p.MateID, NoteAccessWrite, unixMs(p.EndedAt)); err != nil {
			return fmt.Errorf("mate: pulse note activity: %w", err)
		}
	}
	return tx.Commit()
}

// SetPulseDelivery records how the chat turn of round id's brief ended.
func (s *Store) SetPulseDelivery(id, delivery string) error {
	_, err := s.db.Exec(`UPDATE pulse_runs SET delivery = ?, delivered_at = ? WHERE id = ? AND delivery = ''`,
		delivery, time.Now().UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("mate: pulse delivery: %w", err)
	}
	return nil
}

// ListPulses returns a bot's latest rounds, newest first.
func (s *Store) ListPulses(mateID string, limit int) ([]PulseRun, error) {
	rows, err := s.db.Query(`SELECT id, mate_id, at, ended_at, why, decision, summary, brief, wrote, delivery, run_id, nudged
		FROM pulse_runs WHERE mate_id = ? ORDER BY at DESC LIMIT ?`, mateID, limit)
	if err != nil {
		return nil, fmt.Errorf("mate: list pulses: %w", err)
	}
	defer rows.Close()
	var out []PulseRun
	for rows.Next() {
		var p PulseRun
		var at, ended int64
		var wrote, nudged string
		if err := rows.Scan(&p.ID, &p.MateID, &at, &ended, &p.Why, &p.Decision, &p.Summary, &p.Brief, &wrote, &p.Delivery, &p.RunID, &nudged); err != nil {
			return nil, err
		}
		p.At, p.EndedAt = fromMs(at), fromMs(ended)
		if wrote != "" {
			_ = json.Unmarshal([]byte(wrote), &p.Wrote)
		}
		if nudged != "" {
			_ = json.Unmarshal([]byte(nudged), &p.Nudged)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PulseCount counts a bot's rounds since t, and returns when the last one began.
func (s *Store) PulseCount(mateID string, since time.Time) (n int, last time.Time, err error) {
	var lastMs sql.NullInt64
	err = s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM pulse_runs WHERE mate_id = ?1 AND at >= ?2),
		(SELECT MAX(at) FROM pulse_runs WHERE mate_id = ?1)`, mateID, since.UnixMilli()).Scan(&n, &lastMs)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("mate: count pulses: %w", err)
	}
	return n, fromMs(lastMs.Int64), nil
}

// PulseSettled is when the bot's latest round, and the chat turn its brief
// started, were over: what changed since then is news to it, not its own doing.
func (s *Store) PulseSettled(mateID string) (time.Time, error) {
	var at, ended, delivered int64
	err := s.db.QueryRow(`SELECT at, ended_at, delivered_at FROM pulse_runs WHERE mate_id = ? ORDER BY at DESC LIMIT 1`, mateID).
		Scan(&at, &ended, &delivered)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("mate: pulse settled: %w", err)
	}
	return fromMs(max(at, ended, delivered)), nil
}

// LastUserMessageAt is when the user last wrote to the bot, from any app.
func (s *Store) LastUserMessageAt(mateID string) (time.Time, error) {
	var ms sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(created_at) FROM chat_messages WHERE mate_id = ? AND role = 'user'`, mateID).Scan(&ms); err != nil {
		return time.Time{}, fmt.Errorf("mate: last user message: %w", err)
	}
	return fromMs(ms.Int64), nil
}

// LastAppMessage is the user's latest message to the bot from a connected
// app, like Telegram: a sign they have their phone in hand.
func (s *Store) LastAppMessage(mateID string) (app string, at time.Time, err error) {
	var ms int64
	err = s.db.QueryRow(`SELECT app, created_at FROM chat_messages WHERE mate_id = ? AND role = 'user' AND app != ''
		ORDER BY created_at DESC LIMIT 1`, mateID).Scan(&app, &ms)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("mate: last app message: %w", err)
	}
	return app, fromMs(ms), nil
}

// SentWake is a reply a bot sent on its own.
type SentWake struct {
	At           time.Time
	Event        string
	TriggerName  string
	Text         string
	Replied      bool // the user wrote in that chat within replyWindow after it
	ReplyPending bool // too recent to tell
}

const replyWindow = 12 * time.Hour

// SentWakes lists the bot's own replies since t, oldest first.
func (s *Store) SentWakes(mateID string, since, now time.Time) ([]SentWake, error) {
	rows, err := s.db.Query(`SELECT a.updated_at, a.trigger_event, COALESCE(t.name, ''), a.content,
		EXISTS (SELECT 1 FROM chat_messages u WHERE u.conversation_id = a.conversation_id AND u.role = 'user'
		        AND u.created_at > a.updated_at AND u.created_at <= a.updated_at + ?)
		FROM chat_messages a LEFT JOIN mate_triggers t ON t.id = a.trigger_id
		WHERE a.mate_id = ? AND a.role = 'assistant' AND a.trigger_event != '' AND a.status = 'succeeded' AND a.updated_at >= ?
		ORDER BY a.updated_at`, replyWindow.Milliseconds(), mateID, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("mate: sent wakes: %w", err)
	}
	defer rows.Close()
	var out []SentWake
	for rows.Next() {
		var w SentWake
		var at int64
		if err := rows.Scan(&at, &w.Event, &w.TriggerName, &w.Text, &w.Replied); err != nil {
			return nil, err
		}
		w.At = fromMs(at)
		w.ReplyPending = !w.Replied && now.Sub(w.At) < replyWindow
		out = append(out, w)
	}
	return out, rows.Err()
}

func unixMs(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func fromMs(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
