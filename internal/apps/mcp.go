package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hardhacker/hylo/internal/build"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// refusalWatch notes a 401 or 403 from the platform, which the SDK only
// reports as a failed request.
type refusalWatch struct {
	base    http.RoundTripper
	refused atomic.Bool
}

func (w *refusalWatch) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := w.base.RoundTrip(r)
	if err == nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		w.refused.Store(true)
	}
	return resp, err
}

// session opens a short MCP session to a with c; every use is one session,
// so nothing lingers between a bot's calls.
func (m *Manager) session(ctx context.Context, a *App, c *credential) (*sdk.ClientSession, *refusalWatch, error) {
	base := m.http.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	watch := &refusalWatch{base: base}
	tr := &oauth2.Transport{Source: m.tokenSource(a.ID, c), Base: watch}
	cl := sdk.NewClient(&sdk.Implementation{Name: "hylo", Version: build.Get().Version}, nil)
	cs, err := cl.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint:             a.URL,
		HTTPClient:           &http.Client{Transport: tr}, // ctx bounds it: tool calls outlast m.http's timeout
		DisableStandaloneSSE: true,
		MaxRetries:           1,
	}, nil)
	if err != nil {
		return nil, nil, refusedOr(watch, err)
	}
	return cs, watch, nil
}

// tokenSource is c's bearer: a fixed API token, or a sign-in's token,
// refreshed through refreshGrant once it is past its expiry.
func (m *Manager) tokenSource(id string, c *credential) oauth2.TokenSource {
	if c.OAuth == nil {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.Secret, TokenType: "Bearer"})
	}
	return tokenFunc(func() (*oauth2.Token, error) {
		if c.OAuth.Token.Valid() {
			return c.OAuth.Token, nil
		}
		return m.refreshGrant(id)
	})
}

type tokenFunc func() (*oauth2.Token, error)

func (f tokenFunc) Token() (*oauth2.Token, error) { return f() }

// refreshGrant renews app id's expired sign-in. Calls share it, one refresh
// at a time against the stored grant: a platform may rotate refresh tokens,
// so two refreshes with the same one would leave the second refused.
func (m *Manager) refreshGrant(id string) (*oauth2.Token, error) {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	c := m.creds.get(id)
	if c == nil || c.OAuth == nil {
		return nil, ErrNotConnected
	}
	if c.OAuth.Token.Valid() {
		return c.OAuth.Token, nil // another call just refreshed it
	}
	t, err := m.refresh(c.OAuth)
	if err != nil {
		return nil, err
	}
	if err := m.creds.setToken(id, c.ConnectedAt, t); err != nil {
		m.logger.Warn("apps: keep refreshed token", "app", id, "err", err)
	}
	return t, nil
}

// refresh trades g's refresh token for a new token, naming the resource as
// MCP's authorization asks of every token request (RFC 8707).
func (m *Manager) refresh(g *oauthGrant) (*oauth2.Token, error) {
	if g.Token.RefreshToken == "" {
		return nil, fmt.Errorf("%w: the sign-in expired and can't be renewed", ErrRefused)
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {g.Token.RefreshToken},
		"client_id": {g.ClientID}, "resource": {g.Resource}}
	req, err := http.NewRequest(http.MethodPost, g.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("renew sign-in: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: renew sign-in: %s", ErrRefused, firstLine(string(body)))
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("renew sign-in: %s", resp.Status)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return nil, fmt.Errorf("renew sign-in: unexpected answer %q", firstLine(string(body)))
	}
	t := &oauth2.Token{AccessToken: out.AccessToken, TokenType: out.TokenType, RefreshToken: out.RefreshToken}
	if t.RefreshToken == "" {
		t.RefreshToken = g.Token.RefreshToken // not rotated: the old one still holds
	}
	if out.ExpiresIn > 0 {
		t.Expiry = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return t, nil
}

func refusedOr(w *refusalWatch, err error) error {
	if w.refused.Load() && !errors.Is(err, ErrRefused) {
		return fmt.Errorf("%w: %v", ErrRefused, err)
	}
	return err
}

// verifyMCP calls a's Check tool and names the account from its answer.
func (m *Manager) verifyMCP(ctx context.Context, a *App, c *credential) (string, error) {
	res, err := m.callTool(ctx, a, c, a.Check, nil)
	if err != nil {
		return "", err
	}
	if res.IsError {
		return "", &limitedError{msg: firstLine(res.Text)}
	}
	return accountLabel(res), nil
}

func (m *Manager) listTools(ctx context.Context, a *App, c *credential) ([]Tool, error) {
	cs, watch, err := m.session(ctx, a, c)
	if err != nil {
		return nil, err
	}
	defer cs.Close()
	var out []Tool
	for t, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, refusedOr(watch, err)
		}
		out = append(out, Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema,
			ReadOnly: t.Annotations != nil && t.Annotations.ReadOnlyHint})
	}
	return out, nil
}

func (m *Manager) callTool(ctx context.Context, a *App, c *credential, tool string, args map[string]any) (*CallResult, error) {
	cs, watch, err := m.session(ctx, a, c)
	if err != nil {
		return nil, err
	}
	defer cs.Close()
	if args == nil {
		args = map[string]any{} // a nil map goes out as null, which servers reject
	}
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return nil, refusedOr(watch, err)
	}
	out := &CallResult{Data: res.StructuredContent, IsError: res.IsError}
	var texts []string
	for _, ct := range res.Content {
		if t, ok := ct.(*sdk.TextContent); ok {
			texts = append(texts, t.Text)
		}
	}
	out.Text = strings.Join(texts, "\n")
	return out, nil
}

// accountLabel picks who and which plan from a "who am I" answer, whose shape
// varies by platform.
func accountLabel(res *CallResult) string {
	fields, _ := res.Data.(map[string]any)
	if fields == nil {
		_ = json.Unmarshal([]byte(res.Text), &fields)
	}
	if u, ok := fields["user"].(map[string]any); ok {
		for k, v := range u {
			if _, set := fields[k]; !set {
				fields[k] = v
			}
		}
	}
	var parts []string
	for _, k := range []string{"email", "name", "username"} {
		if s, ok := fields[k].(string); ok && s != "" {
			parts = append(parts, s)
			break
		}
	}
	for _, k := range []string{"plan", "planName", "subscription"} {
		if s, ok := fields[k].(string); ok && s != "" {
			parts = append(parts, s)
			break
		}
	}
	return strings.Join(parts, " · ")
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}
