package discord

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

// NewSession creates a discordgo session configured with the required Gateway intents.
// proxyURL is optional (e.g. "socks5://127.0.0.1:1080"); pass "" to use direct connection.
// The caller is responsible for calling session.Open() via RunMonitor.
func NewSession(token, proxyURL string) (*discordgo.Session, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord: create session: %w", err)
	}
	// DirectMessages: receive DMs.
	// MessageContent: privileged intent — must be enabled in the Developer Portal.
	s.Identify.Intents = discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent
	s.Identify.Presence = discordgo.GatewayStatusUpdate{Status: "online"}

	if proxyURL != "" {
		pu, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("discord: invalid proxy_url %q: %w", proxyURL, err)
		}
		transport := &http.Transport{Proxy: http.ProxyURL(pu)}
		s.Client = &http.Client{Transport: transport}
		// WebSocket gateway also needs to go through the proxy.
		s.Dialer = &websocket.Dialer{
			Proxy:            http.ProxyURL(pu),
			HandshakeTimeout: 45 * time.Second,
		}
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
// until ctx is cancelled. discordgo handles reconnection automatically.
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
		if content == "" {
			return
		}
		opts.OnMessage(Message{
			ChannelID: m.ChannelID,
			AuthorID:  m.Author.ID,
			Content:   content,
		})
	})

	opts.Session.AddHandler(func(_ *discordgo.Session, r *discordgo.Ready) {
		log(fmt.Sprintf("Connected as %s#%s", r.User.Username, r.User.Discriminator))
	})

	if err := opts.Session.Open(); err != nil {
		return fmt.Errorf("discord: open gateway: %w", err)
	}
	log("Discord bridge started")

	<-ctx.Done()

	log("Discord bridge stopping")
	return opts.Session.Close()
}
