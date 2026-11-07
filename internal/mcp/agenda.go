package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/mate"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type agendaAddReq struct {
	Text string `json:"text" jsonschema:"what you mean to do or follow up on, enough to act on it later without this chat"`
	Due  string `json:"due,omitempty" jsonschema:"when to be woken for it, local time 'YYYY-MM-DD HH:MM'; omit for no set time"`
}

type agendaDoneReq struct {
	ID string `json:"id" jsonschema:"item id, from agenda_list"`
}

type agendaOut struct {
	Items []mate.AgendaItem `json:"items"`
}

// registerAgendaTools gives the main bot an agenda of its own: promises and
// follow-ups that its rounds (internal/pulse) look at, and wake it for when due.
func registerAgendaTools(s *sdk.Server, c *client.Client) {
	addTool(s, "agenda_list", "List your open agenda items: what you mean to do or follow up on.", readOnly,
		func(ctx context.Context, _ noReq) (agendaOut, error) {
			items, err := c.Agenda(runFrom(ctx))
			if items == nil {
				items = []mate.AgendaItem{}
			}
			return agendaOut{Items: items}, err
		})

	addTool(s, "agenda_add", "Add something you mean to do or follow up on, like a reminder you promised the user or a thread to pick up later. With a due time you are woken for it then; without one you see it each time you look around.", additive,
		func(ctx context.Context, in agendaAddReq) (mate.AgendaItem, error) {
			return c.AgendaAdd(runFrom(ctx), in.Text, in.Due)
		})

	addTool(s, "agenda_done", "Close an agenda item once it is done or no longer needed.", destructiveIdempotent,
		func(ctx context.Context, in agendaDoneReq) (okOut, error) {
			err := c.AgendaDone(runFrom(ctx), in.ID)
			return okOut{OK: err == nil}, err
		})
}
