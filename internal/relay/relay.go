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
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hardhacker/hylo/internal/relayproto"
)

var (
	errSessionClosed = errors.New("relay session closed")
	errPoolExhausted = errors.New("relay connection pool exhausted")
)

const (
	defaultIdleTarget            = 4
	defaultMaxConnections        = 16
	defaultAcquireTimeout        = 10 * time.Second
	defaultControlTimeout        = 60 * time.Second
	defaultResponseHeaderTimeout = 30 * time.Second
)

// ServerCredentials are provisioned on the public Relay. AccessToken is the
// separate public-edge credential; it is never accepted as a Connector token.
type ServerCredentials struct {
	ConnectorToken string         `json:"ConnectorToken"`
	AccessToken    string         `json:"AccessToken"`
	Devices        []DeviceRecord `json:"Devices,omitempty"`
}

type Config struct {
	Servers               map[string]ServerCredentials
	StateFile             string
	AutoRegister          bool
	JoinKey               string
	IdleTarget            int
	MaxConnections        int
	AcquireTimeout        time.Duration
	ControlTimeout        time.Duration
	ResponseHeaderTimeout time.Duration
	AllowAllRoutes        bool
	ExtraAllowedRoutes    []string
	Logger                *slog.Logger
}

type Relay struct {
	mu                    sync.Mutex
	servers               map[string]ServerCredentials
	sessions              map[string]*session
	pairings              map[string]*pairInvite
	activeDevices         map[string]map[string]context.CancelFunc
	idleTarget            int
	max                   int
	wait                  time.Duration
	controlTimeout        time.Duration
	responseHeaderTimeout time.Duration
	policy                *routePolicy
	log                   *slog.Logger
	stateFile             string
	autoRegister          bool
	joinKey               string
	upgrader              websocket.Upgrader
}

type session struct {
	id         string
	idleTarget int
	max        int
	idle       chan net.Conn
	replenish  chan struct{}
	done       chan struct{}
	inFlight   atomic.Int64
	once       sync.Once
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
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = defaultMaxConnections
	}
	if cfg.IdleTarget <= 0 {
		cfg.IdleTarget = defaultIdleTarget
	}
	if cfg.IdleTarget > cfg.MaxConnections {
		cfg.IdleTarget = cfg.MaxConnections
	}
	if cfg.AcquireTimeout <= 0 {
		cfg.AcquireTimeout = defaultAcquireTimeout
	}
	if cfg.ControlTimeout <= 0 {
		cfg.ControlTimeout = defaultControlTimeout
	}
	if cfg.ResponseHeaderTimeout <= 0 {
		cfg.ResponseHeaderTimeout = defaultResponseHeaderTimeout
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
		servers:               servers,
		sessions:              make(map[string]*session),
		pairings:              make(map[string]*pairInvite),
		activeDevices:         make(map[string]map[string]context.CancelFunc),
		idleTarget:            cfg.IdleTarget,
		max:                   cfg.MaxConnections,
		wait:                  cfg.AcquireTimeout,
		controlTimeout:        cfg.ControlTimeout,
		responseHeaderTimeout: cfg.ResponseHeaderTimeout,
		policy:                newRoutePolicy(cfg.AllowAllRoutes, cfg.ExtraAllowedRoutes),
		log:                   cfg.Logger,
		stateFile:             cfg.StateFile,
		autoRegister:          cfg.AutoRegister,
		joinKey:               strings.TrimSpace(cfg.JoinKey),
		// Control and data endpoints are Connector-only. A browser must never
		// be able to turn its Origin into a tunnel session.
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }},
	}, nil
}

