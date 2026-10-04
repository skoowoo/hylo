package view

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
)

// ─── API data types ────────────────────────────────────────────────────────────

type graphAPINode struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Path       string   `json:"path"`
	Tags       []string `json:"tags,omitempty"`
	EntityType string   `json:"entity_type,omitempty"`
}

type graphAPIEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type graphAPIData struct {
	Nodes []graphAPINode `json:"nodes"`
	Edges []graphAPIEdge `json:"edges"`
}

// ─── handlers ──────────────────────────────────────────────────────────────────

// KnowledgeGraphRebuild handles POST /api/graph/rebuild
// Rebuilds the knowledge_links table from the current vault filesystem.
// Safe to call at any time — only touches knowledge_links.
func (vh *ViewHandler) KnowledgeGraphRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	knowledgeDir := ""
	if vh.cfg != nil {
		knowledgeDir = vh.cfg.Vault.KnowledgeDir
	}
	if err := vh.vault.BackfillKnowledgeLinks(knowledgeDir); err != nil {
		http.Error(w, "rebuild: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// KnowledgeGraphData handles GET /api/graph/data?index=PATH
// Returns JSON {nodes, edges} for the knowledge graph.
// When index is provided, only nodes/edges in that index are returned.
func (vh *ViewHandler) KnowledgeGraphData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	indexParam := strings.TrimSpace(r.URL.Query().Get("index"))

	var notes []storage.Note
	var edges []storage.KnowledgeEdge

	if indexParam != "" {
		idxPath, ok := storage.ParsePath(indexParam)
		if !ok {
			http.Error(w, `index must be absolute (start with "/")`, http.StatusBadRequest)
			return
		}
		knowledgePaths, err := vh.vault.GetIndexDeps(idxPath)
		if err != nil {
			http.Error(w, "get index deps: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if len(knowledgePaths) > 0 {
			notes, err = vh.vault.GetNotesByPaths(knowledgePaths)
			if err != nil {
				http.Error(w, "get notes: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		edges, err = vh.vault.GetKnowledgeLinksForIndex(idxPath)
		if err != nil {
			http.Error(w, "get edges: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		var err error
		notes, err = vh.vault.ListAllNotes(storage.ListOptions{
			OnlyKinds: []storage.Kind{storage.KindKnowledge},
		})
		if err != nil {
			http.Error(w, "list knowledge notes: "+err.Error(), http.StatusInternalServerError)
			return
		}
		edges, err = vh.vault.GetAllKnowledgeLinks()
		if err != nil {
			http.Error(w, "get edges: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Not derived from edges: an index view drops links leaving the index, so
	// a node whose links all point outside it would lose its type.
	entityTypeByPath, err := vh.vault.GetKnowledgeEntityTypes()
	if err != nil {
		http.Error(w, "get entity types: "+err.Error(), http.StatusInternalServerError)
		return
	}

	apiNodes := make([]graphAPINode, 0, len(notes))
	for _, n := range notes {
		label := n.Title
		if label == "" {
			label = strings.TrimSuffix(n.Name, ".md")
		}
		// Prefer entity_type from knowledge_links (set at build time from frontmatter).
		// Fall back to tags[0] which compile plugin also sets to entity_type.
		et := entityTypeByPath[n.PathString()]
		if et == "" && len(n.Tags) > 0 {
			et = n.Tags[0]
		}
		apiNodes = append(apiNodes, graphAPINode{
			ID:         n.PathString(),
			Label:      label,
			Path:       n.PathString(),
			Tags:       n.Tags,
			EntityType: et,
		})
	}

	apiEdges := make([]graphAPIEdge, 0, len(edges))
	for _, e := range edges {
		if e.Source == e.Target {
			continue // self-loops store entity_type for leaf nodes; exclude from graph edges
		}
		apiEdges = append(apiEdges, graphAPIEdge{
			Source: e.Source.String(),
			Target: e.Target.String(),
		})
	}

	result := graphAPIData{Nodes: apiNodes, Edges: apiEdges}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "encode: "+err.Error(), http.StatusInternalServerError)
	}
}
