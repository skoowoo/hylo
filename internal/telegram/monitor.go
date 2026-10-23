package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const pollTimeout = 50 * time.Second

// MonitorOpts configures the long-polling listener.
type MonitorOpts struct {
	Client    *Client
	Log       func(string)
	OnMessage func(*Message) // private text messages from people, not bots
}

// RunMonitor long-polls getUpdates until ctx is cancelled, retrying with
// backoff when Telegram is unreachable.
func RunMonitor(ctx context.Context, opts MonitorOpts) error {
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	if opts.Client == nil || opts.OnMessage == nil {
		return fmt.Errorf("telegram: Client and OnMessage are required")
	}
	if err := opts.Client.DeleteWebhook(ctx); err != nil && ctx.Err() == nil {
		log("deleteWebhook failed: " + err.Error())
	}
	log("Telegram bridge started")

	var offset int64
	backoff := time.Second
	for {
		ups, err := opts.Client.GetUpdates(ctx, offset, pollTimeout)
		if ctx.Err() != nil {
			log("Telegram bridge stopping")
			return nil
		}
		if err != nil {
			switch {
			case IsConflict(err):
				log("another program is polling this bot token; stop it or use another bot")
			case IsUnauthorized(err):
				log("bot token rejected; reconnect Telegram in Settings")
			default:
				log("getUpdates failed: " + err.Error())
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, time.Minute)
			continue
		}
		backoff = time.Second
		for _, u := range ups {
			offset = u.UpdateID + 1
			if m := u.Message; isPrivateTurn(m) {
				opts.OnMessage(m)
			}
		}
	}
}

func isPrivatePerson(m *Message) bool {
	return m != nil && m.From != nil && !m.From.IsBot && m.Chat.Type == "private"
}

func isPrivateText(m *Message) bool {
	return isPrivatePerson(m) && strings.TrimSpace(m.Text) != ""
}

// isPrivateTurn is a DM that carries text, a caption, a photo, or a file; the
// bot decides which files are pictures.
func isPrivateTurn(m *Message) bool {
	if !isPrivatePerson(m) {
		return false
	}
	if strings.TrimSpace(m.Text) != "" || strings.TrimSpace(m.Caption) != "" {
		return true
	}
	return len(m.Photo) > 0 || (m.Document != nil && m.Document.FileID != "")
}

// StartParam returns the payload of a "/start <payload>" deep-link message.
func StartParam(m *Message) (string, bool) {
	if !isPrivateText(m) {
		return "", false
	}
	cmd, arg, _ := strings.Cut(strings.TrimSpace(m.Text), " ")
	if cmd != "/start" {
		return "", false
	}
	return strings.TrimSpace(arg), true
}
