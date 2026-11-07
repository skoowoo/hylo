package prompt

import "strings"

// Section IDs.
const (
	SectionGlobal       = "global"
	SectionVault        = "vault"
	SectionBot          = "bot"
	SectionMCP          = "mcp"
	SectionChoices      = "choices"
	SectionEmbed        = "embed"
	SectionCapabilities = "capabilities"
	SectionIntro        = "intro"
	SectionAgenda       = "agenda"
)

// GlobalDefault is the global system prompt, from text/global.md.
var GlobalDefault = load("global")

var (
	vaultText        = load("vault")
	mcpText          = load("mcp")
	choicesText      = load("choices")
	embedText        = load("embed")
	capabilitiesText = load("capabilities")
	agendaText       = load("agenda")
	introTmpl        = parse("intro")
)

// sections is the system prompt from the most general layer to the most
// situational one. A new layer is one entry here.
var sections = []Section{
	{ID: SectionGlobal, Applies: notOneShot, Text: func(Run) string { return GlobalDefault }},
	// Conventions skills and tools rely on: link syntax, where knowledge and memory live.
	{ID: SectionVault, Applies: notOneShot, Text: func(Run) string { return vaultText }},
	{ID: SectionBot, Text: func(r Run) string { return r.Bot }},
	{ID: SectionMCP, Applies: func(r Run) bool { return r.MCP && r.Kind != OneShot }, Text: func(Run) string { return mcpText }},
	// Only Hylo's chat turns a choices block into buttons; elsewhere it shows as raw text.
	{ID: SectionChoices, Applies: func(r Run) bool { return r.Kind == Chat }, Text: func(Run) string { return choicesText }},
	// Chat draws embed blocks as cards.
	{ID: SectionEmbed, Applies: notOneShot, Text: func(Run) string { return embedText }},
	// Routing requests up to skills and automations needs the bot tools and the choices buttons.
	{ID: SectionCapabilities, Applies: func(r Run) bool { return r.Kind == Chat && r.Main && r.MCP && r.Intro == nil }, Text: func(Run) string { return capabilitiesText }},
	// Promises for later only hold while rounds run to wake the bot for them.
	{ID: SectionAgenda, Applies: func(r Run) bool { return r.Agenda && r.Main && r.MCP && r.Kind != OneShot }, Text: func(Run) string { return agendaText }},
	{ID: SectionIntro, Applies: func(r Run) bool { return r.Intro != nil }, Text: func(r Run) string { return r.Intro.text() }},
}

func notOneShot(r Run) bool { return r.Kind != OneShot }

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