func (r *Relay) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(relayproto.ControlPath, r.control)
	mux.HandleFunc(relayproto.DataPath, r.data)
	mux.HandleFunc("POST /relay/v1/pair/create", r.pairCreate)
	mux.HandleFunc("POST /relay/v1/pair/redeem", r.pairRedeem)
	mux.HandleFunc("GET /relay/v1/devices", r.listDevices)
	mux.HandleFunc("DELETE /relay/v1/devices/{device_id}", r.revokeDevice)
	mux.HandleFunc("DELETE /relay/v1/servers/{server_id}/self", r.revokeSelf)
	mux.HandleFunc("GET /relay/v1/servers/{server_id}/status", r.serverStatus)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodConnect {
			writeRelayError(w, http.StatusNotImplemented, "unsupported_protocol", "HTTP CONNECT is not supported")
			return
		}
		// Route /s/ before ServeMux so ServeMux cannot 307-redirect traversal or
		// double-slash paths before parseProxyTarget rejects them.
		if strings.HasPrefix(req.URL.Path, "/s/") {
			r.proxy(w, req)
			return
		}
		mux.ServeHTTP(w, req)
	})
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
	if _, status, code, msg := r.checkAccess(req, id); status != 0 {
		writeRelayError(w, status, code, msg)
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
		writeRelayError(w, http.StatusUnauthorized, "relay_unauthorized", "invalid connector credentials")
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

	s := &session{
		id:         randomID(),
		idleTarget: r.idleTarget,
		max:        r.max,
		idle:       make(chan net.Conn, r.max),
		replenish:  make(chan struct{}, 1),
		done:       make(chan struct{}),
	}
	r.mu.Lock()
	old := r.sessions[id]
	r.sessions[id] = s
	r.mu.Unlock()
	if old != nil {
		old.close()
	}
	defer r.remove(id, s)

	_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.WriteJSON(relayproto.ControlMessage{
		Type:           relayproto.MsgReady,
		Session:        s.id,
		IdleTarget:     s.idleTarget,
		MaxConnections: s.max,
	}); err != nil {
		return
	}
	r.log.Info("connector online", "server_id", id, "session", s.id)

	readErr := make(chan error, 1)
	go func() {
		for {
			if _, _, err := c.NextReader(); err != nil {
				readErr <- err
				return
			}
			_ = c.SetReadDeadline(time.Now().Add(r.controlTimeout))
		}
	}()

	for {
		select {
		case <-readErr:
			r.log.Info("connector offline", "server_id", id, "session", s.id)
			return
		case <-s.done:
			r.log.Info("connector offline", "server_id", id, "session", s.id)
			return
		case <-s.replenish:
			need := s.neededIdle()
			if need <= 0 {
				continue
			}
			_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.WriteJSON(relayproto.ControlMessage{
				Type:  relayproto.MsgOpenData,
				Count: need,
			}); err != nil {
				r.log.Info("connector offline", "server_id", id, "session", s.id)
				return
			}
		}
	}
}

func (r *Relay) data(w http.ResponseWriter, req *http.Request) {
	id, _, ok := r.connectorAuth(req, false)
	if !ok {
		writeRelayError(w, http.StatusUnauthorized, "relay_unauthorized", "invalid connector credentials")
		return
	}
	s := r.get(id)
	if s == nil || req.Header.Get(relayproto.HeaderSession) != s.id {
		writeRelayError(w, http.StatusConflict, "no_active_session", "no active connector session")
		return
	}
	if !s.hasCapacity() {
		writeRelayError(w, http.StatusTooManyRequests, "relay_busy", "connection pool is full")
		return
	}
	c, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	stream := relayproto.NewWSConn(c, "relay", "connector")
	if !s.enqueue(stream) {
		_ = stream.Close()
	}
}

