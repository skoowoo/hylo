package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type botIDReq struct {
	BotID string `json:"bot_id" jsonschema:"bot id, from bot_list"`
}

type botCreateReq struct {
	Name         string `json:"name" jsonschema:"bot name shown in the bot list"`
	Description  string `json:"description,omitempty" jsonschema:"one line on what the bot does"`
	SystemPrompt string `json:"system_prompt,omitempty" jsonschema:"standing instructions for every run of this bot"`
}

type triggerAddReq struct {
	BotID        string   `json:"bot_id" jsonschema:"bot id, from bot_list"`
	Name         string   `json:"name,omitempty" jsonschema:"short label for the trigger"`
	Prompt       string   `json:"prompt" jsonschema:"what the bot is asked to do each time the trigger fires"`
	Schedule     string   `json:"schedule,omitempty" jsonschema:"for a scheduled trigger: 'every 1h', 'daily 08:00' or 'weekly mon,wed 09:00', server local time"`
	Events       []string `json:"events,omitempty" jsonschema:"event types to fire on instead of a schedule, e.g. note_created, short_note_created"`
	PathPrefixes []string `json:"path_prefixes,omitempty" jsonschema:"for note events: only fire for notes under these folders, e.g. /Web Clips/"`
}

type triggerUpdateReq struct {
	BotID     string `json:"bot_id" jsonschema:"bot id, from bot_list"`
	TriggerID string `json:"trigger_id" jsonschema:"trigger id, from bot_get"`
	Name      string `json:"name,omitempty" jsonschema:"new label; empty keeps it"`
	Prompt    string `json:"prompt,omitempty" jsonschema:"new prompt; empty keeps it"`
	Schedule  string `json:"schedule,omitempty" jsonschema:"new schedule for a scheduled trigger; empty keeps it"`
	Enabled   *bool  `json:"enabled,omitempty" jsonschema:"turn the trigger on or off; omit to keep it"`
}

type triggerReq struct {
	BotID     string `json:"bot_id" jsonschema:"bot id, from bot_list"`
	TriggerID string `json:"trigger_id" jsonschema:"trigger id, from bot_get"`
}

type skillListReq struct {
	Category string `json:"category,omitempty" jsonschema:"only skills in this catalog category, e.g. news"`
}

type skillReq struct {
	Name string `json:"name" jsonschema:"skill name, from skills_list"`
}

type botSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Enabled      bool   `json:"enabled"`
	TriggerCount int    `json:"trigger_count"`
}

type triggerOut struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Events       []string `json:"events"`
	Schedule     string   `json:"schedule,omitempty"`
	PathPrefixes []string `json:"path_prefixes,omitempty"`
	Prompt       string   `json:"prompt"`
	Enabled      bool     `json:"enabled"`
}

type botOut struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description,omitempty"`
	SystemPrompt string       `json:"system_prompt,omitempty"`
	Enabled      bool         `json:"enabled"`
	Triggers     []triggerOut `json:"triggers"`
}

type botsOut struct {
	Bots []botSummary `json:"bots"`
}

type triggerIDOut struct {
	TriggerID string `json:"trigger_id"`
}

type okOut struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type skillOut struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Installed   bool   `json:"installed"`
	Enabled     bool   `json:"enabled"`
}

type skillsOut struct {
	Skills []skillOut `json:"skills"`
}

func toBotOut(m *mate.Mate) botOut {
	out := botOut{ID: m.ID, Name: m.Name, Description: m.Description, SystemPrompt: m.SystemPrompt, Enabled: m.Enabled, Triggers: []triggerOut{}}
	for _, t := range m.Triggers {
		out.Triggers = append(out.Triggers, triggerOut{
			ID: t.ID, Name: t.Name, Events: t.EventTypes, Schedule: t.Schedule,
			PathPrefixes: t.PathPrefixes, Prompt: t.Prompt, Enabled: t.Enabled,
		})
	}
	return out
}

