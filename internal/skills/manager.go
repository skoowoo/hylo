package skills

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gogitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/hardhacker/hylo/internal/util"
	builtin "github.com/hardhacker/hylo/skills"
)

//go:embed catalog.json
var catalogJSON []byte

// proxyOnce ensures the go-git HTTP transport is configured with proxy settings
// exactly once per process, using the first Add call's shell environment.
var proxyOnce sync.Once

// DefaultSkills are the built-in skills shipped in the binary: always on at the
// vault root, and neither removable nor editable for good (see SyncBuiltins).
var DefaultSkills = func() []string {
	entries, err := fs.ReadDir(builtin.FS, ".")
	if err != nil {
		panic("skills: read built-ins: " + err.Error())
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}()

// Retired maps built-in skills no longer shipped to the one that replaced them
// ("" when none), so their files, links and the prompts naming them move on.
var Retired = map[string]string{
	"hylo-recap": "hylo-recall",
	// Copied in by older install scripts, never embedded.
	"hylo-quote-extract": "",
}

// state persists where skills are on, beyond built-ins at the vault root.
type state struct {
	// Enabled maps a skill to the vault directories it is on in; "/" is the vault root.
	Enabled map[string][]string `json:"enabled"`
	// EnabledExtra is the older form, skills on at the vault root; loadState migrates it.
	EnabledExtra []string `json:"enabled_extra,omitempty"`
}

// SkillInfo describes one skill added to ~/.hylo/skills.
type SkillInfo struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
	// Dirs lists every vault directory the skill is on in.
	Dirs []string `json:"dirs"`
	// Repo is set when the skill matches a catalog entry by name.
	Repo        string `json:"repo,omitempty"`
	Description string `json:"description,omitempty"`
}

// CatalogEntry is a skill Hylo recommends; Repo and Name are what Add takes.
type CatalogEntry struct {
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Added       bool   `json:"added"`
}

var catalog = func() []CatalogEntry {
	var c []CatalogEntry
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic("skills: bad catalog.json: " + err.Error())
	}
	return c
}()

// Manager tracks which skills are enabled and manages their vault symlinks.
type Manager struct {
	mu         sync.Mutex
	sourceDir  string
	vaultRoot  string
	stateFile  string
	st         state
	shellEnvFn func() []string // returns login-shell env for git operations
	descs      map[string]cachedDesc
	onChange   func()
}

// SetOnChange registers fn to run after a skill is added, turned on or off,
// or removed. Set it before the manager is in use.
func (m *Manager) SetOnChange(fn func()) { m.onChange = fn }

// changed passes err through, telling the hook first when the change went in.
func (m *Manager) changed(err error) error {
	if err == nil && m.onChange != nil {
		m.onChange()
	}
	return err
}

// cachedDesc spares List from re-parsing an unchanged SKILL.md on every call.
type cachedDesc struct {
	mod  time.Time
	text string
}

// Open creates a Manager rooted at vaultRoot. shellEnvFn is called when
// adding skills to obtain the user's login-shell environment (proxy vars,
// etc.); pass nil to fall back to os.Environ.
// State is loaded from disk; errors are silently ignored.
func Open(vaultRoot string, shellEnvFn func() []string) *Manager {
	home, _ := os.UserHomeDir()
	if shellEnvFn == nil {
		shellEnvFn = os.Environ
	}
	m := &Manager{
		sourceDir:  filepath.Join(home, ".hylo", "skills"),
		vaultRoot:  vaultRoot,
		stateFile:  filepath.Join(vaultRoot, ".hylo", "skills_state.json"),
		shellEnvFn: shellEnvFn,
	}
	_ = m.loadState()
	return m
}

func (m *Manager) loadState() error {
	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, &m.st); err != nil {
		return err
	}
	for _, name := range m.st.EnabledExtra {
		m.st.Enabled = addDir(m.st.Enabled, name, "/")
	}
	m.st.EnabledExtra = nil
	return nil
}

