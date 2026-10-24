package gitsync

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gogitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/hardhacker/hylo/internal/config"
)

func newPlugin(t *testing.T, root, branch, remote string) *Plugin {
	t.Helper()
	p, err := New(config.GitSyncConfig{
		Enabled: true, Branch: branch, Remote: remote, InitIfMissing: true,
	}, root, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := p.ensureRepo()
	if err != nil {
		t.Fatal(err)
	}
	p.repo = repo
	return p
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func branchHas(t *testing.T, repo *git.Repository, branch, file string) bool {
	t.Helper()
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		return false
	}
	c, err := repo.CommitObject(ref.Hash())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.File(file)
	return err == nil
}

func TestBranchIsRequired(t *testing.T) {
	_, err := New(config.GitSyncConfig{Enabled: true}, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("no branch accepted")
	}
}

func TestFreshRepoCommitsToConfiguredBranch(t *testing.T) {
	root := t.TempDir()
	p := newPlugin(t, root, "notes", "")
	write(t, root, "a.md", "a")
	if err := p.commitAll(); err != nil {
		t.Fatal(err)
	}
	head, err := p.repo.Head()
	if err != nil || head.Name().Short() != "notes" {
		t.Fatalf("head: %v %v", head, err)
	}
	if !branchHas(t, p.repo, "notes", "a.md") {
		t.Fatal("commit not on notes")
	}
}

func TestExistingBranchIsUsedNotCurrentOne(t *testing.T) {
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	sig := &object.Signature{Name: "u", Email: "u@u", When: time.Now()}
	write(t, root, "base.md", "base")
	wt.Add("base.md")
	if _, err := wt.Commit("base", &git.CommitOptions{Author: sig}); err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("main"), Create: true}); err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("dev"), Create: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gogitcfg.RemoteConfig{Name: "origin", URLs: []string{"https://example.com/mine.git"}}); err != nil {
		t.Fatal(err)
	}

	p := newPlugin(t, root, "main", "")
	write(t, root, "new.md", "new")
	write(t, root, "base.md", "edited")
	if err := p.commitAll(); err != nil {
		t.Fatal(err)
	}
	if !branchHas(t, p.repo, "main", "new.md") || branchHas(t, p.repo, "dev", "new.md") {
		t.Fatal("commit landed on the wrong branch")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "base.md")); string(b) != "edited" {
		t.Fatal("switching branch rewrote vault files")
	}
	origin, err := p.repo.Remote("origin")
	if err != nil || origin.Config().URLs[0] != "https://example.com/mine.git" {
		t.Fatalf("origin changed: %v", err)
	}
	if _, err := p.repo.Remote(remoteName); err == nil {
		t.Fatal("hylo remote added without a remote URL")
	}
}

func TestPushesOnlyToConfiguredRemote(t *testing.T) {
	bare := t.TempDir()
	if _, err := git.PlainInit(bare, true); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	p := newPlugin(t, root, "notes", bare)
	write(t, root, "a.md", "a")
	if err := p.syncRemote(context.Background()); err != nil {
		t.Fatal(err)
	}
	up, _ := git.PlainOpen(bare)
	if !branchHas(t, up, "notes", "a.md") {
		t.Fatal("not pushed to the configured branch")
	}
	if _, err := p.repo.Remote("origin"); err == nil {
		t.Fatal("origin was created")
	}

	// Clearing the remote takes Hylo's remote away again.
	p.cfg.Remote = ""
	if err := p.ensureRemote(p.repo); err != nil {
		t.Fatal(err)
	}
	if _, err := p.repo.Remote(remoteName); err == nil {
		t.Fatal("hylo remote kept after remote was cleared")
	}
}
