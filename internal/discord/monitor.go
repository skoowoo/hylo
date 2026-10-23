package discord

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"

	"github.com/hardhacker/hylo/internal/config"
)

// Identify checks a bot token and returns the bot's user id and username.
func Identify(token string, proxy config.Proxy) (id, username string, err error) {
	s, err := NewSession(token, proxy)
	if err != nil {
		return "", "", err
	}
	u, err := s.User("@me")
	if err != nil {
		return "", "", err
	}
	if u == nil || u.ID == "" {
		return "", "", fmt.Errorf("discord: empty bot user")
	}
	return u.ID, u.Username, nil
}

// NewSession creates a discordgo session configured with the required Gateway intents.
// HTTP and the gateway websocket both use proxy. The caller opens the session via RunMonitor.
func NewSession(token string, proxy config.Proxy) (*discordgo.Session, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord: create session: %w", err)
	}
	// DirectMessages: receive DMs.
	// MessageContent: privileged intent — must be enabled in the Developer Portal.
	s.Identify.Intents = discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent
	s.Identify.Presence = discordgo.GatewayStatusUpdate{Status: "online"}

	s.Client = &http.Client{Transport: proxy.Transport()}
	s.Dialer = &websocket.Dialer{
		Proxy:            proxy.Func(),
		HandshakeTimeout: 45 * time.Second,
	}

	return s, nil
}

// MonitorOpts configures the Discord Gateway listener.
type MonitorOpts struct {
	Session   *discordgo.Session
	Log       func(string)
	OnMessage func(Message)
}

// RunMonitor registers message handlers, opens the Gateway connection, and blocks
// until ctx is cancelled. A failed first connect is retried with backoff;
// after that discordgo handles reconnection.
func RunMonitor(ctx context.Context, opts MonitorOpts) error {
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	if opts.Session == nil {
		return fmt.Errorf("discord: Session is required")
	}
	if opts.OnMessage == nil {
		return fmt.Errorf("discord: OnMessage is required")
	}

	opts.Session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}
		if m.GuildID != "" {
			return
		}
		content := strings.TrimSpace(m.Content)
		atts := attachments(m.Attachments)
		if content == "" && len(atts) == 0 {
			return
		}
		opts.OnMessage(Message{
			ChannelID:   m.ChannelID,
			AuthorID:    m.Author.ID,
			Content:     content,
			Attachments: atts,
		})
	})

	opts.Session.AddHandler(func(_ *discordgo.Session, r *discordgo.Ready) {
		log(fmt.Sprintf("Connected as %s#%s", r.User.Username, r.User.Discriminator))
	})

	backoff := time.Second
	for {
		err := opts.Session.Open()
		if err == nil {
			break
		}
		log("open gateway failed: " + err.Error())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, time.Minute)
	}
	log("Discord bridge started")

	<-ctx.Done()

	log("Discord bridge stopping")
	return opts.Session.Close()
}

// attachments passes every file on; the bot decides which ones are pictures.
func attachments(in []*discordgo.MessageAttachment) []Attachment {
	var out []Attachment
	for _, a := range in {
		if a == nil || a.URL == "" {
			continue
		}
		out = append(out, Attachment{
			URL: a.URL, Name: a.Filename, ContentType: a.ContentType, Size: a.Size,
		})
	}
	return out
}
