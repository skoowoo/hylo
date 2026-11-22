package relay

import (
	"net/http"
	"net/url"
	"path"
	"strings"
)

// defaultAllowedRoutes lists the Hylo API endpoints exposed through the public
// Relay. High-privilege local routes (config edits, server restart, /mcp,
// agent installers, desktop presence, dev pages, and web UI) are excluded.
var defaultAllowedRoutes = []string{
	"POST /version",
	"POST /api/status",
	"GET /api/live",

	"POST /api/chat",
	"POST /api/chat/uploads",
	"GET /api/chat/uploads/{id}",
	"POST /api/runs",
	"GET /api/runs/active",
	"GET /api/runs/by-ref",
	"GET /api/runs/{id}",
	"GET /api/runs/{id}/events",
	"POST /api/runs/{id}/cancel",

	"GET /api/mates",
	"GET /api/mates/{id}",
	"POST /api/mates/{id}/seen",
	"PUT /api/mates/{id}/conversation",
	"GET /api/mates/{id}/chat",
	"GET /api/mate-events",

	"GET /api/conversations",
	"GET /api/conversations/{id}",
	"DELETE /api/conversations/{id}",

	"GET /api/activity",
	"GET /api/activity/days",
	"GET /api/activity/day",
	"POST /api/activity/visit",

	"GET /api/agenda",
	"POST /api/agenda",
	"POST /api/agenda/{id}/done",

	"GET /api/skills",

	"POST /api/notes/resolve",
	"POST /api/notes/exist",
	"POST /api/notes/info",
	"POST /api/notes/meta",
	"POST /api/embeds",
	"POST /api/embeds/progress",

	"POST /api/vault/read",
	"POST /api/vault/stat",
	"POST /api/vault/list",
	"POST /api/vault/sample",
	"POST /api/vault/list-dirs",
	"POST /api/vault/write",
	"POST /api/vault/create-untitled",
	"POST /api/vault/delete",
	"POST /api/vault/move",
	"POST /api/vault/rename",
	"POST /api/vault/pin",
	"GET /api/vault/check-name",
	"GET /api/vault/overview",
	"GET /api/vault/rename-status",
	"POST /api/vault/upload-image",

	"POST /api/vault/shorts",
	"POST /api/vault/shorts/list",
	"POST /api/vault/shorts/get",
	"POST /api/vault/shorts/update",
	"POST /api/vault/shorts/delete",

	"GET /api/images/serve",
	"GET /api/images/at",
	"POST /api/images/delete",
	"GET /api/graph/data",

	"POST /api/search",
	"POST /api/tag/list",
	"POST /api/tag/count",
	"POST /api/tag/delete",
	"POST /api/tag/rename",
}

type routePolicy struct {
	allowAll bool
	mux      *http.ServeMux
}

func newRoutePolicy(allowAll bool, extra []string) *routePolicy {
	if allowAll {
		return &routePolicy{allowAll: true}
	}
	mux := http.NewServeMux()
	noop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, pat := range defaultAllowedRoutes {
		mux.Handle(pat, noop)
	}
	for _, pat := range extra {
		if strings.TrimSpace(pat) != "" {
			mux.Handle(pat, noop)
		}
	}
	return &routePolicy{mux: mux}
}

func (p *routePolicy) allowed(method, targetPath string) bool {
	if p.allowAll {
		return true
	}
	probe := &http.Request{
		Method: method,
		URL:    &url.URL{Path: targetPath},
	}
	_, pattern := p.mux.Handler(probe)
	return pattern != ""
}

// parseProxyTarget extracts serverID and the canonical downstream path from
// /s/{server_id}/{path}. Ambiguous encodings or traversal segments are rejected
// so Relay and Hylo cannot interpret the target path differently.
func parseProxyTarget(u *url.URL) (serverID, targetPath string, ok bool) {
	raw := u.RawPath
	if raw == "" {
		raw = u.Path
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || strings.Contains(lower, "%00") ||
		strings.Contains(raw, "\\") || strings.Contains(u.Path, "\\") || strings.ContainsRune(u.Path, 0) {
		return "", "", false
	}
	if !strings.HasPrefix(u.Path, "/s/") {
		return "", "", false
	}
	rest := strings.TrimPrefix(u.Path, "/s/")
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return "", "", false
	}
	serverID = rest[:slash]
	targetPath = rest[slash:]
	if strings.Contains(targetPath, "//") {
		return "", "", false
	}
	for _, seg := range strings.Split(targetPath, "/") {
		if seg == "." || seg == ".." {
			return "", "", false
		}
	}
	cleaned := path.Clean(targetPath)
	if cleaned != targetPath {
		return "", "", false
	}
	return serverID, targetPath, true
}

func isUnsupportedProtocol(req *http.Request) bool {
	if req.Method == http.MethodConnect {
		return true
	}
	if strings.TrimSpace(req.Header.Get("Upgrade")) != "" {
		return true
	}
	for _, v := range req.Header.Values("Connection") {
		for _, tok := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), "upgrade") {
				return true
			}
		}
	}
	return false
}
