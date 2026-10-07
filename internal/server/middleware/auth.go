package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The app has no route at "/".
const homePath = "/home"

const (
	sessionCookie = "hylo_session"
	sessionMaxAge = 30 * 24 * time.Hour
)

// LoginRenderer writes the sign-in page.
type LoginRenderer func(w http.ResponseWriter, r *http.Request, status int, next, errMsg string)

type authenticator struct {
	render LoginRenderer
	key    string
	token  string
	logger *slog.Logger
}

// Authenticator guards every route when apiKey is set. Programmatic clients send
// X-Hylo-API-Key or a Bearer token; browsers log in at /login and get a cookie.
func Authenticator(apiKey string, logger *slog.Logger, render LoginRenderer) func(http.Handler) http.Handler {
	if apiKey == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	// Derived so the cookie never contains the key and dies when the key rotates.
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte("hylo-session-v1"))
	a := &authenticator{
		render: render,
		key:    apiKey,
		token:  hex.EncodeToString(mac.Sum(nil)),
		logger: logger,
	}
	return a.wrap
}

func (a *authenticator) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			next.ServeHTTP(w, r)
			return
		}
		switch r.URL.Path {
		case "/healthz", "/version":
			next.ServeHTTP(w, r)
			return
		case "/login":
			a.login(w, r)
			return
		case "/logout":
			a.logout(w, r)
			return
		}

		if a.authorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		a.logger.Warn("unauthorized request", "remote_addr", ClientIP(r), "path", r.URL.Path, "method", r.Method)
		a.deny(w, r)
	})
}

func (a *authenticator) authorized(r *http.Request) bool {
	token := r.Header.Get("X-Hylo-API-Key")
	if token == "" {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if token != "" {
		return safeEqual(token, a.key)
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return safeEqual(c.Value, a.token)
	}
	return false
}

func (a *authenticator) deny(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func (a *authenticator) login(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	switch r.Method {
	case http.MethodGet:
		if a.authorized(r) {
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}
		a.render(w, r, http.StatusOK, next, "")
	case http.MethodPost:
		if !safeEqual(strings.TrimSpace(r.FormValue("key")), a.key) {
			a.logger.Warn("login failed", "remote_addr", ClientIP(r))
			a.render(w, r, http.StatusUnauthorized, next, "Invalid API key.")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    a.token,
			Path:     "/",
			MaxAge:   int(sessionMaxAge.Seconds()),
			HttpOnly: true,
			Secure:   isHTTPS(r),
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, next, http.StatusSeeOther)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (a *authenticator) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func safeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// safeNext only allows same-origin paths. Browsers drop tabs and newlines
// inside URLs, so "/\t/evil.com" would otherwise become "//evil.com".
func safeNext(next string) string {
	if next == "/" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return homePath
	}
	for _, c := range next {
		if c < 0x20 || c == 0x7f || c == '\\' {
			return homePath
		}
	}
	if u, err := url.Parse(next); err != nil || u.Scheme != "" || u.Host != "" {
		return homePath
	}
	return next
}

// ClientIP returns the caller address, trusting X-Forwarded-For only from a
// loopback peer (the reverse proxy on the same host).
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	return host
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback() && r.Header.Get("X-Forwarded-Proto") == "https"
}
