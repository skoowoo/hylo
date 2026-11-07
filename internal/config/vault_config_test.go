package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gogitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func globalWithVault(t *testing.T, vault, extra string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "[vault]\npath = \""+filepath.ToSlash(vault)+"\"\n"+extra)
	return path
}

func commitFile(t *testing.T, repo *git.Repository, root, name string) {
	t.Helper()
	writeFile(t, filepath.Join(root, name), name)
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(name); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "t", Email: "t@t", When: time.Now()}
	if _, err := wt.Commit(name, &git.CommitOptions{Author: sig}); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultSearchExcludeSkipsBots(t *testing.T) {
	cfg, err := defaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(cfg.Plugins.Search.ExcludePrefixes, ",")
	for _, want := range []string{"/_assets", "/_bots", "/_knowledge/_indexes"} {
		if !strings.Contains(","+got+",", ","+want+",") {
			t.Fatalf("default exclude %q missing %s", got, want)
		}
	}
}

func TestVaultConfigOverridesGlobal(t *testing.T) {
	vault := t.TempDir()
	global := globalWithVault(t, vault, `
[plugins.search]
exclude_prefixes = ["/global"]
use_jieba = false
[plugins.git_sync]
debounce = "1m"
auth_token = "tok"
[agent]
system_prompt = "custom"
`)
	writeFile(t, VaultConfigPath(vault), `
[vault]
path = "/ignored"
[plugins.search]
exclude_prefixes = ["/vault"]
[plugins.git_sync]
enabled = true
remote = "https://example.com/v.git"
debounce = "9h"
`)
	cfg, _, err := Load(global)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := VaultRoot(cfg.Vault.Path); got != vault {
		t.Fatalf("vault.path is not a vault key: %q", cfg.Vault.Path)
	}
	if strings.Join(cfg.Plugins.Search.ExcludePrefixes, ",") != "/vault" {
		t.Fatalf("exclude: %v", cfg.Plugins.Search.ExcludePrefixes)
	}
	g := cfg.Plugins.GitSync
	if !g.Enabled || g.Remote != "https://example.com/v.git" || g.Branch != "" {
		t.Fatalf("git sync from vault: %+v", g)
	}
	if g.Debounce != "9h" || g.AuthToken != "tok" {
		t.Fatalf("vault debounce or migrated token: %+v", g)
	}
	if cfg.Plugins.Search.UseJieba {
		t.Fatal("use_jieba stayed at the built-in default")
	}
	raw, _ := os.ReadFile(global)
	for _, gone := range []string{"tok", "1m", "/global", "use_jieba", "custom"} {
		if strings.Contains(string(raw), gone) {
			t.Fatalf("global file still has %q:\n%s", gone, raw)
		}
	}
}

func TestSwitchedVaultDoesNotInheritGitSync(t *testing.T) {
	vault := t.TempDir()
	global := globalWithVault(t, vault, `
[plugins.git_sync]
enabled = true
remote = "https://example.com/old.git"
auth_token = "tok"
`)
	cfg, _, err := Load(global)
	if err != nil {
		t.Fatal(err)
	}
	g := cfg.Plugins.GitSync
	if g.Enabled || g.Remote != "" || g.AuthToken != "tok" {
		t.Fatalf("git sync after migration: %+v", g)
	}
	raw, _ := os.ReadFile(global)
	if strings.Contains(string(raw), "old.git") || strings.Contains(string(raw), "tok") {
		t.Fatalf("global file after migration:\n%s", raw)
	}
	local, _ := os.ReadFile(VaultConfigPath(vault))
	if strings.Contains(string(local), "old.git") || !strings.Contains(string(local), "tok") {
		t.Fatalf("vault file:\n%s", local)
	}
}

