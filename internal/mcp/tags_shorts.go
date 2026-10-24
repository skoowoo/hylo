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
		func(_ context.Context, in shortCreateReq) (pathOut, error) {
			p, err := notes.ShortCreate(c, in.Content)
			return pathOut{Path: p}, err
		})

	addTool(s, "short_list", "List individual short note entries, newest first.", readOnly,
		func(_ context.Context, in shortListReq) (shortsOut, error) {
			entries, err := notes.ShortList(c, notes.ShortQuery{
				Limit: in.Limit,
				Time:  notes.TimeFilter{Start: in.Start, End: in.End, Latest: in.Latest},
			})
			return shortsOut{Entries: entries}, err
		})
}
