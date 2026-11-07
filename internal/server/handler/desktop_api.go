package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
)

// DesktopAPI takes the desktop app's presence reports and serves what they add up to.
type DesktopAPI struct {
	tracker *desktop.Tracker
}

func NewDesktopAPI(t *desktop.Tracker) *DesktopAPI {
	return &DesktopAPI{tracker: t}
}

// PresencePOST handles POST /api/desktop/presence: one sample from the desktop app.
// While reports aren't kept it answers 410, and the app asks again much later.
func (d *DesktopAPI) PresencePOST(w http.ResponseWriter, r *http.Request) {
	if !d.tracker.Enabled() {
		http.Error(w, "desktop presence is off", http.StatusGone)
		return
	}
	var s desktop.Sample
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := d.tracker.Report(s, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const screenApps, screenTimeline = 15, 30

// ScreenTimeGET handles GET /api/desktop/screen-time?since=: time per category
// and app since then (default midnight), and the latest stretches.
func (d *DesktopAPI) ScreenTimeGET(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if q := r.URL.Query().Get("since"); q != "" {
		t, err := parseSince(q, now)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		from = t
	}
	out := desktop.ScreenTime{Enabled: d.tracker.Enabled(), Apps: []desktop.Total{}, Timeline: []mate.DesktopSession{}}
	st := d.tracker.Now(now)
	out.Now = desktop.ScreenNow{Known: st.Known, Present: st.Present, App: st.App,
		AwaySince: st.AwaySince, BackAt: st.BackAt, AwayMinutes: int(st.AwayFor.Minutes())}
	if st.Present {
		out.Now.InAppMinutes = int(now.Sub(st.AppSince).Minutes())
	}
	ss, err := d.tracker.Sessions(from, now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if apps := desktop.Totals(ss); len(apps) > 0 {
		out.Apps = apps[:min(len(apps), screenApps)]
	}
	if tl := desktop.Timeline(ss, screenTimeline); len(tl) > 0 {
		out.Timeline = tl
	}
	respondJSON(w, http.StatusOK, out)
}

// DesktopDELETE handles DELETE /api/desktop: forget every desktop session.
func (d *DesktopAPI) DesktopDELETE(w http.ResponseWriter, r *http.Request) {
	if err := d.tracker.Clear(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
