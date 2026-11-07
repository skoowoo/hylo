package config

import (
	"errors"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// migrateVaultScoped moves plugin and proactive settings off the global file,
// where older installs kept them, onto this vault. Git sync's repository keys
// move only when this vault's repository is the one they were syncing; every
// other explicit key moves as-is. Either way the sections leave the global
// file. Returns the vault config's keys when it wrote some.
func migrateVaultScoped(root, globalPath string, legacy GitSyncConfig) map[string]any {
	global, err := ReadTomlMap(globalPath)
	if err != nil || !hasInstallScoped(global) {
		return nil
	}
	vaultRaw, err := ReadTomlMap(VaultConfigPath(root))
	if err != nil {
		return nil
	}

	patch := map[string]any{}
	if plugins, ok := cloneSection(global["plugins"]); ok {
		dropGitIdentity(plugins)
		if left := withoutExisting(plugins, sectionMap(vaultRaw, "plugins")); len(left) > 0 {
			patch["plugins"] = left
		}
	}
	if proactive, ok := cloneSection(global["proactive"]); ok {
		if left := withoutExisting(proactive, sectionMap(vaultRaw, "proactive")); len(left) > 0 {
			patch["proactive"] = left
		}
	}
	if legacy.Branch == "" {
		legacy.Branch = "main" // what older installs committed to by default
	}
	if legacy.Enabled && !hasGitSync(vaultRaw) && syncedWith(root, legacy.Remote, legacy.Branch) {
		addGitIdentity(patch, legacy)
	}

	var adopted map[string]any
	if len(patch) > 0 {
		if writeVaultMerge(root, patch) != nil {
			return nil
		}
		raw, err := ReadTomlMap(VaultConfigPath(root))
		if err != nil {
			return nil
		}
		adopted = pickKeys(raw, true)
	}
	stripGlobal(globalPath)
	return adopted
}

// hasInstallScoped reports whether the global file still carries sections that
// now belong to a vault, or the removed agent prompt.
func hasInstallScoped(m map[string]any) bool {
	for _, k := range []string{"agent", "plugins", "proactive"} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func cloneSection(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil, false
	}
	copied, _ := mergeCopyValue(m).(map[string]any)
	return copied, true
}

func sectionMap(m map[string]any, key string) map[string]any {
	s, _ := m[key].(map[string]any)
	if s == nil {
		return map[string]any{}
	}
	return s
}

// dropGitIdentity removes the repository keys. They are adopted separately,
// and only for the repository they already sync.
func dropGitIdentity(plugins map[string]any) {
	gs, ok := plugins["git_sync"].(map[string]any)
	if !ok {
		return
	}
	delete(gs, "enabled")
	delete(gs, "remote")
	delete(gs, "branch")
	if len(gs) == 0 {
		delete(plugins, "git_sync")
	}
}

// withoutExisting returns the leaves of src whose key the vault file does not
// already set. A vault value wins over the one being migrated.
func withoutExisting(src, existing map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range src {
		ev, ok := existing[k]
		if !ok {
			out[k] = v
			continue
		}
		sm, srcIsMap := v.(map[string]any)
		em, dstIsMap := ev.(map[string]any)
		if srcIsMap && dstIsMap {
			if sub := withoutExisting(sm, em); len(sub) > 0 {
				out[k] = sub
			}
		}
	}
	return out
}

func addGitIdentity(patch map[string]any, legacy GitSyncConfig) {
	plugins, _ := patch["plugins"].(map[string]any)
	if plugins == nil {
		plugins = map[string]any{}
		patch["plugins"] = plugins
	}
	gs, _ := plugins["git_sync"].(map[string]any)
	if gs == nil {
		gs = map[string]any{}
		plugins["git_sync"] = gs
	}
	gs["enabled"] = true
	gs["remote"] = legacy.Remote
	gs["branch"] = legacy.Branch
}

// stripGlobal rewrites the global file from the install sections only.
func stripGlobal(globalPath string) {
	base, err := MergedFromOptionalFile(globalPath)
	if err != nil {
		return
	}
	m, err := ConfigToMap(base)
	if err != nil {
		return
	}
	_ = WriteConfigToml(globalPath, m)
}

// hasGitSync reports whether m sets any of git sync's repository keys.
func hasGitSync(m map[string]any) bool {
	plugins, _ := m["plugins"].(map[string]any)
	gs, _ := plugins["git_sync"].(map[string]any)
	for _, k := range []string{"enabled", "remote", "branch"} {
		if _, ok := gs[k]; ok {
			return true
		}
	}
	return false
}

// syncedWith reports whether the vault's repository already tracks remote:
// origin points there and its history is related to the remote branch. A vault
// switched to after the fact only got the remote wired in, with no shared history.
// Without a remote nothing ties a repository to the old settings, so none is adopted.
func syncedWith(root, remote, branch string) bool {
	if remote == "" {
		return false
	}
	repo, err := git.PlainOpen(root)
	if err != nil {
		return false
	}
	origin, err := repo.Remote("origin")
	if err != nil || len(origin.Config().URLs) == 0 || origin.Config().URLs[0] != remote {
		return false
	}
	tracked, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return true // never fetched yet
	}
	if err != nil {
		return false
	}
	head, err := repo.Head()
	if err != nil {
		return false
	}
	local, err1 := repo.CommitObject(head.Hash())
	upstream, err2 := repo.CommitObject(tracked.Hash())
	if err1 != nil || err2 != nil {
		return false
	}
	bases, err := local.MergeBase(upstream)
	return err == nil && len(bases) > 0
}
