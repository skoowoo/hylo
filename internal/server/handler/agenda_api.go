package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

// RunHeader names the agent run a tool call comes from (mcp.runHeader).
const RunHeader = "X-Hylo-Run"

// agendaBot is the bot whose agenda or rounds a call works on: the main bot.
// A call naming a run comes from a bot (every agent process carries its run),
// so only the main bot's own runs pass; one without is the user's.
func (a *AgentAPI) agendaBot(r *http.Request) (*mate.Mate, int, error) {
	if a.store == nil {
		return nil, http.StatusServiceUnavailable, errors.New("bots are unavailable")
	}
	m, err := a.store.MainBot()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if m == nil {
		return nil, http.StatusNotFound, errors.New("there is no main bot")
	}
	if run := r.Header.Get(RunHeader); run != "" {
		v, ok := a.runBots.Load(run)
		if !ok {
			return nil, http.StatusForbidden, errors.New("the run is over")
		}
		if v.(string) != m.ID {
			return nil, http.StatusForbidden, errors.New("only the main bot keeps an agenda")
		}
	}
	return m, 0, nil
}

// AgendaGET handles GET /api/agenda: the bot's open items.
func (a *AgentAPI) AgendaGET(w http.ResponseWriter, r *http.Request) {
	m, code, err := a.agendaBot(r)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	items, err := a.store.OpenAgenda(m.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []mate.AgendaItem{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AgendaPOST handles POST /api/agenda {text, due}: due is local "YYYY-MM-DD HH:MM" or RFC 3339.
func (a *AgentAPI) AgendaPOST(w http.ResponseWriter, r *http.Request) {
	m, code, err := a.agendaBot(r)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	var body struct {
		Text string `json:"text"`
		Due  string `json:"due"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	due, err := parseDue(body.Due)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	it, err := a.store.AddAgenda(m.ID, body.Text, due)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, it)
}

// AgendaDonePOST handles POST /api/agenda/{id}/done.
func (a *AgentAPI) AgendaDonePOST(w http.ResponseWriter, r *http.Request) {
	m, code, err := a.agendaBot(r)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	err = a.store.FinishAgenda(m.ID, r.PathValue("id"))
	if errors.Is(err, mate.ErrAgendaItemNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func parseDue(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("due %q: want YYYY-MM-DD HH:MM local time", s)
}
