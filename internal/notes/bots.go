package notes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/mate"
)

// BotCreate makes a bot on the same agent and model as the user's first bot:
// whoever calls this can't pick an agent the user has never set up.
func BotCreate(c *client.Client, name, description, systemPrompt string) (*mate.Mate, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name is required")
	}
	bots, err := c.Bots()
	if err != nil {
		return nil, err
	}
	if len(bots) == 0 {
		return nil, errors.New("no bot to take the agent from; set one up in Settings → Agent Bots first")
	}
	return c.CreateBot(mate.Mate{
		Name: name, Description: description, SystemPrompt: systemPrompt,
		AgentID: bots[0].AgentID, Model: bots[0].Model, Enabled: true,
	})
}

// TriggerSpec describes a new trigger; a schedule alone makes it a scheduled one.
type TriggerSpec struct {
	Name         string
	Prompt       string
	Schedule     string
	Events       []string
	PathPrefixes []string
}

// TriggerAdd adds a trigger to a bot and returns its id.
func TriggerAdd(c *client.Client, botID string, s TriggerSpec) (string, error) {
	if strings.TrimSpace(s.Prompt) == "" {
		return "", errors.New("prompt is required")
	}
	events := s.Events
	if len(events) == 0 && s.Schedule != "" {
		events = []string{string(mate.MateEventScheduled)}
	}
	if len(events) == 0 {
		return "", errors.New("give a schedule or events")
	}
	var id string
	err := editBot(c, botID, func(m *mate.Mate) error {
		m.Triggers = append(m.Triggers, mate.MateTrigger{
			Name: s.Name, Prompt: s.Prompt, Schedule: s.Schedule,
			EventTypes: events, PathPrefixes: s.PathPrefixes, Enabled: true,
		})
		return nil
	}, func(m *mate.Mate) { id = m.Triggers[len(m.Triggers)-1].ID })
	return id, err
}

// TriggerChange holds the fields to change; zero values keep what's there.
type TriggerChange struct {
	Name     string
	Prompt   string
	Schedule string
	Enabled  *bool
}

// TriggerUpdate changes one trigger of a bot.
func TriggerUpdate(c *client.Client, botID, triggerID string, ch TriggerChange) error {
	return editBot(c, botID, func(m *mate.Mate) error {
		i, err := triggerIndex(m, triggerID)
		if err != nil {
			return err
		}
		t := &m.Triggers[i]
		if ch.Name != "" {
			t.Name = ch.Name
		}
		if ch.Prompt != "" {
			t.Prompt = ch.Prompt
		}
		if ch.Schedule != "" {
			t.Schedule = ch.Schedule
		}
		if ch.Enabled != nil {
			t.Enabled = *ch.Enabled
		}
		return nil
	}, nil)
}

// TriggerDelete removes one trigger from a bot.
func TriggerDelete(c *client.Client, botID, triggerID string) error {
	return editBot(c, botID, func(m *mate.Mate) error {
		i, err := triggerIndex(m, triggerID)
		if err != nil {
			return err
		}
		m.Triggers = append(m.Triggers[:i], m.Triggers[i+1:]...)
		return nil
	}, nil)
}

// editBot applies edit to a fresh copy of the bot, again if someone else
// changed it in between, so neither change is lost.
func editBot(c *client.Client, botID string, edit func(*mate.Mate) error, saved func(*mate.Mate)) error {
	const attempts = 3
	for i := 0; ; i++ {
		m, err := c.Bot(botID)
		if err != nil {
			return err
		}
		if err := edit(m); err != nil {
			return err
		}
		m, err = c.UpdateBot(*m)
		if errors.Is(err, client.ErrBotConflict) && i < attempts-1 {
			continue
		}
		if err != nil {
			return err
		}
		if saved != nil {
			saved(m)
		}
		return nil
	}
}

func triggerIndex(m *mate.Mate, triggerID string) (int, error) {
	for i, t := range m.Triggers {
		if t.ID == triggerID {
			return i, nil
		}
	}
	return -1, fmt.Errorf("bot %q has no trigger %q", m.Name, triggerID)
}

// SkillsCatalog returns the skills Hylo recommends, only those in category when it's set.
func SkillsCatalog(c *client.Client, category string) ([]client.CatalogEntry, error) {
	list, err := c.SkillsCatalog()
	if err != nil {
		return nil, err
	}
	out := []client.CatalogEntry{}
	for _, s := range list {
		if category == "" || s.Category == category {
			out = append(out, s)
		}
	}
	return out, nil
}
