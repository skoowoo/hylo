package prompt

import (
	"strings"
	"testing"
)

func ids(r Run) string {
	var out []string
	for _, p := range Parts(r) {
		out = append(out, p.ID)
	}
	return strings.Join(out, ",")
}

func TestSectionsApplyByRun(t *testing.T) {
	cases := []struct {
		run  Run
		want string
	}{
		{Run{}, "global,choices"},
		{Run{Kind: Trigger}, "global"},
		{Run{Kind: Trigger, Bot: " ", MCP: true}, "global,mcp"},
		{Run{Bot: "be brief", MCP: true, Intro: &Intro{}}, "global,bot,mcp,choices,intro"},
		{Run{Kind: OneShot, MCP: true}, ""},
	}
	for _, c := range cases {
		if got := ids(c.run); got != c.want {
			t.Errorf("%+v: %s, want %s", c.run, got, c.want)
		}
	}
}

func TestOverrideReplacesGlobal(t *testing.T) {
	got := System(Run{Kind: Trigger, Bot: "bot", Overrides: map[string]string{SectionGlobal: " mine "}})
	if got != "mine\n\n---\n\nbot" {
		t.Fatalf("%q", got)
	}
	if got := System(Run{Kind: Trigger, Overrides: map[string]string{SectionGlobal: "  "}}); got != GlobalDefault {
		t.Fatal("blank override must fall back to the default")
	}
}

func TestMessageResumeSendsOnlyTheRequest(t *testing.T) {
	full := Message("sys", "hi", "/vault", false)
	if !strings.HasPrefix(full, "# Instructions\n\nsys") || !strings.Contains(full, "(vault): /vault") || !strings.HasSuffix(full, "# User request\n\nhi") {
		t.Fatal(full)
	}
	if strings.Contains(Message("sys", "hi", "", false), "working directory") {
		t.Fatal("a bot's own cwd is not named")
	}
	if got := Message("sys", "hi", "/vault", true); got != "hi" {
		t.Fatal(got)
	}
}

// The example has to be a block the chat turns into buttons.
func TestChoicesShowsAWorkingBlock(t *testing.T) {
	if !strings.Contains(choicesText, "```choices\n- ") || !strings.Contains(choicesText, "- [ ] option") {
		t.Fatal(choicesText)
	}
}

func TestMCPHintFindsTheTools(t *testing.T) {
	for _, want := range []string{"mcp__hylo__", "deferred"} {
		if !strings.Contains(mcpText, want) {
			t.Errorf("hint missing %q", want)
		}
	}
}

func TestIntroQuotesTheGreeting(t *testing.T) {
	p := (&Intro{Greeting: " Your Q3 plan keeps asking who it's for.\n", Overview: "- 4 notes"}).text()
	if !strings.Contains(p, `"Your Q3 plan keeps asking who it's for."`) || !strings.Contains(p, "hylo-setup") {
		t.Fatalf("prompt misses the greeting or the skill: %s", p)
	}
	if strings.Contains((&Intro{}).text(), "already opened") {
		t.Fatal("no greeting, nothing to quote")
	}
}

func TestIntroKeepsTheHelloLanguage(t *testing.T) {
	if p := (&Intro{Lang: "zh-Hans-CN"}).text(); !strings.Contains(p, "locale zh-Hans-CN") || strings.Contains(p, "language the user writes in") {
		t.Fatal(p)
	}
	if p := (&Intro{}).text(); !strings.Contains(p, "language the user writes in") {
		t.Fatal(p)
	}
}

func TestHyloBotExplainsTheChatTags(t *testing.T) {
	if !strings.Contains(HyloBot, WechatTag) || !strings.Contains(HyloBot, DiscordTag) {
		t.Fatal(HyloBot)
	}
	if !strings.HasPrefix(TriggerWechat, WechatTag) || !strings.HasPrefix(TriggerDiscord, DiscordTag) {
		t.Fatal("chat triggers must open with their tag")
	}
}

func TestHelloSceneFollowsTheUser(t *testing.T) {
	obsidian := []HelloVault{{Name: "Work", LastOpened: "opened yesterday"}}
	for _, c := range []struct {
		h    Hello
		want string
	}{
		{Hello{Notes: 3, OtherVaults: obsidian}, HelloSeen},
		{Hello{OtherVaults: obsidian, NoteApps: []string{"Bear"}}, HelloObsidian},
		{Hello{NoteApps: []string{"Bear"}}, HelloOtherApp},
		{Hello{}, HelloFresh},
	} {
		if got := c.h.Scene(); got != c.want {
			t.Errorf("%+v: scene %q, want %q", c.h, got, c.want)
		}
	}
}

func TestHelloMatchesTheVault(t *testing.T) {
	empty := HelloMessage(Hello{Vault: "Hylo Vault", Lang: "zh-CN", Now: "Monday 21:40"})
	if !strings.Contains(empty, "brand new and empty") || !strings.Contains(empty, "first note that fits their time of day") || !strings.Contains(empty, "zh-CN") {
		t.Fatal(empty)
	}
	full := HelloMessage(Hello{Vault: "Notes", Notes: 12, Obsidian: true, Folders: []string{"Work"},
		Recent:      []HelloNote{{Title: "Q3 plan", Excerpt: "Who is the first user?", Age: "today"}},
		Older:       []HelloNote{{Title: "Pricing", Age: "5 months ago"}},
		WritingTime: "late at night"})
	for _, want := range []string{`holds 12 notes; it is also their Obsidian vault, mostly in "Work"`, `"Q3 plan", today: "Who is the first user?"`, `"Pricing", 5 months ago`, "mostly write late at night", "only a reader of these notes", "note titles are mostly written in"} {
		if !strings.Contains(full, want) {
			t.Fatalf("missing %q in:\n%s", want, full)
		}
	}
	if p := HelloMessage(Hello{Vault: "Notes", Notes: 3}); strings.Contains(p, "Most recently edited") || strings.Contains(p, "older notes") {
		t.Fatal(p)
	}
	vault := HelloMessage(Hello{Vault: "Hylo Vault", OtherVaults: []HelloVault{{Name: "Work", LastOpened: "opened yesterday"}, {Name: "Old"}}})
	if !strings.Contains(vault, `didn't give Hylo: "Work" (opened yesterday), "Old".`) || !strings.Contains(vault, "Obsidian keeps working on it") || !strings.Contains(vault, "locale English") {
		t.Fatal(vault)
	}
	app := HelloMessage(Hello{Vault: "Hylo Vault", NoteApps: []string{"Bear", "Notion"}})
	if !strings.Contains(app, "installed on this computer: Bear, Notion.") || !strings.Contains(app, "export to Markdown") {
		t.Fatal(app)
	}
}

func TestFillIsSinglePass(t *testing.T) {
	got := Fill("{Name}: {Content} {Missing}", map[string]string{"Name": "a", "Content": "says {Name}"})
	if got != "a: says {Name} {Missing}" {
		t.Fatal(got)
	}
}
