package mate

import (
	"fmt"
	"time"
)

// ChatLine is one message of a bot's chat, as another bot reads it.
type ChatLine struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chatId,omitempty"`
	Role      string    `json:"role"`          // user | assistant
	App       string    `json:"app,omitempty"` // the connected app a user message came from
	Text      string    `json:"text"`
	At        time.Time `json:"at"`
	OnItsOwn  bool      `json:"onItsOwn,omitempty"` // the bot sent it unasked
	Source    string    `json:"source,omitempty"`   // what woke it: a trigger's name, or "rounds"
	Unread    bool      `json:"unread,omitempty"`   // a finished reply the user hasn't seen yet
	Truncated bool      `json:"truncated,omitempty"`
	Status    string    `json:"status,omitempty"` // set when the reply didn't succeed
}

// chatLineMax bounds context returned to another bot.
const chatLineMax = 4000

func (l *ChatLine) limitText() {
	if r := []rune(l.Text); len(r) > chatLineMax {
		l.Text = string(r[:chatLineMax]) + "…"
		l.Truncated = true
	}
}

// BotChat reads up to limit messages of the bot's current chat before
// beforeID (the newest when empty), oldest first. Wake tasks are left out,
// the reply saying what woke it, and don't count toward limit.
func (s *Store) BotChat(mateID, beforeID string, limit int) (chatID string, lines []ChatLine, more bool, err error) {
	chatID, err = s.ActiveConversationID(mateID)
	if err != nil || chatID == "" {
		return "", nil, false, err
	}
	var seenMs int64
	if err := s.db.QueryRow(`SELECT seen_at FROM mates WHERE id = ?`, mateID).Scan(&seenMs); err != nil {
		return "", nil, false, fmt.Errorf("mate: bot chat seen: %w", err)
	}
	msgs, more, err := s.listMessagesPage(chatID, beforeID, limit, false)
	if err != nil {
		return "", nil, false, err
	}
	for _, m := range msgs {
		l := ChatLine{ID: m.ID, Role: m.Role, App: m.App, Text: m.Content, At: m.CreatedAt}
		l.limitText()
		if m.Role == "assistant" {
			if m.Status != "succeeded" {
				l.Status = m.Status
			}
			if m.TriggerEvent != "" {
				l.OnItsOwn = true
				l.Source = m.TriggerName
				if m.TriggerEvent == string(MateEventPulse) {
					l.Source = "rounds"
				} else if l.Source == "" {
					l.Source = EventLabel(MateEventType(m.TriggerEvent))
				}
			}
			l.Unread = m.App == "" && m.Status != "" && m.Status != "running" && m.UpdatedAt.UnixMilli() > seenMs
		}
		lines = append(lines, l)
	}
	return chatID, lines, more, nil
}

// BotChatHistory reads retained chats, keeping user text whole and bounding replies.
func (s *Store) BotChatHistory(mateID string, since time.Time, beforeID string, limit int) ([]ChatLine, bool, error) {
	limit = max(1, min(limit, 50))
	q := `SELECT ` + messageCols + ` FROM chat_messages
		WHERE conversation_id IN (SELECT id FROM conversations WHERE mate_id = ?)
		AND created_at >= ? AND (role = 'user' OR (role = 'assistant' AND status = 'succeeded'))`
	args := []any{mateID, since.UnixMilli()}
	if beforeID != "" {
		q += ` AND (created_at, rowid) < (SELECT created_at, rowid FROM chat_messages
			WHERE id = ? AND conversation_id IN (SELECT id FROM conversations WHERE mate_id = ?))`
		args = append(args, beforeID, mateID)
	}
	q += ` ORDER BY created_at DESC, rowid DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, false, fmt.Errorf("mate: bot chat history: %w", err)
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
	lines := make([]ChatLine, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		l := ChatLine{ID: m.ID, ChatID: m.ConversationID, Role: m.Role,
			App: m.App, Text: m.Content, At: m.CreatedAt, OnItsOwn: m.TriggerEvent != "", Source: m.TriggerEvent}
		if m.Role == "assistant" {
			l.limitText()
		}
		lines = append(lines, l)
	}
	return lines, more, nil
}
