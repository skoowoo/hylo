package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type sampleReq struct {
	Each      int `json:"each,omitempty" jsonschema:"how many to draw from each of raw notes, knowledge units, and short entries; default 2, max 8"`
	OlderThan int `json:"older_than,omitempty" jsonschema:"only things written at least this many days ago; 0 means 90, a negative value keeps the whole history. A kind with nothing that old is drawn from all of it"`
}

type sampleOut struct {
	Items []notes.SampleItem `json:"items"`
}

func registerSampleTool(s *sdk.Server, c *client.Client) {
	addTool(s, "sample", "Draw a random walk through raw notes, knowledge units and short entries, spread across when they were written and across knowledge domains; a unit is dated by its earliest source note.", readOnly,
		func(_ context.Context, in sampleReq) (sampleOut, error) {
			items, err := notes.Sample(c, notes.SampleQuery{Each: in.Each, OlderThan: in.OlderThan})
			return sampleOut{Items: items}, err
		})
}
