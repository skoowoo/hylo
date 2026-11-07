package view

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/pulse"
)

type pulseJournal interface {
	ListPulses(mateID string, limit int) ([]mate.PulseRun, error)
	OpenAgenda(mateID string) ([]mate.AgendaItem, error)
}

// SetPulse lets /dev/pulse show the main bot's rounds; p is nil while they are off.
func (vh *ViewHandler) SetPulse(p *pulse.Pulse, j pulseJournal, desk *desktop.Tracker) {
	vh.pulse, vh.journal, vh.desk = p, j, desk
}

// PulsePage renders the main bot's pacing, journal and agenda (/dev/pulse).
func (vh *ViewHandler) PulsePage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{"On": vh.pulse != nil, "Error": r.URL.Query().Get("error")}
	if vh.pulse != nil && vh.journal != nil {
		st, m := vh.pulse.Status()
		data["Status"] = st
		data["Chance"] = fmt.Sprintf("%.2f%%", st.Odds.Chance*100)
		if m != nil {
			rounds, err := vh.journal.ListPulses(m.ID, 60)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			agenda, err := vh.journal.OpenAgenda(m.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			data["Rounds"], data["Agenda"] = rounds, agenda
		}
	}
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "page_pulse.html", data); err != nil {
		http.Error(w, fmt.Sprintf("render: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

var pulseRunGuard = http.NewCrossOriginProtection()

// PulseRunPOST handles POST /dev/pulse/run: a round now, outside the odds.
func (vh *ViewHandler) PulseRunPOST(w http.ResponseWriter, r *http.Request) {
	if err := pulseRunGuard.Check(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	to := "/dev/pulse"
	switch err := vh.runPulse(); {
	case errors.Is(err, pulse.ErrBusy):
		to += "?error=busy"
	case err != nil:
		to += "?error=" + url.QueryEscape(err.Error())
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// PulseClearDesktopPOST handles POST /dev/pulse/clear-desktop: forget every desktop session.
func (vh *ViewHandler) PulseClearDesktopPOST(w http.ResponseWriter, r *http.Request) {
	if err := pulseRunGuard.Check(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	to := "/dev/pulse"
	if vh.desk == nil {
		to += "?error=" + url.QueryEscape("bots are unavailable")
	} else if err := vh.desk.Clear(); err != nil {
		to += "?error=" + url.QueryEscape(err.Error())
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (vh *ViewHandler) runPulse() error {
	if vh.pulse == nil {
		return errors.New("rounds are off; set proactive.enabled and restart")
	}
	return vh.pulse.RunNow()
}
