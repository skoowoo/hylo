package storage

import (
	"fmt"

	"github.com/hardhacker/hylo/internal/util"
)

// RewriteTag renames tag from to `to` in every note, or removes it when to is
// "" (see util.RewriteTag), and returns the paths it rewrote. The tags column
// picks the notes. Each write is Final, so the tags column and the search index
// are both current when this returns. Idempotent: on a partial failure, running it again finishes.
func (g *Vault) RewriteTag(from, to string) ([]Path, error) {
	from = util.NormalizeTag(from)
	if from == "" {
		return nil, fmt.Errorf("storage: rewrite tag: empty tag")
	}
	g.mu.RLock()
	notes, err := dbListByTag(g.db, from)
	g.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("storage: rewrite tag: %w", err)
	}
	var updated []Path
	var firstErr error
	for _, n := range notes {
		raw, err := g.ReadNote(n.Path())
		if err != nil {
			continue // removed mid-run
		}
		out, changed := util.RewriteTag(raw, from, to)
		if !changed {
			continue
		}
		if err := g.writeNote(n.Path(), out, Note{}, true); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("storage: rewrite tag %q: %w", n.PathString(), err)
			}
			continue
		}
		updated = append(updated, n.Path())
	}
	return updated, firstErr
}
