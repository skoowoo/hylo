package agent

import (
	"strings"
)

// supportsNativeSession lists agents whose CLI or wire protocol can resume
// multi-turn context by session id without replaying host message history.
// Mechanisms:
//   - CLI flags: claude, codex, opencode, cursor-agent, copilot
//   - ACP session/load|resume|prompt: hermes
//   - Pi RPC --session-id (open or create): pi
var supportsNativeSession = map[string]struct{}{
	"claude":       {},
	"codex":        {},
	"opencode":     {},
	"hermes":       {},
	"cursor-agent": {},
	"copilot":      {},
	"pi":           {},
}

// All agent definitions (order matches Open Design AGENT_DEFS).
func definitions() []*AgentDef {
	reasonCodex := []ReasoningOption{
		{ID: "default", Label: "Default"},
		{ID: "none", Label: "None"},
		{ID: "minimal", Label: "Minimal"},
		{ID: "low", Label: "Low"},
		{ID: "medium", Label: "Medium"},
		{ID: "high", Label: "High"},
		{ID: "xhigh", Label: "XHigh"},
	}
	reasonPi := []ReasoningOption{
		{ID: "default", Label: "Default"},
		{ID: "off", Label: "Off"},
		{ID: "minimal", Label: "Minimal"},
		{ID: "low", Label: "Low"},
		{ID: "medium", Label: "Medium"},
		{ID: "high", Label: "High"},
		{ID: "xhigh", Label: "XHigh"},
		{ID: "max", Label: "Max"},
	}
	defs := []*AgentDef{
		{
			ID: "claude", Name: "Claude Code", Bin: "claude",
			FallbackBins:    []string{"openclaude"},
			VersionArgs:     []string{"--version"},
			HelpArgs:        []string{"-p", "--help"},
			CapabilityFlags: map[string]string{"--include-partial-messages": "partialMessages", "--add-dir": "addDir"},
			FallbackModels: []ModelOption{
				DefaultModelOption,
				{ID: "sonnet", Label: "Sonnet (alias)"},
				{ID: "opus", Label: "Opus (alias)"},
				{ID: "haiku", Label: "Haiku (alias)"},
				{ID: "claude-opus-4-5", Label: "claude-opus-4-5"},
				{ID: "claude-sonnet-4-5", Label: "claude-sonnet-4-5"},
				{ID: "claude-haiku-4-5", Label: "claude-haiku-4-5"},
			},
			StreamFormat: StreamClaudeJSON, PromptViaStdin: true,
			build: buildClaude,
		},
		{
			ID: "codex", Name: "Codex CLI", Bin: "codex",
			VersionArgs: []string{"--version"},
			FallbackModels: []ModelOption{
				DefaultModelOption,
				{ID: "gpt-5.5", Label: "gpt-5.5"},
				{ID: "gpt-5.4", Label: "gpt-5.4"},
				{ID: "gpt-5.4-mini", Label: "gpt-5.4-mini"},
				{ID: "gpt-5.3-codex", Label: "gpt-5.3-codex"},
				{ID: "gpt-5-codex", Label: "gpt-5-codex"},
				{ID: "gpt-5", Label: "gpt-5"},
				{ID: "o3", Label: "o3"},
				{ID: "o4-mini", Label: "o4-mini"},
			},
			ReasoningOptions: reasonCodex,
			StreamFormat:     StreamJSONEvent, EventParser: "codex", PromptViaStdin: true,
			build: buildCodex,
		},
		{
			ID: "opencode", Name: "OpenCode", Bin: "opencode",
			VersionArgs:    []string{"--version"},
			listModelsArgs: []string{"models"}, listModelsTimeout: 8000,
			listModelsParse: parseLineSeparatedModels,
			FallbackModels: []ModelOption{
				DefaultModelOption,
				{ID: "anthropic/claude-sonnet-4-5", Label: "anthropic/claude-sonnet-4-5"},
				{ID: "openai/gpt-5", Label: "openai/gpt-5"},
				{ID: "google/gemini-2.5-pro", Label: "google/gemini-2.5-pro"},
			},
			StreamFormat: StreamJSONEvent, EventParser: "opencode", PromptViaStdin: true,
			build: buildOpenCode,
		},
		{
			ID: "hermes", Name: "Hermes", Bin: "hermes",
			VersionArgs: []string{"--version"},
			FallbackModels: []ModelOption{
				DefaultModelOption,
				{ID: "openai-codex:gpt-5.5", Label: "gpt-5.5 (openai-codex:gpt-5.5)"},
				{ID: "openai-codex:gpt-5.4", Label: "gpt-5.4 (openai-codex:gpt-5.4)"},
				{ID: "openai-codex:gpt-5.4-mini", Label: "gpt-5.4-mini (openai-codex:gpt-5.4-mini)"},
			},
			StreamFormat: StreamACPJSONRPC, MCPDiscovery: "mature-acp",
			build: buildHermes, fetchModels: fetchHermesModels,
		},
		{
			ID: "cursor-agent", Name: "Cursor Agent", Bin: "cursor-agent",
			VersionArgs:    []string{"--version"},
			listModelsArgs: []string{"models"}, listModelsTimeout: 5000,
			listModelsParse: parseCursorModels,
			FallbackModels: []ModelOption{
				DefaultModelOption,
				{ID: "auto", Label: "auto"},
				{ID: "sonnet-4", Label: "sonnet-4"},
				{ID: "sonnet-4-thinking", Label: "sonnet-4-thinking"},
				{ID: "gpt-5", Label: "gpt-5"},
			},
			StreamFormat: StreamJSONEvent, EventParser: "cursor-agent", PromptViaStdin: true,
			build: buildCursorAgent,
		},
		{
			ID: "copilot", Name: "GitHub Copilot CLI", Bin: "copilot",
			VersionArgs: []string{"--version"},
			FallbackModels: []ModelOption{
				DefaultModelOption,
				{ID: "claude-sonnet-4.6", Label: "Claude Sonnet 4.6"},
				{ID: "gpt-5.2", Label: "GPT-5.2"},
			},
			StreamFormat: StreamCopilotJSON, PromptViaStdin: true,
			build: buildCopilot,
		},
		{
			ID: "pi", Name: "Pi", Bin: "pi",
			VersionArgs: []string{"--version"},
			fetchModels: fetchPiModels,
			FallbackModels: []ModelOption{
				DefaultModelOption,
				{ID: "anthropic/claude-sonnet-4-5", Label: "Claude Sonnet 4.5 (anthropic)"},
				{ID: "anthropic/claude-opus-4-5", Label: "Claude Opus 4.5 (anthropic)"},
				{ID: "openai/gpt-5", Label: "GPT-5 (openai)"},
				{ID: "openai/o4-mini", Label: "o4-mini (openai)"},
				{ID: "google/gemini-2.5-pro", Label: "Gemini 2.5 Pro (google)"},
				{ID: "google/gemini-2.5-flash", Label: "Gemini 2.5 Flash (google)"},
			},
			ReasoningOptions: reasonPi,
			StreamFormat:     StreamPiRPC, PromptViaStdin: true,
			SupportsImagePaths: true,
			build:              buildPi,
		},
	}
	for _, d := range defs {
		if _, ok := supportsNativeSession[d.ID]; ok {
			d.SupportsNativeSession = true
		}
	}
	return defs
}