func TestMigrationAdoptsTheRepoItSynced(t *testing.T) {
	upstream := t.TempDir()
	up, err := git.PlainInit(upstream, false)
	if err != nil {
		t.Fatal(err)
	}
	commitFile(t, up, upstream, "a.md")

	vault := filepath.Join(t.TempDir(), "vault")
	if _, err := git.PlainClone(vault, false, &git.CloneOptions{URL: upstream}); err != nil {
		t.Fatal(err)
	}
	global := globalWithVault(t, vault, "[plugins.git_sync]\nenabled = true\nremote = \""+upstream+"\"\nbranch = \"master\"\n")
	cfg, _, err := Load(global)
	if err != nil {
		t.Fatal(err)
	}
	g := cfg.Plugins.GitSync
	if !g.Enabled || g.Remote != upstream || g.Branch != "master" {
		t.Fatalf("not adopted: %+v", g)
	}
	raw, _ := os.ReadFile(global)
	if strings.Contains(string(raw), "enabled") {
		t.Fatalf("global still has git sync:\n%s", raw)
	}
	again, _, err := Load(global)
	if err != nil || !again.Plugins.GitSync.Enabled {
		t.Fatalf("adoption did not stick: %v %+v", err, again.Plugins.GitSync)
	}
}

func TestMigrationSkipsRepoWiredAfterSwitch(t *testing.T) {
	upstream := t.TempDir()
	up, err := git.PlainInit(upstream, false)
	if err != nil {
		t.Fatal(err)
	}
	commitFile(t, up, upstream, "a.md")

	vault := t.TempDir()
	repo, err := git.PlainInit(vault, false)
	if err != nil {
		t.Fatal(err)
	}
	commitFile(t, repo, vault, ".gitignore")
	if _, err := repo.CreateRemote(&gogitcfg.RemoteConfig{Name: "origin", URLs: []string{upstream}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(&git.FetchOptions{RemoteName: "origin"}); err != nil {
		t.Fatal(err)
	}
	global := globalWithVault(t, vault, "[plugins.git_sync]\nenabled = true\nremote = \""+upstream+"\"\nbranch = \"master\"\n")
	cfg, _, err := Load(global)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plugins.GitSync.Enabled {
		t.Fatalf("adopted a repo with unrelated history: %+v", cfg.Plugins.GitSync)
	}
}

func TestWriteVaultConfigKeepsOnlyVaultKeys(t *testing.T) {
	vault := t.TempDir()
	vp, gp := SplitVaultPatch(map[string]any{
		"vault": map[string]any{"path": "/x"},
		"plugins": map[string]any{
			"search":   map[string]any{"exclude_prefixes": []any{"/a"}, "use_jieba": true},
			"git_sync": map[string]any{"enabled": true, "branch": "main", "debounce": "1m"},
		},
	})
	if err := WriteVaultConfig(vault, vp); err != nil {
		t.Fatal(err)
	}
	if err := WriteVaultConfig(vault, map[string]any{"plugins": map[string]any{"git_sync": map[string]any{"remote": "r"}}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(VaultConfigPath(vault))
	out := string(raw)
	for _, want := range []string{"exclude_prefixes", "enabled = true", "remote = 'r'"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	for _, leaked := range []string{"path"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("global key %q leaked into the vault file:\n%s", leaked, out)
		}
	}
	for _, want := range []string{"use_jieba", "debounce"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	vaultPart, _ := gp["vault"].(map[string]any)
	if _, plugins := gp["plugins"]; plugins || vaultPart["path"] != "/x" || len(gp) != 1 || len(vaultPart) != 1 {
		t.Fatalf("global part: %v", gp)
	}
}

func TestLegacyGlobalWithoutBranchStillMigrates(t *testing.T) {
	vault := t.TempDir()
	global := globalWithVault(t, vault, "[plugins.git_sync]\nenabled = true\nremote = \"https://example.com/old.git\"\n")
	if _, _, err := Load(global); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(global)
	if strings.Contains(string(raw), "old.git") {
		t.Fatalf("legacy keys not stripped:\n%s", raw)
	}
	if _, err := MergedFromOptionalFile(global); err != nil {
		t.Fatal(err)
	}
}

func TestEnablingProactiveKeepsDefaultHours(t *testing.T) {
	vault := t.TempDir()
	if err := WriteVaultConfig(vault, map[string]any{
		"proactive": map[string]any{"enabled": true},
	}); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(globalWithVault(t, vault, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Proactive.Enabled || cfg.Proactive.ActiveHours != "08:00-24:00" || cfg.Proactive.MinGap != "20m" {
		t.Fatalf("effective config: %+v", cfg.Proactive)
	}
	raw, _ := os.ReadFile(VaultConfigPath(vault))
	if strings.Contains(string(raw), "active_hours") {
		t.Fatalf("default hours were stored in the vault file:\n%s", raw)
	}

	err = WriteVaultConfig(vault, map[string]any{
		"proactive": map[string]any{"active_hours": ""},
	})
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(err.Error(), `got ""`) {
		t.Fatalf("explicit empty hours: %v", err)
	}
}

func TestEnabledVaultNeedsBranch(t *testing.T) {
	vault := t.TempDir()
	err := WriteVaultConfig(vault, map[string]any{"plugins": map[string]any{"git_sync": map[string]any{"enabled": true}}})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("enabled without branch: %v", err)
	}
	if err := WriteVaultConfig(vault, map[string]any{"plugins": map[string]any{"git_sync": map[string]any{"enabled": true, "branch": "bad..name"}}}); !errors.As(err, &ve) {
		t.Fatalf("invalid branch accepted: %v", err)
	}
	if err := WriteVaultConfig(vault, map[string]any{"plugins": map[string]any{"git_sync": map[string]any{"enabled": true, "branch": "notes"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadForAnotherVaultUsesItsConfig(t *testing.T) {
	configured, other := t.TempDir(), t.TempDir()
	global := globalWithVault(t, configured, "")
	writeFile(t, VaultConfigPath(configured), "[plugins.search]\nexclude_prefixes = [\"/configured\"]\n")
	writeFile(t, VaultConfigPath(other), "[plugins.search]\nexclude_prefixes = [\"/other\"]\n")
	cfg, _, err := load(global, other)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Vault.Path != other || strings.Join(cfg.Plugins.Search.ExcludePrefixes, ",") != "/other" {
		t.Fatalf("got %+v %v", cfg.Vault, cfg.Plugins.Search.ExcludePrefixes)
	}
}

func TestAnotherVaultKeepsBuiltInPluginDefaults(t *testing.T) {
	vault, other := t.TempDir(), t.TempDir()
	global := globalWithVault(t, vault, `
[plugins.search]
use_jieba = false
exclude_prefixes = ["/from-global"]
[plugins.git_sync]
auth_token = "tok"
debounce = "1m"
`)
	cfg, _, err := load(global, other)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Plugins.Search.UseJieba || cfg.Plugins.GitSync.AuthToken != "" || cfg.Plugins.GitSync.Debounce != "5m" {
		t.Fatalf("other vault inherited plugin settings: %+v %+v", cfg.Plugins.Search, cfg.Plugins.GitSync)
	}
	if strings.Join(cfg.Plugins.Search.ExcludePrefixes, ",") == "/from-global" {
		t.Fatal("other vault took the global exclude list")
	}
	raw, _ := os.ReadFile(global)
	if !strings.Contains(string(raw), "tok") || !strings.Contains(string(raw), "use_jieba") {
		t.Fatalf("loading another vault stripped the global file:\n%s", raw)
	}
}

func TestLoadForAnotherVaultLeavesLegacySyncAlone(t *testing.T) {
	vault, other := t.TempDir(), t.TempDir()
	global := globalWithVault(t, vault, "[plugins.git_sync]\nenabled = true\nremote = \"https://example.com/v.git\"\nbranch = \"main\"\n")
	cfg, _, err := load(global, other)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plugins.GitSync.Enabled {
		t.Fatalf("another vault picked up legacy sync: %+v", cfg.Plugins.GitSync)
	}
	if raw, _ := os.ReadFile(global); !strings.Contains(string(raw), "example.com") {
		t.Fatalf("legacy sync stripped for a vault it was not migrated to:\n%s", raw)
	}
}

func TestLocalOnlyLegacySyncIsNotAdopted(t *testing.T) {
	vault := t.TempDir()
	repo, err := git.PlainInit(vault, false)
	if err != nil {
		t.Fatal(err)
	}
	commitFile(t, repo, vault, "mine.txt")
	global := globalWithVault(t, vault, "[plugins.git_sync]\nenabled = true\n")
	cfg, _, err := Load(global)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plugins.GitSync.Enabled {
		t.Fatalf("unrelated repo adopted local-only sync: %+v", cfg.Plugins.GitSync)
	}
}
