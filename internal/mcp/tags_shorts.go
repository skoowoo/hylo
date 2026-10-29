package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type tagListReq struct {
	Limit int `json:"limit,omitempty" jsonschema:"cap how many tags are listed, 0 = server default"`
}

type tagReq struct {
	Tag string `json:"tag" jsonschema:"tag name"`
}

type shortCreateReq struct {
	Content string `json:"content" jsonschema:"short note text"`
}

type shortIDReq struct {
	ID string `json:"id" jsonschema:"short note id from short_list, e.g. 2026-10-29-153012"`
}

type shortUpdateReq struct {
	ID      string `json:"id" jsonschema:"short note id from short_list"`
	Content string `json:"content" jsonschema:"new text, replacing the whole entry"`
	Hash    string `json:"hash,omitempty" jsonschema:"hash from short_list/short_get; when set, fails instead of overwriting a newer edit"`
}

type shortDeleteReq struct {
	ID   string `json:"id" jsonschema:"short note id from short_list"`
	Hash string `json:"hash,omitempty" jsonschema:"hash from short_list/short_get; when set, fails if the entry changed since"`
}

type shortListReq struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"max entries, 0 = no limit"`
	Start  string `json:"start,omitempty" jsonschema:"only entries created on or after this date (YYYY-MM-DD)"`
	End    string `json:"end,omitempty" jsonschema:"only entries created before this date (YYYY-MM-DD)"`
	Latest int    `json:"latest,omitempty" jsonschema:"only entries created within the last N days; cannot combine with start/end"`
}

type tagsOut struct {
	Tags []client.TagStat `json:"tags"`
}

type shortsOut struct {
	Entries []client.ShortEntry `json:"entries"`
}

func registerTagTools(s *sdk.Server, c *client.Client) {
	addTool(s, "tag_list", "Show each tag and how many notes use it.", readOnly,
		func(_ context.Context, in tagListReq) (tagsOut, error) {
			tags, err := notes.TagList(c, in.Limit)
			return tagsOut{Tags: tags}, err
		})

	addTool(s, "tag_count", "Count notes that have a tag.", readOnly,
		func(_ context.Context, in tagReq) (*client.TagCountResponse, error) {
			return c.TagCount(in.Tag)
		})

}

func registerShortTools(s *sdk.Server, c *client.Client) {
	addTool(s, "short_create", "Save a short note into today's shorts file.", additive,
		func(_ context.Context, in shortCreateReq) (client.ShortEntry, error) {
			return notes.ShortCreate(c, in.Content)
		})

	addTool(s, "short_list", "List individual short note entries, newest first.", readOnly,
		func(_ context.Context, in shortListReq) (shortsOut, error) {
			entries, err := notes.ShortList(c, notes.ShortQuery{
				Limit: in.Limit,
				Time:  notes.TimeFilter{Start: in.Start, End: in.End, Latest: in.Latest},
			})
			return shortsOut{Entries: entries}, err
		})

	addTool(s, "short_get", "Read one short note entry by id.", readOnly,
		func(_ context.Context, in shortIDReq) (client.ShortEntry, error) {
			return c.GetShort(in.ID)
		})

	addTool(s, "short_update", "Replace a short note entry's text, keeping its time and place in the daily file.", destructiveIdempotent,
		func(_ context.Context, in shortUpdateReq) (client.ShortEntry, error) {
			return notes.ShortUpdate(c, in.ID, in.Hash, in.Content)
		})

	addTool(s, "short_delete", "Permanently delete one short note entry; there is no undo.", destructiveIdempotent,
		func(_ context.Context, in shortDeleteReq) (okOut, error) {
			if err := c.DeleteShort(in.ID, in.Hash); err != nil {
				return okOut{}, err
			}
			return okOut{OK: true}, nil
		})
}
