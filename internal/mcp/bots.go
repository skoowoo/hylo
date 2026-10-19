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

type botUpdateReq struct {
	BotID        string  `json:"bot_id" jsonschema:"bot id, from bot_list"`
	Name         string  `json:"name,omitempty" jsonschema:"new name; empty keeps it"`
	Description  *string `json:"description,omitempty" jsonschema:"new description; omit to keep it, empty string clears it"`
	SystemPrompt *string `json:"system_prompt,omitempty" jsonschema:"new system prompt, replacing the old one; omit to keep it"`
	Enabled      *bool   `json:"enabled,omitempty" jsonschema:"turn the bot on or off; omit to keep it"`
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

type skillCatalogReq struct {
	Category string `json:"category,omitempty" jsonschema:"only skills in this catalog category, e.g. news"`
}

type skillAddReq struct {
	Repo  string `json:"repo" jsonschema:"GitHub repository, as a URL or owner/repo"`
	Skill string `json:"skill,omitempty" jsonschema:"the skill's name, used to find it in the repository and to add it under; default the repository name"`
	Force bool   `json:"force,omitempty" jsonschema:"replace an added skill of the same name"`
}

type skillNameReq struct {
	Name string `json:"name" jsonschema:"skill name, from skills_list"`
}

type skillBotReq struct {
	Name  string `json:"name" jsonschema:"skill name, from skills_list"`
	BotID string `json:"bot_id" jsonschema:"bot id, from bot_list"`
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
	Cwd          string       `json:"cwd"`
	Main         bool         `json:"main,omitempty"`
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

type skillsOut struct {
	Skills []client.SkillInfo `json:"skills"`
}

type catalogOut struct {
	Skills []client.CatalogEntry `json:"skills"`
}

type nameOut struct {
	Name string `json:"name"`
}

func toBotOut(m *mate.Mate) botOut {
	out := botOut{ID: m.ID, Name: m.Name, Description: m.Description, Cwd: m.Cwd, Main: m.Main, SystemPrompt: m.SystemPrompt, Enabled: m.Enabled, Triggers: []triggerOut{}}
	for _, t := range m.Triggers {
		out.Triggers = append(out.Triggers, triggerOut{
			ID: t.ID, Name: t.Name, Events: t.EventTypes, Schedule: t.Schedule,
			PathPrefixes: t.PathPrefixes, Prompt: t.Prompt, Enabled: t.Enabled,
		})
	}
	return out
}

// registerBotTools exposes agent bots, their triggers and the skills they run.
func registerBotTools(s *sdk.Server, c *client.Client) {
	addTool(s, "bot_list", "List agent bots with how many triggers each has.", readOnly,
		func(_ context.Context, _ noReq) (botsOut, error) {
			bots, err := c.Bots()
			out := botsOut{Bots: []botSummary{}}
			for _, m := range bots {
				out.Bots = append(out.Bots, botSummary{ID: m.ID, Name: m.Name, Description: m.Description, Enabled: m.Enabled, TriggerCount: m.TriggerCount})
			}
			return out, err
		})

	addTool(s, "bot_get", "Show a bot with its triggers.", readOnly,
		func(_ context.Context, in botIDReq) (botOut, error) {
			m, err := c.Bot(in.BotID)
			if err != nil {
				return botOut{}, err
			}
			return toBotOut(m), nil
		})

	addTool(s, "bot_create", "Create an agent bot. It runs on the same agent and model as the user's existing bots, in a vault folder of its own; give it skills with skills_enable and triggers with trigger_add. Ask the user first.", additive,
		func(_ context.Context, in botCreateReq) (botOut, error) {
			m, err := notes.BotCreate(c, in.Name, in.Description, in.SystemPrompt)
			if err != nil {
				return botOut{}, err
			}
			return toBotOut(m), nil
		})

	addTool(s, "bot_update", "Change a bot's name, description or system prompt, or turn it on or off. Its triggers and skills are changed with their own tools. Not for the main bot. Ask the user first.", destructiveIdempotent,
		func(_ context.Context, in botUpdateReq) (botOut, error) {
			m, err := notes.BotUpdate(c, in.BotID, notes.BotChange{
				Name: in.Name, Description: in.Description, SystemPrompt: in.SystemPrompt, Enabled: in.Enabled,
			})
			if err != nil {
				return botOut{}, err
			}
			return toBotOut(m), nil
		})

	addTool(s, "bot_delete", "Delete a bot with its triggers and chats; its folder in the vault stays. There is no undo. Not for the main bot. Ask the user first.", destructiveIdempotent,
		func(_ context.Context, in botIDReq) (okOut, error) {
			err := notes.BotDelete(c, in.BotID)
			return okOut{OK: err == nil}, err
		})

	addTool(s, "trigger_add", "Add a trigger to a bot: give a schedule to run it on a timer, or events to run it when they happen. Its output goes to the user's inbox. Ask the user first.", additive,
		func(_ context.Context, in triggerAddReq) (triggerIDOut, error) {
			id, err := notes.TriggerAdd(c, in.BotID, notes.TriggerSpec{
				Name: in.Name, Prompt: in.Prompt, Schedule: in.Schedule, Events: in.Events, PathPrefixes: in.PathPrefixes,
			})
			return triggerIDOut{TriggerID: id}, err
		})

	addTool(s, "trigger_update", "Change a trigger's label, prompt or schedule, or turn it on or off. Ask the user first.", destructiveIdempotent,
		func(_ context.Context, in triggerUpdateReq) (okOut, error) {
			err := notes.TriggerUpdate(c, in.BotID, in.TriggerID, notes.TriggerChange{
				Name: in.Name, Prompt: in.Prompt, Schedule: in.Schedule, Enabled: in.Enabled,
			})
			return okOut{OK: err == nil}, err
		})

	addTool(s, "trigger_delete", "Remove a trigger from a bot; there is no undo. Ask the user first.", destructiveIdempotent,
		func(_ context.Context, in triggerReq) (okOut, error) {
			err := notes.TriggerDelete(c, in.BotID, in.TriggerID)
			return okOut{OK: err == nil}, err
		})

	addTool(s, "trigger_run", "Run a scheduled trigger once now, in the background; the result goes to the user's inbox.", additive,
		func(_ context.Context, in triggerReq) (okOut, error) {
			if err := c.RunTrigger(in.BotID, in.TriggerID); err != nil {
				return okOut{}, err
			}
			return okOut{OK: true, Message: "started; the result arrives in the inbox when it's done"}, nil
		})

	addTool(s, "skills_list", "List the skills added to ~/.hylo/skills and the bots each is on for. Built-in skills are always on for the main bot.", readOnly,
		func(_ context.Context, _ noReq) (skillsOut, error) {
			list, err := c.SkillsList()
			if list == nil {
				list = []client.SkillInfo{}
			}
			return skillsOut{Skills: list}, err
		})

	addTool(s, "skills_catalog", "List the skills Hylo recommends, each with the repo and the skill name that skills_add takes, and whether it is already added.", readOnly,
		func(_ context.Context, in skillCatalogReq) (catalogOut, error) {
			list, err := notes.SkillsCatalog(c, in.Category)
			return catalogOut{Skills: list}, err
		})

	addTool(s, "skills_add", "Download a skill from a GitHub repository into ~/.hylo/skills, on for no bot yet; turn it on for one with skills_enable. "+
		"Fails if a skill of that name is already added, unless force is set, which replaces it. Ask the user first.", destructiveIdempotent,
		func(_ context.Context, in skillAddReq) (nameOut, error) {
			name, err := c.SkillsAdd(in.Repo, in.Skill, in.Force)
			return nameOut{Name: name}, err
		})

	addTool(s, "skills_enable", "Turn an added skill on for a bot, so its runs from now on can use it. Ask the user first.", additiveIdempotent,
		func(_ context.Context, in skillBotReq) (okOut, error) {
			err := c.SkillsEnable(in.Name, in.BotID)
			return okOut{OK: err == nil}, err
		})

	addTool(s, "skills_disable", "Turn a skill off for a bot; its files stay in ~/.hylo/skills.", destructiveIdempotent,
		func(_ context.Context, in skillBotReq) (okOut, error) {
			err := c.SkillsDisable(in.Name, in.BotID)
			return okOut{OK: err == nil}, err
		})

	addTool(s, "skills_remove", "Delete a skill from ~/.hylo/skills and turn it off for every bot; there is no undo. Ask the user first.", destructiveIdempotent,
		func(_ context.Context, in skillNameReq) (okOut, error) {
			err := c.SkillsRemove(in.Name)
			return okOut{OK: err == nil}, err
		})
}
