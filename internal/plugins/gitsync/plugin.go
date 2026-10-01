// Package gitsync implements a Hylo plugin that automatically commits
// vault mutations to a local git repository and optionally syncs the commits
// to a remote (GitHub, GitLab, Gitea, any bare git server).
//
// The plugin hooks into the vault's EventHook to receive mutation events. It
// batches changes using a configurable debounce window before committing, and
// runs a separate periodic timer to pull from and push to the remote.
//
// Vault-internal metadata (.hylo/) is always excluded via .gitignore.
package gitsync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	gogitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugin"
)

// Plugin implements plugin.Plugin for git-based vault sync.
type Plugin struct {
	cfg          config.GitSyncConfig
	vaultRoot    string
	logger       *slog.Logger
	repo         *git.Repository // opened/initialised once in Start
	eventCh      chan plugin.Event
	syncReqCh    chan struct{}
	syncInterval time.Duration
	debounce     time.Duration
}

// New creates a GitSync plugin from the given config.
// Returns an error only if the config values are syntactically invalid;
// repository setup is deferred to Start.
func New(cfg config.GitSyncConfig, vaultRoot string, logger *slog.Logger) (*Plugin, error) {
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.Debounce == "" {
		cfg.Debounce = "15s"
	}
	if cfg.CommitMessage == "" {
		cfg.CommitMessage = "hylo: sync {{.Time}}"
	}

	debounce, err := time.ParseDuration(cfg.Debounce)
	if err != nil {
		return nil, fmt.Errorf("gitsync: invalid debounce %q: %w", cfg.Debounce, err)
	}

	var syncInterval time.Duration
	if cfg.SyncInterval != "" && cfg.SyncInterval != "0" {
		syncInterval, err = time.ParseDuration(cfg.SyncInterval)
		if err != nil {
			return nil, fmt.Errorf("gitsync: invalid sync_interval %q: %w", cfg.SyncInterval, err)
		}
	}

	return &Plugin{
		cfg:          cfg,
		vaultRoot:    vaultRoot,
		logger:       logger,
		eventCh:      make(chan plugin.Event, 128),
		syncReqCh:    make(chan struct{}, 1),
		syncInterval: syncInterval,
		debounce:     debounce,
	}, nil
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "git_sync" }

// Notify implements plugin.Plugin. Non-blocking: drops events when the
// buffer is full (a pending event already covers the change).
func (p *Plugin) Notify(e plugin.Event) {
	switch e.Type {
	case plugin.EventFSCreate, plugin.EventFSWrite,
		plugin.EventVaultCreate, plugin.EventVaultWrite,
		plugin.EventFSDelete, plugin.EventVaultDelete:
		// all vault mutations are relevant for git commits;
		// Manager dedup ensures exactly one of fs_*/vault_* fires per mutation
	default:
		return
	}
	select {
	case p.eventCh <- e:
	default:
	}
}

// TriggerSync requests an immediate pull+push cycle. Non-blocking.
func (p *Plugin) TriggerSync() {
	select {
	case p.syncReqCh <- struct{}{}:
	default:
	}
}

// Start implements plugin.Plugin. It opens (or initialises) the git
// repository, then runs the event loop until ctx is cancelled.
func (p *Plugin) Start(ctx context.Context) error {
	repo, err := p.ensureRepo()
	if err != nil {
		p.logger.Warn("gitsync: repository setup failed — plugin will not run",
			"err", err,
			"hint", "set plugins.git_sync.init_if_missing = true to auto-initialise",
		)
		<-ctx.Done()
		return nil
	}
	p.repo = repo

	var periodicTicker <-chan time.Time
	if p.syncInterval > 0 {
		t := time.NewTicker(p.syncInterval)
		defer t.Stop()
		periodicTicker = t.C
		p.logger.Info("gitsync: periodic sync enabled", "interval", p.syncInterval)
	}

	var debounceTimer <-chan time.Time
	pendingCommit := false

	for {
		select {
		case <-ctx.Done():
			// Flush any pending changes before exit.
			if pendingCommit {
				p.logger.Info("gitsync: flushing pending commit on shutdown")
				if commitErr := p.commitAll(); commitErr != nil {
					p.logger.Warn("gitsync: shutdown commit failed", "err", commitErr)
				}
			}
			return nil

		case <-p.eventCh:
			if !p.cfg.AutoCommit {
				continue
			}
			pendingCommit = true
			// Extend or reset the debounce window on every event.
			debounceTimer = time.After(p.debounce)

		case <-debounceTimer:
			debounceTimer = nil
			if pendingCommit {
				pendingCommit = false
				if commitErr := p.commitAll(); commitErr != nil {
					p.logger.Warn("gitsync: auto-commit failed", "err", commitErr)
				}
			}

		case <-periodicTicker:
			if syncErr := p.syncRemote(ctx); syncErr != nil {
				p.logger.Warn("gitsync: periodic sync failed", "err", syncErr)
			}

		case <-p.syncReqCh:
			// Manual trigger: commit pending changes, then sync immediately.
			pendingCommit = false
			debounceTimer = nil
			p.logger.Info("gitsync: manual sync triggered")
			if syncErr := p.syncRemote(ctx); syncErr != nil {
				p.logger.Warn("gitsync: manual sync failed", "err", syncErr)
			}
		}
	}
}

