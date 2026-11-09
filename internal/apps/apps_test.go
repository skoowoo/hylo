package apps

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeSkills struct {
	added   map[string]bool
	on      map[string]bool
	forced  int
	version string
}

func newFakeSkills() *fakeSkills {
	return &fakeSkills{added: map[string]bool{}, on: map[string]bool{}, version: "1.0.5"}
}
func (f *fakeSkills) Has(n string) bool { return f.added[n] }
func (f *fakeSkills) Add(_, n string, force bool) (string, error) {
	if force {
		f.forced++
	}
	f.added[n] = true
	return n, nil
}
func (f *fakeSkills) Enable(n, dir string) error { f.on[n+"@"+dir] = true; return nil }
func (f *fakeSkills) Remove(n string) error {
	for _, dir := range f.EnabledDirs(n) {
		delete(f.on, n+"@"+dir)
	}
	delete(f.added, n)
	return nil
}
func (f *fakeSkills) Disable(n, dir string) error { delete(f.on, n+"@"+dir); return nil }
func (f *fakeSkills) Version(string) string       { return f.version }
func (f *fakeSkills) EnabledDirs(n string) []string {
	var dirs []string
	for k := range f.on {
		if name, dir, _ := strings.Cut(k, "@"); name == n {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

var (
	mainBot = Bot{ID: "main", Name: "Hylo", Main: true, Dir: "/"}
	newsBot = Bot{ID: "news", Name: "News", Dir: "/_bots/_news"}
)

func twoBots() ([]Bot, error) { return []Bot{mainBot, newsBot}, nil }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func newManager(t *testing.T, cat []App, sk *fakeSkills) *Manager {
	t.Helper()
	vault := t.TempDir()
	m, err := New(Options{Dir: filepath.Join(t.TempDir(), "apps"), StateFile: filepath.Join(vault, ".hylo", "apps.json"),
		Bots: twoBots, Skills: sk, Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSkillAppConnect(t *testing.T) {
	var upgrade, revoked atomic.Bool
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["skill_version"] != "1.0.5" {
			t.Errorf("skill_version = %q", in["skill_version"])
		}
		out := map[string]any{"errcode": 0, "books": []int{1, 2}, "albums": []int{3}, "mp": map[string]any{"x": 1}}
		if r.Header.Get("Authorization") != "Bearer wrk-good" || revoked.Load() {
			out = map[string]any{"errcode": -2013, "errmsg": "鉴权失败"}
		}
		if upgrade.Load() {
			out["upgrade_info"] = map[string]string{"message": "upgrade"}
		}
		writeJSON(w, out)
	}))
	defer gw.Close()
	old := weReadGateway
	weReadGateway = gw.URL
	defer func() { weReadGateway = old }()

	sk := newFakeSkills()
	m := newManager(t, Catalog, sk)
	ctx := context.Background()

	// The skill is the app's: turned on by hand before the app was connected, it goes.
	sk.added["weread-skills"], sk.on["weread-skills@/_bots/_old"] = true, true
	m.Reconcile()
	if len(sk.on) != 0 {
		t.Fatalf("an unconnected app's skill stayed on: %v", sk.on)
	}

	if _, err := m.Connect(ctx, "weread", MethodKey, "abc"); err == nil {
		t.Fatal("a key without the prefix went in")
	}
	if _, err := m.Connect(ctx, "weread", MethodKey, "wrk-bad"); err == nil || err.Error() != "WeChat Reading didn't accept this key" {
		t.Fatalf("a refused key: %v", err)
	}
	if m.status(&Catalog[1]).Connected || sk.on["weread-skills@/"] {
		t.Fatal("a refused key left the app connected or its skill on")
	}

	st, err := m.Connect(ctx, "weread", MethodKey, "wrk-good")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Connected || st.Account != "4 books on the shelf" {
		t.Fatalf("status = %+v", st)
	}
	// On for the main bot by default, off for the rest until turned on.
	if !sk.on["weread-skills@/"] || sk.on["weread-skills@/_bots/_news"] {
		t.Fatalf("skill links = %v", sk.on)
	}
	if got := m.Env(mainBot)["WEREAD_API_KEY"]; got != "wrk-good" {
		t.Fatalf("env key = %q", got)
	}
	if len(m.Env(newsBot)) != 0 || len(m.Connected(newsBot)) != 0 || m.Allows("news", "weread") {
		t.Fatal("a bot got an app it hasn't turned on")
	}
	if err := m.SetOn("news", "weread", true); err != nil {
		t.Fatal(err)
	}
	if !sk.on["weread-skills@/_bots/_news"] || m.Env(newsBot)["WEREAD_API_KEY"] != "wrk-good" || !m.Allows("news", "weread") {
		t.Fatal("turning the app on for a bot didn't reach it")
	}
	if got := m.List()[1].Bots; len(got) != 2 {
		t.Fatalf("bots on = %v", got)
	}
	if err := m.SetOn("main", "weread", false); err != nil {
		t.Fatal(err)
	}
	if sk.on["weread-skills@/"] || len(m.Env(mainBot)) != 0 {
		t.Fatal("turning the app off for the main bot didn't reach it")
	}
	// The switches belong to the vault and survive a restart.
	m2, err := New(Options{Dir: m.creds.dir, StateFile: m.stateFile, Bots: twoBots, Skills: sk})
	if err != nil || m2.Allows("main", "weread") || !m2.Allows("news", "weread") {
		t.Fatal("switches not reloaded")
	}
	if SkillOwner("weread-skills") != "WeChat Reading" || SkillOwner("humanizer") != "" {
		t.Fatal("skill owner not named")
	}
	if _, err := m.Tools(ctx, "weread"); err == nil {
		t.Fatal("a skill app offered app tools")
	}

	upgrade.Store(true)
	m.CheckAll(ctx)
	m.CheckAll(ctx)
	if sk.forced != 1 {
		t.Fatalf("one upgrade_info re-added the skill %d times", sk.forced)
	}
	upgrade.Store(false)

	// The key survives a restart, readable by the user only.
	info, err := os.Stat(filepath.Join(m.creds.dir, "weread.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("credential file: %v %v", info, err)
	}
	m2, err = New(Options{Dir: m.creds.dir, Skills: sk})
	if err != nil || m2.creds.get("weread").Secret != "wrk-good" {
		t.Fatal("credential not reloaded")
	}

	// A refused key takes the skill out of the bots' folders, not just the key out of their runs.
	revoked.Store(true)
	m.CheckAll(ctx)
	if !m.List()[1].Reconnect || len(sk.on) != 0 || len(m.Env(newsBot)) != 0 || len(m.Connected(newsBot)) != 0 {
		t.Fatalf("a refused key left the app usable: links %v", sk.on)
	}
	revoked.Store(false)

	if err := m.Disconnect(ctx, "weread"); err != nil {
		t.Fatal(err)
	}
	if len(sk.on) != 0 || sk.added["weread-skills"] || len(m.Env(newsBot)) != 0 || len(m.Connected(newsBot)) != 0 {
		t.Fatal("disconnect left the app or its skill behind")
	}
}

// mcpServer is a platform: tools behind a bearer token, and an OAuth server
// that knows Hylo by its client document.
type mcpServer struct {
	*httptest.Server
	token   atomic.Value // the bearer the platform accepts now
	plan    string
	codeFor map[string]string // code -> PKCE challenge

	mu        sync.Mutex
	refresh   string // the one refresh token that works: each use rotates it
	refreshes int
}

func newMCPServer(t *testing.T) *mcpServer {
	s := &mcpServer{plan: "Pro", codeFor: map[string]string{}}
	s.token.Store("good")
	srv := sdk.NewServer(&sdk.Implementation{Name: "fake", Version: "0"}, nil)
	sdk.AddTool(srv, &sdk.Tool{Name: "get_me", Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true}},
		func(context.Context, *sdk.CallToolRequest, struct{}) (*sdk.CallToolResult, map[string]any, error) {
			return nil, map[string]any{"email": "me@x.io", "plan": s.plan}, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "needs_pro"},
		func(context.Context, *sdk.CallToolRequest, struct{}) (*sdk.CallToolResult, struct{}, error) {
			return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: "This feature requires a Pro plan."}}}, struct{}{}, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "slow"},
		func(context.Context, *sdk.CallToolRequest, struct{}) (*sdk.CallToolResult, struct{}, error) {
			time.Sleep(300 * time.Millisecond)
			return nil, struct{}{}, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "delete_clip"},
		func(context.Context, *sdk.CallToolRequest, struct{}) (*sdk.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		})
	mcpH := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+s.token.Load().(string) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Like Podwise, take tool arguments only as an object.
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte(`"arguments":null`)) {
			http.Error(w, "arguments: expected record, received null", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		mcpH.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"resource": s.URL + "/mcp", "authorization_servers": []string{s.URL}, "scopes_supported": []string{"podwise"}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"issuer": s.URL, "authorization_endpoint": s.URL + "/authorize",
			"token_endpoint": s.URL + "/token", "response_types_supported": []string{"code"}, "code_challenge_methods_supported": []string{"S256"},
			"client_id_metadata_document_supported": true, "authorization_response_iss_parameter_supported": true})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") == "refresh_token" {
			s.mu.Lock()
			defer s.mu.Unlock()
			if r.Form.Get("client_id") != ClientID || r.Form.Get("resource") != s.URL+"/mcp" || r.Form.Get("refresh_token") != s.refresh {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			s.refreshes++
			access := fmt.Sprintf("good%d", s.refreshes)
			s.refresh = fmt.Sprintf("r%d", s.refreshes+1)
			s.token.Store(access)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","refresh_token":%q,"expires_in":3600}`, access, s.refresh)
			return
		}
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("client_id") != ClientID || r.Form.Get("resource") != s.URL+"/mcp" ||
			s.codeFor[r.Form.Get("code")] != base64.RawURLEncoding.EncodeToString(sum[:]) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		s.mu.Lock()
		s.refresh = "r1"
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"good","token_type":"Bearer","refresh_token":"r1","expires_in":3600}`))
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func (s *mcpServer) app() App {
	return App{ID: "pod", Name: "Pod", Kind: KindMCP, URL: s.URL + "/mcp", Check: "get_me", Hidden: []string{"delete_*"}}
}

func TestMCPAppToken(t *testing.T) {
	s := newMCPServer(t)
	m := newManager(t, []App{s.app()}, newFakeSkills())
	ctx := context.Background()

	if _, err := m.Connect(ctx, "pod", MethodToken, "bad"); err == nil {
		t.Fatal("a refused token went in")
	}
	st, err := m.Connect(ctx, "pod", MethodToken, "good")
	if err != nil {
		t.Fatal(err)
	}
	if st.Account != "me@x.io · Pro" {
		t.Fatalf("account = %q", st.Account)
	}
	tools, err := m.Tools(ctx, "pod")
	if err != nil || len(tools) != 3 || tools[0].Name != "get_me" || !tools[0].ReadOnly {
		t.Fatalf("tools = %+v, %v", tools, err)
	}
	if _, err := m.Call(ctx, "pod", "delete_clip", nil); err == nil {
		t.Fatal("a hidden tool ran")
	}
	res, err := m.Call(ctx, "pod", "get_me", json.RawMessage(`{}`))
	if err != nil || res.Data.(map[string]any)["email"] != "me@x.io" {
		t.Fatalf("call = %+v, %v", res, err)
	}

	// The platform revokes the token: the next call asks to connect again.
	s.token.Store("other")
	if _, err := m.Call(ctx, "pod", "get_me", nil); err == nil {
		t.Fatal("a refused call succeeded")
	}
	if st := m.List()[0]; !st.Reconnect || len(m.Connected(mainBot)) != 0 {
		t.Fatalf("status after refusal = %+v", st)
	}
}

// A tool call may take longer than the outbound client's timeout, which is
// for checks and sign-ins.
func TestMCPCallOutlastsClientTimeout(t *testing.T) {
	s := newMCPServer(t)
	m := newManager(t, []App{s.app()}, newFakeSkills())
	ctx := context.Background()
	if _, err := m.Connect(ctx, "pod", MethodToken, "good"); err != nil {
		t.Fatal(err)
	}
	m.http = &http.Client{Timeout: 100 * time.Millisecond}
	if _, err := m.Call(ctx, "pod", "slow", nil); err != nil {
		t.Fatalf("slow call: %v", err)
	}
}

// A plan without API access answers with a tool error: connected, with the
// platform's notice as the problem.
func TestMCPAppPlanLimit(t *testing.T) {
	s := newMCPServer(t)
	app := s.app()
	app.Check = "needs_pro"
	m := newManager(t, []App{app}, newFakeSkills())
	st, err := m.Connect(context.Background(), "pod", MethodToken, "good")
	if err != nil || !st.Connected || st.Problem != "This feature requires a Pro plan." || st.Reconnect {
		t.Fatalf("status = %+v, %v", st, err)
	}
}

func TestSignIn(t *testing.T) {
	s := newMCPServer(t)
	m := newManager(t, []App{s.app()}, newFakeSkills())
	ctx := context.Background()
	redirect := "http://127.0.0.1:54321" + CallbackPath

	start := func() url.Values {
		raw, err := m.StartSignIn(ctx, "pod", redirect)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(raw)
		q := u.Query()
		if q.Get("client_id") != ClientID || q.Get("redirect_uri") != redirect || q.Get("code_challenge_method") != "S256" ||
			q.Get("resource") != s.URL+"/mcp" || q.Get("scope") != "podwise" || !strings.HasPrefix(raw, s.URL+"/authorize?") {
			t.Fatalf("authorize url = %s", raw)
		}
		return q
	}

	// An answer from another server is refused.
	q := start()
	s.codeFor["c1"] = q.Get("code_challenge")
	if _, err := m.FinishSignIn(ctx, url.Values{"state": {q.Get("state")}, "code": {"c1"}, "iss": {"https://evil.example"}}); err == nil {
		t.Fatal("a foreign issuer was accepted")
	}
	if _, err := m.FinishSignIn(ctx, url.Values{"state": {q.Get("state")}, "code": {"c1"}, "iss": {s.URL}}); err == nil {
		t.Fatal("a sign-in finished twice")
	}

	q = start()
	s.codeFor["c2"] = q.Get("code_challenge")
	name, err := m.FinishSignIn(ctx, url.Values{"state": {q.Get("state")}, "code": {"c2"}, "iss": {s.URL}})
	if err != nil || name != "Pod" {
		t.Fatalf("finish = %q, %v", name, err)
	}
	st := m.List()[0]
	if !st.Connected || st.Method != MethodOAuth || st.Account != "me@x.io · Pro" {
		t.Fatalf("status = %+v", st)
	}
	c := m.creds.get("pod")
	if c.OAuth == nil || c.OAuth.Token.RefreshToken != "r1" {
		t.Fatalf("grant = %+v", c.OAuth)
	}
}

// signedIn is a manager with s's app connected through a browser sign-in.
func signedIn(t *testing.T, s *mcpServer) *Manager {
	t.Helper()
	m := newManager(t, []App{s.app()}, newFakeSkills())
	raw, err := m.StartSignIn(context.Background(), "pod", "http://127.0.0.1:1"+CallbackPath)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	s.codeFor["c"] = u.Query().Get("code_challenge")
	if _, err := m.FinishSignIn(context.Background(), url.Values{"state": {u.Query().Get("state")}, "code": {"c"}, "iss": {s.URL}}); err != nil {
		t.Fatal(err)
	}
	return m
}

func expire(t *testing.T, m *Manager, id string) {
	t.Helper()
	c := m.creds.get(id)
	c.OAuth.Token.Expiry = time.Now().Add(-time.Minute)
	if err := m.creds.put(id, c); err != nil {
		t.Fatal(err)
	}
}

// An expired sign-in renews once for calls made together, keeps the rotated
// refresh token, and asks to connect again only when the platform refuses it.
func TestSignInRefresh(t *testing.T) {
	s := newMCPServer(t)
	m := signedIn(t, s)
	expire(t, m, "pod")

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.Call(context.Background(), "pod", "get_me", nil)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("call after expiry: %v", err)
		}
	}
	if s.refreshes != 1 {
		t.Fatalf("refreshed %d times for calls made together", s.refreshes)
	}
	g := m.creds.get("pod").OAuth
	if g.Token.AccessToken != "good1" || g.Token.RefreshToken != "r2" || !g.Token.Valid() {
		t.Fatalf("kept token = %+v", g.Token)
	}

	// Renewed again from what was kept, not from the first refresh token.
	expire(t, m, "pod")
	if _, err := m.Call(context.Background(), "pod", "get_me", nil); err != nil || s.refreshes != 2 {
		t.Fatalf("second renewal: %v, %d refreshes", err, s.refreshes)
	}

	expire(t, m, "pod")
	s.mu.Lock()
	s.refresh = "revoked"
	s.mu.Unlock()
	if _, err := m.Call(context.Background(), "pod", "get_me", nil); err == nil {
		t.Fatal("a refused renewal went through")
	}
	if st := m.List()[0]; !st.Reconnect {
		t.Fatalf("status after refused renewal = %+v", st)
	}
}

// A check that renews an expired sign-in keeps the renewed token: writing its
// copy back would restore a refresh token the platform already rotated away.
func TestCheckKeepsRenewedSignIn(t *testing.T) {
	s := newMCPServer(t)
	m := signedIn(t, s)
	expire(t, m, "pod")
	m.CheckAll(context.Background())
	if g := m.creds.get("pod").OAuth; g.Token.RefreshToken != "r2" || !g.Token.Valid() {
		t.Fatalf("kept token after check = %+v", g.Token)
	}
	if _, err := m.Call(context.Background(), "pod", "get_me", nil); err != nil || s.refreshes != 1 {
		t.Fatalf("call after check: %v, %d refreshes", err, s.refreshes)
	}
}

// What a call or check finds lands only on the connection it ran with: not on
// a disconnected app, which would come back, nor on a newer connection.
func TestFindingsLandOnTheirConnection(t *testing.T) {
	s := newMCPServer(t)
	m := newManager(t, []App{s.app()}, newFakeSkills())
	ctx := context.Background()
	if _, err := m.Connect(ctx, "pod", MethodToken, "good"); err != nil {
		t.Fatal(err)
	}
	old := m.creds.get("pod")
	if err := m.Disconnect(ctx, "pod"); err != nil {
		t.Fatal(err)
	}
	_ = m.refused(&m.catalog[0], old, ErrRefused)
	if m.creds.get("pod") != nil {
		t.Fatal("a refusal brought a disconnected app back")
	}

	if _, err := m.Connect(ctx, "pod", MethodToken, "good"); err != nil {
		t.Fatal(err)
	}
	_ = m.refused(&m.catalog[0], old, ErrRefused)
	if st := m.List()[0]; st.Reconnect {
		t.Fatalf("an old connection's refusal marked the new one: %+v", st)
	}
}

func TestSignInNeedsClientDocument(t *testing.T) {
	as := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource/mcp":
			writeJSON(w, map[string]any{"resource": "http://" + r.Host + "/mcp", "authorization_servers": []string{"http://" + r.Host}})
		case "/.well-known/oauth-authorization-server":
			writeJSON(w, map[string]any{"issuer": "http://" + r.Host, "authorization_endpoint": "http://" + r.Host + "/a",
				"token_endpoint": "http://" + r.Host + "/t", "response_types_supported": []string{"code"}, "code_challenge_methods_supported": []string{"S256"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer as.Close()
	m := newManager(t, []App{{ID: "x", Name: "X", Kind: KindMCP, URL: as.URL + "/mcp"}}, newFakeSkills())
	if _, err := m.StartSignIn(context.Background(), "x", "http://127.0.0.1:1"+CallbackPath); !errors.Is(err, ErrNoSignIn) {
		t.Fatalf("err = %v", err)
	}
}

func TestHides(t *testing.T) {
	a := App{Hidden: []string{"delete_clip", "enterprise_*"}}
	for tool, want := range map[string]bool{"delete_clip": true, "enterprise_export": true, "get_me": false, "delete_clips": false} {
		if a.hides(tool) != want {
			t.Errorf("hides(%q) = %v", tool, !want)
		}
	}
}

// A bot that is gone loses the app's skill from its folder, though no switch moved.
func TestReconcileDropsGoneBots(t *testing.T) {
	sk := newFakeSkills()
	sk.added["weread-skills"] = true
	bots := []Bot{mainBot, newsBot}
	m, err := New(Options{Dir: t.TempDir(), StateFile: filepath.Join(t.TempDir(), "apps.json"), Skills: sk,
		Bots: func() ([]Bot, error) { return bots, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.creds.put("weread", &credential{Method: MethodKey, Secret: "wrk-x"}); err != nil {
		t.Fatal(err)
	}
	if err := m.SetOn("news", "weread", true); err != nil {
		t.Fatal(err)
	}
	if !sk.on["weread-skills@/_bots/_news"] {
		t.Fatal("not linked for the bot")
	}
	bots = []Bot{mainBot}
	m.Reconcile()
	if sk.on["weread-skills@/_bots/_news"] || !sk.on["weread-skills@/"] {
		t.Fatalf("links after the bot went = %v", sk.on)
	}
	if _, ok := m.state.Bots["news"]; ok {
		t.Fatal("the gone bot's switch stayed")
	}
	// A bot list that can't be read takes nothing out.
	m.bots = func() ([]Bot, error) { return nil, errors.New("down") }
	m.Reconcile()
	if !sk.on["weread-skills@/"] {
		t.Fatal("an unreadable bot list emptied the folders")
	}
}
