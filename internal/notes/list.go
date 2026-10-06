package notes

import (
	"fmt"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

type NoteEntry struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	Size      string `json:"size"`
	UpdatedAt string `json:"updated_at"`
	Indexed   bool   `json:"indexed,omitempty"`
	Kind      string `json:"kind"`
}

type IndexEntry struct {
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

type ListQuery struct {
	Dir   string // vault-absolute directory; empty lists the whole vault
	Limit int    // 0 = no limit
	Time  TimeFilter
	Kind  string
	// Knowledge restricts the listing to knowledge and index notes.
	Knowledge bool
}

func (q ListQuery) options() (storage.ListOptions, error) {
	opts := storage.ListOptions{SortByTime: true, Limit: q.Limit}
	var err error
	if opts.After, opts.Before, err = q.Time.Bounds(); err != nil {
		return opts, err
	}

	if q.Knowledge {
		switch q.Kind {
		case "":
			opts.OnlyKinds = []storage.Kind{storage.KindKnowledge, storage.KindIndex}
		case "knowledge":
			opts.OnlyKinds = []storage.Kind{storage.KindKnowledge}
		case "index":
			opts.OnlyKinds = []storage.Kind{storage.KindIndex}
		default:
			return opts, fmt.Errorf("unknown kind %q: must be knowledge or index", q.Kind)
		}
		return opts, nil
	}

	switch q.Kind {
	case "":
	case "raw":
		opts.ExcludeKinds = []storage.Kind{storage.KindShort, storage.KindKnowledge, storage.KindIndex}
	case "short":
		opts.OnlyKinds = []storage.Kind{storage.KindShort}
	case "knowledge":
		opts.OnlyKinds = []storage.Kind{storage.KindKnowledge}
	case "index":
		opts.OnlyKinds = []storage.Kind{storage.KindIndex}
	default:
		return opts, fmt.Errorf("unknown kind %q: must be raw, short, knowledge, or index", q.Kind)
	}
	return opts, nil
}

func List(c *client.Client, q ListQuery) ([]NoteEntry, error) {
	opts, err := q.options()
	if err != nil {
		return nil, err
	}
	var notes []storage.Note
	if q.Dir == "" {
		notes, err = c.ListAllNotes(opts)
	} else {
		notes, err = c.ListDir(q.Dir, opts)
	}
	if err != nil {
		return nil, err
	}
	return Entries(notes), nil
}

func Entries(notes []storage.Note) []NoteEntry {
	out := make([]NoteEntry, len(notes))
	for i, n := range notes {
		kind := string(n.Kind)
		if kind == "" {
			kind = "raw"
		}
		out[i] = NoteEntry{
			Path:      n.PathString(),
			Name:      n.Name,
			Dir:       n.Dir,
			Size:      util.FormatSize(n.Size),
			UpdatedAt: util.FormatTime(n.UpdatedAt),
			Indexed:   n.Indexed,
			Kind:      kind,
		}
	}
	return out
}

func ListIndexes(c *client.Client) ([]IndexEntry, error) {
	notes, err := c.ListAllNotes(storage.ListOptions{
		SortByTime: true,
		OnlyKinds:  []storage.Kind{storage.KindIndex},
	})
	if err != nil {
		return nil, err
	}
	out := make([]IndexEntry, len(notes))
	for i, n := range notes {
		out[i] = IndexEntry{Domain: n.Title, Path: n.PathString()}
	}
	return out, nil
}
