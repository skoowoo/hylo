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
	Dir     string `json:"dir,omitempty" jsonschema:"shorts directory override, default _shorts"`
}

type shortListReq struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"max entries, 0 = no limit"`
	Start  string `json:"start,omitempty" jsonschema:"only entries created on or after this date (YYYY-MM-DD)"`
	End    string `json:"end,omitempty" jsonschema:"only entries created before this date (YYYY-MM-DD)"`
	Latest int    `json:"latest,omitempty" jsonschema:"only entries created within the last N days; cannot combine with start/end"`
	Dir    string `json:"dir,omitempty" jsonschema:"shorts directory, default _shorts"`
}

type tagsOut struct {
	Tags []client.TagStat `json:"tags"`
}

type shortsOut struct {
	Entries []client.ShortEntry `json:"entries"`
}

func registerTagTools(s *sdk.Server, c *client.Client) {
	addTool(s, "tag_list", "Show each tag and how many notes use it.", true,
		func(_ context.Context, in tagListReq) (tagsOut, error) {
			tags, err := notes.TagList(c, in.Limit)
			return tagsOut{Tags: tags}, err
		})

	addTool(s, "tag_count", "Count notes that have a tag.", true,
		func(_ context.Context, in tagReq) (*client.TagCountResponse, error) {
			return c.TagCount(in.Tag)
		})

	addTool(s, "tag_delete", "Remove all notes with this tag from the search index; note files on disk are untouched.", false,
		func(_ context.Context, in tagReq) (*client.TagDeleteResponse, error) {
			resp, err := c.TagDelete(in.Tag)
			if err != nil {
				return nil, err
			}
			if resp.Paths == nil {
				resp.Paths = []string{}
			}
			return resp, nil
		})
}

func registerShortTools(s *sdk.Server, c *client.Client) {
	addTool(s, "short_create", "Save a short note into today's shorts file.", false,
		func(_ context.Context, in shortCreateReq) (pathOut, error) {
			p, err := notes.ShortCreate(c, in.Content, in.Dir)
			return pathOut{Path: p}, err
		})

	addTool(s, "short_list", "List individual short note entries, newest first.", true,
		func(_ context.Context, in shortListReq) (shortsOut, error) {
			entries, err := notes.ShortList(c, notes.ShortQuery{
				Dir:   in.Dir,
				Limit: in.Limit,
				Time:  notes.TimeFilter{Start: in.Start, End: in.End, Latest: in.Latest},
			})
			return shortsOut{Entries: entries}, err
		})
}
