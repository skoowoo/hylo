// Package relay implements the public reverse-tunnel endpoint.
package relay

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hardhacker/hylo/internal/relayproto"
)

var (
	errSessionClosed = errors.New("relay session closed")
	errPoolExhausted = errors.New("relay connection pool exhausted")
)

// ServerCredentials are provisioned on the public Relay. AccessToken is the
// separate public-edge credential; it is never accepted as a Connector token.
type ServerCredentials struct {
	ConnectorToken string
	AccessToken    string
}

type Config struct {
	Servers        map[string]ServerCredentials
	StateFile      string
	AutoRegister   bool
	JoinKey        string
	MaxConnections int
	AcquireTimeout time.Duration
	ControlTimeout time.Duration
	Logger         *slog.Logger
}

type Relay struct {
	mu             sync.Mutex
	servers        map[string]ServerCredentials
	sessions       map[string]*session
	max            int
	wait           time.Duration
	controlTimeout time.Duration
	log            *slog.Logger
	stateFile      string
	autoRegister   bool
	joinKey        string
	upgrader       websocket.Upgrader
}

type session struct {
	id   string
	idle chan net.Conn
	done chan struct{}
	once sync.Once
}

func New(cfg Config) (*Relay, error) {
	if len(cfg.Servers) == 0 && !cfg.AutoRegister {
		return nil, errors.New("relay: at least one server credential is required")
	}
	for id, c := range cfg.Servers {
		if strings.TrimSpace(id) == "" || c.ConnectorToken == "" || c.AccessToken == "" {
			return nil, errors.New("relay: every server needs an id, connector token, and access token")
		}
	}
	if cfg.MaxConnections == 0 {
		cfg.MaxConnections = 16
	}
	if cfg.AcquireTimeout == 0 {
		cfg.AcquireTimeout = 10 * time.Second
	}
	if cfg.ControlTimeout == 0 {
		cfg.ControlTimeout = 60 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	servers := make(map[string]ServerCredentials, len(cfg.Servers))
	for k, v := range cfg.Servers {
		servers[k] = v
	}
	if cfg.StateFile != "" {
		if b, err := os.ReadFile(cfg.StateFile); err == nil {
			if err := json.Unmarshal(b, &servers); err != nil {
				return nil, errors.New("relay: invalid state file")
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return &Relay{
		servers:        servers,
		sessions:       make(map[string]*session),
		max:            cfg.MaxConnections,
		wait:           cfg.AcquireTimeout,
		controlTimeout: cfg.ControlTimeout,
		log:            cfg.Logger,
		stateFile:      cfg.StateFile,
		autoRegister:   cfg.AutoRegister,
		joinKey:        strings.TrimSpace(cfg.JoinKey),
		// Control and data endpoints are Connector-only. A browser must never
		// be able to turn its Origin into a tunnel session.
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }},
	}, nil
}

func (r *Relay) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(relayproto.ControlPath, r.control)
	mux.HandleFunc(relayproto.DataPath, r.data)
	mux.HandleFunc("GET /relay/v1/servers/{server_id}/status", r.serverStatus)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/s/", r.proxy)
	return mux
}

// Close terminates all active connector sessions.
func (r *Relay) Close() {
	r.mu.Lock()
	active := make([]*session, 0, len(r.sessions))
	for id, s := range r.sessions {
		active = append(active, s)
		delete(r.sessions, id)
	}
	r.mu.Unlock()
	for _, s := range active {
		s.close()
	}
}

func (r *Relay) serverStatus(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("server_id")
	if !r.accessAuth(req, id) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"server_id": id,
		"online":    r.get(id) != nil,
	})
}

func (r *Relay) control(w http.ResponseWriter, req *http.Request) {
	id, _, ok := r.connectorAuth(req, true)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer c.Close()

	_ = c.SetReadDeadline(time.Now().Add(r.controlTimeout))
	c.SetPingHandler(func(appData string) error {
		_ = c.SetReadDeadline(time.Now().Add(r.controlTimeout))
		return c.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(r.controlTimeout))
	})

	s := &session{id: randomID(), idle: make(chan net.Conn, r.max), done: make(chan struct{})}
	r.mu.Lock()
	old := r.sessions[id]
	r.sessions[id] = s
	r.mu.Unlock()
	if old != nil {
		old.close()
	}
	defer r.remove(id, s)

	if err := c.WriteJSON(map[string]string{"type": "ready", "session": s.id}); err != nil {
		return
	}
	r.log.Info("connector online", "server_id", id, "session", s.id)
	for {
		if _, _, err := c.NextReader(); err != nil {
			break
		}
		_ = c.SetReadDeadline(time.Now().Add(r.controlTimeout))
	}
	r.log.Info("connector offline", "server_id", id, "session", s.id)
}