func (m *Manager) saveState() error {
	if err := os.MkdirAll(filepath.Dir(m.stateFile), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(&m.st)
	if err != nil {
		return err
	}
	return os.WriteFile(m.stateFile, data, 0644)
}

func isDefault(name string) bool {
	for _, n := range DefaultSkills {
		if n == name {
			return true
		}
	}
	return false
}

// dirsOf lists the vault directories name is on in; callers must hold m.mu.
func (m *Manager) dirsOf(name string) []string {
	dirs := m.st.Enabled[name]
	if isDefault(name) && !slices.Contains(dirs, "/") {
		dirs = append([]string{"/"}, dirs...)
	}
	return slices.Sorted(slices.Values(dirs))
}

func addDir(enabled map[string][]string, name, dir string) map[string][]string {
	if enabled == nil {
		enabled = map[string][]string{}
	}
	if !slices.Contains(enabled[name], dir) {
		enabled[name] = append(enabled[name], dir)
	}
	return enabled
}

// linkDirs are where agents started in the vault directory dir look for skills.
func (m *Manager) linkDirs(dir string) []string {
	base := filepath.Join(m.vaultRoot, filepath.FromSlash(dir))
	return []string{filepath.Join(base, ".agents", "skills"), filepath.Join(base, ".claude", "skills")}
}

// List returns the skills added to ~/.hylo/skills.
func (m *Manager) List() ([]SkillInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.sourceDir)
	if os.IsNotExist(err) {
		return []SkillInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	repos := map[string]string{}
	for _, e := range catalog {
		repos[e.Name] = e.Repo
	}
	out := []SkillInfo{}
	for _, e := range entries {
		name := e.Name()
		dir := filepath.Join(m.sourceDir, name)
		// Skips staging dirs like .<name>.new that a crash mid-sync leaves behind.
		if !ValidName(name) || !isSkillDir(dir) {
			continue
		}
		dirs := m.dirsOf(name)
		out = append(out, SkillInfo{
			Name:        name,
			Default:     isDefault(name),
			Dirs:        dirs,
			Repo:        repos[name],
			Description: m.skillDescription(dir),
		})
	}
	return out, nil
}

// Catalog returns the skills Hylo recommends, marking the ones already added.
func (m *Manager) Catalog() []CatalogEntry {
	out := make([]CatalogEntry, len(catalog))
	for i, e := range catalog {
		e.Added = isSkillDir(filepath.Join(m.sourceDir, e.Name))
		out[i] = e
	}
	return out
}

// Enable turns name on in the vault directory dir ("" or "/" for the root) by linking
// it where agents started there look, creating dir if needed. Built-in skills are
// always on at the root.
func (m *Manager) Enable(name, dir string) error {
	dir, err := util.CleanVaultDir(dir)
	if err != nil {
		return err
	}
	if isDefault(name) && dir == "/" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	src := filepath.Join(m.sourceDir, name)
	if !isSkillDir(src) {
		return fmt.Errorf("skill %q is not added; add it first", name)
	}
	for _, t := range m.linkDirs(dir) {
		if err := os.MkdirAll(t, 0755); err != nil {
			return err
		}
		dst := filepath.Join(t, name)
		_ = os.Remove(dst)
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}
	m.st.Enabled = addDir(m.st.Enabled, name, dir)
	return m.changed(m.saveState())
}

// Disable turns name off in the vault directory dir ("" or "/" for the root).
// Built-in skills cannot be turned off at the root.
func (m *Manager) Disable(name, dir string) error {
	dir, err := util.CleanVaultDir(dir)
	if err != nil {
		return err
	}
	if isDefault(name) && dir == "/" {
		return fmt.Errorf("skill %q is built in and always on for the main bot", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unlink(name, dir)
	if dirs := slices.DeleteFunc(m.st.Enabled[name], func(d string) bool { return d == dir }); len(dirs) > 0 {
		m.st.Enabled[name] = dirs
	} else {
		delete(m.st.Enabled, name)
	}
	return m.changed(m.saveState())
}

// DisableDir turns every skill off in the vault directory dir, as when the bot
// working there is deleted. The vault root keeps its skills.
func (m *Manager) DisableDir(dir string) error {
	dir, err := util.CleanVaultDir(dir)
	if err != nil || dir == "/" {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, dirs := range m.st.Enabled {
		if !slices.Contains(dirs, dir) {
			continue
		}
		m.unlink(name, dir)
		if dirs = slices.DeleteFunc(dirs, func(d string) bool { return d == dir }); len(dirs) > 0 {
			m.st.Enabled[name] = dirs
		} else {
			delete(m.st.Enabled, name)
		}
	}
	return m.changed(m.saveState())
}

// unlink removes name's links from dir, leaving anything that isn't a link alone.
func (m *Manager) unlink(name, dir string) {
	for _, t := range m.linkDirs(dir) {
		dst := filepath.Join(t, name)
		if fi, err := os.Lstat(dst); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(dst)
		}
	}
}

// Remove turns the skill off everywhere and deletes it from ~/.hylo/skills/.
// Returns an error for built-in skills.
func (m *Manager) Remove(name string) error {
	if isDefault(name) {
		return fmt.Errorf("skill %q is a built-in skill and cannot be removed", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if !isSkillDir(filepath.Join(m.sourceDir, name)) {
		return fmt.Errorf("skill %q is not added", name)
	}
	for _, dir := range m.dirsOf(name) {
		m.unlink(name, dir)
	}
	delete(m.st.Enabled, name)
	if err := m.saveState(); err != nil {
		return err
	}
	return m.changed(os.RemoveAll(filepath.Join(m.sourceDir, name)))
}

// Add clones a GitHub repository and copies the skill in it to ~/.hylo/skills/<name>,
// turned off. repo is a GitHub URL or owner/repo; the skill is found in it by name
// (see findSkillDir), which defaults to the repository's name. An added skill of the
// same name is replaced only with force.
// It returns the name the skill was added under.
func (m *Manager) Add(repo, name string, force bool) (string, error) {
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(strings.TrimRight(repo, "/")), ".git")
	}
	if !ValidName(name) {
		return "", fmt.Errorf("invalid skill name %q", name)
	}
	if isDefault(name) {
		return "", fmt.Errorf("%q is a built-in skill", name)
	}
	dest := filepath.Join(m.sourceDir, name)
	if !force && isSkillDir(dest) {
		return "", fmt.Errorf("skill %q is already added (use force to replace it)", name)
	}
	repoURL := normalizeRepoURL(repo)

	// Register proxy transport once, using the captured shell environment.
	proxyOnce.Do(func() { setupProxyTransport(m.shellEnvFn()) })

	tmp, err := os.MkdirTemp("", "hylo-skill-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	_, err = git.PlainClone(tmp, false, &git.CloneOptions{
		URL:           repoURL,
		Depth:         1,
		ReferenceName: plumbing.HEAD,
		SingleBranch:  true,
	})
	if err != nil {
		return "", fmt.Errorf("clone %s: %w", repoURL, err)
	}

	src, err := findSkillDir(tmp, name)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(m.sourceDir, 0755); err != nil {
		return "", fmt.Errorf("create skills dir: %w", err)
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", fmt.Errorf("remove existing skill: %w", err)
	}
	if err := copyDir(src, dest); err != nil {
		return "", fmt.Errorf("copy skill: %w", err)
	}
	return name, m.changed(nil)
}

// ValidName reports whether name is safe as a directory under ~/.hylo/skills.
func ValidName(name string) bool {
	return name != "" && !strings.ContainsAny(name, `/\`) && !strings.Contains(name, "..") && !strings.HasPrefix(name, ".")
}

// setupProxyTransport installs a custom go-git HTTP/HTTPS transport that routes
// through the proxy found in env (http_proxy, https_proxy, all_proxy).
// When no proxy is configured, Go's default ProxyFromEnvironment is preserved.
func setupProxyTransport(env []string) {
	proxyURL := pickProxyFromEnv(env)
	var proxyFn func(*http.Request) (*url.URL, error)
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil && u.Host != "" {
			proxyFn = http.ProxyURL(u)
		}
	}
	if proxyFn == nil {
		return // no proxy configured; go-git default is fine
	}
	customClient := &http.Client{
		Transport: &http.Transport{Proxy: proxyFn},
	}
	transport := githttp.NewClient(customClient)
	gogitclient.InstallProtocol("http", transport)
	gogitclient.InstallProtocol("https", transport)
}

// pickProxyFromEnv returns the first non-empty proxy value found in env,
// checking https_proxy, HTTPS_PROXY, http_proxy, HTTP_PROXY, all_proxy, ALL_PROXY
// in that order.
func pickProxyFromEnv(env []string) string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	for _, key := range []string{"https_proxy", "HTTPS_PROXY", "http_proxy", "HTTP_PROXY", "all_proxy", "ALL_PROXY"} {
		if v := m[key]; v != "" {
			return v
		}
	}
	return ""
}

// normalizeRepoURL converts "owner/repo" shorthand to a GitHub HTTPS URL.
func normalizeRepoURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "git@") {
		return raw
	}
	return "https://github.com/" + raw
}

// skillCandidatePaths returns the ordered list of sub-paths to check for a
// skill named skillName inside a cloned repository root.
func skillCandidatePaths(skillName string) []string {
	return []string{
		skillName,
		filepath.Join("skills", skillName),
	}
}

// findSkillDir searches the cloned repo at repoRoot for the skill source
// directory. It tries, in order:
//  1. Repo root itself (the whole repo is one skill)
//  2. <skillName>/SKILL.md           — flat layout
//  3. skills/<skillName>/SKILL.md    — standard container
//  4. skills/<category>/<skillName>/SKILL.md — catalog layout
//  5. skills/ or skill/ directory itself is the skill
func findSkillDir(repoRoot, skillName string) (string, error) {
	// 1. Repo root is the skill itself.
	if isSkillDir(repoRoot) {
		return repoRoot, nil
	}

	// 2–3. Named subdirectory candidates.
	for _, rel := range skillCandidatePaths(skillName) {
		p := filepath.Join(repoRoot, rel)
		if isSkillDir(p) {
			return p, nil
		}
	}

	// 4. Catalog layout: skills/<category>/<skillName>/SKILL.md
	skillsContainer := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsContainer)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(skillsContainer, e.Name(), skillName)
			if isSkillDir(p) {
				return p, nil
			}
		}
	}

	// 5. The skills/ or skill/ directory itself is the skill.
	if isSkillDir(skillsContainer) {
		return skillsContainer, nil
	}
	if p := filepath.Join(repoRoot, "skill"); isSkillDir(p) {
		return p, nil
	}

	return "", fmt.Errorf(
		"skill %q not found in repository; searched: repo root, %s/, skills/%s/, skills/*/%s/, skills/, skill/",
		skillName, skillName, skillName, skillName,
	)
}

// isSkillDir follows symlinks, so a skill linked into ~/.hylo/skills counts like a copied one.
func isSkillDir(p string) bool {
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(p, "SKILL.md"))
	return err == nil
}

// skillDescription is called with m.mu held.
func (m *Manager) skillDescription(dir string) string {
	p := filepath.Join(dir, "SKILL.md")
	fi, err := os.Stat(p)
	if err != nil {
		return ""
	}
	if c, ok := m.descs[p]; ok && c.mod.Equal(fi.ModTime()) {
		return c.text
	}
	src, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var text string
	fm, _ := util.ParseFrontmatter(src)
	for _, e := range fm.All {
		if e.Key == "description" {
			text = strings.Join(strings.Fields(e.Value), " ")
			break
		}
	}
	if m.descs == nil {
		m.descs = make(map[string]cachedDesc)
	}
	m.descs[p] = cachedDesc{mod: fi.ModTime(), text: text}
	return text
}

// copyDir recursively copies src directory to dst, skipping .git directories.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// SyncBuiltins writes each built-in skill into ~/.hylo/skills when what is there
// differs from the binary's copy, replacing it whole, and clears out retired ones.
// A built-in linked in by hand (a development checkout) is left alone.
// Called once at server startup, before LinkEnabled.
func (m *Manager) SyncBuiltins(logger *slog.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.sourceDir, 0755); err != nil {
		logger.Warn("skills: create source dir", "path", m.sourceDir, "err", err)
		return
	}
	for _, name := range DefaultSkills {
		if err := m.syncBuiltin(name); err != nil {
			logger.Warn("skills: sync built-in", "name", name, "err", err)
		}
	}
	for old, repl := range Retired {
		if err := os.RemoveAll(filepath.Join(m.sourceDir, old)); err != nil {
			logger.Warn("skills: remove retired", "name", old, "err", err)
		}
		m.unlink(old, "/")
		dirs, ok := m.st.Enabled[old]
		if !ok {
			continue
		}
		for _, dir := range dirs {
			m.unlink(old, dir)
			if repl != "" {
				m.st.Enabled = addDir(m.st.Enabled, repl, dir)
			}
		}
		delete(m.st.Enabled, old)
		if err := m.saveState(); err != nil {
			logger.Warn("skills: save state", "err", err)
		}
	}
}

func (m *Manager) syncBuiltin(name string) error {
	dest := filepath.Join(m.sourceDir, name)
	if fi, err := os.Lstat(dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	want, err := treeHash(builtin.FS, name)
	if err != nil {
		return err
	}
	if got, err := treeHash(os.DirFS(m.sourceDir), name); err == nil && got == want {
		return nil
	}
	sub, err := fs.Sub(builtin.FS, name)
	if err != nil {
		return err
	}
	tmp := filepath.Join(m.sourceDir, "."+name+".new")
	_ = os.RemoveAll(tmp)
	if err := os.CopyFS(tmp, sub); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// treeHash fingerprints every file under root, paths included, so an edited,
// added or deleted file all change it.
func treeHash(fsys fs.FS, root string) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(b))
		h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

// LinkEnabled recreates the links of every skill in every vault directory it is on in.
// Called once at server startup.
func (m *Manager) LinkEnabled(logger *slog.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.sourceDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("skills: read source dir", "path", m.sourceDir, "err", err)
		}
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		src := filepath.Join(m.sourceDir, name)
		if !ValidName(name) || !isSkillDir(src) {
			continue
		}
		for _, dir := range m.dirsOf(name) {
			for _, t := range m.linkDirs(dir) {
				if err := os.MkdirAll(t, 0755); err != nil {
					logger.Warn("skills: create target dir", "path", t, "err", err)
					continue
				}
				dst := filepath.Join(t, name)
				_ = os.Remove(dst)
				if err := os.Symlink(src, dst); err != nil {
					logger.Warn("skills: symlink failed", "src", src, "dst", dst, "err", err)
				}
			}
		}
	}
	logger.Info("skills linked", "src", m.sourceDir)
}
