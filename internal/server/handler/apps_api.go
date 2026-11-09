package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"

	"github.com/hardhacker/hylo/internal/apps"
)

// AppsHTTP handles /api/apps: connecting the user's outside apps from
// Settings, turning them on for bots, and bots' calls to them.
type AppsHTTP struct {
	mgr    *apps.Manager
	runBot func(r *http.Request) (string, int, error)
}

// NewAppsHTTP constructs the handler; runBot names the bot whose run made a
// call ("" for the user's own).
func NewAppsHTTP(mgr *apps.Manager, runBot func(*http.Request) (string, int, error)) *AppsHTTP {
	return &AppsHTTP{mgr: mgr, runBot: runBot}
}

// allowed refuses a call from a bot that hasn't turned the app on; the user's own calls pass.
func (h *AppsHTTP) allowed(w http.ResponseWriter, r *http.Request, app string) bool {
	bot, code, err := h.runBot(r)
	if err != nil {
		http.Error(w, err.Error(), code)
		return false
	}
	if bot != "" && !h.mgr.Allows(bot, app) {
		http.Error(w, "this bot doesn't have the "+app+" app on; the user turns it on in the bot's settings", http.StatusForbidden)
		return false
	}
	return true
}

// SetBot handles PUT /api/apps/{id}/bots/{bot} {on}: turns the app on or off for a bot.
func (h *AppsHTTP) SetBot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		On bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	if err := h.mgr.SetOn(r.PathValue("bot"), r.PathValue("id"), body.On); err != nil {
		appsErr(w, err, http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func appsStatus(err error, fallback int) int {
	switch {
	case errors.Is(err, apps.ErrUnknown):
		return http.StatusNotFound
	case errors.Is(err, apps.ErrNotConnected), errors.Is(err, apps.ErrNoSignIn):
		return http.StatusConflict
	}
	return fallback
}

func appsErr(w http.ResponseWriter, err error, fallback int) {
	respondJSON(w, appsStatus(err, fallback), map[string]any{"error": err.Error()})
}

// signInRedirect is where a browser sign-in comes back to, or "" when it
// can't: Hylo's client document allows only loopback addresses, so the page
// must be open on this machine (the desktop app, or a local browser).
func signInRedirect(r *http.Request) string {
	if r.TLS != nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return ""
	}
	return "http://" + r.Host + apps.CallbackPath
}

// List handles GET /api/apps.
func (h *AppsHTTP) List(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"apps": h.mgr.List(), "signIn": signInRedirect(r) != ""})
}

// Check handles POST /api/apps/check: asks every platform whether its connection still works.
func (h *AppsHTTP) Check(w http.ResponseWriter, r *http.Request) {
	h.mgr.CheckAll(r.Context())
	h.List(w, r)
}

// Connect handles POST /api/apps/{id}/connect {method, secret}: a key or an API token.
func (h *AppsHTTP) Connect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Method string `json:"method"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	st, err := h.mgr.Connect(r.Context(), r.PathValue("id"), body.Method, body.Secret)
	if err != nil {
		appsErr(w, err, http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, st)
}

// SignIn handles POST /api/apps/{id}/sign-in: the page that starts a browser sign-in.
func (h *AppsHTTP) SignIn(w http.ResponseWriter, r *http.Request) {
	redirect := signInRedirect(r)
	if redirect == "" {
		appsErr(w, fmt.Errorf("%w: open Hylo on this computer to sign in, or connect with an API token", apps.ErrNoSignIn), http.StatusConflict)
		return
	}
	u, err := h.mgr.StartSignIn(r.Context(), r.PathValue("id"), redirect)
	if err != nil {
		appsErr(w, err, http.StatusBadGateway)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"url": u})
}

// CancelSignIn handles DELETE /api/apps/{id}/sign-in.
func (h *AppsHTTP) CancelSignIn(w http.ResponseWriter, r *http.Request) {
	h.mgr.CancelSignIn(r.PathValue("id"))
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Disconnect handles DELETE /api/apps/{id}.
func (h *AppsHTTP) Disconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.mgr.Disconnect(r.Context(), r.PathValue("id")); err != nil {
		appsErr(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

var signedInPage = template.Must(template.New("").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Hylo</title>
<style>
:root{color-scheme:light dark}
body{margin:0;min-height:100vh;display:grid;place-items:center;font:15px/1.5 -apple-system,system-ui,sans-serif;background:Canvas;color:CanvasText}
main{max-width:360px;padding:24px;text-align:center}
h1{font-size:17px;font-weight:600;margin:0 0 6px}
p{margin:0;opacity:.7}
</style></head>
<body><main>{{if .Err}}<h1>Couldn't connect{{with .App}} {{.}}{{end}}</h1><p>{{.Err}}</p>{{else}}<h1>{{.App}} is connected</h1><p>You can close this tab and go back to Hylo.</p>{{end}}</main></body></html>`))

// Callback handles GET /apps/oauth/callback, where the platform sends the
// browser after sign-in. It needs no Hylo session: the system browser has
// none, and the state only a sign-in started here knows stands in for it.
func (h *AppsHTTP) Callback(w http.ResponseWriter, r *http.Request) {
	name, err := h.mgr.FinishSignIn(r.Context(), r.URL.Query())
	data := struct{ App, Err string }{App: name}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		data.Err = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = signedInPage.Execute(w, data)
}

// Tools handles GET /api/apps/{id}/tools: what a connected MCP app offers.
func (h *AppsHTTP) Tools(w http.ResponseWriter, r *http.Request) {
	if !h.allowed(w, r, r.PathValue("id")) {
		return
	}
	tools, err := h.mgr.Tools(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), appsStatus(err, http.StatusBadGateway))
		return
	}
	if tools == nil {
		tools = []apps.Tool{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

// Call handles POST /api/apps/{id}/call {tool, args}.
func (h *AppsHTTP) Call(w http.ResponseWriter, r *http.Request) {
	if !h.allowed(w, r, r.PathValue("id")) {
		return
	}
	var body struct {
		Tool string          `json:"tool"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tool == "" {
		http.Error(w, "need {tool, args}", http.StatusBadRequest)
		return
	}
	res, err := h.mgr.Call(r.Context(), r.PathValue("id"), body.Tool, body.Args)
	if err != nil {
		http.Error(w, err.Error(), appsStatus(err, http.StatusBadGateway))
		return
	}
	respondJSON(w, http.StatusOK, res)
}
