package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type sampleReq struct {
	Each         int      `json:"each,omitempty" jsonschema:"how many to draw from each of raw notes, knowledge units, and short entries; default 2, max 8"`
	OlderThan    int      `json:"older_than,omitempty" jsonschema:"only things written at least this many days ago; 0 means 90, a negative value keeps the whole history. A kind with nothing that old is drawn from all of it"`
	ExcludePaths []string `json:"exclude_paths,omitempty" jsonschema:"vault-absolute paths to omit, including recently shown notes and already inspected candidates; excluding a daily short note omits all entries that day"`
}

type sampleOut struct {
	Items []notes.SampleItem `json:"items"`
}

func registerSampleTool(s *sdk.Server, c *client.Client) {
	addTool(s, "sample", "Draw random candidates from raw notes, knowledge units and short entries using balanced population bands across history. Use each=6, older_than=-1 for a broad recall candidate pool, and exclude_paths for recent repeats. Results are candidates to curate, not a required reading list; dates may reflect import time.", readOnly,
		func(_ context.Context, in sampleReq) (sampleOut, error) {
			items, err := notes.Sample(c, notes.SampleQuery{Each: in.Each, OlderThan: in.OlderThan, ExcludePaths: in.ExcludePaths})
			return sampleOut{Items: items}, err
		})
}
