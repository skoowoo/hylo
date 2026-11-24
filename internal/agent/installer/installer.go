package installer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// InstallSource identifies how an already-installed CLI was put on disk.
// Only "native" installations are updated by Hylo; external installations
// (Homebrew, npm, Nix, custom) are left untouched with an update hint.
type InstallSource string

const (
	SourceNone     InstallSource = ""
	SourceNative   InstallSource = "native"
	SourceHomebrew InstallSource = "homebrew"
	SourceNPM      InstallSource = "npm"
	SourceNix      InstallSource = "nix"
	SourceCustom   InstallSource = "custom"
)

// Installer encapsulates install, update, and version-check logic for one CLI.
type Installer interface {
	AgentID() string
	Supported() bool
	BinDirs(home string) []string
	InstallCmd() string
	Install(ctx context.Context, env []string, out io.Writer) error
	Update(ctx context.Context, binPath string, env []string, out io.Writer) error
	NormalizeVersion(raw string) string
	LatestVersion(ctx context.Context, client *http.Client) (string, error)
	ExternalUpdateHint(source InstallSource, binPath string) string
}

var registry = []Installer{
	claudeInstaller{},
	codexInstaller{},
	openCodeInstaller{},
	cursorInstaller{},
	copilotInstaller{},
	piInstaller{},
	agyInstaller{},
}

func Get(agentID string) Installer {
	for _, inst := range registry {
		if inst.AgentID() == agentID {
			return inst
		}
	}
	return nil
}

func All() []Installer { return registry }

// AllBinDirs returns the deduplicated union of every installer's bin directories.
func AllBinDirs(home string) []string {
	if home == "" {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, inst := range registry {
		for _, d := range inst.BinDirs(home) {
			d = filepath.Clean(d)
			if d == "" || d == "." {
				continue
			}
			if _, ok := seen[d]; ok {
				continue
			}
			seen[d] = struct{}{}
			out = append(out, d)
		}
	}
	return out
}

func isMacOS() bool { return runtime.GOOS == "darwin" }

// DetectInstallSource classifies where binPath came from without modifying anything.
func DetectInstallSource(inst Installer, binPath, home string) InstallSource {
	if binPath == "" {
		return SourceNone
	}
	clean := filepath.Clean(binPath)
	resolved := clean
	if target, err := filepath.EvalSymlinks(clean); err == nil && target != "" {
		resolved = filepath.Clean(target)
	}
	for _, p := range []string{resolved, clean} {
		if s := matchExternalSource(p); s != SourceNone {
			return s
		}
	}
	if inst != nil && home != "" {
		for _, dir := range inst.BinDirs(home) {
			if isWithinDir(clean, dir) || isWithinDir(resolved, dir) {
				return SourceNative
			}
		}
	}
	return SourceCustom
}

func matchExternalSource(p string) InstallSource {
	slashed := filepath.ToSlash(p)
	if strings.HasPrefix(slashed, "/opt/homebrew/") ||
		strings.Contains(slashed, "/Cellar/") ||
		strings.Contains(slashed, "/Caskroom/") {
		return SourceHomebrew
	}
	if strings.HasPrefix(slashed, "/nix/store/") {
		return SourceNix
	}
	for _, marker := range []string{
		"/node_modules/", "/.nvm/", "/.fnm/", "/.volta/",
		"/.asdf/", "/.mise/", "/.pnpm", "/.npm", "/.bun/",
	} {
		if strings.Contains(slashed, marker) {
			return SourceNPM
		}
	}
	return SourceNone
}

func isWithinDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func defaultExternalHint(source InstallSource, binPath, brewCmd, npmCmd, nixCmd string) string {
	switch source {
	case SourceHomebrew:
		if brewCmd != "" {
			return "Managed by Homebrew — run: " + brewCmd
		}
		return "Managed by Homebrew — update with brew upgrade"
	case SourceNPM:
		if npmCmd != "" {
			return "Managed by npm — run: " + npmCmd
		}
		return "Managed by npm — update with npm"
	case SourceNix:
		if nixCmd != "" {
			return "Managed by Nix — run: " + nixCmd
		}
		return "Managed by Nix — update with nix"
	case SourceCustom:
		if binPath != "" {
			return "Installed externally at " + binPath + " — update using your own install method"
		}
		return "Installed externally — update using your own install method"
	default:
		return ""
	}
}

var ErrBusy = errors.New("operation already in progress for this agent")

const maxOutputBytes = 16 * 1024

type activeOp struct {
	action string
	cancel context.CancelFunc
}

// Manager guards per-agent install/update operations against concurrent runs
// and allows canceling an in-flight operation.
type Manager struct {
	mu     sync.Mutex
	active map[string]*activeOp
}

func NewManager() *Manager {
	return &Manager{active: map[string]*activeOp{}}
}

// Run executes an install or update synchronously, returning combined output and error.
func (m *Manager) Run(parent context.Context, inst Installer, action, binPath string, env []string) (string, error) {
	id := inst.AgentID()
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)

	m.mu.Lock()
	if _, busy := m.active[id]; busy {
		m.mu.Unlock()
		cancel()
		return "", ErrBusy
	}
	op := &activeOp{action: action, cancel: cancel}
	m.active[id] = op
	m.mu.Unlock()

	defer func() {
		cancel()
		m.mu.Lock()
		if m.active[id] == op {
			delete(m.active, id)
		}
		m.mu.Unlock()
	}()

	var buf bytes.Buffer
	w := &tailWriter{buf: &buf, max: maxOutputBytes}
	var err error
	if action == "update" {
		err = inst.Update(ctx, binPath, env, w)
	} else {
		err = inst.Install(ctx, env, w)
	}
	out := strings.TrimSpace(buf.String())
	if errors.Is(ctx.Err(), context.Canceled) {
		return out, context.Canceled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return out, context.DeadlineExceeded
	}
	return out, err
}

func (m *Manager) Cancel(agentID string) bool {
	m.mu.Lock()
	op := m.active[agentID]
	m.mu.Unlock()
	if op == nil {
		return false
	}
	op.cancel()
	return true
}

// Snapshot returns currently running operations as agentID -> action.
func (m *Manager) Snapshot() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(m.active))
	for id, op := range m.active {
		out[id] = op.action
	}
	return out
}

// tailWriter keeps at most max trailing bytes so verbose installer output stays bounded.
type tailWriter struct {
	buf *bytes.Buffer
	max int
}

func (t *tailWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n >= t.max {
		t.buf.Reset()
		t.buf.Write(p[n-t.max:])
		return n, nil
	}
	if excess := t.buf.Len() + n - t.max; excess > 0 {
		t.buf.Next(excess)
	}
	t.buf.Write(p)
	return n, nil
}
