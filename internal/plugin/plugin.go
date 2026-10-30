// Package plugin defines the interface that all Hylo plugins must implement
// and the Event type that the vault emits after every mutation.
package plugin

import (
	"context"
	"time"
)

// EventType describes the kind of vault mutation that occurred.
type EventType string

const (
	// Filesystem-driven events — emitted by the vault watcher for changes made
	// outside the Vault API (external editors, sync clients). The watcher drops
	// the echoes of Vault API writes, so each mutation reaches plugins once.
	EventFSCreate EventType = "fs_create" // a note the vault did not know about appeared
	EventFSWrite  EventType = "fs_write"  // a known note's content changed
	EventFSDelete EventType = "fs_delete" // a known note was deleted or renamed away

	// Vault API-driven events — emitted synchronously by Vault operations
	// immediately after the filesystem write and lock release.
	EventVaultCreate EventType = "vault_create" // note written for the first time via Vault API
	EventVaultWrite  EventType = "vault_write"  // existing note overwritten or appended via Vault API
	EventVaultDelete EventType = "vault_delete" // note(s) permanently deleted via Vault API

	// EventVaultRename is emitted by Vault.RenameNote once the filename change
	// itself has committed (filesystem + metadata DB). Path carries the new
	// vault-absolute path, OldPath the previous one. Always followed by a
	// vault_delete(OldPath)+vault_create(Path) pair (same as MoveNote), so
	// plugins that only care about content/index sync (e.g. search) don't
	// need to special-case it — this event exists for plugins that care
	// specifically about the rename itself, currently just
	// internal/plugins/renamesync, which uses it purely as a low-latency
	// wake-up to check for pending RenameJob rows (see storage.RenameJob);
	// its correctness never depends on this event actually arriving.
	EventVaultRename EventType = "vault_rename"

	// EventVaultShortAppend is emitted after every successful AppendShort call,
	// regardless of whether the daily file is new or existing.
	EventVaultShortAppend EventType = "vault_short_append"

	// EventVaultPin is emitted when a note is pinned or unpinned; Path is the note.
	// Pinning only touches metadata, so no fs_* event pairs with it.
	EventVaultPin EventType = "vault_pin"

	// EventVaultImage is emitted when an image is saved to or deleted from the
	// vault via the Vault API; Path is the image.
	EventVaultImage EventType = "vault_image"

	// EventFSImage is the watcher's counterpart: an image was added, changed or
	// removed outside the Vault API.
	EventFSImage EventType = "fs_image"

	// EventCompileRequested is emitted when the user manually requests compilation
	// of a vault note via the compile API. Path carries the vault-absolute note
	// path. Handled by a mate agent.
	EventCompileRequested EventType = "compile_requested"
)

// Event carries information about a single vault mutation.
type Event struct {
	Type    EventType
	Path    string // vault-relative slash path
	Time    time.Time
	IsDir   bool   // true when Path is a directory (only meaningful for delete events)
	Content string // note content or short note body

	// Moved marks an fs_create for a note that arrived with its directory
	// (folder rename, move or sync pull) rather than being written.
	Moved bool

	// OldPath is the vault-absolute path the note had before the mutation.
	// Only set for EventVaultRename, where Path is the new path.
	OldPath string

	// ShortID names the entry a vault write to a daily short file touched;
	// ShortHash is its new content hash, empty once the entry is deleted.
	ShortID   string
	ShortHash string

	// Pinned is the note's new pinned state on an EventVaultPin.
	Pinned bool
}

// Plugin is the interface every Hylo plugin must implement.
//
// Plugins are long-lived services started alongside the server and stopped
// during graceful shutdown. They receive vault mutation events via Notify
// and may also perform background work independently (e.g., periodic sync).
type Plugin interface {
	// Name returns a stable, unique identifier for the plugin (e.g. "git_sync").
	Name() string

	// Start runs the plugin's main loop until ctx is cancelled.
	// It must return promptly once ctx.Done() is closed.
	Start(ctx context.Context) error

	// Stop performs any cleanup that needs to happen after Start has returned
	// (e.g., flushing buffers, closing connections).
	Stop() error

	// Notify is called by the vault after every successful mutation.
	// Implementations must be non-blocking; drop or buffer the event rather
	// than blocking the vault's write-lock critical section.
	Notify(e Event)
}
