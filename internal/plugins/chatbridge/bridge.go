// Package chatbridge is what the chat app plugins (WeChat, Discord, Telegram)
// share: only the owner's messages reach the bots, and each one gets a reply
// callback that shows typing until the bot answers.
package chatbridge

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

// App is the part of a chat app protocol the bridge drives.
type App interface {
	// Format adapts an agent reply to what the app renders.
	Format(text string) string
	// SendText sends one message, already within the bridge's limit.
	SendText(ctx context.Context, chatID, text string) error
	// Typing shows the typing state in chatID until ctx is done.
	Typing(ctx context.Context, chatID string)
}

// Bridge turns a chat app's inbound messages into EventChatMessage events.
type Bridge struct {
	name   string // plugin name, also the event's Channel
	label  string
	limit  int // longest message the app accepts, in UTF-16 units
	owner  string
	app    App
	logger *slog.Logger

	dispatch atomic.Pointer[func(plugin.Event)]
}

// New creates a bridge that answers only owner; with no owner it answers no one.
func New(name, label string, limit int, owner string, app App, logger *slog.Logger) *Bridge {
	return &Bridge{name: name, label: label, limit: limit, owner: owner, app: app, logger: logger}
}

// SetDispatch wires the plugin event bus (called from server setup).
func (b *Bridge) SetDispatch(fn func(plugin.Event)) {
	b.dispatch.Store(&fn)
}

// Receive hands a direct message to the bots; the reply goes back to chatID.
func (b *Bridge) Receive(chatID, userID, text string) {
	if b.owner == "" || userID != b.owner {
		// Anyone can message a bot account; never let a stranger drive the agent.
		b.logger.Warn(b.name+": dropped message from a non-owner", "user", userID)
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	fn := b.dispatch.Load()
	if fn == nil {
		b.logger.Warn(b.name+": dispatch not wired, dropping message", "chat", chatID)
		return
	}
	b.logger.Info(b.name+": message received", "chat", chatID, "preview", preview(text))
	(*fn)(plugin.Event{
		Type:    plugin.EventChatMessage,
		Channel: b.name,
		Path:    "/" + b.name + "/" + chatID,
		Content: text,
		Time:    time.Now(),
		UserID:  userID,
		ChatID:  chatID,
		Reply:   b.reply(chatID),
	})
}

func (b *Bridge) reply(chatID string) plugin.ReplyFunc {
	typingCtx, stopTyping := context.WithCancel(context.Background())
	go b.app.Typing(typingCtx, chatID)
	return func(ctx context.Context, r plugin.ReplyResult) error {
		stopTyping()
		text := strings.TrimSpace(r.Text)
		switch {
		case r.Status == "unhandled":
			text = fmt.Sprintf("No bot answers %s messages yet. Add a %s_message trigger to one in Hylo.", b.label, b.name)
		case text == "" && r.Status == "failed":
			text = "⚠️ Agent run failed"
		case text == "":
			return nil
		}
		return b.Send(ctx, chatID, text)
	}
}

// Send delivers text to chatID, split into as many messages as the app needs.
func (b *Bridge) Send(ctx context.Context, chatID, text string) error {
	for _, seg := range SplitText(b.app.Format(text), b.limit) {
		if err := b.app.SendText(ctx, chatID, seg); err != nil {
			return err
		}
	}
	return nil
}

func preview(text string) string {
	if r := []rune(text); len(r) > 50 {
		return string(r[:50]) + "..."
	}
	return text
}

// KeepTyping calls show now and every interval until ctx is done; apps drop
// the typing state after a few seconds, so it has to be renewed.
func KeepTyping(ctx context.Context, every time.Duration, show func()) {
	show()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			show()
		}
	}
}
