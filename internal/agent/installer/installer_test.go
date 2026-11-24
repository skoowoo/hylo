package installer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractSemverAndIsNewer(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"opencode v2.3.2", "2.3.2"},
		{"2.1.12 (Claude Code)", "2.1.12"},
		{"codex-cli 0.104.0", "0.104.0"},
		{"rust-v0.105.1", "0.105.1"},
		{"v1.2.3-beta.1", "1.2.3-beta.1"},
	}
	for _, c := range cases {
		if got := ExtractSemver(c.raw); got != c.want {
			t.Errorf("ExtractSemver(%q) = %q, want %q", c.raw, got, c.want)
		}
	}

	cmp := []struct {
		a, b string
		want bool
	}{
		{"2.3.1", "2.3.2", true},
		{"2.3.2", "2.3.2", false},
		{"2.4.0", "2.3.9", false},
		{"1.0.0-rc.1", "1.0.0", true},
		{"1.0.0", "1.0.0-rc.1", false},
		{"bad", "1.0.0", false},
	}
	for _, c := range cmp {
		if got := IsNewer(c.a, c.b); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestDetectInstallSource(t *testing.T) {
	home := "/Users/alice"
	cases := []struct {
		agentID string
		binPath string
		want    InstallSource
	}{
		{"opencode", "/Users/alice/.opencode/bin/opencode", SourceNative},
		{"claude", "/Users/alice/.local/bin/claude", SourceNative},
		{"agy", "/Users/alice/.local/bin/agy", SourceNative},
		{"codex", "/opt/homebrew/bin/codex", SourceHomebrew},
		{"claude", "/usr/local/Cellar/claude/1.0/bin/claude", SourceHomebrew},
		{"pi", "/Users/alice/.nvm/versions/node/v22.0.0/bin/pi", SourceNPM},
		{"codex", "/Users/alice/.volta/bin/codex", SourceNPM},
		{"pi", "/nix/store/abc-pi/bin/pi", SourceNix},
		{"codex", "/usr/local/bin/codex", SourceCustom},
		{"codex", "", SourceNone},
	}
	for _, c := range cases {
		inst := Get(c.agentID)
		if inst == nil {
			t.Fatalf("missing installer for %s", c.agentID)
		}
		if got := DetectInstallSource(inst, c.binPath, home); got != c.want {
			t.Errorf("DetectInstallSource(%s, %q) = %q, want %q", c.agentID, c.binPath, got, c.want)
		}
	}
}

func TestAllBinDirsIncludesOpenCodeAndLocal(t *testing.T) {
	home := "/Users/alice"
	dirs := AllBinDirs(home)
	want := map[string]bool{
		filepath.Join(home, ".local", "bin"):    false,
		filepath.Join(home, ".opencode", "bin"): false,
	}
	for _, d := range dirs {
		if _, ok := want[d]; ok {
			want[d] = true
		}
	}
	for d, found := range want {
		if !found {
			t.Errorf("AllBinDirs missing %q in %v", d, dirs)
		}
	}
}

type stubInstaller struct {
	id      string
	block   chan struct{}
	started chan struct{}
}

func (s stubInstaller) AgentID() string                                        { return s.id }
func (s stubInstaller) Supported() bool                                        { return true }
func (s stubInstaller) BinDirs(string) []string                                { return nil }
func (s stubInstaller) InstallCmd() string                                     { return "stub" }
func (s stubInstaller) NormalizeVersion(r string) string                       { return r }
func (s stubInstaller) LatestVersion(context.Context, *http.Client) (string, error) { return "", nil }
func (s stubInstaller) ExternalUpdateHint(InstallSource, string) string        { return "" }
func (s stubInstaller) Update(ctx context.Context, _ string, env []string, out io.Writer) error {
	return s.Install(ctx, env, out)
}
func (s stubInstaller) Install(ctx context.Context, _ []string, out io.Writer) error {
	_, _ = io.WriteString(out, "step 1\n")
	if s.started != nil {
		close(s.started)
	}
	select {
	case <-s.block:
		_, _ = io.WriteString(out, "done\n")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestManagerConcurrencyAndCancel(t *testing.T) {
	mgr := NewManager()
	started := make(chan struct{})
	inst := stubInstaller{id: "test-cli", block: make(chan struct{}), started: started}

	done := make(chan error, 1)
	go func() {
		_, err := mgr.Run(context.Background(), inst, "install", "", nil)
		done <- err
	}()
	<-started

	if _, err := mgr.Run(context.Background(), inst, "install", "", nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Run = %v, want ErrBusy", err)
	}
	if snap := mgr.Snapshot(); snap["test-cli"] != "install" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if !mgr.Cancel("test-cli") {
		t.Fatal("Cancel returned false")
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to cancel")
	}
}
