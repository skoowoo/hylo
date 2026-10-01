package renamesync

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/storage"
)

func newTestVault(t *testing.T) *storage.Vault {
	t.Helper()
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitForJobDone polls until the job reaches a terminal state or the timeout
// elapses, returning the last observed job.
func waitForJobDone(t *testing.T, v *storage.Vault, id int64, timeout time.Duration) storage.RenameJob {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		job, err := v.GetRenameJob(id)
		if err != nil {
			t.Fatalf("GetRenameJob: %v", err)
		}
		if job.Status == storage.RenameJobDone || job.Status == storage.RenameJobFailed {
			return job
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for job %d to finish, last state: %+v", id, job)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestPluginResumesJobOnStart is effectively the crash-resume test: the
// RenameJob row is inserted by Vault.RenameNote (simulating "the process
// crashed right after the rename committed"), and a *fresh* Plugin is started
// afterwards with no EventVaultRename ever delivered to it — Start's initial
// PendingRenameJobs() scan must pick the job up on its own.
func TestPluginResumesJobOnStart(t *testing.T) {
	v := newTestVault(t)

	target := storage.Path("/notes/old.md")
	referrer := storage.Path("/notes/referrer.md")
	if err := v.WriteNote(target, []byte("target body"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote(referrer, []byte("see [[old]] and [[old.md]] for more"), ""); err != nil {
		t.Fatal(err)
	}

	newPath, jobID, err := v.RenameNote(target, "new.md")
	if err != nil {
		t.Fatal(err)
	}
	if newPath != storage.Path("/notes/new.md") {
		t.Fatalf("newPath = %q", newPath)
	}

	// A fresh plugin instance, as if the process had just restarted.
	p := New(v, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = p.Start(ctx) }()

	job := waitForJobDone(t, v, jobID, 2*time.Second)
	if job.Status != storage.RenameJobDone {
		t.Fatalf("job did not complete: %+v", job)
	}
	if job.UpdatedCount != 1 {
		t.Fatalf("updated_count = %d, want 1", job.UpdatedCount)
	}

	out, err := v.ReadNote(referrer)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "[[old]]") || strings.Contains(string(out), "[[old.md]]") {
		t.Fatalf("wikilinks should have been rewritten: %s", out)
	}
	if !strings.Contains(string(out), "[[new]]") {
		t.Fatalf("expected the new name to appear: %s", out)
	}
}

// TestPluginBatchesConcurrentJobs covers two renames enqueued before the
// plugin ever runs: both must be resolved correctly by one batched pass (see
// runBatch), not just whichever job happened to run first.
func TestPluginBatchesConcurrentJobs(t *testing.T) {
	v := newTestVault(t)

	a := storage.Path("/notes/a.md")
	b := storage.Path("/notes/b.md")
	referrer := storage.Path("/notes/referrer.md")
	if err := v.WriteNote(a, []byte("a body"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote(b, []byte("b body"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote(referrer, []byte("see [[a]] and [[b]]"), ""); err != nil {
		t.Fatal(err)
	}

	_, jobA, err := v.RenameNote(a, "new-a.md")
	if err != nil {
		t.Fatal(err)
	}
	_, jobB, err := v.RenameNote(b, "new-b.md")
	if err != nil {
		t.Fatal(err)
	}

	p := New(v, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = p.Start(ctx) }()

	waitForJobDone(t, v, jobA, 2*time.Second)
	waitForJobDone(t, v, jobB, 2*time.Second)

	out, err := v.ReadNote(referrer)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[[new-a]]") || !strings.Contains(string(out), "[[new-b]]") {
		t.Fatalf("both wikilinks should have been rewritten by one batched pass: %s", out)
	}
}

// TestPluginNotifyWakesSweepPromptly checks the low-latency path: Notify
// should let the sweep start well before the idlePoll fallback would.
func TestPluginNotifyWakesSweepPromptly(t *testing.T) {
	v := newTestVault(t)
	target := storage.Path("/notes/old.md")
	if err := v.WriteNote(target, []byte("body"), ""); err != nil {
		t.Fatal(err)
	}

	p := New(v, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = p.Start(ctx) }()
	// Give Start a moment to reach its idle select before nudging it, so this
	// exercises the wake channel rather than the very first resume scan.
	time.Sleep(20 * time.Millisecond)

	_, jobID, err := v.RenameNote(target, "new.md")
	if err != nil {
		t.Fatal(err)
	}
	p.Notify(plugin.Event{
		Type: plugin.EventVaultRename, Path: "/notes/new.md", OldPath: string(target), Time: time.Now(),
	})

	job := waitForJobDone(t, v, jobID, idlePoll/2)
	if job.Status != storage.RenameJobDone {
		t.Fatalf("Notify should have woken the sweep well before idlePoll elapsed, got: %+v", job)
	}
}
