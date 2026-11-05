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

// ── Conversations ─────────────────────────────────────────────────────────────

// keepChats is how many of a bot's chats are kept; starting one past it drops the oldest.
const keepChats = 10

const convCols = `id, mate_id, type, agent_session_id, agent_session_agent_id, agent_session_cwd, created_at, updated_at`

// CreateConversation adds a chat for a mate without making it the current one.
func (s *Store) CreateConversation(mateID string) (Conversation, error) {
	return s.createConversation(mateID, uuid.NewString())
}

func (s *Store) createConversation(mateID, id string) (Conversation, error) {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(
		`INSERT INTO conversations(id, mate_id, type, agent_session_id, agent_session_agent_id, agent_session_cwd, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		id, mateID, ConvTypeChat, "", "", "", now, now,
	)
	if err != nil {
		return Conversation{}, fmt.Errorf("mate: create conversation: %w", err)
	}
	return Conversation{
		ID:        id,
		MateID:    mateID,
		Type:      ConvTypeChat,
		CreatedAt: time.UnixMilli(now),
		UpdatedAt: time.UnixMilli(now),
	}, nil
}

// StartConversation makes a new chat the mate's current one, dropping the
// oldest ones past keepChats along with any that never got a message. id is
// the one the desktop allocated for it, or "" for a fresh one.
func (s *Store) StartConversation(mateID, id string) (Conversation, error) {
	if id == "" {
		id = uuid.NewString()
	}
	conv, err := s.createConversation(mateID, id)
	if err != nil {
		return Conversation{}, err
	}
	if _, err := s.db.Exec(`UPDATE mates SET active_conversation_id = ? WHERE id = ?`, conv.ID, mateID); err != nil {
		return Conversation{}, fmt.Errorf("mate: start conversation: %w", err)
	}
	_, _ = s.db.Exec(
		`DELETE FROM conversations WHERE mate_id = ?1 AND id != ?2 AND id NOT IN (
		   SELECT id FROM conversations c WHERE mate_id = ?1
		     AND EXISTS (SELECT 1 FROM chat_messages WHERE conversation_id = c.id)
		   ORDER BY updated_at DESC LIMIT ?3)`,
		mateID, conv.ID, keepChats-1,
	)
	s.chatChanged()
	return conv, nil
}

// ActiveConversationID returns the mate's current chat, or "" while a new one
// waits for its first message.
func (s *Store) ActiveConversationID(mateID string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT active_conversation_id FROM mates WHERE id = ?`, mateID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("mate: active conversation: %w", err)
	}
	return id, nil
}

