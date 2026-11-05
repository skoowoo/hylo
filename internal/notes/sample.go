package notes

import (
	"time"

	"github.com/hardhacker/hylo/internal/client"
)

// SampleQuery is a random walk through the vault. Zero Each and OlderThan
// take the defaults (2 of each kind, reaching back 90 days).
type SampleQuery struct {
	Each      int
	OlderThan int
}

// SampleItem is one drawn note, knowledge unit, or short entry.
type SampleItem struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Title       string `json:"title,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Perspective string `json:"perspective,omitempty"`
	CreatedAt   string `json:"created_at"`
	Excerpt     string `json:"excerpt,omitempty"`
	ID          string `json:"id,omitempty"`
}

func Sample(c *client.Client, q SampleQuery) ([]SampleItem, error) {
	got, err := c.Sample(q.Each, q.OlderThan)
	if err != nil {
		return nil, err
	}
	out := make([]SampleItem, len(got))
	for i, it := range got {
		date := ""
		if !it.CreatedAt.IsZero() {
			date = it.CreatedAt.In(time.Local).Format(time.DateOnly)
		}
		out[i] = SampleItem{
			Kind:        it.Kind,
			Path:        it.Path,
			Title:       it.Title,
			Domain:      it.Domain,
			Perspective: it.Perspective,
			CreatedAt:   date,
			Excerpt:     it.Excerpt,
			ID:          it.ID,
		}
	}
	return out, nil
}
