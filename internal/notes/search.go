package notes

import (
	"fmt"
	"math"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/util"
)

type SearchQuery struct {
	Query string
	Field string // name, content, tag; empty searches all
	Limit int    // default 20
	// KnowledgeOnly keeps only knowledge and index hits.
	KnowledgeOnly bool
}

type SearchEntry struct {
	Name      string  `json:"name"`
	Dir       string  `json:"dir"`
	Kind      string  `json:"kind"`
	UpdatedAt string  `json:"updated_at"`
	Score     float64 `json:"score"`
	HitLines  []int   `json:"hit_lines,omitempty"`
}

type SearchResult struct {
	Query   string        `json:"query"`
	Total   int           `json:"total"`
	Results []SearchEntry `json:"results"`
}

func Search(c *client.Client, q SearchQuery) (SearchResult, error) {
	switch q.Field {
	case "", "name", "content", "tag":
	default:
		return SearchResult{}, fmt.Errorf("unknown field %q: must be name, content, or tag (default: all)", q.Field)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	// The index has no kind filter, so over-fetch and filter client-side.
	fetch := limit
	if q.KnowledgeOnly {
		fetch = min(max(limit*25, 100), 3000)
	}
	resp, err := c.Search(q.Query, q.Field, fetch)
	if err != nil {
		return SearchResult{}, err
	}

	out := SearchResult{Query: resp.Query, Results: []SearchEntry{}}
	for _, hit := range resp.Results {
		if q.KnowledgeOnly && hit.Kind != "knowledge" && hit.Kind != "index" {
			continue
		}
		out.Results = append(out.Results, SearchEntry{
			Name:      hit.Name,
			Dir:       hit.Dir,
			Kind:      hit.Kind,
			UpdatedAt: util.FormatTime(hit.UpdatedAt),
			Score:     math.Round(hit.Score*1000) / 1000,
			HitLines:  hit.Lines,
		})
		if len(out.Results) >= limit {
			break
		}
	}
	out.Total = len(out.Results)
	return out, nil
}
