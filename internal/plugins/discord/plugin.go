// Package discordplugin bridges Discord DMs into the mate event bus.
package discordplugin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/discord"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/plugins/chatbridge"
)

// Discord's typing indicator lasts ~10 seconds.
const typingRefreshInterval = 8 * time.Second

// Plugin implements plugin.Plugin for the Discord bridge.
type Plugin struct {
	*chatbridge.Bridge
	cfg     config.DiscordConfig
	logger  *slog.Logger
	session *discordgo.Session
}

// New creates a Discord bridge plugin. Register only when cfg.Enabled.
func New(cfg config.DiscordConfig, logger *slog.Logger) *Plugin {
	p := &Plugin{cfg: cfg, logger: logger}
	p.Bridge = chatbridge.New("discord", "Discord", discord.TextChunkLimit, cfg.UserID, p, logger)
	return p
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "discord" }

// Notify implements plugin.Plugin.
func (p *Plugin) Notify(plugin.Event) {}

// Start implements plugin.Plugin.
func (p *Plugin) Start(ctx context.Context) error {
	if p.cfg.BotToken == "" {
		p.logger.Warn("discord: bot_token not set; configure plugins.discord.bot_token")
		return nil
	}
	if p.cfg.UserID == "" {
		p.logger.Warn("discord: user_id not set; DMs are ignored until plugins.discord.user_id names you")
	}

	s, err := discord.NewSession(p.cfg.BotToken, p.cfg.ProxyURL)
	if err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	p.session = s

	return discord.RunMonitor(ctx, discord.MonitorOpts{
		Session: s,
		Log:     p.logInfo,
		OnMessage: func(m discord.Message) {
			p.Receive(m.ChannelID, m.AuthorID, m.Content)
		},
	})
}

// Stop implements plugin.Plugin.
func (p *Plugin) Stop() error { return nil }

func (p *Plugin) logInfo(msg string) {
	p.logger.Info("discord: " + msg)
}

// Format implements chatbridge.App: Discord renders Markdown itself.
func (p *Plugin) Format(text string) string { return strings.TrimSpace(text) }

// SendText implements chatbridge.App.
func (p *Plugin) SendText(_ context.Context, channelID, text string) error {
	if p.session == nil {
		return fmt.Errorf("discord: session not open")
	}
	_, err := p.session.ChannelMessageSend(channelID, text)
	return err
}

// Typing implements chatbridge.App.
func (p *Plugin) Typing(ctx context.Context, channelID string) {
	chatbridge.KeepTyping(ctx, typingRefreshInterval, func() {
		if p.session != nil {
			_ = p.session.ChannelTyping(channelID)
		}
	})
}
