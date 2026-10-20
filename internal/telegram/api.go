// Package telegram is a minimal Telegram Bot API client: long polling for
// private messages and plain-text replies.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// TextChunkLimit is the longest message Telegram accepts, in UTF-16 units.
const TextChunkLimit = 4096

const apiBase = "https://api.telegram.org"

// User is a Telegram account.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// Chat is where a message was sent; Type is "private" for a DM.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// Message is an inbound message; Text is empty for media.
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

// Update is one item of getUpdates; only messages are requested.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// APIError is an error the Bot API answered with.
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram: %d %s", e.Code, e.Description)
}

// IsConflict reports whether another getUpdates poller holds the token.
func IsConflict(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Code == http.StatusConflict
}

// IsUnauthorized reports whether the bot token was rejected.
func IsUnauthorized(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Code == http.StatusUnauthorized
}

// Client calls the Bot API with one bot token.
type Client struct {
	token string
	base  string
	http  *http.Client
}

// NewClient creates a client; proxyURL is optional (e.g. "http://127.0.0.1:7890").
func NewClient(token, proxyURL string) (*Client, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		pu, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("telegram: invalid proxy_url %q: %w", proxyURL, err)
		}
		tr.Proxy = http.ProxyURL(pu)
	}
	return &Client{token: token, base: apiBase, http: &http.Client{Transport: tr}}, nil
}

func (c *Client) call(ctx context.Context, method string, params, out any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// The URL carries the token; keep it out of logs.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("telegram: %s: %w", method, err)
	}
	defer resp.Body.Close()
	var env struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("telegram: %s: HTTP %d", method, resp.StatusCode)
	}
	if !env.OK {
		return &APIError{Code: env.ErrorCode, Description: env.Description}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Result, out)
}

// GetMe returns the bot's own account; it also checks the token.
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	err := c.call(ctx, "getMe", struct{}{}, &u)
	return u, err
}

// DeleteWebhook turns off webhook delivery, which would block getUpdates.
func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.call(ctx, "deleteWebhook", struct{}{}, nil)
}

// GetUpdates long-polls for messages after offset, waiting up to timeout.
// Asking with an offset confirms every update before it.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]Update, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout+15*time.Second)
	defer cancel()
	var ups []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         int(timeout.Seconds()),
		"allowed_updates": []string{"message"},
	}, &ups)
	return ups, err
}

// SendMessage sends plain text to a chat.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id":              chatID,
		"text":                 text,
		"link_preview_options": map[string]any{"is_disabled": true},
	}, nil)
}

// SendTyping shows "typing…" in a chat for about five seconds.
func (c *Client) SendTyping(ctx context.Context, chatID int64) error {
	return c.call(ctx, "sendChatAction", map[string]any{"chat_id": chatID, "action": "typing"}, nil)
}

// BotCommand is an entry of the bot's command menu.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// SetMyCommands sets the command menu Telegram shows in the bot's chat.
func (c *Client) SetMyCommands(ctx context.Context, cmds []BotCommand) error {
	return c.call(ctx, "setMyCommands", map[string]any{"commands": cmds}, nil)
}
