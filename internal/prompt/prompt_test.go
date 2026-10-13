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

func TestMCPMentionsMapping(t *testing.T) {
	for _, want := range []string{"extract_section", "knowledge_list_indexes", "--help", "Fall back"} {
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

func TestHelloMatchesTheVault(t *testing.T) {
	empty := HelloMessage(Hello{Vault: "Hylo Vault", Lang: "zh-CN", Now: "Monday 21:40"})
	if !strings.Contains(empty, "brand new and empty") || !strings.Contains(empty, HelloEmptyAngles[0]) || !strings.Contains(empty, "zh-CN") {
		t.Fatal(empty)
	}
	full := HelloMessage(Hello{Vault: "Notes", Notes: 12, Folders: []string{"Work"}, Recent: []string{"Q3 plan"}, Seed: 1})
	if !strings.Contains(full, `holds 12 notes, mostly in "Work"`) || !strings.Contains(full, `"Q3 plan"`) || !strings.Contains(full, HelloNotesAngles[1]) || !strings.Contains(full, "locale English") {
		t.Fatal(full)
	}
	if p := HelloMessage(Hello{Vault: "Notes", Notes: 3}); strings.Contains(p, "Most recently edited") {
		t.Fatal(p)
	}
}

func TestFillIsSinglePass(t *testing.T) {
	got := Fill("{Name}: {Content} {Missing}", map[string]string{"Name": "a", "Content": "says {Name}"})
	if got != "a: says {Name} {Missing}" {
		t.Fatal(got)
	}
}
