package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type botChatReq struct {
	BotID  string `json:"bot_id" jsonschema:"bot id, from bot_list"`
	Before string `json:"before,omitempty" jsonschema:"message id: read the messages before it, to page back"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many messages, default 10, at most 50"`
}

type activityReq struct {
	Since string `json:"since,omitempty" jsonschema:"how far back: a duration like '6h' or '3d', or local 'YYYY-MM-DD HH:MM'; default a day"`
	Actor string `json:"actor,omitempty" jsonschema:"only this actor: 'user', or a bot id"`
	Limit int    `json:"limit,omitempty" jsonschema:"at most this many entries, newest first; default 100"`
}

type sinceReq struct {
	Since string `json:"since,omitempty" jsonschema:"from when: a duration like '6h' or '3d', or local 'YYYY-MM-DD HH:MM'; default today since midnight"`
}

type activityOut struct {
	Entries []client.ActivityEntry `json:"entries"`
}

// registerExploreTools let a bot look at what the user's other bots said and
// did, and at who read and wrote which notes.
func registerExploreTools(s *sdk.Server, c *client.Client) {
	addTool(s, "bot_chat", "Read a bot's current chat, newest messages last. Messages it sent on its own say what woke it, and whether the user has opened the chat since (unread).", readOnly,
		func(_ context.Context, in botChatReq) (client.BotChat, error) {
			out, err := c.BotChat(in.BotID, in.Before, in.Limit)
			if out.Messages == nil {
				out.Messages = []mate.ChatLine{}
			}
			return out, err
		})

	addTool(s, "activity", "List who read and wrote which notes, newest first: the user, or each bot. Reads count only real reading, not listing or searching.", readOnly,
		func(_ context.Context, in activityReq) (activityOut, error) {
			entries, err := c.Activity(in.Since, in.Actor, in.Limit)
			if entries == nil {
				entries = []client.ActivityEntry{}
			}
			return activityOut{Entries: entries}, err
		})

	addTool(s, "screen_time", "Where the user's time at the computer went, from the desktop app: whether they are there now and in which app, time per category and app, and the latest stretches. App names only. Use it to judge timing and what they're working on; don't comment on their habits unless asked.", readOnly,
		func(_ context.Context, in sinceReq) (desktop.ScreenTime, error) {
			return c.ScreenTime(in.Since)
		})
}
