package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Bot is a bot as apps see it.
type Bot struct {
	ID, Name string
	Main     bool
	Dir      string // its working directory in the vault, where its skills go
}

// BotRef names a bot an app is on for.
type BotRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// botState keeps where the user moved an app off its default for a bot: on
// for the main bot, off for the rest, so a new bot loads no app until asked.
// It lives in the vault, with the bots; credentials stay on the machine.
type botState struct {
	Bots map[string]map[string]bool `json:"bots"`
}

func loadBotState(path string) (botState, error) {
	st := botState{Bots: map[string]map[string]bool{}}
	if path == "" {
		return st, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, fmt.Errorf("%s: %w", path, err)
	}
	if st.Bots == nil {
		st.Bots = map[string]map[string]bool{}
	}
	return st, nil
}

// saveBots is called with m.mu held.
func (m *Manager) saveBots() error {
	if m.stateFile == "" {
		return nil
	}
	b, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.stateFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.stateFile, b, 0o644)
}

func (m *Manager) isOn(b Bot, app string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if on, ok := m.state.Bots[b.ID][app]; ok {
		return on
	}
	return b.Main
}

// listBots is every bot; ok is false when they can't be listed, so nothing
// is taken out of folders for a bot list that only looks empty.
func (m *Manager) listBots() (bots []Bot, ok bool) {
	if m.bots == nil {
		return nil, false
	}
	bots, err := m.bots()
	if err != nil {
		m.logger.Warn("apps: list bots", "err", err)
		return nil, false
	}
	return bots, true
}

func (m *Manager) bot(id string) (Bot, error) {
	bots, _ := m.listBots()
	for _, b := range bots {
		if b.ID == id {
			return b, nil
		}
	}
	return Bot{}, fmt.Errorf("no bot %q", id)
}

// Allows reports whether bot id may use app.
func (m *Manager) Allows(botID, app string) bool {
	b, err := m.bot(botID)
	return err == nil && m.isOn(b, app)
}

// SetOn turns app on or off for bot id, and moves its skill to match.
func (m *Manager) SetOn(botID, app string, on bool) error {
	if _, err := m.app(app); err != nil {
		return err
	}
	b, err := m.bot(botID)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if on == b.Main {
		delete(m.state.Bots[b.ID], app)
		if len(m.state.Bots[b.ID]) == 0 {
			delete(m.state.Bots, b.ID)
		}
	} else {
		if m.state.Bots[b.ID] == nil {
			m.state.Bots[b.ID] = map[string]bool{}
		}
		m.state.Bots[b.ID][app] = on
	}
	err = m.saveBots()
	m.mu.Unlock()
	if err != nil {
		return err
	}
	m.Reconcile()
	m.changed()
	return nil
}

// botsOn names the bots app is on for.
func (m *Manager) botsOn(app string) []BotRef {
	out := []BotRef{}
	bots, _ := m.listBots()
	for _, b := range bots {
		if m.isOn(b, app) {
			out = append(out, BotRef{ID: b.ID, Name: b.Name})
		}
	}
	return out
}

// Reconcile puts each skill app's skill in exactly the folders of the bots it
// is on for: none while the app isn't usable, and none for a bot that is
// gone. It touches only what differs, and runs whenever apps, bots or their
// switches change.
func (m *Manager) Reconcile() {
	bots, ok := m.listBots()
	if !ok {
		return
	}
	m.mu.Lock()
	gone := false
	for id := range m.state.Bots {
		if !slices.ContainsFunc(bots, func(b Bot) bool { return b.ID == id }) {
			delete(m.state.Bots, id)
			gone = true
		}
	}
	if gone {
		if err := m.saveBots(); err != nil {
			m.logger.Warn("apps: save bot switches", "err", err)
		}
	}
	m.mu.Unlock()
	for i := range m.catalog {
		a := &m.catalog[i]
		if a.Kind != KindSkill {
			continue
		}
		want := map[string]bool{}
		if m.creds.get(a.ID).usable() && m.skills.Has(a.Skill) {
			for _, b := range bots {
				if m.isOn(b, a.ID) {
					want[b.Dir] = true
				}
			}
		}
		have := m.skills.EnabledDirs(a.Skill)
		for dir := range want {
			if !slices.Contains(have, dir) {
				m.skillErr(a, dir, m.skills.Enable(a.Skill, dir))
			}
		}
		for _, dir := range have {
			if !want[dir] {
				m.skillErr(a, dir, m.skills.Disable(a.Skill, dir))
			}
		}
	}
}

func (m *Manager) skillErr(a *App, dir string, err error) {
	if err != nil {
		m.logger.Warn("apps: move skill", "app", a.ID, "dir", dir, "err", err)
	}
}
