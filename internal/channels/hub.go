// Package channels runs each Hylo bot's own WeChat, Discord and Telegram
// contacts. A message from one of them is a turn in that bot's current chat,
// the same conversation the desktop is in. It is not a vault event.
package channels

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/discord"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/telegram"
	"github.com/hardhacker/hylo/internal/wechat"
)

// Chatter runs one user turn in a bot's current chat and returns the reply.
// images are absolute paths under the upload directory; the caption stays text.
type Chatter interface {
	Deliver(ctx context.Context, mateID, app, text string, images []string) (reply, status string)
}

// outbound is one live app contact.
type outbound interface {
	SendText(ctx context.Context, chatID, text string) error
	Typing(ctx context.Context, chatID string)
}

type runner interface {
	outbound
	Run(ctx context.Context) error
}

// Hub starts and stops one connection per binding.
type Hub struct {
	store  *mate.Store
	root   string
	upload string
	log    *slog.Logger
	proxy  config.Proxy
	chat   Chatter
	kick   chan struct{}

	mu      sync.Mutex
	running map[string]*live
	held    map[string]*heldImages
}

// imageCaptionWait holds a picture that arrived by itself. On a channel the
// caption is often the next message, so the picture waits to go out with that
// text. A message that already has text is submitted at once.
var imageCaptionWait = 2 * time.Second

type heldImages struct {
	ctx    context.Context
	ch     mate.Channel
	chatID string
	out    outbound
	paths  []string
	timer  *time.Timer
}

type live struct {
	fp     string
	cancel context.CancelFunc
	done   chan struct{}
}

func NewHub(store *mate.Store, vaultRoot, uploadDir string, logger *slog.Logger, proxy config.Proxy) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		store:   store,
		root:    vaultRoot,
		proxy:   proxy,
		upload:  uploadDir,
		log:     logger,
		kick:    make(chan struct{}, 1),
		running: map[string]*live{},
		held:    map[string]*heldImages{},
	}
}

// SetChatter wires the bot chat runtime. Call it before the hub starts.
func (h *Hub) SetChatter(c Chatter) { h.chat = c }

func (h *Hub) Name() string { return "channels" }

func (h *Hub) Notify(plugin.Event) {}

// Start reconciles in this one goroutine, so an old connection is gone before
// its replacement starts and callers of Reload never wait on a stop.
func (h *Hub) Start(ctx context.Context) error {
	h.reconcile(ctx)
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			all := h.running
			h.running = map[string]*live{}
			h.mu.Unlock()
			for _, l := range all {
				l.stop()
			}
			return nil
		case <-h.kick:
			h.reconcile(ctx)
		}
	}
}

func (h *Hub) Stop() error { return nil }

// Reload asks the hub to match live connections to the bindings of enabled bots.
func (h *Hub) Reload() {
	select {
	case h.kick <- struct{}{}:
	default:
	}
}

func (h *Hub) reconcile(ctx context.Context) {
	want, err := h.store.ListActiveChannels()
	if err != nil {
		h.log.Warn("channels: list", "err", err)
		return
	}
	h.mu.Lock()
	have := make(map[string]string, len(h.running))
	for id, l := range h.running {
		have[id] = l.fp
	}
	stopIDs, start := plan(have, want)
	stopping := make([]*live, 0, len(stopIDs))
	for _, id := range stopIDs {
		stopping = append(stopping, h.running[id])
		delete(h.running, id)
	}
	h.mu.Unlock()
	for _, l := range stopping {
		l.stop()
	}
	h.sweep()
	for _, c := range start {
		h.spawn(ctx, c)
	}
}