func parseLineSeparatedModels(stdout string) []ModelOption {
	lines := strings.Split(stdout, "\n")
	seen := map[string]struct{}{}
	out := []ModelOption{DefaultModelOption}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, ModelOption{ID: line, Label: line})
	}
	if len(out) <= 1 {
		return nil
	}
	return out
}

func parseCursorModels(stdout string) []ModelOption {
	s := strings.TrimSpace(stdout)
	if s == "" || strings.Contains(strings.ToLower(s), "no models available") {
		return nil
	}
	// cursor-agent models outputs "id - Label" per line with header/footer lines.
	lines := strings.Split(s, "\n")
	seen := map[string]struct{}{}
	out := []ModelOption{DefaultModelOption}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Available models") || strings.HasPrefix(line, "Tip:") {
			continue
		}
		parts := strings.SplitN(line, " - ", 2)
		id := strings.TrimSpace(parts[0])
		if id == "" {
			continue
		}
		label := id
		if len(parts) == 2 {
			label = strings.TrimSpace(parts[1])
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, ModelOption{ID: id, Label: label})
	}
	if len(out) <= 1 {
		return nil
	}
	return out
}

var builtinDefs = definitions()

// BuiltInAgents returns the static definition table.
func BuiltInAgents() []*AgentDef {
	return builtinDefs
}

// GetAgentDef finds a definition by id.
func GetAgentDef(id string) *AgentDef {
	for _, d := range builtinDefs {
		if d.ID == id {
			return d
		}
	}
	return nil
}
