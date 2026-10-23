package channels

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/wechat"
)

const (
	wechatTypingEvery = 5 * time.Second
	typingTicketTTL   = 24 * time.Hour
)

type typingTicket struct {
	ticket    string
	expiresAt time.Time
}

type wechatLink struct {
	baseURL string
	token   string
	owner   string
	client  *wechat.Client
	dir     string
	log     *slog.Logger
	on      func(chatID, userID, text string, fetch imageFetch)

	mu           sync.Mutex
	contextToken string

	typingMu sync.Mutex
	tickets  map[string]typingTicket
}

func openWechat(ch mate.Channel, dir string, proxy config.Proxy, log *slog.Logger, on func(chatID, userID, text string, fetch imageFetch)) (runner, error) {
	base := ch.BaseURL
	if base == "" {
		base = wechat.DefaultBaseURL
	}
	w := &wechatLink{
		baseURL: base,
		token:   ch.Token,
		owner:   ch.OwnerUserID,
		client:  wechat.NewClient(proxy),
		dir:     dir,
		log:     log,
		on:      on,
		tickets: map[string]typingTicket{},
	}
	w.contextToken = wechat.LoadContextToken(dir)
	return w, nil
}

func (w *wechatLink) Run(ctx context.Context) error {
	return wechat.RunMonitor(ctx, wechat.MonitorOpts{
		BaseURL:    w.baseURL,
		Token:      w.token,
		StorageDir: w.dir,
		Client:     w.client,
		Log:        func(msg string) { w.log.Info("wechat: " + msg) },
		OnMessage:  w.receive,
	})
}

func (w *wechatLink) receive(msg wechat.Message) {
	if msg.MessageType != wechat.MessageTypeUser || msg.GroupID != "" {
		return
	}
	if msg.FromUserID == "" || msg.ContextToken == "" {
		return
	}
	text := wechat.ExtractText(msg)
	images := wechat.Images(msg)
	if text == "" && len(images) == 0 {
		return
	}
	if msg.FromUserID == w.owner {
		w.remember(msg.ContextToken)
	}
	var fetch imageFetch
	if len(images) > 0 {
		fetch = func(ctx context.Context, dir string) ([]string, string, error) {
			return w.fetchImages(ctx, dir, images)
		}
	}
	w.on(msg.FromUserID, msg.FromUserID, text, fetch)
}

func (w *wechatLink) fetchImages(ctx context.Context, dir string, items []wechat.ImageItem) ([]string, string, error) {
	items, notice := capImages(items)
	var paths []string
	for _, item := range items {
		query := ""
		if item.Media != nil {
			query = item.Media.EncryptQueryParam
		}
		body, err := w.client.DownloadCDN(ctx, query)
		if err != nil {
			return nil, "", errImageFetch
		}
		plain, err := wechat.DecodeImage(item, body)
		if err != nil {
			return nil, "", errImageFetch
		}
		abs, err := saveChannelImage(dir, plain, "", "")
		if err != nil {
			return nil, "", err
		}
		paths = append(paths, abs)
	}
	return paths, notice, nil
}

func (w *wechatLink) remember(ct string) {
	w.mu.Lock()
	changed := w.contextToken != ct
	w.contextToken = ct
	w.mu.Unlock()
	if changed {
		if err := wechat.SaveContextToken(w.dir, ct); err != nil {
			w.log.Warn("wechat: failed to persist context token", "err", err)
		}
	}
}

func (w *wechatLink) context() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.contextToken
}

func (w *wechatLink) SendText(ctx context.Context, userID, text string) error {
	_, err := wechat.SendTextMessage(ctx, userID, text, wechat.SendOpts{
		BaseURL:      w.baseURL,
		Token:        w.token,
		ContextToken: w.context(),
		Client:       w.client,
	})
	return err
}

func (w *wechatLink) Typing(ctx context.Context, userID string) {
	ct := w.context()
	keepTyping(ctx, wechatTypingEvery, func() {
		w.sendTyping(ctx, userID, ct, wechat.TypingStatusTyping)
	})
	// The typing state outlives the refreshes unless it is cancelled.
	cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	w.sendTyping(cancelCtx, userID, ct, wechat.TypingStatusCancel)
}

func (w *wechatLink) sendTyping(ctx context.Context, userID, contextToken string, status int) {
	ticket, ok := w.typingTicket(ctx, userID, contextToken)
	if !ok {
		return
	}
	_ = w.client.SendTyping(ctx, w.baseURL, w.token, wechat.SendTypingReq{
		ILinkUserID:  userID,
		TypingTicket: ticket,
		Status:       status,
	})
}

func (w *wechatLink) typingTicket(ctx context.Context, userID, contextToken string) (string, bool) {
	w.typingMu.Lock()
	if cached, ok := w.tickets[userID]; ok && cached.expiresAt.After(time.Now()) {
		ticket := cached.ticket
		w.typingMu.Unlock()
		return ticket, true
	}
	w.typingMu.Unlock()

	resp, err := w.client.GetConfig(ctx, w.baseURL, w.token, userID, contextToken)
	if err != nil || resp == nil || resp.TypingTicket == "" {
		return "", false
	}
	w.typingMu.Lock()
	w.tickets[userID] = typingTicket{ticket: resp.TypingTicket, expiresAt: time.Now().Add(typingTicketTTL)}
	w.typingMu.Unlock()
	return resp.TypingTicket, true
}
