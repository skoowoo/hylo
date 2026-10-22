package channels

import (
	"context"
	"log/slog"
	"time"

	"github.com/hardhacker/hylo/internal/discord"
	"github.com/hardhacker/hylo/internal/mate"

	"github.com/bwmarrin/discordgo"
)

const discordTypingEvery = 8 * time.Second

type discordLink struct {
	session *discordgo.Session
	log     *slog.Logger
	on      func(chatID, userID, text string)
}

func openDiscord(ch mate.Channel, log *slog.Logger, on func(chatID, userID, text string)) (runner, error) {
	s, err := discord.NewSession(ch.Token, ch.ProxyURL)
	if err != nil {
		return nil, err
	}
	return &discordLink{session: s, log: log, on: on}, nil
}

// Run hands messages to one worker, so they are handled in order like the
// other apps. discordgo calls handlers on their own goroutines, and blocking
// its reader for a whole agent run would stall the gateway heartbeat.
func (d *discordLink) Run(ctx context.Context) error {
	queue := make(chan discord.Message, 32)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-queue:
				d.on(m.ChannelID, m.AuthorID, m.Content)
			}
		}
	}()
	return discord.RunMonitor(ctx, discord.MonitorOpts{
		Session: d.session,
		Log:     func(msg string) { d.log.Info("discord: " + msg) },
		OnMessage: func(m discord.Message) {
			select {
			case queue <- m:
			case <-ctx.Done():
			}
		},
	})
}

func (d *discordLink) SendText(_ context.Context, channelID, text string) error {
	_, err := d.session.ChannelMessageSend(channelID, text)
	return err
}

func (d *discordLink) Typing(ctx context.Context, channelID string) {
	keepTyping(ctx, discordTypingEvery, func() {
		_ = d.session.ChannelTyping(channelID)
	})
}
