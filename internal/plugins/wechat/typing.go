package wechatplugin

import (
	"context"
	"time"

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

// startTypingIndicators sends typing immediately and refreshes every typingRefreshInterval
// until the returned cancel function is called.
func (p *Plugin) startTypingIndicators(userID, contextToken string) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		p.sendTypingStatus(ctx, userID, contextToken, wechat.TypingStatusTyping)
		ticker := time.NewTicker(typingRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.sendTypingStatus(ctx, userID, contextToken, wechat.TypingStatusTyping)
			}
		}
	}()
	return cancel
}

func (p *Plugin) cancelTypingIndicators(userID, contextToken string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.sendTypingStatus(ctx, userID, contextToken, wechat.TypingStatusCancel)
}