// Stop implements plugin.Plugin (cleanup after Start returns).
func (p *Plugin) Stop() error { return nil }

// ── repository management ────────────────────────────────────────────────────

// ensureRepo opens the existing repository or initialises one when
// cfg.InitIfMissing is true.
func (p *Plugin) ensureRepo() (*git.Repository, error) {
	repo, err := git.PlainOpen(p.vaultRoot)
	if err == nil {
		if gitignoreErr := p.ensureGitignore(); gitignoreErr != nil {
			p.logger.Warn("gitsync: could not update .gitignore", "err", gitignoreErr)
		}
		if remErr := p.ensureRemote(repo); remErr != nil {
			p.logger.Warn("gitsync: could not configure remote", "err", remErr)
		}
		return repo, nil
	}
	if !errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	if !p.cfg.InitIfMissing {
		return nil, fmt.Errorf(
			"no git repository found at %q (set plugins.git_sync.init_if_missing = true)",
			p.vaultRoot,
		)
	}

	p.logger.Info("gitsync: initialising new git repository", "path", p.vaultRoot)
	repo, err = git.PlainInit(p.vaultRoot, false)
	if err != nil {
		return nil, fmt.Errorf("git init: %w", err)
	}

	if p.cfg.Remote != "" {
		if _, remErr := repo.CreateRemote(&gogitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{p.cfg.Remote},
		}); remErr != nil {
			return nil, fmt.Errorf("add remote: %w", remErr)
		}
		p.logger.Info("gitsync: remote configured", "url", p.cfg.Remote)
	}

	if gitignoreErr := p.ensureGitignore(); gitignoreErr != nil {
		p.logger.Warn("gitsync: could not write .gitignore", "err", gitignoreErr)
	}
	return repo, nil
}

// ensureRemote adds the "origin" remote when the config specifies a remote URL
// but the existing repository has none. This handles the case where the remote
// was added to config after the repo was already initialised.
func (p *Plugin) ensureRemote(repo *git.Repository) error {
	if p.cfg.Remote == "" {
		return nil
	}
	existing, err := repo.Remote("origin")
	if err == nil {
		if len(existing.Config().URLs) > 0 && existing.Config().URLs[0] == p.cfg.Remote {
			return nil // already correct
		}
		// URL changed (e.g. SSH → HTTPS): delete and recreate.
		if delErr := repo.DeleteRemote("origin"); delErr != nil {
			return fmt.Errorf("update remote: %w", delErr)
		}
		p.logger.Info("gitsync: remote URL changed, updating", "url", p.cfg.Remote)
	} else if !errors.Is(err, git.ErrRemoteNotFound) {
		return fmt.Errorf("check remote: %w", err)
	}
	if _, createErr := repo.CreateRemote(&gogitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{p.cfg.Remote},
	}); createErr != nil {
		return fmt.Errorf("add remote: %w", createErr)
	}
	p.logger.Info("gitsync: remote configured", "url", p.cfg.Remote)
	return nil
}

// ensureGitignore creates or updates .gitignore in the vault root so that
// hidden files/directories and vault's internal metadata are excluded from git tracking.
func (p *Plugin) ensureGitignore() error {
	entries := []string{
		".*",          // exclude all hidden files and directories
		"!.gitignore", // but keep .gitignore itself
	}
	path := filepath.Join(p.vaultRoot, ".gitignore")

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	content := string(data)
	allPresent := true
	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			allPresent = false
			break
		}
	}
	if allPresent {
		return nil // already present
	}

	var sb strings.Builder
	if len(data) > 0 {
		sb.Write(data)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("# Exclude hidden files and directories (including .hylo/ and .Hylo/)\n")
	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			sb.WriteString(entry)
			sb.WriteByte('\n')
		}
	}

	if writeErr := os.WriteFile(path, []byte(sb.String()), 0o644); writeErr != nil {
		return fmt.Errorf("write .gitignore: %w", writeErr)
	}
	p.logger.Info("gitsync: .gitignore updated")
	return nil
}

