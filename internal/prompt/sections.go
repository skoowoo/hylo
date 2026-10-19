package prompt

import "strings"

// Section IDs, also the keys of Run.Overrides.
const (
	SectionGlobal       = "global"
	SectionVault        = "vault"
	SectionBot          = "bot"
	SectionMCP          = "mcp"
	SectionChoices      = "choices"
	SectionCapabilities = "capabilities"
	SectionIntro        = "intro"
)

// GlobalDefault is the built-in global prompt, used until the user sets
// agent.system_prompt.
var GlobalDefault = load("global")

var (
	vaultText        = load("vault")
	mcpText          = load("mcp")
	choicesText      = load("choices")
	capabilitiesText = load("capabilities")
	introTmpl        = parse("intro")
)

// sections is the system prompt from the most general layer to the most
// situational one. A new layer is one entry here.
var sections = []Section{
	{ID: SectionGlobal, Applies: notOneShot, Text: func(Run) string { return GlobalDefault }},
	// What skills and tools rely on (link syntax, where knowledge and memory live), so the
	// user's global prompt doesn't replace it.
	{ID: SectionVault, Applies: notOneShot, Text: func(Run) string { return vaultText }},
	{ID: SectionBot, Text: func(r Run) string { return r.Bot }},
	{ID: SectionMCP, Applies: func(r Run) bool { return r.MCP && r.Kind != OneShot }, Text: func(Run) string { return mcpText }},
	// Only Hylo's chat turns a choices block into buttons; elsewhere it shows as raw text.
	{ID: SectionChoices, Applies: func(r Run) bool { return r.Kind == Chat }, Text: func(Run) string { return choicesText }},
	// Routing requests up to skills and automations needs the bot tools and the choices buttons.
	{ID: SectionCapabilities, Applies: func(r Run) bool { return r.Kind == Chat && r.Main && r.MCP && r.Intro == nil }, Text: func(Run) string { return capabilitiesText }},
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
