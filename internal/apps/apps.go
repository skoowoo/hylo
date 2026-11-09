// Package apps connects Hylo to the user's own accounts on outside platforms
// (podcasts, reading), so bots can use their personal data there.
//
// An app reaches its platform in one of two ways (Kind): a remote MCP server
// Hylo signs in to and calls for the bot, or a skill the agent follows itself
// with a key Hylo hands its process. Connecting, checking, statuses and the
// credential store are the same for both; only the driver differs.
package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// Kind is how an app reaches its platform.
type Kind string

const (
	KindMCP   Kind = "mcp"
	KindSkill Kind = "skill"
)

// Ways to connect, as Connect takes them and a credential records them.
const (
	MethodOAuth = "oauth" // browser sign-in (MCP apps)
	MethodToken = "token" // an API token, the fallback when sign-in can't reach Hylo (MCP apps)
	MethodKey   = "key"   // an API key (skill apps)
)

var methodNoun = map[string]string{MethodOAuth: "sign-in", MethodToken: "API token", MethodKey: "key"}

var (
	ErrUnknown      = errors.New("no such app")
	ErrNotConnected = errors.New("app is not connected")
	// ErrRefused means the platform turned the credential down: connect again.
	ErrRefused = errors.New("the platform refused the credential")
)

// limitedError is a credential that works for an account the platform won't
// serve yet, such as a plan without API access: connected, with a problem.
type limitedError struct{ msg string }

func (e *limitedError) Error() string { return e.msg }

// App is one app Hylo knows how to connect.
type App struct {
	ID    string
	Name  string
	About string // what it gives the bot, under its name in Settings
	Kind  Kind

	// MCP apps.
	URL string
	// Hidden tools are never offered to bots. A trailing "*" matches a prefix.
	Hidden []string
	// Check is a read-only tool whose answer names the account.
	Check string

	// KeyURL is where the user gets a key, or an MCP app's API token.
	KeyURL string

	// Skill apps.
	Repo, Skill string
	Env         string // the variable the skill reads its key from
	KeyPrefix   string
	// verifyKey checks a key against the platform and names the account;
	// upgrade is the platform's ask for a newer skill, or "".
	verifyKey func(ctx context.Context, hc *http.Client, key, skillVersion string) (account, upgrade string, err error)
}

func (a *App) hides(tool string) bool {
	for _, h := range a.Hidden {
		if p, ok := strings.CutSuffix(h, "*"); h == tool || ok && strings.HasPrefix(tool, p) {
			return true
		}
	}
	return false
}

// SkillHost keeps apps' skills, apart from the user's, and turns them on for
// a bot (internal/skills, rooted at ~/.hylo/apps/skills).
type SkillHost interface {
	Has(name string) bool
	Add(repo, name string, force bool) (string, error)
	Remove(name string) error
	Enable(name, dir string) error
	Disable(name, dir string) error
	EnabledDirs(name string) []string
	Version(name string) string
}

// Status is where one app stands, for Settings.
type Status struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	About     string   `json:"about"`
	Kind      Kind     `json:"kind"`
	Connected bool     `json:"connected"`
	Method    string   `json:"method,omitempty"`
	Account   string   `json:"account,omitempty"`
	Problem   string   `json:"problem,omitempty"`
	Reconnect bool     `json:"reconnect,omitempty"` // the problem goes away by connecting again
	KeyURL    string   `json:"keyUrl,omitempty"`
	KeyPrefix string   `json:"keyPrefix,omitempty"`
	Bots      []BotRef `json:"bots,omitempty"` // the bots it is on for, once connected
}

// Brief is a connected app as a bot's prompt names it.
type Brief struct {
	ID    string
	Name  string
	About string
	Skill string
}

// Tool is one tool of a connected MCP app.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
	ReadOnly    bool   `json:"readOnly,omitempty"`
}

// CallResult is what a tool of a connected MCP app answered.
type CallResult struct {
	Text    string `json:"text,omitempty"`
	Data    any    `json:"data,omitempty"`
	IsError bool   `json:"isError,omitempty"`
}

