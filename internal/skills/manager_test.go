package skills

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	home, vault := t.TempDir(), t.TempDir()
	m := &Manager{
		sourceDir: filepath.Join(home, "skills"),
		targets:   []string{filepath.Join(vault, ".claude", "skills")},
		stateFile: filepath.Join(vault, ".hylo", "skills_state.json"),
	}
	return m, vault
}

func writeSkill(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func listed(t *testing.T, m *Manager) map[string]SkillInfo {
	t.Helper()
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]SkillInfo{}
	for _, s := range list {
		if s.Installed {
			out[s.Name] = s
		}
	}
	return out
}

// A skill copied or linked into the source dir by hand shows up installed but off,
// and enabling it links it into the vault where agents look.
func TestManualSkillCanBeEnabled(t *testing.T) {
	m, vault := newTestManager(t)
	writeSkill(t, filepath.Join(m.sourceDir, "copied"))
	elsewhere := filepath.Join(t.TempDir(), "linked")
	writeSkill(t, elsewhere)
	if err := os.Symlink(elsewhere, filepath.Join(m.sourceDir, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(m.sourceDir, "not-a-skill"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := listed(t, m)
	if len(got) != 2 || got["copied"].Enabled || got["linked"].Enabled {
		t.Fatalf("listed = %+v", got)
	}

	for _, name := range []string{"copied", "linked"} {
		if err := m.Enable(name); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(vault, ".claude", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("%s not linked into the vault: %v", name, err)
		}
	}
	if got := listed(t, m); !got["copied"].Enabled || !got["linked"].Enabled {
		t.Fatalf("after enable = %+v", got)
	}

	if err := m.Disable("copied"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(vault, ".claude", "skills", "copied")); !os.IsNotExist(err) {
		t.Errorf("copied still linked: %v", err)
	}
}

func TestEditorSkillIsDefault(t *testing.T) {
	m, vault := newTestManager(t)
	writeSkill(t, filepath.Join(m.sourceDir, "hylo-editor"))
	if s := listed(t, m)["hylo-editor"]; !s.Default || !s.Enabled {
		t.Fatalf("hylo-editor = %+v", s)
	}
	m.LinkEnabled(discardLogger())
	if _, err := os.Stat(filepath.Join(vault, ".claude", "skills", "hylo-editor", "SKILL.md")); err != nil {
		t.Fatalf("hylo-editor not linked: %v", err)
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
