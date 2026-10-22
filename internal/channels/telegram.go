package channels

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/telegram"
)

const telegramTypingEvery = 4 * time.Second

type telegramLink struct {
	client *telegram.Client
	log    *slog.Logger
	on     func(chatID, userID, text string)
}

func openTelegram(ch mate.Channel, log *slog.Logger, on func(chatID, userID, text string)) (runner, error) {
	c, err := telegram.NewClient(ch.Token, ch.ProxyURL)
	if err != nil {
		return nil, err
	}
	return &telegramLink{client: c, log: log, on: on}, nil
}

func (t *telegramLink) Run(ctx context.Context) error {
	if err := t.client.SetMyCommands(ctx, []telegram.BotCommand{{
		Command: "new", Description: "Start a new conversation",
	}}); err != nil && ctx.Err() == nil {
		t.log.Warn("telegram: setMyCommands failed", "err", err)
	}
	return telegram.RunMonitor(ctx, telegram.MonitorOpts{
		Client: t.client,
		Log:    func(msg string) { t.log.Info("telegram: " + msg) },
		OnMessage: func(m *telegram.Message) {
			// /start is how a chat is opened, including the pairing deep link.
			if _, ok := telegram.StartParam(m); ok {
				return
			}
			t.on(strconv.FormatInt(m.Chat.ID, 10), strconv.FormatInt(m.From.ID, 10), m.Text)
		},
	})
}

func (t *telegramLink) SendText(ctx context.Context, chatID, text string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: bad chat id %q", chatID)
	}
	return t.client.SendMessage(ctx, id, text)
}

func (t *telegramLink) Typing(ctx context.Context, chatID string) {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return
	}
	keepTyping(ctx, telegramTypingEvery, func() {
		_ = t.client.SendTyping(ctx, id)
	})
}
