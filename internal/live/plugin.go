package live

import (
	"context"

	"github.com/hardhacker/hylo/internal/plugin"
)

// Plugin turns vault events into note changes; each write arrives once,
// whether it came from the API or disk.
type Plugin struct{ feed *Feed }

func NewPlugin(f *Feed) *Plugin { return &Plugin{feed: f} }

func (p *Plugin) Name() string { return "live" }

func (p *Plugin) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) Notify(e plugin.Event) {
	if c, ok := noteChange(e); ok {
		p.feed.Publish(c)
	}
}

// ShortRef is the Data of a write that touched one short entry, so a page can
// update that card in place; Hash is empty once the entry is deleted.
type ShortRef struct {
	ID   string `json:"id"`
	Hash string `json:"hash,omitempty"`
}

func noteChange(e plugin.Event) (Change, bool) {
	c := Change{Topic: TopicNotes, Path: e.Path, IsDir: e.IsDir}
	if e.ShortID != "" {
		c.Data = ShortRef{ID: e.ShortID, Hash: e.ShortHash}
	}
	switch e.Type {
	case plugin.EventVaultCreate, plugin.EventFSCreate:
		c.Op = "create"
	case plugin.EventVaultWrite, plugin.EventFSWrite:
		c.Op = "write"
	case plugin.EventVaultDelete, plugin.EventFSDelete:
		c.Op = "delete"
	case plugin.EventVaultRename:
		c.Op = "rename"
		c.OldPath = e.OldPath
	case plugin.EventVaultPin:
		c.Op = "pin"
	case plugin.EventVaultImage, plugin.EventFSImage:
		c.Op = "image"
	default:
		return Change{}, false
	}
	return c, true
}
