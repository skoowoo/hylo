package mate

import (
	"errors"
	"fmt"
)

// Wake asks a bot to act on its own. It runs as a turn in the bot's current
// chat, where the reply reads as the bot reaching out; triggers are one source.
type Wake struct {
	Event     MateEventType
	TriggerID string
	Prompt    string // the task, already rendered
	// Key drops a wake while one with the same key still waits for the chat.
	Key string
	// Path is the note the wake is about, if any, so its progress can be looked up by note.
	Path string
}

// WakeFunc runs w in m's chat. It returns at once; the turn waits its turn
// there. A wake already waiting under w.Key takes w's place and is no error.
type WakeFunc func(m *Mate, w Wake) error

// ErrBotBusy refuses a wake: the bot has as many waiting as it takes.
var ErrBotBusy = errors.New("the bot has too many tasks waiting; try again later")

// ErrTriggerDisabled refuses a turn whose bot, or the trigger that woke it, is
// disabled or gone.
var ErrTriggerDisabled = errors.New("the trigger or its bot is disabled")

// TurnBot returns the bot a turn in mateID's chat runs on, if it may still run:
// the bot, and the trigger behind a wake (triggerID), exist and are enabled.
// A turn can wait a while for the chat, so it is asked again once it has it.
func (s *Store) TurnBot(mateID, triggerID string) (*Mate, error) {
	m, err := s.GetMate(mateID)
	if err != nil {
		return nil, err
	}
	if m == nil || !m.Enabled {
		return nil, ErrTriggerDisabled
	}
	if triggerID == "" {
		return m, nil
	}
	t, err := s.getTrigger(triggerID)
	if err != nil {
		return nil, err
	}
	if t == nil || !t.Enabled || t.MateID != mateID {
		return nil, ErrTriggerDisabled
	}
	return m, nil
}

// WakeReason names what woke a bot, for the agent to read, e.g.
// `Your trigger "Memory" (daily 09:00)`. The task itself carries the note.
func (s *Store) WakeReason(triggerID string, event MateEventType) string {
	what := EventLabel(event)
	var name string
	if t, err := s.getTrigger(triggerID); err == nil && t != nil {
		name = t.Name
		if t.Schedule != "" {
			what = t.Schedule
		}
	}
	if name == "" {
		return "Your trigger (" + what + ")"
	}
	return fmt.Sprintf("Your trigger %q (%s)", name, what)
}
