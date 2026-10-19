package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerCapabilitiesTool(s *sdk.Server, c *client.Client) {
	addTool(s, "capabilities", "Show what the user's automation can do in one call: each bot with its skills and triggers, catalog skills not added yet, skills added but on for no bot, and the events a trigger can fire on. Built-in skills are always on for the main bot and aren't listed. "+
		"Use it before proposing or building an automation; trigger prompts come from bot_get.", readOnly,
		func(_ context.Context, _ noReq) (notes.Capabilities, error) { return notes.GetCapabilities(c) })
}
