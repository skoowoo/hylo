package mate

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ── Conversations ─────────────────────────────────────────────────────────────

// CreateConversation creates a user-facing chat conversation bound to a mate (type='chat').
func (s *Store) CreateConversation(mateID, title string) (Conversation, error) {
	return s.createConversation(mateID, title, ConvTypeChat, "")
}

// CreateTriggerConversation creates a background trigger-run conversation (type='trigger').
func (s *Store) CreateTriggerConversation(mateID, title string) (Conversation, error) {
	return s.createConversation(mateID, title, ConvTypeTrigger, "")
}

func (s *Store) createConversation(mateID, title, convType, userKey string) (Conversation, error) {
	now := time.Now().UnixMilli()
	id := uuid.NewString()
	_, err := s.db.Exec(
		`INSERT INTO conversations(id, mate_id, title, type, user_key, agent_session_id, agent_session_agent_id, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		id, mateID, title, convType, userKey, "", "", now, now,
	)
	if err != nil {
		return Conversation{}, fmt.Errorf("mate: create conversation: %w", err)
	}
	return Conversation{
		ID:        id,
		MateID:    mateID,
		Title:     title,
		Type:      convType,
		UserKey:   userKey,
		CreatedAt: time.UnixMilli(now),
		UpdatedAt: time.UnixMilli(now),
	}, nil
}

func (s *Store) GetConversation(id string) (Conversation, error) {
	var c Conversation
	var createdMs, updatedMs int64
	err := s.db.QueryRow(
		`SELECT id, mate_id, title, type, user_key, agent_session_id, agent_session_agent_id, created_at, updated_at FROM conversations WHERE id = ?`, id,
	).Scan(&c.ID, &c.MateID, &c.Title, &c.Type, &c.UserKey, &c.AgentSessionID, &c.AgentSessionAgentID, &createdMs, &updatedMs)
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, fmt.Errorf("mate: conversation not found: %s", id)
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("mate: get conversation: %w", err)
	}
	c.CreatedAt = time.UnixMilli(createdMs)
	c.UpdatedAt = time.UnixMilli(updatedMs)
	return c, nil
}

// GetOrCreateActiveChatConvID returns the mate's current chat conversation (most recently
// updated), creating one if the mate has no chat history yet.
func (s *Store) GetOrCreateActiveChatConvID(mateID string) (string, error) {
	convs, err := s.ListConversations(mateID)
	if err != nil {
		return "", err
	}
	if len(convs) > 0 {
		return convs[0].ID, nil
	}
	conv, err := s.CreateConversation(mateID, "")
	if err != nil {
		return "", err
	}
	return conv.ID, nil
}

// GetOrCreateDefaultTriggerConv returns the single shared trigger conversation for a mate
// (type='trigger'). All no-reply trigger runs append their messages here; each run uses
// an independent ephemeral agent session.
func (s *Store) GetOrCreateDefaultTriggerConv(mateID string) (string, error) {
	var id string
	err := s.db.QueryRow(
		`SELECT id FROM conversations WHERE type = ? AND mate_id = ? ORDER BY created_at DESC LIMIT 1`,
		ConvTypeTrigger, mateID,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("mate: get default trigger conv: %w", err)
	}
	conv, err := s.createConversation(mateID, "Auto runs", ConvTypeTrigger, "")
	if err != nil {
		return "", err
	}
	return conv.ID, nil
}

// EnsureNewConv returns a fresh empty conversation for the given (mateID, convType, userKey).
// If the most recent conversation of that type is already empty (no messages), it is returned as-is.
// Otherwise a new conversation is created with the given title.
// When keep > 0, conversations older than the most recent keep are pruned before creation.
func (s *Store) EnsureNewConv(mateID, convType, userKey, title string, keep int) (Conversation, error) {
	var latestID string
	err := s.db.QueryRow(
		`SELECT id FROM conversations WHERE type=? AND mate_id=? AND user_key=? ORDER BY created_at DESC LIMIT 1`,
		convType, mateID, userKey,
	).Scan(&latestID)
	if err == nil {
		var msgCount int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE conversation_id=?`, latestID).Scan(&msgCount)
		if msgCount == 0 {
			return s.GetConversation(latestID)
		}
	}
	if keep > 0 {
		_, _ = s.db.Exec(
			`DELETE FROM conversations
			 WHERE type=? AND mate_id=? AND user_key=?
			   AND id NOT IN (
			     SELECT id FROM conversations
			     WHERE type=? AND mate_id=? AND user_key=?
			     ORDER BY created_at DESC LIMIT ?
			   )`,
			convType, mateID, userKey,
			convType, mateID, userKey, keep,
		)
	}
	return s.createConversation(mateID, title, convType, userKey)
}

// GetOrCreateTriggerReplyConv returns the persistent trigger_reply conversation for a
// (mateID, userKey) pair. userKey is an external user identifier (e.g. wechat user id).
// The conversation persists agent session state for multi-turn replies.
func (s *Store) GetOrCreateTriggerReplyConv(mateID, userKey string) (string, error) {
	var id string
	err := s.db.QueryRow(
		`SELECT id FROM conversations WHERE type = ? AND mate_id = ? AND user_key = ? ORDER BY created_at ASC LIMIT 1`,
		ConvTypeTriggerReply, mateID, userKey,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("mate: get trigger reply conv: %w", err)
	}
	conv, err := s.createConversation(mateID, "", ConvTypeTriggerReply, userKey)
	if err != nil {
		return "", err
	}
	return conv.ID, nil
}

