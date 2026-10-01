// Package wechatplugin bridges WeChat iLink direct messages into the mate event bus.
package wechatplugin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/wechat"
)

// DispatchFunc fans out plugin events (typically plugin.Manager.Dispatch).
type DispatchFunc func(plugin.Event)

// Plugin implements plugin.Plugin for the WeChat bridge.
type Plugin struct {
	cfg        config.WechatConfig
	logger     *slog.Logger
	client     *wechat.Client
	dispatchFn atomicDispatch

	token      *wechat.TokenData
	storageDir string

	mu               sync.Mutex
	queues           map[string]*userQueue
	lastContextToken string

	typingMu      sync.Mutex
	typingTickets map[string]typingTicketCache
}

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

type userQueue struct {
	items      []queuedMsg
	processing bool
}

type queuedMsg struct {
	userID       string
	contextToken string
	text         string
}

// New creates a WeChat bridge plugin. Register only when cfg.Enabled.
func New(cfg config.WechatConfig, logger *slog.Logger) *Plugin {
	return &Plugin{
		cfg:           cfg,
		logger:        logger,
		client:        wechat.NewClient(),
		queues:        make(map[string]*userQueue),
		typingTickets: make(map[string]typingTicketCache),
	}
}

// SetDispatch wires the plugin event bus (called from server setup).
func (p *Plugin) SetDispatch(fn DispatchFunc) {
	p.dispatchFn.Store(fn)
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "wechat" }

// Notify implements plugin.Plugin. Handles EventWechatNotify to proactively send messages.
func (p *Plugin) Notify(e plugin.Event) {
	if e.Type != plugin.EventWechatNotify {
		return
	}
	if p.token == nil {
		p.logger.Warn("wechat: notify skipped, not logged in")
		return
	}
	userID := e.WechatUserID
	if userID == "" {
		userID = p.token.UserID
	}
	p.mu.Lock()
	contextToken := p.lastContextToken
	p.mu.Unlock()
	if userID == "" || contextToken == "" || strings.TrimSpace(e.Content) == "" {
		if contextToken == "" {
			p.logger.Warn("wechat: notify skipped, no context token yet (user must send a message first)")
		}
		return
	}
	go func() {
		if err := p.sendReply(context.Background(), userID, contextToken, e.Content); err != nil {
			p.logger.Error("wechat: notification failed", "user", userID, "err", err)
		}
	}()
}

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
	if msg.MessageType != wechat.MessageTypeUser {
		return
	}
	if msg.GroupID != "" {
		return
	}
	userID := msg.FromUserID
	contextToken := msg.ContextToken
	if userID == "" || contextToken == "" {
		return
	}

	text := wechat.ExtractText(msg)
	if text == "" {
		p.logger.Info("wechat: skip non-text message", "user", userID, "preview", wechat.PreviewMessage(msg))
		return
	}

	p.mu.Lock()
	if p.lastContextToken != contextToken {
		p.lastContextToken = contextToken
		p.mu.Unlock()
		if err := wechat.SaveContextToken(p.storageDir, contextToken); err != nil {
			p.logger.Warn("wechat: failed to persist context token", "err", err)
		}
	} else {
		p.mu.Unlock()
	}

	p.logger.Info("wechat: message received", "user", userID, "preview", wechat.PreviewMessage(msg))
	p.enqueue(userID, contextToken, text)
}

func (p *Plugin) enqueue(userID, contextToken, text string) {
	p.mu.Lock()
	q, ok := p.queues[userID]
	if !ok {
		q = &userQueue{}
		p.queues[userID] = q
	}
	q.items = append(q.items, queuedMsg{userID: userID, contextToken: contextToken, text: text})
	start := !q.processing
	q.processing = true
	p.mu.Unlock()

	if start {
		go p.processQueue(userID)
	}
}

func (p *Plugin) processQueue(userID string) {
	for {
		p.mu.Lock()
		q := p.queues[userID]
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

		p.dispatchWechatMessage(item)
	}
}

func (p *Plugin) dispatchWechatMessage(item queuedMsg) {
	dispatch := p.dispatchFn.Load()
	if dispatch == nil {
		p.logger.Warn("wechat: dispatch not wired, dropping message", "user", item.userID)
		return
	}

	reply := p.makeReplyFunc(item.userID, item.contextToken)
	dispatch(plugin.Event{
		Type:         plugin.EventWechatMessage,
		Path:         "/wechat/" + item.userID,
		Content:      item.text,
		Time:         time.Now(),
		WechatUserID: item.userID,
		Reply:        reply,
	})
}

func (p *Plugin) makeReplyFunc(userID, contextToken string) plugin.ReplyFunc {
	stopTyping := p.startTypingIndicators(userID, contextToken)
	return func(ctx context.Context, result plugin.ReplyResult) error {
		stopTyping()
		defer p.cancelTypingIndicators(userID, contextToken)

		if p.token == nil {
			return fmt.Errorf("wechat: not logged in")
		}
		text := result.Text
		if strings.TrimSpace(text) == "" {
			if result.Status == "failed" {
				text = "⚠️ Agent run failed"
			} else {
				return nil
			}
		}
		return p.sendReply(ctx, userID, contextToken, text)
	}
}

func (p *Plugin) sendReply(ctx context.Context, userID, contextToken, text string) error {
	formatted := wechat.FormatForWeChat(text)
	segments := wechat.SplitText(formatted, wechat.TextChunkLimit)
	for _, seg := range segments {
		if _, err := wechat.SendTextMessage(ctx, userID, seg, wechat.SendOpts{
			BaseURL:      p.token.BaseURL,
			Token:        p.token.Token,
			ContextToken: contextToken,
			Client:       p.client,
		}); err != nil {
			return err
		}
	}
	return nil
}
