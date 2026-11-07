package client

import (
	"net/http"
	"net/url"

	"github.com/hardhacker/hylo/internal/mate"
)

const runHeader = "X-Hylo-Run"

func runHdr(run string) http.Header {
	if run == "" {
		return nil
	}
	return http.Header{runHeader: {run}}
}

// Agenda calls GET /api/agenda: the open items of run's bot.
func (c *Client) Agenda(run string) ([]mate.AgendaItem, error) {
	var out struct {
		Items []mate.AgendaItem `json:"items"`
	}
	err := c.botJSONWith(http.MethodGet, "/api/agenda", nil, &out, runHdr(run))
	return out.Items, err
}

// AgendaAdd calls POST /api/agenda.
func (c *Client) AgendaAdd(run, text, due string) (mate.AgendaItem, error) {
	var out mate.AgendaItem
	err := c.botJSONWith(http.MethodPost, "/api/agenda", map[string]string{"text": text, "due": due}, &out, runHdr(run))
	return out, err
}

// AgendaDone calls POST /api/agenda/{id}/done.
func (c *Client) AgendaDone(run, id string) error {
	return c.botJSONWith(http.MethodPost, "/api/agenda/"+url.PathEscape(id)+"/done", nil, nil, runHdr(run))
}
