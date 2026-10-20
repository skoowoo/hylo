package wechatplugin

import (
	"context"
	"time"

	"github.com/hardhacker/hylo/internal/plugins/chatbridge"
	"github.com/hardhacker/hylo/internal/wechat"
)

const (
	typingRefreshInterval = 5 * time.Second
	typingTicketTTL       = 24 * time.Hour
)

type typingTicketCache struct {
	ticket    string
	expiresAt time.Time
}

func (p *Plugin) getTypingTicket(ctx context.Context, userID, contextToken string) (string, bool) {
	p.typingMu.Lock()
	if cached, ok := p.typingTickets[userID]; ok && cached.expiresAt.After(time.Now()) {
		ticket := cached.ticket
		p.typingMu.Unlock()
		return ticket, true
	}
	p.typingMu.Unlock()

	if p.token == nil {
		return "", false
	}
	resp, err := p.client.GetConfig(ctx, p.token.BaseURL, p.token.Token, userID, contextToken)
	if err != nil || resp == nil || resp.TypingTicket == "" {
		return "", false
	}

	p.typingMu.Lock()
	p.typingTickets[userID] = typingTicketCache{
		ticket:    resp.TypingTicket,
		expiresAt: time.Now().Add(typingTicketTTL),
	}
	p.typingMu.Unlock()
	return resp.TypingTicket, true
}

func (p *Plugin) sendTypingStatus(ctx context.Context, userID, contextToken string, status int) {
	if p.token == nil {
		return
	}
	ticket, ok := p.getTypingTicket(ctx, userID, contextToken)
	if !ok {
		return
	}
	_ = p.client.SendTyping(ctx, p.token.BaseURL, p.token.Token, wechat.SendTypingReq{
		ILinkUserID:  userID,
		TypingTicket: ticket,
		Status:       status,
	})
}

// Typing implements chatbridge.App.
func (p *Plugin) Typing(ctx context.Context, userID string) {
	ct := p.contextToken()
	chatbridge.KeepTyping(ctx, typingRefreshInterval, func() {
		p.sendTypingStatus(ctx, userID, ct, wechat.TypingStatusTyping)
	})
	// The typing state outlives our refreshes unless it is cancelled.
	cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.sendTypingStatus(cancelCtx, userID, ct, wechat.TypingStatusCancel)
}