// ── git operations ───────────────────────────────────────────────────────────

// commitAll stages every change in the vault and creates a commit.
// Returns nil without committing when the working tree is already clean.
func (p *Plugin) commitAll() error {
	wt, err := p.repo.Worktree()
	if err != nil {
		return fmt.Errorf("gitsync: worktree: %w", err)
	}

	// Ensure we're on the correct branch
	if err := p.ensureBranch(wt); err != nil {
		return fmt.Errorf("gitsync: ensure branch: %w", err)
	}

	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("gitsync: git add: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("gitsync: status: %w", err)
	}
	if status.IsClean() {
		return nil
	}

	msg := p.buildCommitMessage()
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Hylo",
			Email: "hylo@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("gitsync: commit: %w", err)
	}
	p.logger.Info("gitsync: committed", "hash", hash.String()[:8], "message", msg)
	return nil
}

// syncRemote commits any pending local changes and then pulls from /
// pushes to the configured remote. When no remote is configured it only
// runs commitAll.
func (p *Plugin) syncRemote(ctx context.Context) error {
	if err := p.commitAll(); err != nil {
		return err
	}

	if p.cfg.Remote == "" {
		return nil // local-only; nothing to sync
	}

	wt, err := p.repo.Worktree()
	if err != nil {
		return fmt.Errorf("gitsync: worktree: %w", err)
	}

	auth, err := p.auth()
	if err != nil {
		return err
	}

	// Pull (merge) from remote. Ignore "already up to date" and missing
	// branch (first push to a new remote).
	pullErr := wt.PullContext(ctx, &git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
		Force:      false,
	})
	switch {
	case pullErr == nil:
		p.logger.Info("gitsync: pulled from remote")
	case errors.Is(pullErr, git.NoErrAlreadyUpToDate):
		// nothing to do
	case isRefNotFound(pullErr):
		// Remote branch does not exist yet; will be created by push.
		p.logger.Debug("gitsync: remote branch not found yet, will create on push")
	default:
		// Log and continue so we still attempt the push.
		p.logger.Warn("gitsync: pull failed", "err", pullErr)
	}

	// Push to remote, specifying the branch refspec explicitly so it works
	// for both existing and new branches.
	branch := p.cfg.Branch
	refSpec := gogitcfg.RefSpec(
		"refs/heads/" + branch + ":refs/heads/" + branch,
	)
	pushErr := p.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gogitcfg.RefSpec{refSpec},
		Auth:       auth,
	})
	switch {
	case pushErr == nil:
		p.logger.Info("gitsync: pushed to remote")
	case errors.Is(pushErr, git.NoErrAlreadyUpToDate):
		p.logger.Debug("gitsync: remote already up to date")
	default:
		return fmt.Errorf("gitsync: push: %w", pushErr)
	}

	return nil
}

// auth returns the go-git AuthMethod for HTTPS remotes, or nil for public repos.
func (p *Plugin) auth() (transport.AuthMethod, error) {
	if p.cfg.AuthToken != "" {
		return &githttp.BasicAuth{
			Username: "git",
			Password: p.cfg.AuthToken,
		}, nil
	}
	return nil, nil
}

// ensureBranch checks if we're on the configured branch and creates/switches to it if needed.
func (p *Plugin) ensureBranch(wt *git.Worktree) error {
	head, err := p.repo.Head()
	if err != nil {
		// No HEAD yet (empty repo), branch will be created on first commit
		return nil
	}

	currentBranch := head.Name().Short()
	if currentBranch == p.cfg.Branch {
		return nil // already on the correct branch
	}

	// Try to checkout the configured branch
	branchRef := plumbing.ReferenceName("refs/heads/" + p.cfg.Branch)
	checkoutErr := wt.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
		Create: true,
	})
	if checkoutErr == nil {
		p.logger.Info("gitsync: switched to branch", "branch", p.cfg.Branch)
		return nil
	}

	// If checkout failed, log warning but continue (will use current branch)
	p.logger.Warn("gitsync: could not switch to configured branch, using current",
		"configured", p.cfg.Branch,
		"current", currentBranch,
		"err", checkoutErr,
	)
	return nil
}

// buildCommitMessage substitutes {{.Time}} in the configured template.
func (p *Plugin) buildCommitMessage() string {
	now := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	return strings.ReplaceAll(p.cfg.CommitMessage, "{{.Time}}", now)
}

// isRefNotFound returns true for go-git errors that indicate the remote
// branch does not exist yet (first push to a new remote).
func isRefNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "reference not found") ||
		strings.Contains(msg, "couldn't find remote ref")
}
