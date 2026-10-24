package config

import (
	"errors"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// migrateGitSync moves git sync off the global file, where older installs kept
// it. The settings go to this vault only when its repository is the one they
// were syncing; either way they leave the global file. Returns the vault
// config's new keys when it adopted them.
func migrateGitSync(root, globalPath string, legacy GitSyncConfig) map[string]any {
	global, err := ReadTomlMap(globalPath)
	if err != nil || !hasGitSync(global) {
		return nil
	}
	if legacy.Branch == "" {
		legacy.Branch = "main" // what older installs committed to by default
	}
	var adopted map[string]any
	raw, _ := ReadTomlMap(VaultConfigPath(root))
	if legacy.Enabled && !hasGitSync(raw) && syncedWith(root, legacy.Remote, legacy.Branch) {
		patch := map[string]any{"plugins": map[string]any{"git_sync": map[string]any{
			"enabled": true, "remote": legacy.Remote, "branch": legacy.Branch,
		}}}
		if WriteVaultConfig(root, patch) != nil {
			return nil
		}
		adopted, _ = ReadTomlMap(VaultConfigPath(root))
	}
	if base, err := MergedFromOptionalFile(globalPath); err == nil {
		if m, err := ConfigToMap(base); err == nil {
			_ = WriteConfigToml(globalPath, m)
		}
	}
	if adopted == nil {
		return nil
	}
	return pickKeys(adopted, true)
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