// Options configure a Manager.
type Options struct {
	Dir       string // where credentials live, outside the vault
	StateFile string // where per-bot switches live, in the vault
	Bots      func() ([]Bot, error)
	Skills    SkillHost
	HTTP      *http.Client // outbound, through the server's proxy
	Logger    *slog.Logger
	OnChange  func()
	Catalog   []App // tests only; Catalog otherwise
}

// Manager is the one place apps are connected, checked and used.
type Manager struct {
	catalog  []App
	creds    *credStore
	skills   SkillHost
	http     *http.Client
	logger   *slog.Logger
	onChange func()

	bots      func() ([]Bot, error)
	stateFile string

	mu       sync.Mutex
	flows    map[string]*flow // browser sign-ins under way, by state
	state    botState
	upgraded map[string]string // app id → the platform's upgrade ask its skill was re-added for

	refreshMu sync.Mutex // one sign-in refresh at a time
}

// New opens the credential store in o.Dir.
func New(o Options) (*Manager, error) {
	if o.Skills == nil {
		return nil, errors.New("apps: no skill host")
	}
	cs, err := openCreds(o.Dir)
	if err != nil {
		return nil, err
	}
	st, err := loadBotState(o.StateFile)
	if err != nil {
		return nil, err
	}
	m := &Manager{catalog: o.Catalog, creds: cs, skills: o.Skills, http: o.HTTP, logger: o.Logger, onChange: o.OnChange,
		bots: o.Bots, stateFile: o.StateFile, flows: map[string]*flow{}, state: st, upgraded: map[string]string{}}
	if m.catalog == nil {
		m.catalog = Catalog
	}
	if m.http == nil {
		m.http = http.DefaultClient
	}
	if m.logger == nil {
		m.logger = slog.Default()
	}
	return m, nil
}