// registerBotTools exposes agent bots, their triggers and the skills they run.
// Every change goes through gate, so only the user can make it.
func registerBotTools(s *sdk.Server, c *client.Client, gate func(run string) error) {
	allowed := func(ctx context.Context) error {
		if gate == nil {
			return nil
		}
		return gate(runFrom(ctx))
	}

	addTool(s, "bot_list", "List agent bots with how many triggers each has.", true,
		func(_ context.Context, _ noReq) (botsOut, error) {
			bots, err := c.Bots()
			out := botsOut{Bots: []botSummary{}}
			for _, m := range bots {
				out.Bots = append(out.Bots, botSummary{ID: m.ID, Name: m.Name, Description: m.Description, Enabled: m.Enabled, TriggerCount: m.TriggerCount})
			}
			return out, err
		})

	addTool(s, "bot_get", "Show a bot with its triggers.", true,
		func(_ context.Context, in botIDReq) (botOut, error) {
			m, err := c.Bot(in.BotID)
			if err != nil {
				return botOut{}, err
			}
			return toBotOut(m), nil
		})

	addTool(s, "bot_create", "Create an agent bot. It runs on the same agent and model as the user's existing bots; add triggers with trigger_add. Ask the user first.", false,
		func(ctx context.Context, in botCreateReq) (botOut, error) {
			if err := allowed(ctx); err != nil {
				return botOut{}, err
			}
			m, err := notes.BotCreate(c, in.Name, in.Description, in.SystemPrompt)
			if err != nil {
				return botOut{}, err
			}
			return toBotOut(m), nil
		})

	addTool(s, "trigger_add", "Add a trigger to a bot: give a schedule to run it on a timer, or events to run it when they happen. Its output goes to the user's inbox. Ask the user first.", false,
		func(ctx context.Context, in triggerAddReq) (triggerIDOut, error) {
			if err := allowed(ctx); err != nil {
				return triggerIDOut{}, err
			}
			id, err := notes.TriggerAdd(c, in.BotID, notes.TriggerSpec{
				Name: in.Name, Prompt: in.Prompt, Schedule: in.Schedule, Events: in.Events, PathPrefixes: in.PathPrefixes,
			})
			return triggerIDOut{TriggerID: id}, err
		})

	addTool(s, "trigger_update", "Change a trigger's label, prompt or schedule, or turn it on or off.", false,
		func(ctx context.Context, in triggerUpdateReq) (okOut, error) {
			if err := allowed(ctx); err != nil {
				return okOut{}, err
			}
			err := notes.TriggerUpdate(c, in.BotID, in.TriggerID, notes.TriggerChange{
				Name: in.Name, Prompt: in.Prompt, Schedule: in.Schedule, Enabled: in.Enabled,
			})
			return okOut{OK: err == nil}, err
		})

	addTool(s, "trigger_delete", "Remove a trigger from a bot.", false,
		func(ctx context.Context, in triggerReq) (okOut, error) {
			if err := allowed(ctx); err != nil {
				return okOut{}, err
			}
			err := notes.TriggerDelete(c, in.BotID, in.TriggerID)
			return okOut{OK: err == nil}, err
		})

	addTool(s, "trigger_run", "Run a scheduled trigger once now, in the background; the result goes to the user's inbox.", false,
		func(ctx context.Context, in triggerReq) (okOut, error) {
			if err := allowed(ctx); err != nil {
				return okOut{}, err
			}
			if err := c.RunTrigger(in.BotID, in.TriggerID); err != nil {
				return okOut{}, err
			}
			return okOut{OK: true, Message: "started; the result arrives in the inbox when it's done"}, nil
		})

	addTool(s, "skills_list", "List skills: installed ones, and ones in Hylo's catalog that can be installed.", true,
		func(_ context.Context, in skillListReq) (skillsOut, error) {
			list, err := notes.SkillsList(c, in.Category)
			out := skillsOut{Skills: []skillOut{}}
			for _, sk := range list {
				out.Skills = append(out.Skills, skillOut{Name: sk.Name, Description: sk.Description, Category: sk.Category, Installed: sk.Installed, Enabled: sk.Enabled})
			}
			return out, err
		})

	addTool(s, "skills_install", "Install a skill from Hylo's catalog and turn it on, or turn on one already installed. Agents started from now on can use it.", false,
		func(ctx context.Context, in skillReq) (okOut, error) {
			if err := allowed(ctx); err != nil {
				return okOut{}, err
			}
			msg, err := notes.SkillsInstall(c, in.Name)
			return okOut{OK: err == nil, Message: msg}, err
		})
}
