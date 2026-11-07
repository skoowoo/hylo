package mate

import (
	"fmt"
	"time"
)

// ChatLine is one message of a bot's chat, as another bot reads it.
type ChatLine struct {
	ID       string    `json:"id"`
	Role     string    `json:"role"`          // user | assistant
	App      string    `json:"app,omitempty"` // the connected app a user message came from
	Text     string    `json:"text"`
	At       time.Time `json:"at"`
	OnItsOwn bool      `json:"onItsOwn,omitempty"` // the bot sent it unasked
	Source   string    `json:"source,omitempty"`   // what woke it: a trigger's name, or "rounds"
	Unread   bool      `json:"unread,omitempty"`   // sent on its own after the user last opened the chat
	Status   string    `json:"status,omitempty"`   // set when the reply didn't succeed
}

// chatLineMax bounds one message's text: a long digest is still readable whole by note.
const chatLineMax = 4000

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
		if r := []rune(l.Text); len(r) > chatLineMax {
			l.Text = string(r[:chatLineMax]) + "…"
		}
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
				l.Unread = m.Status != "" && m.Status != "running" && m.UpdatedAt.UnixMilli() > seenMs
			}
		}
		lines = append(lines, l)
	}
	return chatID, lines, more, nil
}
