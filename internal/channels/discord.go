package channels

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/discord"
	"github.com/hardhacker/hylo/internal/mate"

	"github.com/bwmarrin/discordgo"
)

const discordTypingEvery = 8 * time.Second

type discordLink struct {
	session *discordgo.Session
	owner   string
	log     *slog.Logger
	on      func(chatID, userID, text string, fetch imageFetch)

	mu sync.Mutex
	dm string
}

func openDiscord(ch mate.Channel, proxy config.Proxy, log *slog.Logger, on func(chatID, userID, text string, fetch imageFetch)) (runner, error) {
	s, err := discord.NewSession(ch.Token, proxy)
	if err != nil {
		return nil, err
	}
	return &discordLink{session: s, owner: ch.OwnerUserID, log: log, on: on}, nil
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
				d.deliver(m)
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

func (d *discordLink) deliver(m discord.Message) {
	var atts []discord.Attachment
	for _, a := range m.Attachments {
		if maybeImage(a.ContentType, a.Name) {
			atts = append(atts, a)
		}
	}
	var fetch imageFetch
	if len(atts) > 0 {
		fetch = func(ctx context.Context, dir string) ([]string, string, error) {
			return d.fetchAttachments(ctx, dir, atts)
		}
	}
	d.on(m.ChannelID, m.AuthorID, m.Content, fetch)
}

func (d *discordLink) fetchAttachments(ctx context.Context, dir string, atts []discord.Attachment) ([]string, string, error) {
	atts, notice := capImages(atts)
	var paths []string
	for _, a := range atts {
		if rejectedImage(a.ContentType, a.Name) {
			return nil, "", errImageType
		}
		if a.Size > maxChannelImageBytes {
			return nil, "", errImageTooBig
		}
		body, err := d.download(ctx, a.URL)
		if err != nil {
			return nil, "", errImageFetch
		}
		abs, err := saveChannelImage(dir, body, a.ContentType, a.Name)
		if err != nil {
			return nil, "", err
		}
		paths = append(paths, abs)
	}
	return paths, notice, nil
}

func (d *discordLink) download(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	client := http.DefaultClient
	if d.session != nil && d.session.Client != nil {
		client = d.session.Client
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errImageFetch
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxChannelImageBytes+1))
}

func (d *discordLink) SendText(_ context.Context, channelID, text string) error {
	_, err := d.session.ChannelMessageSend(channelID, text)
	return err
}

// OwnerChat opens, or finds, the DM channel with the owner.
func (d *discordLink) OwnerChat(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dm != "" {
		return d.dm, nil
	}
	c, err := d.session.UserChannelCreate(d.owner, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}
	d.dm = c.ID
	return d.dm, nil
}

func (d *discordLink) Typing(ctx context.Context, channelID string) {
	keepTyping(ctx, discordTypingEvery, func() {
		_ = d.session.ChannelTyping(channelID)
	})
}
