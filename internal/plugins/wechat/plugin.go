// Package wechatplugin bridges WeChat iLink direct messages into the mate event bus.
package wechatplugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/plugins/chatbridge"
	"github.com/hardhacker/hylo/internal/wechat"
)

// Plugin implements plugin.Plugin for the WeChat bridge.
type Plugin struct {
	*chatbridge.Bridge
	cfg    config.WechatConfig
	logger *slog.Logger
	client *wechat.Client

	token      *wechat.TokenData
	storageDir string

	mu               sync.Mutex
	lastContextToken string

	typingMu      sync.Mutex
	typingTickets map[string]typingTicketCache
}

// New creates a WeChat bridge plugin. Register only when cfg.Enabled.
func New(cfg config.WechatConfig, logger *slog.Logger) *Plugin {
	p := &Plugin{
		cfg:           cfg,
		logger:        logger,
		client:        wechat.NewClient(),
		typingTickets: make(map[string]typingTicketCache),
	}
	// The logged-in account is the owner: QR login binds the bot to whoever scanned.
	p.Bridge = chatbridge.New("wechat", "WeChat", wechat.TextChunkLimit, cfg.UserID, p, logger)
	return p
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "wechat" }

// Notify implements plugin.Plugin.
func (p *Plugin) Notify(plugin.Event) {}

// Start implements plugin.Plugin.
func (p *Plugin) Start(ctx context.Context) error {
	td := wechat.TokenFromFields(
		p.cfg.Token, p.cfg.BaseURL, p.cfg.AccountID, p.cfg.UserID, p.cfg.SavedAt,
	)
	if td == nil {
		p.logger.Warn("wechat: not logged in; connect WeChat in Settings → Config → WeChat")
		return nil
	}
	p.token = td
	storageDir := wechat.DefaultStorageDir()
	p.storageDir = storageDir

	if ct := wechat.LoadContextToken(storageDir); ct != "" {
		p.mu.Lock()
		p.lastContextToken = ct
		p.mu.Unlock()
		p.logger.Info("wechat: context token restored from disk")
	}

	p.logger.Info("wechat: bridge starting", "bot", td.AccountID)

	return wechat.RunMonitor(ctx, wechat.MonitorOpts{
		BaseURL:    td.BaseURL,
		Token:      td.Token,
		StorageDir: storageDir,
		Client:     p.client,
		Log:        p.logInfo,
		OnMessage:  p.handleMessage,
	})
}

// Stop implements plugin.Plugin.
func (p *Plugin) Stop() error { return nil }

func (p *Plugin) logInfo(msg string) {
	p.logger.Info("wechat: " + msg)
}

func (p *Plugin) handleMessage(msg wechat.Message) {
	if msg.MessageType != wechat.MessageTypeUser || msg.GroupID != "" {
		return
	}
	if msg.FromUserID == "" || msg.ContextToken == "" {
		return
	}
	text := wechat.ExtractText(msg)
	if text == "" {
		p.logger.Info("wechat: skip empty message", "user", msg.FromUserID)
		return
	}
	if msg.FromUserID == p.cfg.UserID {
		p.rememberContextToken(msg.ContextToken)
	}
	p.Receive(msg.FromUserID, msg.FromUserID, text)
}

// rememberContextToken keeps the newest token: iLink needs one to send anything.
func (p *Plugin) rememberContextToken(ct string) {
	p.mu.Lock()
	changed := p.lastContextToken != ct
	p.lastContextToken = ct
	p.mu.Unlock()
	if changed {
		if err := wechat.SaveContextToken(p.storageDir, ct); err != nil {
			p.logger.Warn("wechat: failed to persist context token", "err", err)
		}
	}
}

func (p *Plugin) contextToken() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastContextToken
}

// Format implements chatbridge.App: WeChat shows plain text.
func (p *Plugin) Format(text string) string { return chatbridge.PlainText(text) }

// SendText implements chatbridge.App.
func (p *Plugin) SendText(ctx context.Context, userID, text string) error {
	if p.token == nil {
		return fmt.Errorf("wechat: not logged in")
	}
	_, err := wechat.SendTextMessage(ctx, userID, text, wechat.SendOpts{
		BaseURL:      p.token.BaseURL,
		Token:        p.token.Token,
		ContextToken: p.contextToken(),
		Client:       p.client,
	})
	return err
}
