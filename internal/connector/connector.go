// Package connector maintains the home computer's outbound Relay tunnel.
package connector

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/relayproto"
)

const (
	defaultPoolSize     = 4
	controlPingInterval = 20 * time.Second
	controlReadTimeout  = 60 * time.Second
	stableOnlineWindow  = 5 * time.Second
)

type Connector struct {
	cfg    config.RelayConfig
	target string
	log    *slog.Logger
	dialer websocket.Dialer
	mu     sync.RWMutex
	status Status
}

type Status struct {
	State       string    `json:"state"`
	Error       string    `json:"error,omitempty"`
	ConnectedAt time.Time `json:"connected_at,omitempty"`
}

func New(cfg config.RelayConfig, target string, logger *slog.Logger) *Connector {
	if logger == nil {
		logger = slog.Default()
	}
	return &Connector{
		cfg:    cfg,
		target: target,
		log:    logger,
		dialer: websocket.Dialer{HandshakeTimeout: 15 * time.Second},
		status: Status{State: "connecting"},
	}
}

func (c *Connector) Status() Status     { c.mu.RLock(); defer c.mu.RUnlock(); return c.status }
func (c *Connector) setStatus(s Status) { c.mu.Lock(); c.status = s; c.mu.Unlock() }

func (c *Connector) Name() string        { return "relay_connector" }
func (c *Connector) Notify(plugin.Event) {}
func (c *Connector) Stop() error         { return nil }

func (c *Connector) Start(ctx context.Context) error {
	backoff := time.Second
	for ctx.Err() == nil {
		c.setStatus(Status{State: "connecting"})
		onlineFor, err := c.session(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if onlineFor >= stableOnlineWindow {
			backoff = time.Second
		}
		c.log.Warn("relay connector disconnected", "err", err, "retry_in", backoff)
		c.setStatus(Status{State: "offline", Error: err.Error()})
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
	return nil
}

func (c *Connector) session(ctx context.Context) (time.Duration, error) {
	controlURL, err := endpoint(c.cfg.URL, relayproto.ControlPath)
	if err != nil {
		return 0, err
	}
	ws, _, err := c.dialer.DialContext(ctx, controlURL, c.headers(""))
	if err != nil {
		return 0, fmt.Errorf("connect control: %w", err)
	}
	defer ws.Close()

	_ = ws.SetReadDeadline(time.Now().Add(15 * time.Second))
	var ready struct{ Type, Session string }
	if err := ws.ReadJSON(&ready); err != nil {
		return 0, fmt.Errorf("read control ready: %w", err)
	}
	if ready.Type != "ready" || ready.Session == "" {
		return 0, fmt.Errorf("invalid control response")
	}

	connectedAt := time.Now()
	c.log.Info("relay connector online", "server_id", c.cfg.ServerID)
	c.setStatus(Status{State: "online", ConnectedAt: connectedAt})

	workersCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for range defaultPoolSize {
		wg.Add(1)
		go func() { defer wg.Done(); c.worker(workersCtx, ready.Session) }()
	}
	err = c.watchControl(ctx, ws)
	cancel()
	_ = ws.Close()
	wg.Wait()
	return time.Since(connectedAt), err
}

func (c *Connector) watchControl(ctx context.Context, ws *websocket.Conn) error {
	_ = ws.SetReadDeadline(time.Now().Add(controlReadTimeout))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(controlReadTimeout))
	})
	ping := time.NewTicker(controlPingInterval)
	defer ping.Stop()
	errs := make(chan error, 1)
	go func() {
		for {
			if _, _, err := ws.NextReader(); err != nil {
				errs <- err
				return
			}
			_ = ws.SetReadDeadline(time.Now().Add(controlReadTimeout))
		}
	}()
	for {
		select {
		case err := <-errs:
			return err
		case <-ctx.Done():
			return nil
		case <-ping.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return err
			}
		}
	}
}

func (c *Connector) worker(ctx context.Context, session string) {
	dataURL, err := endpoint(c.cfg.URL, relayproto.DataPath)
	if err != nil {
		c.log.Error("relay data URL", "err", err)
		return
	}
	retry := 200 * time.Millisecond
	firstBuf := make([]byte, 32*1024)

	for ctx.Err() == nil {
		ws, _, err := c.dialer.DialContext(ctx, dataURL, c.headers(session))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Debug("relay data connection failed", "err", err, "retry_in", retry)
			if !sleepCtx(ctx, retry) {
				return
			}
			if retry < 5*time.Second {
				retry *= 2
				if retry > 5*time.Second {
					retry = 5 * time.Second
				}
			}
			continue
		}
		retry = 200 * time.Millisecond
		c.serveDataConn(ctx, ws, firstBuf)
	}
}

// serveDataConn waits until Relay actually dispatches a request onto this idle
// data connection before dialing the local Hylo server.
func (c *Connector) serveDataConn(ctx context.Context, ws *websocket.Conn, firstBuf []byte) {
	stream := relayproto.NewWSConn(ws, "home", "relay")
	stopClose := context.AfterFunc(ctx, func() { _ = stream.Close() })
	defer func() {
		stopClose()
		_ = stream.Close()
	}()

	n, err := stream.Read(firstBuf)
	if err != nil || n == 0 {
		return
	}

	local, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", c.target)
	if err != nil {
		c.log.Warn("relay cannot reach local Hylo", "target", c.target, "err", err)
		return
	}
	defer local.Close()

	if _, err := local.Write(firstBuf[:n]); err != nil {
		return
	}
	bridge(local, stream)
}

func (c *Connector) headers(session string) http.Header {
	h := make(http.Header)
	h.Set("Authorization", "Bearer "+c.cfg.ConnectorToken)
	h.Set(relayproto.HeaderServer, c.cfg.ServerID)
	h.Set(relayproto.HeaderAccess, c.cfg.AccessToken)
	if strings.TrimSpace(c.cfg.JoinKey) != "" {
		h.Set(relayproto.HeaderJoinKey, strings.TrimSpace(c.cfg.JoinKey))
	}
	if session != "" {
		h.Set(relayproto.HeaderSession, session)
	}
	return h
}

func endpoint(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme != "wss" && u.Scheme != "ws" {
		return "", fmt.Errorf("relay URL must use http, https, ws, or wss")
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + path
	return u.String(), nil
}

func bridge(a net.Conn, b net.Conn) {
	var wg sync.WaitGroup
	copyOne := func(dst io.WriteCloser, src io.Reader) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		_ = dst.Close()
	}
	wg.Add(2)
	go copyOne(a, b)
	go copyOne(b, a)
	wg.Wait()
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

var _ plugin.Plugin = (*Connector)(nil)
