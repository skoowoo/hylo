package skills

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gogitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

//go:embed default_catalog.txt
var defaultCatalog string

// proxyOnce ensures the go-git HTTP transport is configured with proxy settings
// exactly once per process, using the first Install call's shell environment.
var proxyOnce sync.Once

// DefaultSkills are always enabled and cannot be disabled by the user.
var DefaultSkills = []string{
	"hylo-compile-note",
	"hylo-index-knowledge",
	"hylo-memory",
	"hylo-notes",
}

// state persists only the non-default skills the user has explicitly enabled.
type state struct {
	EnabledExtra []string `json:"enabled_extra"`
}

// SkillInfo describes one skill, installed or available in the catalog.
type SkillInfo struct {
	Name      string `json:"name"`
	Default   bool   `json:"default"`
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	RepoURL   string `json:"repoUrl,omitempty"`
	SubPath   string `json:"subPath,omitempty"`
}

// catalogEntry holds the metadata parsed from a catalog line.
type catalogEntry struct {
	RepoURL string
	SubPath string
}

// Manager tracks which skills are enabled and manages their vault symlinks.
type Manager struct {
	mu         sync.Mutex
	sourceDir  string
	targets    []string
	stateFile  string
	st         state
	shellEnvFn func() []string // returns login-shell env for git operations
}

// Open creates a Manager rooted at vaultRoot. shellEnvFn is called when
// installing skills to obtain the user's login-shell environment (proxy vars,
// etc.); pass nil to fall back to os.Environ.
// State is loaded from disk; errors are silently ignored.
func Open(vaultRoot string, shellEnvFn func() []string) *Manager {
	home, _ := os.UserHomeDir()
	if shellEnvFn == nil {
		shellEnvFn = os.Environ
	}
	m := &Manager{
		sourceDir: filepath.Join(home, ".hylo", "skills"),
		targets: []string{
			filepath.Join(vaultRoot, ".agents", "skills"),
			filepath.Join(vaultRoot, ".claude", "skills"),
		},
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
	return json.Unmarshal(data, &m.st)
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

// isExtraEnabled reads m.st without locking; callers must hold m.mu.
func (m *Manager) isExtraEnabled(name string) bool {
	for _, n := range m.st.EnabledExtra {
		if n == name {
			return true
		}
	}
	return false
}

func (m *Manager) isEnabled(name string) bool {
	if isDefault(name) {
		return true
	}
	return m.isExtraEnabled(name)
}

// parseCatalog parses an external-skills.txt formatted string and returns a
// map of skill-dir-name → catalogEntry (RepoURL + SubPath).
func parseCatalog(content string) map[string]catalogEntry {
	result := make(map[string]catalogEntry)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		githubPath := strings.TrimSpace(parts[0])
		subPath := strings.TrimSpace(parts[1])
		skillName := strings.TrimSpace(parts[2])
		if githubPath == "" || skillName == "" {
			continue
		}
		if subPath == "." {
			subPath = ""
		}
		result[skillName] = catalogEntry{
			RepoURL: "https://github.com/" + githubPath,
			SubPath: subPath,
		}
	}
	return result
}

// List returns all skills: installed ones and uninstalled catalog entries.
// Installed skills appear first (default, then external), followed by
// uninstalled catalog entries that can be installed via the UI.
func (m *Manager) List() ([]SkillInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	catalog := parseCatalog(defaultCatalog)

	entries, err := os.ReadDir(m.sourceDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	installed := make(map[string]bool)
	out := make([]SkillInfo, 0)

	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			installed[name] = true
			entry := catalog[name]
			out = append(out, SkillInfo{
				Name:      name,
				Default:   isDefault(name),
				Installed: true,
				Enabled:   m.isEnabled(name),
				RepoURL:   entry.RepoURL,
				SubPath:   entry.SubPath,
			})
		}
	}

	// Append uninstalled catalog entries so the UI can offer an Install button.
	for name, entry := range catalog {
		if installed[name] {
			continue
		}
		out = append(out, SkillInfo{
			Name:      name,
			Default:   false,
			Installed: false,
			Enabled:   false,
			RepoURL:   entry.RepoURL,
			SubPath:   entry.SubPath,
		})
	}

	return out, nil
}

