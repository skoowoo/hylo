package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type knowledgeListReq struct {
	Path   string `json:"path,omitempty" jsonschema:"vault-absolute directory to scope the listing"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max results, 0 = no limit"`
	Start  string `json:"start,omitempty" jsonschema:"only notes updated on or after this date (YYYY-MM-DD)"`
	End    string `json:"end,omitempty" jsonschema:"only notes updated before this date (YYYY-MM-DD)"`
	Latest int    `json:"latest,omitempty" jsonschema:"only notes updated within the last N days; cannot combine with start/end"`
	Kind   string `json:"kind,omitempty" jsonschema:"filter by kind: knowledge or index"`
}

type indexesOut struct {
	Indexes []notes.IndexEntry `json:"indexes"`
}

func registerKnowledgeTools(s *sdk.Server, c *client.Client) {
	addTool(s, "knowledge_list", "List knowledge notes and index notes, most recently updated first.", true,
		func(_ context.Context, in knowledgeListReq) (notesOut, error) {
			entries, err := notes.List(c, notes.ListQuery{
				Dir:       in.Path,
				Limit:     in.Limit,
				Time:      notes.TimeFilter{Start: in.Start, End: in.End, Latest: in.Latest},
				Kind:      in.Kind,
				Knowledge: true,
			})
			return notesOut{Notes: entries}, err
		})

	addTool(s, "knowledge_read", "Read a knowledge note's full content.", true,
		func(_ context.Context, in readReq) (readOut, error) {
			p, content, err := notes.Read(c, in.Path, true)
			return readOut{Path: p, Content: content}, err
		})

	addTool(s, "knowledge_search", "Full-text search over knowledge notes only.", true,
		func(_ context.Context, in searchReq) (notes.SearchResult, error) {
			return notes.Search(c, notes.SearchQuery{Query: in.Query, Field: in.Field, Limit: in.Limit, KnowledgeOnly: true})
		})

	addTool(s, "knowledge_delete", "Permanently delete a knowledge note.", false,
		func(_ context.Context, in pathReq) (pathOut, error) {
			return pathOut{Path: in.Path}, notes.DeleteKnowledge(c, in.Path)
		})

	addTool(s, "knowledge_list_indexes", "List index notes with their domain and vault path.", true,
		func(_ context.Context, _ noReq) (indexesOut, error) {
			entries, err := notes.ListIndexes(c)
			return indexesOut{Indexes: entries}, err
		})
}
