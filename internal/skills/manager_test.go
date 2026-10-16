package skills

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	home, vault := t.TempDir(), t.TempDir()
	m := &Manager{
		sourceDir: filepath.Join(home, "skills"),
		vaultRoot: vault,
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
		out[s.Name] = s
	}
	return out
}

// A skill copied or linked into the source dir by hand shows up added but off,
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
	if len(got) != 2 || onAtRoot(got["copied"]) || onAtRoot(got["linked"]) {
		t.Fatalf("listed = %+v", got)
	}

	for _, name := range []string{"copied", "linked"} {
		if err := m.Enable(name, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(vault, ".claude", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("%s not linked into the vault: %v", name, err)
		}
	}
	if got := listed(t, m); !onAtRoot(got["copied"]) || !onAtRoot(got["linked"]) {
		t.Fatalf("after enable = %+v", got)
	}

	if err := m.Disable("copied", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(vault, ".claude", "skills", "copied")); !os.IsNotExist(err) {
		t.Errorf("copied still linked: %v", err)
	}
}

func TestEditorSkillIsDefault(t *testing.T) {
	m, vault := newTestManager(t)
	writeSkill(t, filepath.Join(m.sourceDir, "hylo-editor"))
	if s := listed(t, m)["hylo-editor"]; !s.Default || !onAtRoot(s) {
		t.Fatalf("hylo-editor = %+v", s)
	}
	m.LinkEnabled(discardLogger())
	if _, err := os.Stat(filepath.Join(vault, ".claude", "skills", "hylo-editor", "SKILL.md")); err != nil {
		t.Fatalf("hylo-editor not linked: %v", err)
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestCatalogMarksAddedSkills(t *testing.T) {
	m, _ := newTestManager(t)
	find := func() CatalogEntry {
		for _, e := range m.Catalog() {
			if e.Name == "hackernews" {
				return e
			}
		}
		t.Fatal("hackernews not in the catalog")
		return CatalogEntry{}
	}
	if e := find(); e.Repo == "" || e.Category != "news" || e.Description == "" || e.Added {
		t.Fatalf("hackernews = %+v", e)
	}
	writeSkill(t, filepath.Join(m.sourceDir, "hackernews"))
	if !find().Added {
		t.Fatal("added skill not marked")
	}
}

// Adding and turning on are separate steps, so nothing turns on or is replaced by surprise.
func TestAddAndEnableAreSeparate(t *testing.T) {
	m, _ := newTestManager(t)
	if err := m.Enable("missing", ""); err == nil {
		t.Error("enabled a skill that was never added")
	}
	if err := m.Remove("missing"); err == nil {
		t.Error("removed a skill that was never added")
	}
	writeSkill(t, filepath.Join(m.sourceDir, "mine"))
	if _, err := m.Add("owner/mine", "", false); err == nil {
		t.Error("replaced an added skill without force")
	}
	if _, err := m.Add("owner/repo", "hylo-notes", true); err == nil {
		t.Error("replaced a built-in skill")
	}
	if _, err := m.Add("owner/repo", "../escape", true); err == nil {
		t.Error("accepted a name outside the skills dir")
	}
}

func TestManagerReportsSkillChanges(t *testing.T) {
	m, _ := newTestManager(t)
	writeSkill(t, filepath.Join(m.sourceDir, "mine"))
	n := 0
	m.SetOnChange(func() { n++ })
	if err := m.Enable("mine", ""); err != nil {
		t.Fatal(err)
	}
	if err := m.Disable("mine", ""); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove("mine"); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("%d changes reported, want 3", n)
	}
}

// A skill can be on in a vault folder, for agents started there, apart from the root.
func TestEnableInVaultDir(t *testing.T) {
	m, vault := newTestManager(t)
	if err := m.Disable("never-on", "/_news"); err != nil {
		t.Fatalf("disable with nothing on: %v", err)
	}
	writeSkill(t, filepath.Join(m.sourceDir, "news"))
	writeSkill(t, filepath.Join(m.sourceDir, "hylo-notes"))
	for _, name := range []string{"news", "hylo-notes"} {
		if err := m.Enable(name, "/_news/"); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"_news/.claude/skills/news", "_news/.agents/skills/news", "_news/.claude/skills/hylo-notes"} {
		if _, err := os.Stat(filepath.Join(vault, p, "SKILL.md")); err != nil {
			t.Errorf("%s not linked: %v", p, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(vault, ".claude", "skills", "news")); !os.IsNotExist(err) {
		t.Error("enabling in /_news also linked it at the root")
	}
	got := listed(t, m)
	if n := got["news"]; onAtRoot(n) || len(n.Dirs) != 1 || n.Dirs[0] != "/_news" {
		t.Fatalf("news = %+v", n)
	}
	if h := got["hylo-notes"]; !onAtRoot(h) || len(h.Dirs) != 2 {
		t.Fatalf("hylo-notes = %+v", h)
	}

	// Startup relinks every directory from the saved state.
	_ = os.RemoveAll(filepath.Join(vault, "_news", ".claude"))
	reopened := &Manager{sourceDir: m.sourceDir, vaultRoot: vault, stateFile: m.stateFile}
	if err := reopened.loadState(); err != nil {
		t.Fatal(err)
	}
	reopened.LinkEnabled(discardLogger())
	if _, err := os.Stat(filepath.Join(vault, "_news", ".claude", "skills", "news", "SKILL.md")); err != nil {
		t.Fatalf("not relinked: %v", err)
	}

	if err := m.Disable("news", "/_news"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(vault, "_news", ".agents", "skills", "news")); !os.IsNotExist(err) {
		t.Error("still linked after disable")
	}
	if err := m.Disable("hylo-notes", "/"); err == nil {
		t.Error("turned a built-in off at the root")
	}
	if err := m.DisableDir("/_news"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(vault, "_news", ".claude", "skills", "hylo-notes")); !os.IsNotExist(err) {
		t.Error("a deleted bot's folder kept its skills")
	}
	if h := listed(t, m)["hylo-notes"]; len(h.Dirs) != 1 || h.Dirs[0] != "/" {
		t.Fatalf("hylo-notes after DisableDir = %+v", h)
	}
	if err := m.Remove("news"); err != nil || len(listed(t, m)["news"].Dirs) != 0 {
		t.Fatalf("remove: %v", err)
	}
}

func TestOldStateMigratesToRoot(t *testing.T) {
	m, _ := newTestManager(t)
	if err := os.MkdirAll(filepath.Dir(m.stateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.stateFile, []byte(`{"enabled_extra":["old"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.loadState(); err != nil {
		t.Fatal(err)
	}
	if d := m.st.Enabled["old"]; len(d) != 1 || d[0] != "/" || m.st.EnabledExtra != nil {
		t.Fatalf("state = %+v", m.st)
	}
}

func onAtRoot(s SkillInfo) bool { return slices.Contains(s.Dirs, "/") }
