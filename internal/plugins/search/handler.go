package search

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Searcher is satisfied by the search.Plugin.
type Searcher interface {
	Search(query string, opts SearchOptions) ([]SearchResult, error)
	TagDocCount(tag string) (uint64, error)
	TagDistribution(topN int) ([]TagCount, error)
}

// Handler handles requests for full-text and filename search.
type Handler struct {
	searcher Searcher
}

// NewHandler returns a handler backed by the given Searcher.
func NewHandler(s Searcher) *Handler {
	return &Handler{searcher: s}
}

type searchRequest struct {
	Q      string `json:"q"`
	Type   string `json:"type"`
	Limit  int    `json:"limit"`
	After  string `json:"after"`
	Before string `json:"before"`
}

// Search handles POST /api/search.
// Body: {"q": "query", "type": "name|content|tag", "limit": N, "after": "RFC3339", "before": "RFC3339"}
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Q == "" {
		http.Error(w, "missing required field: q", http.StatusBadRequest)
		return
	}

	opts := SearchOptions{
		Type:  req.Type,
		Limit: req.Limit,
	}
	if req.After != "" {
		if t, err := time.Parse(time.RFC3339, req.After); err == nil {
			opts.After = t
		}
	}
	if req.Before != "" {
		if t, err := time.Parse(time.RFC3339, req.Before); err == nil {
			opts.Before = t
		}
	}

	results, err := h.searcher.Search(req.Q, opts)
	if err != nil {
		http.Error(w, "search: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []SearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"query":   req.Q,
		"total":   len(results),
		"results": results,
	})
}

type tagListRequest struct {
	// Limit is the maximum number of distinct tag terms (facet size). Zero means server default.
	Limit int `json:"limit"`
}

// TagList handles POST /api/tag/list.
// Body (optional): {"limit": N}
func (h *Handler) TagList(w http.ResponseWriter, r *http.Request) {
	var req tagListRequest
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	rows, err := h.searcher.TagDistribution(req.Limit)
	if err != nil {
		http.Error(w, "tag list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []TagCount{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"tags": rows,
	})
}

type tagCountRequest struct {
	Tag string `json:"tag"`
}

// TagCount handles POST /api/tag/count.
// Body: {"tag": "..."}
func (h *Handler) TagCount(w http.ResponseWriter, r *http.Request) {
	var req tagCountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Tag == "" {
		http.Error(w, `missing required field: "tag"`, http.StatusBadRequest)
		return
	}
	n, err := h.searcher.TagDocCount(req.Tag)
	if err != nil {
		http.Error(w, "tag count: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"tag":   req.Tag,
		"total": n,
	})
}
