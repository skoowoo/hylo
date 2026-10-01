// Package renamesync finishes what Vault.RenameNote starts: rewriting every
// other note's [[wikilink]]/bare source_notes: reference to a renamed note's
// old name. The sweep is idempotent (re-scanning a note with no stale
// reference is a no-op), so a crash just means Start re-runs whatever is
// still 'pending'/'running' from scratch — no checkpointing needed.
package renamesync

import (
	"context"
	"log/slog"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// idlePoll bounds latency when the EventVaultRename wake-up is missed
// (Notify is best-effort/non-blocking); not the common case.
const idlePoll = 5 * time.Second

// progressEvery is how often (in notes walked) a batch persists progress.
const progressEvery = 25

// Plugin implements plugin.Plugin for the vault-wide rename reference sweep.
type Plugin struct {
	vault  *storage.Vault
	logger *slog.Logger
	wake   chan struct{}
}

// New creates a rename-sync Plugin backed by the given vault.
func New(vault *storage.Vault, logger *slog.Logger) *Plugin {
	return &Plugin{
		vault:  vault,
		logger: logger,
		wake:   make(chan struct{}, 1),
	}
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "rename_sync" }

// Notify implements plugin.Plugin: just a wake-up, never the source of truth
// (that's the RenameJob row Vault.RenameNote already wrote by the time this fires).
func (p *Plugin) Notify(e plugin.Event) {
	if e.Type != plugin.EventVaultRename {
		return
	}
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Start implements plugin.Plugin: resumes leftover jobs, then loops running
// every pending batch until ctx is cancelled. Pending jobs are run as one
// batch (single ListAllNotes + one read per note) rather than one sweep per
// job, so N renames queued together cost one vault pass, not N.
func (p *Plugin) Start(ctx context.Context) error {
	for {
		jobs, err := p.vault.PendingRenameJobs()
		if err != nil {
			p.logger.Warn("rename_sync: list pending jobs failed", "err", err)
		}
		if len(jobs) > 0 {
			p.runBatch(ctx, jobs)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-p.wake:
		case <-time.After(idlePoll):
		}
	}
}

// Stop implements plugin.Plugin. Nothing to release.
func (p *Plugin) Stop() error { return nil }

// runBatch walks every note once, applying each job's rewrite in turn, and
// tracks each job's own progress/completion. Best-effort per note: a read/
// write failure is logged and skipped, it doesn't abort the batch or fail any
// job — same philosophy as Vault.rewriteDependentPathRefs.
func (p *Plugin) runBatch(ctx context.Context, jobs []storage.RenameJob) {
	notes, err := p.vault.ListAllNotes(storage.ListOptions{})
	if err != nil {
		p.logger.Warn("rename_sync: list all notes failed", "err", err)
		for _, job := range jobs {
			if ferr := p.vault.FailRenameJob(job.ID, err.Error()); ferr != nil {
				p.logger.Warn("rename_sync: mark job failed also failed", "job_id", job.ID, "err", ferr)
			}
		}
		return
	}
	for _, job := range jobs {
		if err := p.vault.StartRenameJob(job.ID, len(notes)); err != nil {
			p.logger.Warn("rename_sync: start job failed", "job_id", job.ID, "err", err)
		}
	}

	updated := make([]int, len(jobs))
	done := 0
	for _, n := range notes {
		if ctx.Err() != nil {
			return
		}
		p.rewriteOne(n, jobs, updated)
		done++
		if done%progressEvery == 0 {
			p.saveProgress(jobs, updated, done)
		}
	}

	for i, job := range jobs {
		if err := p.vault.FinishRenameJob(job.ID, len(notes), updated[i]); err != nil {
			p.logger.Warn("rename_sync: finish job failed", "job_id", job.ID, "err", err)
		}
		p.logger.Info("rename_sync: sweep complete",
			"dir", job.Dir, "old_name", job.OldName, "new_name", job.NewName,
			"notes_scanned", len(notes), "notes_updated", updated[i])
	}
}

func (p *Plugin) saveProgress(jobs []storage.RenameJob, updated []int, done int) {
	for i, job := range jobs {
		if err := p.vault.UpdateRenameJobProgress(job.ID, done, updated[i]); err != nil {
			p.logger.Warn("rename_sync: progress update failed", "job_id", job.ID, "err", err)
		}
	}
}

// rewriteOne applies every job's rewrite to one note in turn, writing it back
// at most once if any of them changed it, and increments each job's own
// updated[i] that actually matched.
func (p *Plugin) rewriteOne(n storage.Note, jobs []storage.RenameJob, updated []int) {
	path := n.Path()
	raw, err := p.vault.ReadNote(path)
	if err != nil {
		return // best-effort: e.g. a note deleted mid-sweep
	}

	out := raw
	matched := make([]bool, len(jobs))
	changed := false
	for i, job := range jobs {
		var c1, c2 bool
		out, c1 = util.RewriteWikilinkTarget(out, job.OldName, job.NewName)
		out, c2 = util.RewriteFrontmatterBareNameRef(out, job.OldName, job.NewName)
		if c1 || c2 {
			matched[i] = true
			changed = true
		}
	}
	if !changed {
		return
	}
	// Empty kind: a content-only rewrite must never touch the kind column
	// (dbUpsert leaves it alone on conflict regardless; "" just states the intent).
	if err := p.vault.WriteNote(path, out, ""); err != nil {
		p.logger.Warn("rename_sync: rewrite failed", "path", n.PathString(), "err", err)
		return // don't credit any job with an update that was never actually written
	}
	for i, m := range matched {
		if m {
			updated[i]++
		}
	}
}
