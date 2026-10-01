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
	// Filesystem-driven events — emitted by the vault watcher in response to
	// fsnotify signals. These fire asynchronously (after a short write debounce
	// for Write events) and serve as the authoritative source for changes made
	// by external editors or tools that bypass the Vault API entirely.
	EventFSCreate EventType = "fs_create" // file created or overwritten (temp→rename makes Create/Write indistinguishable)
	EventFSWrite  EventType = "fs_write"  // existing file content changed in-place
	EventFSDelete EventType = "fs_delete" // file deleted or renamed away

	// Vault API-driven events — emitted synchronously by Vault operations
	// immediately after the filesystem write and lock release. For every
	// Vault-originated mutation both a vault_* event and a corresponding fs_*
	// event will be dispatched; the plugin Manager deduplicates them within a
	// short time window so each plugin sees exactly one event per mutation.
	//
	// External writes (not via the Vault API) produce only fs_* events.
	EventVaultCreate EventType = "vault_create" // note written for the first time via Vault API
	EventVaultWrite  EventType = "vault_write"  // existing note overwritten or appended via Vault API
	EventVaultDelete EventType = "vault_delete" // note(s) permanently deleted via Vault API

	// EventVaultRename is emitted by Vault.RenameNote once the filename change
	// itself has committed (filesystem + metadata DB). Path carries the new
	// vault-absolute path, OldPath the previous one. Always paired with a
	// vault_delete(OldPath)+vault_create(Path) pair (same as MoveNote), so
	// plugins that only care about content/index sync (e.g. search) don't
	// need to special-case it — this event exists for plugins that care
	// specifically about the rename itself, currently just
	// internal/plugins/renamesync, which uses it purely as a low-latency
	// wake-up to check for pending RenameJob rows (see storage.RenameJob);
	// its correctness never depends on this event actually arriving.
	// Not subject to vault deduplication.
	EventVaultRename EventType = "vault_rename"

	// EventVaultShortAppend is emitted after every successful AppendShort call,
	// regardless of whether the daily file is new or existing. It is not subject
	// to deduplication and is never paired with an fs_* event.
	EventVaultShortAppend EventType = "vault_short_append"

	// EventWechatMessage is emitted when the WeChat bridge receives a direct text message.
	// Not subject to vault deduplication.
	EventWechatMessage EventType = "wechat_message"

	// EventWechatNotify triggers the WeChat plugin to proactively send Content to WechatUserID.
	// If WechatUserID is empty the plugin uses the logged-in owner's user ID.
	// Not subject to vault deduplication.
	EventWechatNotify EventType = "wechat_notify"

	// EventDiscordMessage is emitted when the Discord bridge receives a DM.
	// Not subject to vault deduplication.
	EventDiscordMessage EventType = "discord_message"

	// EventDiscordNotify triggers the Discord plugin to proactively send Content to DiscordUserID.
	// If DiscordUserID is empty the plugin uses the configured owner's user ID.
	// Not subject to vault deduplication.
	EventDiscordNotify EventType = "discord_notify"

	// EventCompileRequested is emitted when the user manually requests compilation
	// of a vault note via the compile API. Path carries the vault-absolute note
	// path. Handled by a mate agent; not subject to vault deduplication.
	EventCompileRequested EventType = "compile_requested"
)

// Event carries information about a single vault mutation.
type Event struct {
	Type    EventType
	Path    string // vault-relative slash path, or /wechat/{userId} for WeChat
	Time    time.Time
	IsDir   bool   // true when Path is a directory (only meaningful for delete events)
	Content string // note content, short note body, or WeChat message text

	// OldPath is the vault-absolute path the note had before the mutation.
	// Only set for EventVaultRename, where Path is the new path.
	OldPath string

	// WechatUserID is the iLink user ID associated with the event.
	// For EventWechatMessage: the sender's user ID.
	// For EventWechatNotify: the target user ID (empty = logged-in owner).
	WechatUserID string

	// DiscordChannelID is the Discord channel ID (DM channel) associated with the event.
	// For EventDiscordMessage: the channel the message arrived in.
	// For EventDiscordNotify: unused (plugin opens a DM channel from DiscordUserID).
	DiscordChannelID string

	// DiscordUserID is the Discord user ID associated with the event.
	// For EventDiscordMessage: the sender's user ID.
	// For EventDiscordNotify: the target user ID (empty = configured owner).
	DiscordUserID string

	// DiscordMessageID is the original Discord message ID, used to reply-link the response.
	DiscordMessageID string

	// Reply is an optional callback invoked after a matched mate trigger run completes.
	Reply ReplyFunc
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
