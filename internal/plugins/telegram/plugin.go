// Package telegramplugin bridges Telegram private messages into the mate event bus.
package telegramplugin

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/plugins/chatbridge"
	"github.com/hardhacker/hylo/internal/telegram"
)

// Telegram shows "typing…" for five seconds per call.
const typingRefreshInterval = 4 * time.Second

// Plugin implements plugin.Plugin for the Telegram bridge.
type Plugin struct {
	*chatbridge.Bridge
	cfg    config.TelegramConfig
	logger *slog.Logger
	client *telegram.Client
}

// New creates a Telegram bridge plugin. Register only when cfg.Enabled.
func New(cfg config.TelegramConfig, logger *slog.Logger) *Plugin {
	p := &Plugin{cfg: cfg, logger: logger}
	p.Bridge = chatbridge.New("telegram", "Telegram", telegram.TextChunkLimit, cfg.UserID, p, logger)
	return p
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "telegram" }

// Notify implements plugin.Plugin.
func (p *Plugin) Notify(plugin.Event) {}

// Start implements plugin.Plugin.
func (p *Plugin) Start(ctx context.Context) error {
	if p.cfg.BotToken == "" || p.cfg.UserID == "" {
		p.logger.Warn("telegram: not paired; connect Telegram in Settings → Server → Config → Telegram")
		return nil
	}
	c, err := telegram.NewClient(p.cfg.BotToken, p.cfg.ProxyURL)
	if err != nil {
		return err
	}
	p.client = c
	if err := c.SetMyCommands(ctx, []telegram.BotCommand{{Command: "new", Description: "Start a new conversation"}}); err != nil && ctx.Err() == nil {
		p.logger.Warn("telegram: setMyCommands failed", "err", err)
	}
	return telegram.RunMonitor(ctx, telegram.MonitorOpts{
		Client:    c,
		Log:       func(msg string) { p.logger.Info("telegram: " + msg) },
		OnMessage: p.handleMessage,
	})
}

// Stop implements plugin.Plugin.
func (p *Plugin) Stop() error { return nil }

func (p *Plugin) handleMessage(m *telegram.Message) {
	// /start is how Telegram opens a chat, not something to answer; pairing
	// already used its payload before this plugin ran.
	if _, ok := telegram.StartParam(m); ok {
		return
	}
	p.Receive(strconv.FormatInt(m.Chat.ID, 10), strconv.FormatInt(m.From.ID, 10), m.Text)
}

// Format implements chatbridge.App: replies go out as plain text, since a
// stray character in Telegram's Markdown gets the whole message rejected.
func (p *Plugin) Format(text string) string { return chatbridge.PlainText(text) }

// SendText implements chatbridge.App.
func (p *Plugin) SendText(ctx context.Context, chatID, text string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: bad chat id %q", chatID)
	}
	if p.client == nil {
		return fmt.Errorf("telegram: not started")
	}
	return p.client.SendMessage(ctx, id, text)
}

// Typing implements chatbridge.App.
func (p *Plugin) Typing(ctx context.Context, chatID string) {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil || p.client == nil {
		return
	}
	chatbridge.KeepTyping(ctx, typingRefreshInterval, func() {
		_ = p.client.SendTyping(ctx, id)
	})
}
