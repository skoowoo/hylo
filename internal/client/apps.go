package client

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/hardhacker/hylo/internal/apps"
)

// Apps calls GET /api/apps: every app, connected or not.
func (c *Client) Apps() ([]apps.Status, error) {
	var out struct {
		Apps []apps.Status `json:"apps"`
	}
	err := c.botJSONWith(http.MethodGet, "/api/apps", nil, &out, nil)
	return out.Apps, err
}

// AppTools calls GET /api/apps/{id}/tools for run's bot.
func (c *Client) AppTools(run, id string) ([]apps.Tool, error) {
	var out struct {
		Tools []apps.Tool `json:"tools"`
	}
	err := c.botJSONWith(http.MethodGet, "/api/apps/"+url.PathEscape(id)+"/tools", nil, &out, runHdr(run))
	return out.Tools, err
}

// AppCall calls POST /api/apps/{id}/call for run's bot; args is a JSON object.
func (c *Client) AppCall(run, id, tool string, args json.RawMessage) (*apps.CallResult, error) {
	if len(args) == 0 {
		args = nil // marshals as null, not as an invalid empty message
	}
	var out apps.CallResult
	long := *c
	long.http = &http.Client{Transport: c.http.Transport, Timeout: apps.CallTimeout + time.Minute}
	err := long.botJSONWith(http.MethodPost, "/api/apps/"+url.PathEscape(id)+"/call", map[string]any{"tool": tool, "args": args}, &out, runHdr(run))
	return &out, err
}
