// Package discordplugin bridges Discord DMs into the mate event bus.
package discordplugin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/discord"
	"github.com/hardhacker/hylo/internal/plugin"
	"log/slog"
)

// DispatchFunc fans out plugin events (typically plugin.Manager.Dispatch).
type DispatchFunc func(plugin.Event)

type atomicDispatch struct {
	mu sync.RWMutex
	fn DispatchFunc
}

func (a *atomicDispatch) Store(fn DispatchFunc) {
	a.mu.Lock()
	a.fn = fn
	a.mu.Unlock()
}

func (a *atomicDispatch) Load() DispatchFunc {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.fn
}

// Plugin implements plugin.Plugin for the Discord bridge.
type Plugin struct {
	cfg        config.DiscordConfig
	logger     *slog.Logger
	session    *discordgo.Session
	dispatchFn atomicDispatch

	mu     sync.Mutex
	queues map[string]*channelQueue
}

type channelQueue struct {
	items      []queuedMsg
	processing bool
}

type queuedMsg struct {
	channelID string
	messageID string
	userID    string
	text      string
}

// New creates a Discord bridge plugin. Register only when cfg.Enabled.
func New(cfg config.DiscordConfig, logger *slog.Logger) *Plugin {
	return &Plugin{
		cfg:    cfg,
		logger: logger,
		queues: make(map[string]*channelQueue),
	}
}

// SetDispatch wires the plugin event bus (called from server setup).
func (p *Plugin) SetDispatch(fn DispatchFunc) {
	p.dispatchFn.Store(fn)
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "discord" }

// Notify implements plugin.Plugin. Handles EventDiscordNotify to proactively send DMs.
func (p *Plugin) Notify(e plugin.Event) {
	if e.Type != plugin.EventDiscordNotify {
		return
	}
	if p.session == nil {
		p.logger.Warn("discord: notify skipped, session not open")
		return
	}
	userID := e.DiscordUserID
	if userID == "" {
		userID = p.cfg.UserID
	}
	if userID == "" || strings.TrimSpace(e.Content) == "" {
		return
	}
	go func() {
		ch, err := p.session.UserChannelCreate(userID)
		if err != nil {
			p.logger.Error("discord: failed to open DM channel", "user", userID, "err", err)
			return
		}
		if err := p.sendMessage(context.Background(), ch.ID, "", e.Content); err != nil {
			p.logger.Error("discord: notification failed", "user", userID, "err", err)
		}
	}()
}

// Start implements plugin.Plugin.
func (p *Plugin) Start(ctx context.Context) error {
	if p.cfg.BotToken == "" {
		p.logger.Warn("discord: bot_token not set; configure plugins.discord.bot_token")
		return nil
	}

	s, err := discord.NewSession(p.cfg.BotToken, p.cfg.ProxyURL)
	if err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	p.session = s

	return discord.RunMonitor(ctx, discord.MonitorOpts{
		Session:   s,
		Log:       p.logInfo,
		OnMessage: p.handleMessage,
	})
}

// Stop implements plugin.Plugin.
func (p *Plugin) Stop() error { return nil }

func (p *Plugin) logInfo(msg string) {
	p.logger.Info("discord: " + msg)
}

func (p *Plugin) handleMessage(msg discord.Message) {
	text := discord.ExtractText(msg)
	if text == "" {
		p.logger.Info("discord: skip empty message", "channel", msg.ChannelID)
		return
	}
	p.logger.Info("discord: message received", "channel", msg.ChannelID, "preview", discord.PreviewMessage(msg))
	p.enqueue(msg.ChannelID, msg.MessageID, msg.AuthorID, text)
}

func (p *Plugin) enqueue(channelID, messageID, userID, text string) {
	p.mu.Lock()
	q, ok := p.queues[channelID]
	if !ok {
		q = &channelQueue{}
		p.queues[channelID] = q
	}
	q.items = append(q.items, queuedMsg{
		channelID: channelID,
		messageID: messageID,
		userID:    userID,
		text:      text,
	})
	start := !q.processing
	q.processing = true
	p.mu.Unlock()

	if start {
		go p.processQueue(channelID)
	}
}

func (p *Plugin) processQueue(channelID string) {
	for {
		p.mu.Lock()
		q := p.queues[channelID]
		if q == nil || len(q.items) == 0 {
			if q != nil {
				q.processing = false
			}
			p.mu.Unlock()
			return
		}
		item := q.items[0]
		q.items = q.items[1:]
		p.mu.Unlock()

		p.dispatchDiscordMessage(item)
	}
}

func (p *Plugin) dispatchDiscordMessage(item queuedMsg) {
	dispatch := p.dispatchFn.Load()
	if dispatch == nil {
		p.logger.Warn("discord: dispatch not wired, dropping message", "channel", item.channelID)
		return
	}

	reply := p.makeReplyFunc(item.channelID)
	dispatch(plugin.Event{
		Type:            plugin.EventDiscordMessage,
		Path:            "/discord/" + item.channelID,
		Content:         item.text,
		Time:            time.Now(),
		DiscordChannelID: item.channelID,
		DiscordUserID:   item.userID,
		DiscordMessageID: item.messageID,
		Reply:           reply,
	})
}

func (p *Plugin) makeReplyFunc(channelID string) plugin.ReplyFunc {
	stopTyping := p.startTypingIndicator(channelID)
	return func(ctx context.Context, result plugin.ReplyResult) error {
		stopTyping()

		text := result.Text
		if strings.TrimSpace(text) == "" {
			if result.Status == "failed" {
				text = "⚠️ Agent run failed"
			} else {
				return nil
			}
		}
		return p.sendMessage(ctx, channelID, "", text)
	}
}

func (p *Plugin) sendMessage(ctx context.Context, channelID, replyToMessageID, text string) error {
	if p.session == nil {
		return fmt.Errorf("discord: session not open")
	}
	formatted := discord.FormatForDiscord(text)
	segments := discord.SplitText(formatted, discord.TextChunkLimit)
	for i, seg := range segments {
		var data *discordgo.MessageSend
		if i == 0 && replyToMessageID != "" {
			data = &discordgo.MessageSend{
				Content: seg,
				Reference: &discordgo.MessageReference{
					MessageID: replyToMessageID,
					ChannelID: channelID,
				},
			}
		} else {
			data = &discordgo.MessageSend{Content: seg}
		}
		if _, err := p.session.ChannelMessageSendComplex(channelID, data); err != nil {
			return err
		}
	}
	return nil
}