// SetActiveConversation makes one of the mate's chats the current one; "" opens a new chat.
func (s *Store) SetActiveConversation(mateID, convID string) error {
	if cur, err := s.ActiveConversationID(mateID); err == nil && cur == convID {
		return nil
	}
	res, err := s.db.Exec(
		`UPDATE mates SET active_conversation_id = ?2 WHERE id = ?1
		   AND (?2 = '' OR EXISTS (SELECT 1 FROM conversations WHERE id = ?2 AND mate_id = ?1))`,
		mateID, convID,
	)
	if err != nil {
		return fmt.Errorf("mate: set active conversation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("mate: conversation %q not found for %q", convID, mateID)
	}
	s.chatChanged()
	return nil
}

// CurrentConversationID returns the mate's current chat, starting one when it has none.
func (s *Store) CurrentConversationID(mateID string) (string, error) {
	id, err := s.ActiveConversationID(mateID)
	if err != nil || id != "" {
		return id, err
	}
	conv, err := s.StartConversation(mateID, "")
	if err != nil {
		return "", err
	}
	return conv.ID, nil
}

func (s *Store) GetConversation(id string) (Conversation, error) {
	c, err := scanConversation(s.db.QueryRow(`SELECT `+convCols+` FROM conversations WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, fmt.Errorf("mate: conversation not found: %s", id)
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("mate: get conversation: %w", err)
	}
	return c, nil
}

// ListConversations returns a mate's chats that have messages, most recently
// active first. A chat is titled by the user's first message; one the bot
// started on its own, by the trigger that first woke it, else its first reply.
func (s *Store) ListConversations(mateID string) ([]Conversation, error) {
	rows, err := s.db.Query(
		`SELECT `+convCols+`,
		   COALESCE(
		     (SELECT NULLIF(substr(content, 1, 200), '') FROM chat_messages
		       WHERE conversation_id = conversations.id AND role = 'user'
		       ORDER BY created_at ASC, rowid ASC LIMIT 1),
		     (SELECT NULLIF(t.name, '') FROM chat_messages m JOIN mate_triggers t ON t.id = m.trigger_id
		       WHERE m.conversation_id = conversations.id AND m.trigger_id != ''
		       ORDER BY m.created_at ASC, m.rowid ASC LIMIT 1),
		     (SELECT NULLIF(substr(content, 1, 200), '') FROM chat_messages
		       WHERE conversation_id = conversations.id AND role = 'assistant'
		       ORDER BY created_at ASC, rowid ASC LIMIT 1),
		     '')
		 FROM conversations
		 WHERE mate_id = ? AND EXISTS (SELECT 1 FROM chat_messages WHERE conversation_id = conversations.id)
		 ORDER BY updated_at DESC`,
		mateID,
	)
	if err != nil {
		return nil, fmt.Errorf("mate: list conversations: %w", err)
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		var createdMs, updatedMs int64
		if err := rows.Scan(&c.ID, &c.MateID, &c.Type, &c.AgentSessionID, &c.AgentSessionAgentID, &c.AgentSessionCwd, &createdMs, &updatedMs, &c.Title); err != nil {
			return nil, err
		}
		c.CreatedAt = time.UnixMilli(createdMs)
		c.UpdatedAt = time.UnixMilli(updatedMs)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) touchConversation(id string) {
	now := time.Now().UnixMilli()
	s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, id) //nolint:errcheck
}

func (s *Store) DeleteConversation(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`UPDATE mates SET active_conversation_id = '' WHERE active_conversation_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM conversations WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.chatChanged()
	return nil
}

// SetConversationAgentSession stores the agent-native session id for multi-turn resume,
// with the agent and cwd it lives under.
func (s *Store) SetConversationAgentSession(conversationID, agentID, cwd, sessionID string) error {
	_, err := s.db.Exec(
		`UPDATE conversations SET agent_session_id = ?, agent_session_agent_id = ?, agent_session_cwd = ? WHERE id = ?`,
		sessionID, agentID, cwd, conversationID,
	)
	if err != nil {
		return fmt.Errorf("mate: set agent session: %w", err)
	}
	return nil
}

// ClearConversationAgentSession drops the stored agent session (e.g. agent or cwd switch).
func (s *Store) ClearConversationAgentSession(conversationID string) error {
	_, err := s.db.Exec(
		`UPDATE conversations SET agent_session_id = '', agent_session_agent_id = '', agent_session_cwd = '' WHERE id = ?`,
		conversationID,
	)
	if err != nil {
		return fmt.Errorf("mate: clear agent session: %w", err)
	}
	return nil
}

// ── Messages ──────────────────────────────────────────────────────────────────

func (s *Store) InsertMessage(m Message) (Message, error) {
	now := time.Now().UnixMilli()
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	m.CreatedAt = time.UnixMilli(now)
	m.UpdatedAt = time.UnixMilli(now)
	_, err := s.db.Exec(
		`INSERT INTO chat_messages(id, conversation_id, role, content, attachments, agent_id, mate_id, model_id, run_id, status, trigger_event, trigger_id, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.ConversationID, m.Role, m.Content, encodeAttachments(m.Attachments),
		m.AgentID, m.MateID, m.ModelID, m.RunID, m.Status, m.TriggerEvent, m.TriggerID, now, now,
	)
	if err != nil {
		return Message{}, fmt.Errorf("mate: insert message: %w", err)
	}
	s.touchConversation(m.ConversationID)
	s.chatChanged(m.ID)
	return m, nil
}

// MarkStalledRunMessagesAsFailed marks assistant messages left with status='running'
// as 'failed' and bumps their updated_at. Call once on server startup to recover
// messages whose runs were interrupted when the process was killed.
func (s *Store) MarkStalledRunMessagesAsFailed() error {
	_, err := s.db.Exec(
		`UPDATE chat_messages SET status = 'failed', updated_at = ? WHERE status = 'running' AND role = 'assistant'`,
		time.Now().UnixMilli(),
	)
	return err
}

// DeleteRetried drops a failed or stopped reply that is being run again, plus
// the prompt (or wake) right before it when that is what's being resent, so a
// retry replaces the attempt instead of piling up next to it.
func (s *Store) DeleteRetried(convID, assistantID, prompt string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var role, status string
	err = tx.QueryRow(`SELECT role, status FROM chat_messages WHERE id = ? AND conversation_id = ?`, assistantID, convID).Scan(&role, &status)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if role != "assistant" || (status != "failed" && status != "canceled") {
		return nil
	}
	var prevID, prevRole, prevContent string
	err = tx.QueryRow(
		`SELECT id, role, content FROM chat_messages WHERE conversation_id = ?
		   AND (created_at, rowid) < (SELECT created_at, rowid FROM chat_messages WHERE id = ?)
		 ORDER BY created_at DESC, rowid DESC LIMIT 1`, convID, assistantID,
	).Scan(&prevID, &prevRole, &prevContent)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	deleted := []string{assistantID}
	if (prevRole == "user" || prevRole == "wake") && prevContent == prompt {
		if _, err := tx.Exec(`DELETE FROM chat_messages WHERE id = ?`, prevID); err != nil {
			return err
		}
		deleted = append(deleted, prevID)
	}
	if _, err := tx.Exec(`DELETE FROM chat_messages WHERE id = ?`, assistantID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.chatChanged(deleted...)
	return nil
}

// DeleteMessages drops a turn the chat shouldn't show, like a wake the bot chose to stay silent on.
func (s *Store) DeleteMessages(ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := s.db.Exec(`DELETE FROM chat_messages WHERE id IN (?`+strings.Repeat(",?", len(ids)-1)+`)`, args...)
	if err != nil {
		return err
	}
	s.chatChanged(ids...)
	return nil
}

// UpdateMessageDone stores a finished run: its reply, and the agent events
// (agent.TurnLog) the chat rebuilds the trace from.
func (s *Store) UpdateMessageDone(id, content, trace, status string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(
		`UPDATE chat_messages SET content = ?, trace = ?, status = ?, updated_at = ? WHERE id = ?`,
		content, trace, status, now, id,
	)
	if err != nil {
		return err
	}
	s.chatChanged(id)
	return nil
}

const messageCols = `id, conversation_id, role, content, agent_id, mate_id, model_id, run_id, status, trigger_event, trigger_id, trace, attachments, created_at, updated_at`

func (s *Store) ListMessages(conversationID string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT `+messageCols+` FROM chat_messages WHERE conversation_id = ? ORDER BY created_at ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("mate: list messages: %w", err)
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	if err := s.attachNoteAccess(msgs); err != nil {
		return nil, err
	}
	if err := s.attachTriggers(msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// ListMessagesPage returns up to limit messages older than beforeID (the newest
// ones when beforeID is empty), oldest first, plus whether older ones remain.
// rowid breaks created_at ties so the cursor never skips or repeats a row.
func (s *Store) ListMessagesPage(conversationID, beforeID string, limit int) ([]Message, bool, error) {
	q := `SELECT ` + messageCols + ` FROM chat_messages WHERE conversation_id = ?`
	args := []any{conversationID}
	if beforeID != "" {
		q += ` AND (created_at, rowid) < (SELECT created_at, rowid FROM chat_messages WHERE id = ?)`
		args = append(args, beforeID)
	}
	q += ` ORDER BY created_at DESC, rowid DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, false, fmt.Errorf("mate: list messages page: %w", err)
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, false, err
	}
	more := len(msgs) > limit
	if more {
		msgs = msgs[:limit]
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	if err := s.attachNoteAccess(msgs); err != nil {
		return nil, false, err
	}
	if err := s.attachTriggers(msgs); err != nil {
		return nil, false, err
	}
	return msgs, more, nil
}

// ListMessagesSince returns messages in a conversation whose updated_at is after sinceMs (Unix ms).
// Using updated_at (not created_at) ensures that assistant placeholders inserted as "running"
// are re-returned after UpdateMessageDone advances their updated_at on completion.
func (s *Store) ListMessagesSince(conversationID string, sinceMs int64) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT `+messageCols+` FROM chat_messages WHERE conversation_id = ? AND updated_at > ? ORDER BY created_at ASC`,
		conversationID, sinceMs,
	)
	if err != nil {
		return nil, fmt.Errorf("mate: list messages since: %w", err)
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	if err := s.attachNoteAccess(msgs); err != nil {
		return nil, err
	}
	if err := s.attachTriggers(msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// ── Note access ───────────────────────────────────────────────────────────────

// InsertMessageNoteAccess records the notes an assistant run touched. Called once
// per run after the agent process exits, from the normalized list any backend's
// event stream produced (see agent.NoteAccessTracker). Writes are also logged
// as the message's bot's note activity, and so are reads that read the note in
// a run the user asked for; a wake's sweep of notes is not reading them.
func (s *Store) InsertMessageNoteAccess(messageID string, entries []NoteAccessEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("mate: note access tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	stmt, err := tx.Prepare(
		`INSERT INTO chat_message_note_access(message_id, path, action, tool_name, created_at) VALUES (?,?,?,?,?)`,
	)
	if err != nil {
		return fmt.Errorf("mate: note access prepare: %w", err)
	}
	defer stmt.Close()
	activity, err := tx.Prepare(
		`INSERT INTO note_activity(path, actor, action, at)
		 SELECT ?, mate_id, ?, ? FROM chat_messages WHERE id = ? AND mate_id != ''
		   AND (? = 'write' OR trigger_event = '')`,
	)
	if err != nil {
		return fmt.Errorf("mate: note activity prepare: %w", err)
	}
	defer activity.Close()
	for _, e := range entries {
		at := e.At
		if at.IsZero() {
			at = time.Now()
		}
		if _, err := stmt.Exec(messageID, e.Path, e.Action, e.Tool, at.UnixMilli()); err != nil {
			return fmt.Errorf("mate: insert note access: %w", err)
		}
		if e.Skim && e.Action != NoteAccessWrite {
			continue
		}
		if _, err := activity.Exec(e.Path, e.Action, at.UnixMilli(), messageID, e.Action); err != nil {
			return fmt.Errorf("mate: insert note activity: %w", err)
		}
	}
	return tx.Commit()
}

// ListMessageNoteAccess returns the notes a single message touched, oldest first.
func (s *Store) ListMessageNoteAccess(messageID string) ([]NoteAccessEntry, error) {
	rows, err := s.db.Query(
		`SELECT path, action, tool_name, created_at FROM chat_message_note_access WHERE message_id = ? ORDER BY created_at ASC`,
		messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("mate: list note access: %w", err)
	}
	defer rows.Close()
	return scanNoteAccess(rows)
}

// attachNoteAccess fills Message.NoteAccess for every assistant message in msgs,
// in a single query rather than one per message.
func (s *Store) attachNoteAccess(msgs []Message) error {
	var ids []string
	for _, m := range msgs {
		if m.Role == "assistant" {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT message_id, path, action, tool_name, created_at FROM chat_message_note_access
		 WHERE message_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY created_at ASC`,
		args...,
	)
	if err != nil {
		return fmt.Errorf("mate: list note access for messages: %w", err)
	}
	defer rows.Close()
	byID := make(map[string][]NoteAccessEntry)
	for rows.Next() {
		var mid string
		var e NoteAccessEntry
		var at int64
		if err := rows.Scan(&mid, &e.Path, &e.Action, &e.Tool, &at); err != nil {
			return err
		}
		e.At = time.UnixMilli(at)
		byID[mid] = append(byID[mid], e)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range msgs {
		if v, ok := byID[msgs[i].ID]; ok {
			msgs[i].NoteAccess = v
		}
	}
	return nil
}

// attachTriggers names the trigger behind every wake and wake reply in msgs, in one query.
func (s *Store) attachTriggers(msgs []Message) error {
	var ids []any
	var marks []string
	for _, m := range msgs {
		if m.TriggerID != "" {
			ids = append(ids, m.TriggerID)
			marks = append(marks, "?")
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.db.Query(`SELECT id, name, schedule FROM mate_triggers WHERE id IN (`+strings.Join(marks, ",")+`)`, ids...)
	if err != nil {
		return fmt.Errorf("mate: triggers for messages: %w", err)
	}
	defer rows.Close()
	type named struct{ name, schedule string }
	byID := map[string]named{}
	for rows.Next() {
		var id string
		var t named
		if err := rows.Scan(&id, &t.name, &t.schedule); err != nil {
			return err
		}
		byID[id] = t
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range msgs {
		if t, ok := byID[msgs[i].TriggerID]; ok {
			msgs[i].TriggerName, msgs[i].TriggerSchedule = t.name, t.schedule
		}
	}
	return nil
}

func scanNoteAccess(rows *sql.Rows) ([]NoteAccessEntry, error) {
	var out []NoteAccessEntry
	for rows.Next() {
		var e NoteAccessEntry
		var at int64
		if err := rows.Scan(&e.Path, &e.Action, &e.Tool, &at); err != nil {
			return nil, err
		}
		e.At = time.UnixMilli(at)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ── scan helpers ──────────────────────────────────────────────────────────────

func scanConversation(row *sql.Row) (Conversation, error) {
	var c Conversation
	var createdMs, updatedMs int64
	if err := row.Scan(&c.ID, &c.MateID, &c.Type, &c.AgentSessionID, &c.AgentSessionAgentID, &c.AgentSessionCwd, &createdMs, &updatedMs); err != nil {
		return Conversation{}, err
	}
	c.CreatedAt = time.UnixMilli(createdMs)
	c.UpdatedAt = time.UnixMilli(updatedMs)
	return c, nil
}

func encodeAttachments(atts []Attachment) string {
	if len(atts) == 0 {
		return ""
	}
	b, err := json.Marshal(atts)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeAttachments(raw string) []Attachment {
	if raw == "" {
		return nil
	}
	var atts []Attachment
	if err := json.Unmarshal([]byte(raw), &atts); err != nil {
		return nil
	}
	return atts
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	var out []Message
	for rows.Next() {
		var m Message
		var createdMs, updatedMs int64
		var trace, attachments string
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Role, &m.Content,
			&m.AgentID, &m.MateID, &m.ModelID, &m.RunID, &m.Status, &m.TriggerEvent, &m.TriggerID,
			&trace, &attachments, &createdMs, &updatedMs,
		); err != nil {
			return nil, err
		}
		if trace != "" {
			m.Trace = json.RawMessage(trace)
		}
		m.Attachments = decodeAttachments(attachments)
		m.CreatedAt = time.UnixMilli(createdMs)
		m.UpdatedAt = time.UnixMilli(updatedMs)
		out = append(out, m)
	}
	return out, rows.Err()
}