// Enable enables a skill by creating symlinks and persisting state.
// Default skills are always enabled; this is a no-op for them.
func (m *Manager) Enable(name string) error {
	if isDefault(name) {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	src := filepath.Join(m.sourceDir, name)
	if _, err := os.Stat(src); err != nil {
		return err
	}

	for _, t := range m.targets {
		if err := os.MkdirAll(t, 0755); err != nil {
			return err
		}
		dst := filepath.Join(t, name)
		_ = os.Remove(dst)
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}

	if !m.isExtraEnabled(name) {
		m.st.EnabledExtra = append(m.st.EnabledExtra, name)
	}
	return m.saveState()
}

// Disable disables a skill by removing symlinks and persisting state.
// Returns an error for default skills.
func (m *Manager) Disable(name string) error {
	if isDefault(name) {
		return fmt.Errorf("skill %q is a default skill and cannot be disabled", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.targets {
		_ = os.Remove(filepath.Join(t, name))
	}

	var extra []string
	for _, n := range m.st.EnabledExtra {
		if n != name {
			extra = append(extra, n)
		}
	}
	m.st.EnabledExtra = extra
	return m.saveState()
}

// Remove disables the skill (removes all symlinks) and deletes its source
// directory from ~/.hylo/skills/. Returns an error for built-in skills.
func (m *Manager) Remove(name string) error {
	if isDefault(name) {
		return fmt.Errorf("skill %q is a built-in skill and cannot be removed", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove symlinks from every target directory.
	for _, t := range m.targets {
		dst := filepath.Join(t, name)
		// Resolve and remove symlink target dir if it is a real symlink.
		if fi, err := os.Lstat(dst); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(dst)
		}
	}

	// Drop from persisted enabled state.
	var extra []string
	for _, n := range m.st.EnabledExtra {
		if n != name {
			extra = append(extra, n)
		}
	}
	m.st.EnabledExtra = extra
	if err := m.saveState(); err != nil {
		return err
	}

	// Delete the source directory.
	return os.RemoveAll(filepath.Join(m.sourceDir, name))
}

// Install clones a GitHub repository and copies the named skill directory into
// the skills source directory. repoURL accepts the forms:
//
//	https://github.com/owner/repo
//	owner/repo
//
// subPath is the path inside the repository where the skill lives (e.g. "skills").
// When non-empty it is tried first; if it does not contain a SKILL.md the
// function falls back to the usual heuristic search. Pass an empty string to
// always use the heuristic search.
func (m *Manager) Install(repoURL, subPath, skillName string) error {
	repoURL = normalizeRepoURL(repoURL)

	// Register proxy transport once, using the captured shell environment.
	proxyOnce.Do(func() { setupProxyTransport(m.shellEnvFn()) })

	tmp, err := os.MkdirTemp("", "hylo-skill-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	_, err = git.PlainClone(tmp, false, &git.CloneOptions{
		URL:           repoURL,
		Depth:         1,
		ReferenceName: plumbing.HEAD,
		SingleBranch:  true,
	})
	if err != nil {
		return fmt.Errorf("clone %s: %w", repoURL, err)
	}

	var src string
	if subPath != "" {
		candidate := filepath.Join(tmp, filepath.FromSlash(subPath))
		if isSkillDir(candidate) {
			src = candidate
		}
	}
	if src == "" {
		src, err = findSkillDir(tmp, skillName)
		if err != nil {
			return err
		}
	}

	dest := filepath.Join(m.sourceDir, skillName)
	if err := os.MkdirAll(m.sourceDir, 0755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("remove existing skill: %w", err)
	}
	if err := copyDir(src, dest); err != nil {
		return fmt.Errorf("copy skill: %w", err)
	}
	return m.Enable(skillName)
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

func isSkillDir(p string) bool {
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(p, "SKILL.md"))
	return err == nil
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

// LinkEnabled creates symlinks for all currently-enabled skills.
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

	for _, t := range m.targets {
		if err := os.MkdirAll(t, 0755); err != nil {
			logger.Warn("skills: create target dir", "path", t, "err", err)
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !m.isEnabled(name) {
			continue
		}
		src := filepath.Join(m.sourceDir, name)
		for _, t := range m.targets {
			dst := filepath.Join(t, name)
			_ = os.Remove(dst)
			if err := os.Symlink(src, dst); err != nil {
				logger.Warn("skills: symlink failed", "src", src, "dst", dst, "err", err)
			}
		}
	}
	logger.Info("skills linked", "src", m.sourceDir)
}