// GetOrCreateTriggerConvID is deprecated: legacy trigger runs used a separate conversation.
func (s *Store) GetOrCreateTriggerConvID(m *Mate) (string, error) {
	if m.TriggerConvID != "" {
		return m.TriggerConvID, nil
	}
	conv, err := s.createConversation(m.ID, "Auto runs", ConvTypeTrigger, "")
	if err != nil {
		return "", err
	}
	now := time.Now().UnixMilli()
	_, err = s.db.Exec(`UPDATE mates SET trigger_conv_id = ?, updated_at = ? WHERE id = ?`, conv.ID, now, m.ID)
	if err != nil {
		return "", fmt.Errorf("mate: set trigger_conv_id: %w", err)
	}
	m.TriggerConvID = conv.ID
	return conv.ID, nil
}

// ListConversations returns chat-type conversations for a specific mate, ordered by most recent first.
func (s *Store) ListConversations(mateID string) ([]Conversation, error) {
	return s.ListConversationsByType(mateID, ConvTypeChat)
}

// ListConversationsByType returns conversations of a given type for a specific mate, ordered by most recent first.
func (s *Store) ListConversationsByType(mateID, convType string) ([]Conversation, error) {
	rows, err := s.db.Query(
		`SELECT id, mate_id, title, type, user_key, agent_session_id, agent_session_agent_id, created_at, updated_at FROM conversations WHERE type = ? AND mate_id = ? ORDER BY updated_at DESC`,
		convType, mateID,
	)
	if err != nil {
		return nil, fmt.Errorf("mate: list conversations by type: %w", err)
	}
	defer rows.Close()
	return scanConversations(rows)
}

func (s *Store) UpdateConversationTitle(id, title string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(
		`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`, title, now, id,
	)
	return err
}

func (s *Store) touchConversation(id string) {
	now := time.Now().UnixMilli()
	s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, id) //nolint:errcheck
}

func (s *Store) DeleteConversation(id string) error {
	_, err := s.db.Exec(`DELETE FROM conversations WHERE id = ?`, id)
	return err
}

// SetConversationAgentSession stores the agent-native session id for multi-turn resume.
func (s *Store) SetConversationAgentSession(conversationID, agentID, sessionID string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(
		`UPDATE conversations SET agent_session_id = ?, agent_session_agent_id = ?, updated_at = ? WHERE id = ?`,
		sessionID, agentID, now, conversationID,
	)
	if err != nil {
		return fmt.Errorf("mate: set agent session: %w", err)
	}
	return nil
}

// ClearConversationAgentSession drops the stored agent session (e.g. agent switch).
func (s *Store) ClearConversationAgentSession(conversationID string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(
		`UPDATE conversations SET agent_session_id = '', agent_session_agent_id = '', updated_at = ? WHERE id = ?`,
		now, conversationID,
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
		`INSERT INTO chat_messages(id, conversation_id, role, content, agent_id, mate_id, model_id, run_id, status, trigger_event, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.ConversationID, m.Role, m.Content,
		m.AgentID, m.MateID, m.ModelID, m.RunID, m.Status, m.TriggerEvent, now, now,
	)
	if err != nil {
		return Message{}, fmt.Errorf("mate: insert message: %w", err)
	}
	s.touchConversation(m.ConversationID)
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

func (s *Store) UpdateMessageDone(id, content, status string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(
		`UPDATE chat_messages SET content = ?, status = ?, updated_at = ? WHERE id = ?`,
		content, status, now, id,
	)
	return err
}

func (s *Store) ListMessages(conversationID string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, conversation_id, role, content, agent_id, mate_id, model_id, run_id, status, trigger_event, created_at, updated_at
		 FROM chat_messages WHERE conversation_id = ? ORDER BY created_at ASC`,
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
	return msgs, nil
}

// ListMessagesSince returns messages in a conversation whose updated_at is after sinceMs (Unix ms).
// Using updated_at (not created_at) ensures that assistant placeholders inserted as "running"
// are re-returned after UpdateMessageDone advances their updated_at on completion.
func (s *Store) ListMessagesSince(conversationID string, sinceMs int64) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, conversation_id, role, content, agent_id, mate_id, model_id, run_id, status, trigger_event, created_at, updated_at
		 FROM chat_messages WHERE conversation_id = ? AND updated_at > ? ORDER BY created_at ASC`,
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
	return msgs, nil
}

// ── Note access ───────────────────────────────────────────────────────────────

// InsertMessageNoteAccess records the notes an assistant run touched. Called once
// per run after the agent process exits, from the normalized list any backend's
// event stream produced (see agent.NoteAccessTracker).
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
	for _, e := range entries {
		at := e.At.UnixMilli()
		if at == 0 {
			at = time.Now().UnixMilli()
		}
		if _, err := stmt.Exec(messageID, e.Path, e.Action, e.Tool, at); err != nil {
			return fmt.Errorf("mate: insert note access: %w", err)
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

func scanConversations(rows *sql.Rows) ([]Conversation, error) {
	var out []Conversation
	for rows.Next() {
		var c Conversation
		var createdMs, updatedMs int64
		if err := rows.Scan(&c.ID, &c.MateID, &c.Title, &c.Type, &c.UserKey, &c.AgentSessionID, &c.AgentSessionAgentID, &createdMs, &updatedMs); err != nil {
			return nil, err
		}
		c.CreatedAt = time.UnixMilli(createdMs)
		c.UpdatedAt = time.UnixMilli(updatedMs)
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	var out []Message
	for rows.Next() {
		var m Message
		var createdMs, updatedMs int64
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Role, &m.Content,
			&m.AgentID, &m.MateID, &m.ModelID, &m.RunID, &m.Status, &m.TriggerEvent,
			&createdMs, &updatedMs,
		); err != nil {
			return nil, err
		}
		m.CreatedAt = time.UnixMilli(createdMs)
		m.UpdatedAt = time.UnixMilli(updatedMs)
		out = append(out, m)
	}
	return out, rows.Err()
}