func (m *Manager) app(id string) (*App, error) {
	for i := range m.catalog {
		if m.catalog[i].ID == id {
			return &m.catalog[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknown, id)
}

func (m *Manager) changed() {
	if m.onChange != nil {
		m.onChange()
	}
}

// List is every app, connected or not, in catalog order.
func (m *Manager) List() []Status {
	out := make([]Status, 0, len(m.catalog))
	for i := range m.catalog {
		out = append(out, m.status(&m.catalog[i]))
	}
	return out
}

func (m *Manager) status(a *App) Status {
	s := Status{ID: a.ID, Name: a.Name, About: a.About, Kind: a.Kind, KeyURL: a.KeyURL, KeyPrefix: a.KeyPrefix}
	if c := m.creds.get(a.ID); c != nil {
		s.Connected, s.Method, s.Account, s.Problem, s.Reconnect = true, c.Method, c.Account, c.Problem, c.Reconnect
		s.Bots = m.botsOn(a.ID)
	}
	return s
}

// Connect connects app id with a key (skill apps) or an API token (MCP apps),
// once the platform takes it.
func (m *Manager) Connect(ctx context.Context, id, method, secret string) (Status, error) {
	a, err := m.app(id)
	if err != nil {
		return Status{}, err
	}
	switch {
	case secret == "":
		return Status{}, errors.New("empty " + method)
	case a.Kind == KindSkill && method == MethodKey, a.Kind == KindMCP && method == MethodToken:
	default:
		return Status{}, fmt.Errorf("%s doesn't connect with a %s", a.Name, method)
	}
	if !strings.HasPrefix(secret, a.KeyPrefix) {
		return Status{}, fmt.Errorf("a %s key starts with %s", a.Name, a.KeyPrefix)
	}
	return m.save(ctx, a, &credential{Method: method, Secret: secret})
}

// save checks c with the platform, sets up what the app needs, then keeps c.
// A refused credential is never kept.
func (m *Manager) save(ctx context.Context, a *App, c *credential) (Status, error) {
	if a.Kind == KindSkill {
		if err := m.installSkill(a, false); err != nil {
			return Status{}, err
		}
	}
	k := m.verify(ctx, a, c)
	if k.reconnect {
		return Status{}, fmt.Errorf("%s didn't accept this %s", a.Name, methodNoun[c.Method])
	}
	if k.unreachable() {
		return Status{}, errors.New(k.problem)
	}
	k.apply(c)
	c.ConnectedAt = time.Now()
	if err := m.creds.put(a.ID, c); err != nil {
		return Status{}, err
	}
	m.Reconcile()
	m.changed()
	return m.status(a), nil
}

// checkTimeout bounds one platform's answer to a check.
const checkTimeout = 30 * time.Second

// check is what asking a platform about a credential found.
type check struct {
	account   string
	problem   string
	reconnect bool // the platform refused the credential
	limited   bool // a limitedError: connected, with a problem
}

// unreachable is a check with no answer from the platform, which changes nothing.
func (k check) unreachable() bool { return k.problem != "" && !k.reconnect && !k.limited }

func (k check) apply(c *credential) {
	c.Problem, c.Reconnect = k.problem, k.reconnect
	if k.problem == "" {
		c.Account = k.account
	}
}

func refusal(a *App) check {
	return check{problem: a.Name + " no longer accepts this connection. Connect again.", reconnect: true}
}

// verify asks the platform about c.
func (m *Manager) verify(ctx context.Context, a *App, c *credential) check {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	var account string
	var err error
	switch a.Kind {
	case KindMCP:
		account, err = m.verifyMCP(ctx, a, c)
	case KindSkill:
		var upgrade string
		account, upgrade, err = a.verifyKey(ctx, m.http, c.Secret, m.skills.Version(a.Skill))
		m.upgradeSkill(a, upgrade)
	}
	var lim *limitedError
	switch {
	case err == nil:
		return check{account: account}
	case errors.Is(err, ErrRefused):
		return refusal(a)
	case errors.As(err, &lim):
		return check{problem: lim.msg, limited: true}
	default:
		return check{problem: err.Error()}
	}
}

// upgradeSkill re-adds a's skill once per upgrade the platform asks for: one
// that keeps asking (its repository not caught up yet) mustn't clone on every check.
func (m *Manager) upgradeSkill(a *App, ask string) {
	m.mu.Lock()
	done := ask == "" || m.upgraded[a.ID] == ask
	m.mu.Unlock()
	if done {
		return
	}
	if err := m.installSkill(a, true); err != nil {
		m.logger.Warn("apps: upgrade skill", "app", a.ID, "err", err)
		return
	}
	m.mu.Lock()
	m.upgraded[a.ID] = ask
	m.mu.Unlock()
}

func (m *Manager) installSkill(a *App, force bool) error {
	if !force && m.skills.Has(a.Skill) {
		return nil
	}
	if _, err := m.skills.Add(a.Repo, a.Skill, force); err != nil {
		return fmt.Errorf("install the %s skill: %w", a.Name, err)
	}
	return nil
}

// Disconnect forgets app id's credential and turns off what it set up.
func (m *Manager) Disconnect(ctx context.Context, id string) error {
	a, err := m.app(id)
	if err != nil {
		return err
	}
	c := m.creds.get(id)
	if c == nil {
		return nil
	}
	if c.OAuth != nil {
		m.revoke(ctx, c.OAuth)
	}
	if err := m.creds.remove(id); err != nil {
		return err
	}
	m.Reconcile()
	if a.Kind == KindSkill && m.skills.Has(a.Skill) {
		// The skill lives and goes with the app; connecting again fetches it anew.
		if err := m.skills.Remove(a.Skill); err != nil {
			m.logger.Warn("apps: remove skill", "app", id, "err", err)
		}
	}
	m.changed()
	return nil
}

// CheckAll asks each platform whether its connection still works, and puts
// back what a skill app needs (another vault, a deleted skill).
// Only answers change a status; a platform that can't be reached keeps it.
func (m *Manager) CheckAll(ctx context.Context) {
	for i := range m.catalog {
		a := &m.catalog[i]
		c := m.creds.get(a.ID)
		if c == nil {
			continue
		}
		if a.Kind == KindSkill {
			if err := m.installSkill(a, false); err != nil {
				m.logger.Warn("apps: skill", "app", a.ID, "err", err)
			}
		}
		k := m.verify(ctx, a, c)
		if k.unreachable() {
			m.logger.Warn("apps: check", "app", a.ID, "err", k.problem)
			continue
		}
		if err := m.creds.update(a.ID, c.ConnectedAt, k.apply); err != nil {
			m.logger.Warn("apps: save check", "app", a.ID, "err", err)
		}
	}
	m.Reconcile()
	m.changed()
}

// Env is what b's agent process needs for its skill apps: their keys.
func (m *Manager) Env(b Bot) map[string]string {
	env := map[string]string{}
	for i := range m.catalog {
		a := &m.catalog[i]
		if c := m.creds.get(a.ID); a.Kind == KindSkill && c.usable() && m.isOn(b, a.ID) {
			env[a.Env] = c.Secret
		}
	}
	return env
}

// Connected lists the apps b can use now.
func (m *Manager) Connected(b Bot) []Brief {
	var out []Brief
	for i := range m.catalog {
		a := &m.catalog[i]
		if m.creds.get(a.ID).usable() && m.isOn(b, a.ID) {
			out = append(out, Brief{ID: a.ID, Name: a.Name, About: a.About, Skill: a.Skill})
		}
	}
	return out
}

// SkillOwner names the app a skill name belongs to, or "": the user's own
// skills can't take it, since both end up linked under that name in a bot's folder.
func SkillOwner(skill string) string {
	for i := range Catalog {
		if a := &Catalog[i]; a.Kind == KindSkill && a.Skill == skill {
			return a.Name
		}
	}
	return ""
}

// mcpApp is id when it is a connected MCP app the bot can call.
func (m *Manager) mcpApp(id string) (*App, *credential, error) {
	a, err := m.app(id)
	if err != nil {
		return nil, nil, err
	}
	if a.Kind != KindMCP {
		return nil, nil, fmt.Errorf("%s works through its skill %s, not through app tools", a.Name, a.Skill)
	}
	c := m.creds.get(id)
	if c == nil {
		return nil, nil, fmt.Errorf("%w: connect %s in Settings → Apps", ErrNotConnected, a.Name)
	}
	if c.Reconnect {
		return nil, nil, fmt.Errorf("%s: %s", a.Name, c.Problem)
	}
	return a, c, nil
}

// Tools lists the tools of connected MCP app id, without hidden ones.
func (m *Manager) Tools(ctx context.Context, id string) ([]Tool, error) {
	a, c, err := m.mcpApp(id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	tools, err := m.listTools(ctx, a, c)
	if err != nil {
		return nil, m.refused(a, c, err)
	}
	return slices.DeleteFunc(tools, func(t Tool) bool { return a.hides(t.Name) }), nil
}

// CallTimeout bounds one tool call: some (asking about an episode, processing
// one) take the platform minutes.
const CallTimeout = 10 * time.Minute

// Call runs tool of connected MCP app id with args, a JSON object.
func (m *Manager) Call(ctx context.Context, id, tool string, args json.RawMessage) (*CallResult, error) {
	a, c, err := m.mcpApp(id)
	if err != nil {
		return nil, err
	}
	if a.hides(tool) {
		return nil, fmt.Errorf("%s's %s isn't offered to bots", a.Name, tool)
	}
	var in map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, fmt.Errorf("args must be a JSON object: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	res, err := m.callTool(ctx, a, c, tool, in)
	if err != nil {
		return nil, m.refused(a, c, err)
	}
	return res, nil
}

// refused records a credential the platform turned down mid-use, so Settings
// asks to connect again; other errors pass through.
func (m *Manager) refused(a *App, c *credential, err error) error {
	if !errors.Is(err, ErrRefused) {
		return err
	}
	k := refusal(a)
	if perr := m.creds.update(a.ID, c.ConnectedAt, k.apply); perr != nil {
		m.logger.Warn("apps: save refusal", "app", a.ID, "err", perr)
	}
	m.changed()
	return fmt.Errorf("%s: %s Settings → Apps", a.Name, k.problem)
}
