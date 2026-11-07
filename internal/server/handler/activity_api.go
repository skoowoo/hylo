package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

// ActivityAPI records what the user does with notes; bots' activity is logged
// by the runs themselves.
type ActivityAPI struct {
	store *mate.Store
}

func NewActivityAPI(store *mate.Store) *ActivityAPI {
	return &ActivityAPI{store: store}
}

// VisitPOST handles POST /api/activity/visit: the note in front of the user,
// reported when the visit counts and again when it ends.
func (a *ActivityAPI) VisitPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path   string `json:"path"`
		Action string `json:"action"`
		At     int64  `json:"at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(body.Path, "/") || !strings.HasSuffix(strings.ToLower(body.Path), ".md") || body.At <= 0 {
		http.Error(w, "invalid visit", http.StatusBadRequest)
		return
	}
	// The client's clock may run ahead of the server's.
	at := min(body.At, time.Now().UnixMilli())
	if err := a.store.RecordUserVisit(body.Path, body.Action, time.UnixMilli(at)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ActivityEntry is one touch of a note, with who made it.
type ActivityEntry struct {
	Path   string    `json:"path"`
	Actor  string    `json:"actor"`           // "user", or the bot's name
	BotID  string    `json:"botId,omitempty"` // set for a bot
	Action string    `json:"action"`          // read | write
	At     time.Time `json:"at"`
}

// ActivityGET handles GET /api/activity?since=&actor=&limit=: who read and
// wrote which notes, newest first. since is a duration back ("6h", "3d") or a
// local "YYYY-MM-DD HH:MM", default a day; actor is "user" or a bot id.
func (a *ActivityAPI) ActivityGET(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	now := time.Now()
	from, err := parseSince(q.Get("since"), now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	limit := 100
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = min(n, 500)
	}
	rows, err := a.store.RecentNoteActivity(from, now.Add(time.Second), q.Get("actor"), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bots, err := a.store.ListMates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	names := map[string]string{}
	for _, b := range bots {
		names[b.ID] = b.Name
	}
	out := make([]ActivityEntry, 0, len(rows))
	for _, row := range rows {
		e := ActivityEntry{Path: row.Path, Actor: row.Actor, Action: row.Action, At: row.At}
		if row.Actor != mate.ActorUser {
			e.BotID, e.Actor = row.Actor, names[row.Actor]
		}
		out = append(out, e)
	}
	respondJSON(w, http.StatusOK, map[string]any{"entries": out})
}

func parseSince(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return now.Add(-24 * time.Hour), nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		if n, err := strconv.Atoi(days); err == nil && n > 0 {
			return now.AddDate(0, 0, -n), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return now.Add(-d), nil
	}
	if t, err := parseDue(s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("since %q: want a duration like 6h or 3d, or YYYY-MM-DD HH:MM", s)
}
