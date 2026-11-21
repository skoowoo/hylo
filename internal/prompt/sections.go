package prompt

import "strings"

// Section IDs.
const (
	SectionGlobal       = "global"
	SectionVault        = "vault"
	SectionMain         = "main"
	SectionBot          = "bot"
	SectionMCP          = "mcp"
	SectionChoices      = "choices"
	SectionEmbed        = "embed"
	SectionCapabilities = "capabilities"
	SectionIntro        = "intro"
	SectionAgenda       = "agenda"
	SectionApps         = "apps"
)

// GlobalDefault is the global system prompt, from text/system/global.md.
var GlobalDefault = load("system/global")

var (
	vaultText        = load("system/vault")
	mcpText          = load("system/mcp")
	choicesText      = load("system/choices")
	embedText        = load("system/embed")
	capabilitiesText = load("system/capabilities")
	agendaText       = load("system/agenda")
	introTmpl        = parse("setup/intro")
	appsTmpl         = parse("system/apps")
)

// sections is the system prompt from the most general layer to the most
// situational one. A new layer is one entry here.
var sections = []Section{
	{ID: SectionGlobal, Applies: notOneShot, Text: func(Run) string { return GlobalDefault }},
	// Conventions skills and tools rely on: link syntax, where knowledge and memory live.
	{ID: SectionVault, Applies: notOneShot, Text: func(Run) string { return vaultText }},
	{ID: SectionMain, Applies: func(r Run) bool { return r.Main && notOneShot(r) }, Text: func(Run) string { return HyloBot }},
	{ID: SectionBot, Text: botText},
	{ID: SectionMCP, Applies: func(r Run) bool { return r.MCP && r.Kind != OneShot }, Text: func(Run) string { return mcpText }},
	// Only Hylo's chat turns a choices block into buttons; elsewhere it shows as raw text.
	{ID: SectionChoices, Applies: inConversation, Text: func(Run) string { return choicesText }},
	// Chat draws embed blocks as cards.
	{ID: SectionEmbed, Applies: inConversation, Text: func(Run) string { return embedText }},
	// A wake may start the session that later user requests resume.
	{ID: SectionCapabilities, Applies: func(r Run) bool { return inConversation(r) && r.Main && r.MCP && r.Intro == nil }, Text: func(Run) string { return capabilitiesText }},
	// Promises for later only hold while rounds run to wake the bot for them.
	{ID: SectionAgenda, Applies: func(r Run) bool { return r.Agenda && r.Main && r.MCP && r.Kind != OneShot }, Text: func(Run) string { return agendaText }},
	{ID: SectionApps, Applies: notOneShot, Text: appsText},
	{ID: SectionIntro, Applies: func(r Run) bool { return r.Kind == Chat && r.Intro != nil }, Text: func(r Run) string { return r.Intro.text() }},
}

func botText(r Run) string {
	if !r.Main {
		return r.Bot
	}
	t := strings.TrimSpace(r.Bot)
	if _, builtIn := SupersededMain[t]; builtIn || t == "" {
		return ""
	}
	return "Additional preferences for the main bot, within its core role and this turn's scope:\n\n" + t
}

func appsText(r Run) string {
	var available []App
	for _, a := range r.Apps {
		if r.MCP || a.Skill != "" {
			available = append(available, a)
		}
	}
	if len(available) == 0 {
		return ""
	}
	return execute(appsTmpl, available)
}

func notOneShot(r Run) bool { return r.Kind != OneShot }

func inConversation(r Run) bool { return r.Kind == Chat || r.Kind == Wake }

// Intro starts the hylo-setup skill on the first chat after onboarding. It
// carries only what the skill would find slowly or not at all.
type Intro struct {
	// Greeting is what the bot already said before the user wrote anything.
	Greeting string
	Overview string
	// Lang is the locale the onboarding hello picked; empty means follow the user.
	Lang string
}

func (i *Intro) text() string {
	d := *i
	d.Greeting = strings.TrimSpace(d.Greeting)
	return execute(introTmpl, d)
}
