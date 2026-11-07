package pulse

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/storage"
)

// change is a note's content changing. Whoever wrote it, the user or a bot
// for them, it is news to a round, unless the round's own turn wrote it
// (Store.PulseSettled draws that line).
type change struct {
	Path    string    `json:"path"`
	Created bool      `json:"created,omitempty"`
	Short   string    `json:"short,omitempty"`   // the short entry written, in a shorts file
	New     bool      `json:"new,omitempty"`     // the short was just appended
	Content string    `json:"content,omitempty"` // a new short's text
	At      time.Time `json:"at"`
}

// ignoredDirs hold notes made for tools rather than reading.
var ignoredDirs = []string{storage.KnowledgeDir + "/_indexes/", storage.BotsDir + "/", storage.AssetsDir + "/", "/.hylo/"}

func ignored(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		return true
	}
	for _, d := range ignoredDirs {
		if strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

// changes keeps the vault's content changes over the digest's reach. The file,
// when set, carries them over a restart; they are only ever this recent.
type changes struct {
	mu    sync.Mutex
	list  []change
	file  string
	dirty bool
}

func (c *changes) load(file string, now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.file = file
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &c.list); err != nil {
		c.list = nil
		return err
	}
	c.prune(now)
	return nil
}

// save writes the changes out if they changed since the last save.
func (c *changes) save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file == "" || !c.dirty {
		return nil
	}
	data, err := json.Marshal(c.list)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.file), 0755); err != nil {
		return err
	}
	tmp := c.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.file); err != nil {
		return err
	}
	c.dirty = false
	return nil
}

// add records e if it changed a note's content. A short's append arrives as
// a write naming the entry, then the append carrying its text.
func (c *changes) add(e plugin.Event, now time.Time) {
	if e.IsDir || ignored(e.Path) {
		return
	}
	at := e.Time
	if at.IsZero() {
		at = now
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	switch e.Type {
	case plugin.EventVaultShortAppend:
		for i := len(c.list) - 1; i >= 0; i-- {
			if ch := &c.list[i]; ch.Path == e.Path && ch.Short != "" {
				ch.New, ch.Content = true, e.Content
				c.dirty = true
				break
			}
		}
		return
	case plugin.EventVaultCreate, plugin.EventFSCreate:
		if e.Moved {
			return
		}
	case plugin.EventVaultWrite, plugin.EventFSWrite:
		if e.ShortID != "" && e.ShortHash == "" {
			return // the short was deleted
		}
	default:
		return
	}
	c.dirty = true
	created := e.Type == plugin.EventVaultCreate || e.Type == plugin.EventFSCreate
	// Autosave writes a note over and over; one entry per edit stretch.
	if n := len(c.list); n > 0 {
		if last := &c.list[n-1]; last.Path == e.Path && last.Short == e.ShortID && !last.New {
			last.At, last.Created = at, last.Created || created
			c.prune(now)
			return
		}
	}
	c.list = append(c.list, change{Path: e.Path, Created: created, Short: e.ShortID, At: at})
	c.prune(now)
}

func (c *changes) prune(now time.Time) {
	i := 0
	for i < len(c.list) && now.Sub(c.list[i].At) > digestWindow {
		i++
	}
	c.list = c.list[i:]
}

// since returns the changes after t, oldest first.
func (c *changes) since(t time.Time) []change {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []change
	for _, ch := range c.list {
		if ch.At.After(t) {
			out = append(out, ch)
		}
	}
	return out
}