func (r *Relay) proxy(w http.ResponseWriter, req *http.Request) {
	if isUnsupportedProtocol(req) {
		writeRelayError(w, http.StatusNotImplemented, "unsupported_protocol", "protocol upgrade is not supported")
		return
	}
	id, targetPath, ok := parseProxyTarget(req.URL)
	if !ok {
		writeRelayError(w, http.StatusBadRequest, "invalid_path", "invalid target path")
		return
	}
	deviceID, status, code, msg := r.checkAccess(req, id)
	if status != 0 {
		writeRelayError(w, status, code, msg)
		return
	}
	if !r.policy.allowed(req.Method, targetPath) {
		writeRelayError(w, http.StatusForbidden, "route_not_allowed", "route is not allowed through relay")
		return
	}
	s := r.get(id)
	if s == nil {
		writeRelayError(w, http.StatusServiceUnavailable, "home_offline", "home server is offline")
		return
	}
	proxyCtx, untrack := r.trackDeviceRequest(deviceID, req.Context())
	defer untrack()

	req2 := req.Clone(proxyCtx)
	req2.URL.Path = targetPath
	req2.URL.RawPath = ""
	req2.URL.Scheme, req2.URL.Host = "http", "hylo"
	req2.RequestURI = ""
	req2.Header = sanitizeProxyHeaders(req.Header)

	transport := &http.Transport{
		DisableKeepAlives:     true,
		DialContext:           s.dial(r.wait),
		ResponseHeaderTimeout: r.responseHeaderTimeout,
	}
	defer transport.CloseIdleConnections()
	resp, err := transport.RoundTrip(req2)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return
		case errors.Is(err, errPoolExhausted):
			w.Header().Set("Retry-After", "1")
			writeRelayError(w, http.StatusTooManyRequests, "relay_busy", "relay connection pool is busy")
		case errors.Is(err, errSessionClosed):
			writeRelayError(w, http.StatusServiceUnavailable, "home_offline", "home server is offline")
		case isTimeoutError(err):
			writeRelayError(w, http.StatusGatewayTimeout, "home_timeout", "home server response timed out")
		default:
			writeRelayError(w, http.StatusBadGateway, "home_unreachable", "home server is unreachable")
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

// checkAccess distinguishes a missing Relay credential (401) from a credential
// that is not authorized for the requested server (404), avoiding server ID enumeration.
func (r *Relay) checkAccess(req *http.Request, id string) (string, int, string, string) {
	key := req.Header.Get(relayproto.HeaderKey)
	if key == "" {
		return "", http.StatusUnauthorized, "relay_unauthorized", "missing relay key"
	}
	digest := tokenHash(key)
	r.mu.Lock()
	cred, ok := r.servers[id]
	var matchedDevice string
	if ok && !safeEqual(key, cred.AccessToken) {
		for _, d := range cred.Devices {
			if safeEqual(digest, d.TokenHash) {
				matchedDevice = d.ID
				break
			}
		}
		if matchedDevice == "" {
			ok = false
		}
	}
	r.mu.Unlock()
	if !ok {
		return "", http.StatusNotFound, "server_not_found", "server not found"
	}
	return matchedDevice, 0, "", ""
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

func (s *session) hasCapacity() bool {
	return int(s.inFlight.Load())+len(s.idle) < s.max
}

func (s *session) neededIdle() int {
	idle := len(s.idle)
	availCap := s.max - int(s.inFlight.Load()) - idle
	need := s.idleTarget - idle
	if need > availCap {
		need = availCap
	}
	if need < 0 {
		return 0
	}
	return need
}

func (s *session) triggerReplenish() {
	if s.neededIdle() <= 0 {
		return
	}
	select {
	case s.replenish <- struct{}{}:
	default:
	}
}

func (s *session) enqueue(c net.Conn) bool {
	if !s.hasCapacity() {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
	}
	select {
	case s.idle <- c:
		return true
	case <-s.done:
		return false
	default:
		return false
	}
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
			s.inFlight.Add(1)
			s.triggerReplenish()
			return &trackedConn{
				Conn: c,
				onClose: func() {
					s.inFlight.Add(-1)
					s.triggerReplenish()
				},
			}, nil
		case <-s.done:
			return nil, errSessionClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, errPoolExhausted
		}
	}
}

type trackedConn struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (t *trackedConn) Close() error {
	err := t.Conn.Close()
	t.once.Do(t.onClose)
	return err
}

func writeRelayError(w http.ResponseWriter, status int, code, message string) {
	reqID := randomID()[:16]
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(relayproto.HeaderRequestID, reqID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":      code,
		"message":    message,
		"request_id": reqID,
	})
}

func sanitizeProxyHeaders(src http.Header) http.Header {
	dst := src.Clone()
	for _, connVal := range dst.Values("Connection") {
		for _, field := range strings.Split(connVal, ",") {
			if f := strings.TrimSpace(field); f != "" {
				dst.Del(f)
			}
		}
	}
	for _, h := range []string{
		relayproto.HeaderKey,
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
		// Connector reaches Hylo through loopback; never trust public forwarded headers.
		"X-Forwarded-For",
		"X-Forwarded-Proto",
		"X-Forwarded-Host",
		"X-Real-IP",
		"Forwarded",
	} {
		dst.Del(h)
	}
	return dst
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func safeEqual(a, b string) bool {
	return a != "" && len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func bearer(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		if k == "Connection" || k == "Upgrade" || k == "Keep-Alive" || k == "Transfer-Encoding" {
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
