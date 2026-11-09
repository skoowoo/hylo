package apps

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// ClientID is Hylo's OAuth client: a Client ID Metadata Document on its site,
// so platforms know Hylo without registering it. It allows redirects to
// CallbackPath on loopback addresses, any port.
const ClientID = "https://skoo.build/apps/hylo/oauth/client.json"

// CallbackPath is where the platform sends the browser back after sign-in.
const CallbackPath = "/apps/oauth/callback"

// flowTTL is how long a sign-in waits for the browser to come back.
const flowTTL = 10 * time.Minute

// ErrNoSignIn means the platform can't take Hylo's browser sign-in: connect with a token.
var ErrNoSignIn = errors.New("browser sign-in isn't available for this app")

// flow is a browser sign-in under way.
type flow struct {
	app      string
	cfg      *oauth2.Config
	verifier string
	issuer   string
	needIss  bool
	resource string
	revoke   string
	until    time.Time
}

// StartSignIn begins a browser sign-in to MCP app id that comes back to
// redirect, and returns the page to open. An earlier sign-in for id is dropped.
func (m *Manager) StartSignIn(ctx context.Context, id, redirect string) (string, error) {
	a, err := m.app(id)
	if err != nil {
		return "", err
	}
	if a.Kind != KindMCP {
		return "", fmt.Errorf("%w: %s connects with a key", ErrNoSignIn, a.Name)
	}
	prm, err := oauthex.GetProtectedResourceMetadata(ctx, resourceMetadataURL(a.URL), a.URL, m.http)
	if err != nil {
		return "", fmt.Errorf("discover %s sign-in: %w", a.Name, err)
	}
	if len(prm.AuthorizationServers) == 0 {
		return "", fmt.Errorf("%w: %s names no sign-in server", ErrNoSignIn, a.Name)
	}
	asm, err := auth.GetAuthServerMetadata(ctx, prm.AuthorizationServers[0], m.http)
	if err != nil {
		return "", fmt.Errorf("discover %s sign-in: %w", a.Name, err)
	}
	if asm == nil || !asm.ClientIDMetadataDocumentSupported {
		return "", fmt.Errorf("%w: %s doesn't accept Hylo's client document", ErrNoSignIn, a.Name)
	}
	cfg := &oauth2.Config{
		ClientID:    ClientID,
		Endpoint:    oauth2.Endpoint{AuthURL: asm.AuthorizationEndpoint, TokenURL: asm.TokenEndpoint, AuthStyle: oauth2.AuthStyleInParams},
		RedirectURL: redirect,
		Scopes:      prm.ScopesSupported,
	}
	f := &flow{app: id, cfg: cfg, verifier: oauth2.GenerateVerifier(), issuer: asm.Issuer,
		needIss: asm.AuthorizationResponseIssParameterSupported, resource: prm.Resource, revoke: asm.RevocationEndpoint,
		until: time.Now().Add(flowTTL)}
	state := randomState()
	m.mu.Lock()
	for s, old := range m.flows {
		if old.app == id || time.Now().After(old.until) {
			delete(m.flows, s)
		}
	}
	m.flows[state] = f
	m.mu.Unlock()
	return cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(f.verifier), oauth2.SetAuthURLParam("resource", f.resource)), nil
}

// CancelSignIn drops id's sign-in under way.
func (m *Manager) CancelSignIn(id string) {
	m.mu.Lock()
	for s, f := range m.flows {
		if f.app == id {
			delete(m.flows, s)
		}
	}
	m.mu.Unlock()
}

// FinishSignIn completes the sign-in the browser came back with (the
// callback's query) and returns the app's name.
func (m *Manager) FinishSignIn(ctx context.Context, q url.Values) (string, error) {
	state := q.Get("state")
	m.mu.Lock()
	f := m.flows[state]
	delete(m.flows, state)
	m.mu.Unlock()
	if f == nil || time.Now().After(f.until) {
		return "", errors.New("this sign-in is over or was started elsewhere; connect again from Hylo")
	}
	a, err := m.app(f.app)
	if err != nil {
		return "", err
	}
	defer m.changed()
	if e := q.Get("error"); e != "" {
		if d := q.Get("error_description"); d != "" {
			e = d
		}
		return a.Name, fmt.Errorf("%s: %s", a.Name, e)
	}
	// RFC 9207: the answer must come from the server the sign-in started with.
	if iss := q.Get("iss"); (f.needIss || iss != "") && strings.TrimSuffix(iss, "/") != strings.TrimSuffix(f.issuer, "/") {
		return a.Name, errors.New("the sign-in came back from an unexpected server")
	}
	xctx := context.WithValue(ctx, oauth2.HTTPClient, m.http)
	tok, err := f.cfg.Exchange(xctx, q.Get("code"), oauth2.VerifierOption(f.verifier), oauth2.SetAuthURLParam("resource", f.resource))
	if err != nil {
		return a.Name, fmt.Errorf("finish %s sign-in: %w", a.Name, err)
	}
	c := &credential{Method: MethodOAuth, OAuth: &oauthGrant{ClientID: f.cfg.ClientID, TokenURL: f.cfg.Endpoint.TokenURL,
		RevokeURL: f.revoke, Resource: f.resource, Token: tok}}
	_, err = m.save(ctx, a, c)
	return a.Name, err
}

// revoke tells the platform to drop a sign-in, best effort: the user can
// still revoke it on the platform's own site.
func (m *Manager) revoke(ctx context.Context, g *oauthGrant) {
	if g.RevokeURL == "" || g.Token == nil {
		return
	}
	for _, t := range []struct{ tok, hint string }{{g.Token.RefreshToken, "refresh_token"}, {g.Token.AccessToken, "access_token"}} {
		if t.tok == "" {
			continue
		}
		form := url.Values{"token": {t.tok}, "token_type_hint": {t.hint}, "client_id": {g.ClientID}}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.RevokeURL, strings.NewReader(form.Encode()))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if resp, err := m.http.Do(req); err == nil {
			resp.Body.Close()
		} else {
			m.logger.Warn("apps: revoke", "err", err)
		}
	}
}

// resourceMetadataURL is where RFC 9728 puts a resource's metadata: the
// well-known path inserted before the resource's own path.
func resourceMetadataURL(resource string) string {
	u, err := url.Parse(resource)
	if err != nil {
		return resource
	}
	u.Path = "/.well-known/oauth-protected-resource" + strings.TrimSuffix(u.Path, "/")
	u.RawQuery = ""
	return u.String()
}

func randomState() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