// sweep removes the state dirs of bindings that no longer exist. A disabled
// bot keeps its binding, and with it the dir.
func (h *Hub) sweep() {
	dir := h.stateDir("")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	all, err := h.store.ListChannels()
	if err != nil {
		return
	}
	keep := make(map[string]bool, len(all))
	for _, c := range all {
		keep[c.ID] = true
	}
	h.mu.Lock()
	for id := range h.running {
		keep[id] = true
	}
	h.mu.Unlock()
	for _, e := range entries {
		if !keep[e.Name()] {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

func (h *Hub) stateDir(id string) string {
	return filepath.Join(h.root, ".hylo", "channels", id)
}

func (h *Hub) spawn(parent context.Context, ch mate.Channel) {
	ctx, cancel := context.WithCancel(parent)
	var link runner
	var err error
	on := func(chatID, userID, text string, fetch imageFetch) {
		h.handle(ctx, ch, chatID, userID, text, fetch, link)
	}
	switch ch.Kind {
	case mate.ChannelTelegram:
		link, err = openTelegram(ch, h.proxy, h.log, on)
	case mate.ChannelDiscord:
		link, err = openDiscord(ch, h.proxy, h.log, on)
	case mate.ChannelWechat:
		link, err = openWechat(ch, h.stateDir(ch.ID), h.proxy, h.log, on)
	default:
		cancel()
		return
	}
	if err != nil {
		cancel()
		h.log.Warn("channels: open", "kind", ch.Kind, "mate", ch.MateID, "err", err)
		return
	}
	l := &live{fp: fingerprint(ch), cancel: cancel, done: make(chan struct{})}
	h.mu.Lock()
	h.running[ch.ID] = l
	h.mu.Unlock()
	go func() {
		err := link.Run(ctx)
		close(l.done)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, wechat.ErrSessionExpired) {
			// The delete reloads the hub, which retires this link with its binding.
			h.log.Info("channels: wechat disconnected", "mate", ch.MateID)
			if derr := h.store.DeleteChannelByID(ch.ID); derr != nil {
				h.log.Warn("channels: drop wechat", "err", derr)
			}
			return
		}
		if err != nil {
			h.log.Error("channels: stopped", "kind", ch.Kind, "mate", ch.MateID, "err", err)
		}
	}()
}

func (l *live) stop() {
	l.cancel()
	select {
	case <-l.done:
	case <-time.After(8 * time.Second):
	}
}

func fingerprint(c mate.Channel) string {
	return strings.Join([]string{c.Token, c.OwnerUserID, c.ExternalID, c.BaseURL}, "\x00")
}

// plan decides which live connections to drop and which bindings to start.
// have maps a channel id to the fingerprint it was started with.
func plan(have map[string]string, want []mate.Channel) (stop []string, start []mate.Channel) {
	next := make(map[string]mate.Channel, len(want))
	for _, c := range want {
		next[c.ID] = c
		if have[c.ID] != fingerprint(c) {
			if _, ok := have[c.ID]; ok {
				stop = append(stop, c.ID)
			}
			start = append(start, c)
		}
	}
	for id := range have {
		if _, ok := next[id]; !ok {
			stop = append(stop, id)
		}
	}
	return stop, start
}

func (h *Hub) handle(ctx context.Context, ch mate.Channel, chatID, userID, text string, fetch imageFetch, out outbound) {
	text = strings.TrimSpace(text)
	if text == "" && fetch == nil {
		return
	}
	if ch.OwnerUserID == "" || userID != ch.OwnerUserID {
		h.log.Warn("channels: dropped message from a non-owner", "kind", ch.Kind, "user", userID)
		return
	}
	if isNewChat(text) {
		h.takeImages(ch, chatID)
		h.reset(ctx, ch, chatID, out)
		return
	}
	if h.chat == nil {
		h.log.Warn("channels: chat runtime not wired", "kind", ch.Kind)
		return
	}
	var paths []string
	if fetch != nil {
		var notice string
		var err error
		paths, notice, err = fetch(ctx, h.upload)
		if err != nil {
			h.log.Warn("channels: image", "kind", ch.Kind, "err", err)
			_ = h.send(ctx, out, ch.Kind, chatID, imageProblem(err))
			return
		}
		if notice != "" {
			_ = h.send(ctx, out, ch.Kind, chatID, notice)
		}
	}
	if text == "" && len(paths) == 0 {
		return
	}
	var notice string
	if text == "" {
		notice = h.holdImages(ctx, ch, chatID, paths, out)
	} else {
		paths, notice = capImages(append(h.takeImages(ch, chatID), paths...))
	}
	if notice != "" {
		_ = h.send(ctx, out, ch.Kind, chatID, notice)
	}
	if text == "" {
		return
	}
	h.finish(ctx, ch, chatID, text, paths, out)
}

func (h *Hub) finish(ctx context.Context, ch mate.Channel, chatID, text string, paths []string, out outbound) {
	shown := text
	if shown == "" {
		shown = "(image)"
	}
	h.log.Info("channels: message", "kind", ch.Kind, "mate", ch.MateID, "preview", preview(shown))
	typingCtx, stop := context.WithCancel(ctx)
	go out.Typing(typingCtx, chatID)
	reply, status := h.chat.Deliver(ctx, ch.MateID, ch.Kind, text, paths)
	stop()
	switch status {
	case "succeeded":
		if strings.TrimSpace(reply) == "" {
			return
		}
	case "canceled":
		return
	default:
		if strings.TrimSpace(reply) == "" {
			reply = "Agent run failed"
		}
	}
	if err := h.send(ctx, out, ch.Kind, chatID, reply); err != nil {
		h.log.Warn("channels: send", "kind", ch.Kind, "err", err)
	}
}

func pendingKey(ch mate.Channel, chatID string) string {
	return ch.MateID + "\x00" + ch.Kind + "\x00" + chatID
}

// holdImages keeps a captionless picture until a text message arrives or the
// wait ends. The notice is set once the chat is past the image cap.
func (h *Hub) holdImages(ctx context.Context, ch mate.Channel, chatID string, paths []string, out outbound) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := pendingKey(ch, chatID)
	p := h.held[key]
	if p == nil {
		p = &heldImages{}
		h.held[key] = p
	}
	p.ctx, p.ch, p.chatID, p.out = ctx, ch, chatID, out
	var notice string
	p.paths, notice = capImages(append(p.paths, paths...))
	if p.timer != nil {
		p.timer.Stop()
	}
	p.timer = time.AfterFunc(imageCaptionWait, func() { h.flushImages(key, p) })
	return notice
}

func (h *Hub) takeImages(ch mate.Channel, chatID string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := pendingKey(ch, chatID)
	p := h.held[key]
	if p == nil {
		return nil
	}
	p.timer.Stop()
	delete(h.held, key)
	return p.paths
}

// flushImages sends p unless a text took it first. Identity, not a counter:
// a batch held after p was taken reuses the key.
func (h *Hub) flushImages(key string, p *heldImages) {
	h.mu.Lock()
	if h.held[key] != p {
		h.mu.Unlock()
		return
	}
	delete(h.held, key)
	h.mu.Unlock()
	if p.ctx.Err() != nil {
		return
	}
	h.finish(p.ctx, p.ch, p.chatID, "", p.paths, p.out)
}

func (h *Hub) reset(ctx context.Context, ch mate.Channel, chatID string, out outbound) {
	title := time.Now().Format("2006-01-02 15:04")
	if _, err := h.store.EnsureNewConv(ch.MateID, title, 10); err != nil {
		h.log.Warn("channels: new chat", "err", err)
		_ = h.send(ctx, out, ch.Kind, chatID, "对话重置失败，请稍后再试")
		return
	}
	_ = h.send(ctx, out, ch.Kind, chatID, "新对话已开启")
}

func (h *Hub) send(ctx context.Context, out outbound, kind, chatID, text string) error {
	text = unwrapCards(text)
	formatted := strings.TrimSpace(text)
	limit := discord.TextChunkLimit
	switch kind {
	case mate.ChannelTelegram:
		formatted = plainText(text)
		limit = telegram.TextChunkLimit
	case mate.ChannelWechat:
		formatted = plainText(text)
		limit = wechat.TextChunkLimit
	}
	for _, seg := range splitText(formatted, limit) {
		if strings.TrimSpace(seg) == "" {
			continue
		}
		if err := out.SendText(ctx, chatID, seg); err != nil {
			return err
		}
	}
	return nil
}

func isNewChat(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "/new" || t == "/clear"
}

func preview(text string) string {
	if r := []rune(text); len(r) > 50 {
		return string(r[:50]) + "..."
	}
	return text
}
