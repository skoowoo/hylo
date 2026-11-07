package client

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
)

// BotChat is a bot's current chat as another bot reads it.
type BotChat struct {
	Bot      string          `json:"bot"`
	ChatID   string          `json:"chatId"`
	Messages []mate.ChatLine `json:"messages"`
	HasMore  bool            `json:"hasMore"`
}

// BotChat calls GET /api/mates/{id}/chat.
func (c *Client) BotChat(botID, before string, limit int) (BotChat, error) {
	q := url.Values{}
	if before != "" {
		q.Set("before", before)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out BotChat
	err := c.botJSON(http.MethodGet, "/api/mates/"+url.PathEscape(botID)+"/chat?"+q.Encode(), nil, &out)
	return out, err
}

// ActivityEntry is one touch of a note, with who made it.
type ActivityEntry struct {
	Path   string `json:"path"`
	Actor  string `json:"actor"`
	BotID  string `json:"botId,omitempty"`
	Action string `json:"action"`
	At     string `json:"at"`
}

// Activity calls GET /api/activity.
func (c *Client) Activity(since, actor string, limit int) ([]ActivityEntry, error) {
	q := url.Values{}
	if since != "" {
		q.Set("since", since)
	}
	if actor != "" {
		q.Set("actor", actor)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out struct {
		Entries []ActivityEntry `json:"entries"`
	}
	err := c.botJSON(http.MethodGet, "/api/activity?"+q.Encode(), nil, &out)
	return out.Entries, err
}

// ScreenTime calls GET /api/desktop/screen-time.
func (c *Client) ScreenTime(since string) (desktop.ScreenTime, error) {
	q := url.Values{}
	if since != "" {
		q.Set("since", since)
	}
	var out desktop.ScreenTime
	err := c.botJSON(http.MethodGet, "/api/desktop/screen-time?"+q.Encode(), nil, &out)
	return out, err
}