func (r *Relay) data(w http.ResponseWriter, req *http.Request) {
	id, _, ok := r.connectorAuth(req, false)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s := r.get(id)
	if s == nil || req.Header.Get(relayproto.HeaderSession) != s.id {
		http.Error(w, "no active session", http.StatusConflict)
		return
	}
	c, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	stream := relayproto.NewWSConn(c, "relay", "connector")
	select {
	case s.idle <- stream:
	case <-s.done:
		_ = stream.Close()
	}
}

func (r *Relay) proxy(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/s/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		http.NotFound(w, req)
		return
	}
	id := parts[0]
	if !r.accessAuth(req, id) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s := r.get(id)
	if s == nil {
		http.Error(w, "home offline", http.StatusServiceUnavailable)
		return
	}
	req2 := req.Clone(req.Context())
	req2.URL.Path = "/" + strings.Join(parts[1:], "/")
	req2.URL.RawPath = ""
	req2.URL.Scheme, req2.URL.Host = "http", "hylo"
	req2.RequestURI = ""
	req2.Header = req.Header.Clone()
	req2.Header.Del(relayproto.HeaderKey)
	// Connector reaches Hylo through loopback. Never let a public client forge
	// headers that Hylo intentionally trusts from a loopback reverse proxy.
	req2.Header.Del("X-Forwarded-For")
	req2.Header.Del("X-Forwarded-Proto")
	req2.Header.Del("Forwarded")

	transport := &http.Transport{DisableKeepAlives: true, DialContext: s.dial(r.wait)}
	defer transport.CloseIdleConnections()
	resp, err := transport.RoundTrip(req2)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return
		case errors.Is(err, errPoolExhausted):
			http.Error(w, "relay busy", http.StatusTooManyRequests)
		case errors.Is(err, errSessionClosed):
			http.Error(w, "home offline", http.StatusServiceUnavailable)
		default:
			http.Error(w, "home unavailable", http.StatusBadGateway)
		}
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	copyResponse(w, resp.Body)
}

func (r *Relay) connectorAuth(req *http.Request, allowRegister bool) (string, ServerCredentials, bool) {
	id := req.Header.Get(relayproto.HeaderServer)
	if r.joinKey != "" && !safeEqual(req.Header.Get(relayproto.HeaderJoinKey), r.joinKey) {
		return id, ServerCredentials{}, false
	}
	r.mu.Lock()
	cred, ok := r.servers[id]
	if !ok && allowRegister && r.autoRegister && id != "" && bearer(req) != "" && req.Header.Get(relayproto.HeaderAccess) != "" {
		cred = ServerCredentials{ConnectorToken: bearer(req), AccessToken: req.Header.Get(relayproto.HeaderAccess)}
		r.servers[id] = cred
		if err := r.persistLocked(); err != nil {
			delete(r.servers, id)
			r.mu.Unlock()
			return id, ServerCredentials{}, false
		}
		ok = true
	}
	r.mu.Unlock()
	return id, cred, ok && safeEqual(bearer(req), cred.ConnectorToken)
}

func (r *Relay) accessAuth(req *http.Request, id string) bool {
	r.mu.Lock()
	cred, ok := r.servers[id]
	r.mu.Unlock()
	return ok && safeEqual(req.Header.Get(relayproto.HeaderKey), cred.AccessToken)
}

func (r *Relay) persistLocked() error {
	if r.stateFile == "" {
		return nil
	}
	b, err := json.Marshal(r.servers)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.stateFile), 0700); err != nil {
		return err
	}
	tmp := r.stateFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, r.stateFile)
}

func (r *Relay) get(id string) *session { r.mu.Lock(); defer r.mu.Unlock(); return r.sessions[id] }
func (r *Relay) remove(id string, s *session) {
	r.mu.Lock()
	if r.sessions[id] == s {
		delete(r.sessions, id)
	}
	r.mu.Unlock()
	s.close()
}
func (s *session) close() {
	s.once.Do(func() {
		close(s.done)
		for {
			select {
			case c := <-s.idle:
				_ = c.Close()
			default:
				return
			}
		}
	})
}
func (s *session) dial(wait time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case c := <-s.idle:
			return c, nil
		case <-s.done:
			return nil, errSessionClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, errPoolExhausted
		}
	}
}
func safeEqual(a, b string) bool {
	return a != "" && len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func bearer(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		if k == "Connection" || k == "Upgrade" {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

func copyResponse(w http.ResponseWriter, src io.Reader) {
	buf := make([]byte, 32*1024)
	flush := http.NewResponseController(w).Flush
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			// This is required for Hylo's agent and event streams; buffering them
			// until the run ends would make remote chat appear stalled.
			_ = flush()
		}
		if err != nil {
			return
		}
	}
}

func randomID() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
