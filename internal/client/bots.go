package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hardhacker/hylo/internal/mate"
)

// ErrBotConflict means the bot changed between reading and updating it.
var ErrBotConflict = errors.New("bot changed concurrently")

// Bots calls GET /api/mates.
func (c *Client) Bots() ([]mate.Mate, error) {
	var out struct {
		Mates []mate.Mate `json:"mates"`
	}
	if err := c.botJSON(http.MethodGet, "/api/mates", nil, &out); err != nil {
		return nil, err
	}
	return out.Mates, nil
}

// Bot calls GET /api/mates/{id}; the bot comes with its triggers.
func (c *Client) Bot(id string) (*mate.Mate, error) {
	var out struct {
		Mate mate.Mate `json:"mate"`
	}
	if err := c.botJSON(http.MethodGet, "/api/mates/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out.Mate, nil
}

// CreateBot calls POST /api/mates.
func (c *Client) CreateBot(m mate.Mate) (*mate.Mate, error) {
	var out struct {
		Mate mate.Mate `json:"mate"`
	}
	if err := c.botJSON(http.MethodPost, "/api/mates", m, &out); err != nil {
		return nil, err
	}
	return &out.Mate, nil
}

// UpdateBot calls PUT /api/mates/{id}, which replaces the bot's triggers with m.Triggers.
// It fails with ErrBotConflict if the bot changed since m was read.
func (c *Client) UpdateBot(m mate.Mate) (*mate.Mate, error) {
	var out struct {
		Mate mate.Mate `json:"mate"`
	}
	var hdr http.Header
	if !m.UpdatedAt.IsZero() {
		hdr = http.Header{"If-Match": {strconv.FormatInt(m.UpdatedAt.UnixMilli(), 10)}}
	}
	if err := c.botJSONWith(http.MethodPut, "/api/mates/"+url.PathEscape(m.ID), m, &out, hdr); err != nil {
		return nil, err
	}
	return &out.Mate, nil
}

// RunTrigger calls POST /api/mates/{id}/triggers/{tid}/run.
func (c *Client) RunTrigger(botID, triggerID string) error {
	return c.botJSON(http.MethodPost, "/api/mates/"+url.PathEscape(botID)+"/triggers/"+url.PathEscape(triggerID)+"/run", nil, nil)
}

func (c *Client) botJSON(method, path string, body, out any) error {
	return c.botJSONWith(method, path, body, out, nil)
}

func (c *Client) botJSONWith(method, path string, body, out any, hdr http.Header) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.baseURL+path, r)
	if err != nil {
		return err
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.do(req)
	if err != nil {
		return wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusConflict {
			return fmt.Errorf("%w: %s", ErrBotConflict, statusMsg(resp.StatusCode, raw))
		}
		return fmt.Errorf("%s %s: %s", method, path, statusMsg(resp.StatusCode, raw))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
