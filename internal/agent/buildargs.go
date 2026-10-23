package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// ClampCodexReasoning mirrors agents.ts clampCodexReasoning.
func ClampCodexReasoning(modelID, effort string) string {
	if effort == "" {
		return effort
	}
	raw := strings.TrimSpace(modelID)
	id := raw
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		id = raw[i+1:]
	}
	isGpt5Late := id == "" || id == "default" ||
		strings.HasPrefix(id, "gpt-5.2") || strings.HasPrefix(id, "gpt-5.3") ||
		strings.HasPrefix(id, "gpt-5.4") || strings.HasPrefix(id, "gpt-5.5")
	if isGpt5Late && effort == "minimal" {
		return "low"
	}
	if id == "gpt-5.1" && effort == "xhigh" {
		return "high"
	}
	if id == "gpt-5.1-codex-mini" {
		if effort == "high" || effort == "xhigh" {
			return "high"
		}
		return "medium"
	}
	return effort
}

func filterDirs(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

func filterAbsDirs(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d != "" && filepath.IsAbs(d) {
			out = append(out, d)
		}
	}
	return out
}

func buildClaude(def *AgentDef, c BuildArgsContext) []string {
	caps := c.Caps
	// Stdin is one stream-json user frame on every turn, images or not.
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--input-format", "stream-json"}
	if c.SessionID != "" {
		if c.FirstSession {
			args = append(args, "--session-id", c.SessionID)
		} else {
			args = append(args, "--resume", c.SessionID)
		}
	}
	if capTrue(caps, "partialMessages") {
		args = append(args, "--include-partial-messages")
	}
	if c.Model != "" && c.Model != "default" {
		args = append(args, "--model", c.Model)
	}
	dirs := filterDirs(c.ExtraAllowedDirs)
	if len(dirs) > 0 && caps["addDir"] {
		args = append(args, "--add-dir")
		args = append(args, dirs...)
	}
	args = append(args, "--permission-mode", "bypassPermissions")
	return args
}

func buildCodex(_ *AgentDef, c BuildArgsContext) []string {
	var args []string
	if c.SessionID != "" {
		args = []string{"exec", "resume", c.SessionID, "--json", "--skip-git-repo-check", "--sandbox", "workspace-write",
			"-c", "sandbox_workspace_write.network_access=true"}
	} else {
		args = []string{
			"exec", "--json", "--skip-git-repo-check", "--sandbox", "workspace-write",
			"-c", "sandbox_workspace_write.network_access=true",
		}
	}
	if os.Getenv("OD_CODEX_DISABLE_PLUGINS") == "1" {
		args = append(args, "--disable", "plugins")
	}
	if c.Cwd != "" {
		args = append(args, "-C", c.Cwd)
	}
	for _, d := range filterDirs(c.ExtraAllowedDirs) {
		args = append(args, "--add-dir", d)
	}
	if c.Model != "" && c.Model != "default" {
		args = append(args, "--model", c.Model)
	}
	if c.Reasoning != "" && c.Reasoning != "default" {
		eff := ClampCodexReasoning(c.Model, c.Reasoning)
		args = append(args, "-c", `model_reasoning_effort="`+eff+`"`)
	}
	// A path typed into the prompt is text. -i is what attaches the file as an image.
	return appendImageArgs(args, "-i", c.ImagePaths)
}

func buildOpenCode(_ *AgentDef, c BuildArgsContext) []string {
	args := []string{"run", "--format", "json", "--dangerously-skip-permissions"}
	if c.Cwd != "" {
		args = append(args, "--dir", c.Cwd)
	}
	if c.Model != "" && c.Model != "default" {
		args = append(args, "--model", c.Model)
	}
	if c.SessionID != "" {
		args = append(args, "-s", c.SessionID)
	}
	// -f/--file labels every attachment text/plain, so a picture arrives as
	// garbage text. The path goes in the prompt and the read tool sniffs it.
	// --dangerously-skip-permissions already approves that external-directory read.
	return args
}

func buildCursorAgent(_ *AgentDef, c BuildArgsContext) []string {
	args := []string{
		"--print", "--output-format", "stream-json", "--stream-partial-output",
		"--force",
	}
	if c.SessionID != "" {
		args = append(args, "--resume", c.SessionID)
	}
	if c.Cwd != "" {
		args = append(args, "--workspace", c.Cwd)
	}
	if c.Model != "" && c.Model != "default" {
		args = append(args, "--model", c.Model)
	}
	// The read tool only sees workspace roots. Uploads sit outside the bot's cwd.
	return appendUploadDirs(args, c.ImagePaths)
}

// appendUploadDirs grants each image's directory as an extra workspace root.
func appendUploadDirs(args []string, paths []string) []string {
	seen := map[string]bool{}
	for _, p := range paths {
		if dir := filepath.Dir(p); !seen[dir] {
			seen[dir] = true
			args = append(args, "--add-dir", dir)
		}
	}
	return args
}

func buildCopilot(_ *AgentDef, c BuildArgsContext) []string {
	args := []string{"--allow-all-tools", "--output-format", "json"}
	if c.SessionID != "" {
		args = append(args, "--resume", c.SessionID)
	}
	if c.Model != "" && c.Model != "default" {
		args = append(args, "--model", c.Model)
	}
	for _, d := range filterDirs(c.ExtraAllowedDirs) {
		args = append(args, "--add-dir", d)
	}
	// --attachment is the non-interactive way to hand Copilot a file.
	return appendImageArgs(args, "--attachment", c.ImagePaths)
}

// appendImageArgs repeats flag once per path. Both CLIs attach one file per occurrence.
func appendImageArgs(args []string, flag string, paths []string) []string {
	for _, p := range paths {
		args = append(args, flag, p)
	}
	return args
}

// UserText is the caption a CLI receives. Agents that cannot send a captionless
// image get a stand-in sentence. That sentence is not the message saved in the
// conversation.
func UserText(agentID, message string, images int) string {
	if strings.TrimSpace(message) != "" || images == 0 {
		return message
	}
	switch agentID {
	case "codex", "copilot", "cursor-agent", "opencode":
		if images == 1 {
			return "Look at the attached image."
		}
		return "Look at the attached images."
	default:
		return message
	}
}

// TurnText is the user text written to the CLI. Cursor and OpenCode have no
// image attachment flag, so the absolute paths are part of this text and their
// read tool opens the files. The stored chat message stays the user's caption.
func TurnText(agentID, message string, paths []string) string {
	text := UserText(agentID, message, len(paths))
	if agentID != "cursor-agent" && agentID != "opencode" {
		return text
	}
	var lines []string
	if strings.TrimSpace(text) != "" {
		lines = append(lines, text)
	}
	return strings.Join(append(lines, paths...), "\n")
}

func buildPi(_ *AgentDef, c BuildArgsContext) []string {
	args := []string{"--mode", "rpc"}
	if c.SessionID != "" {
		// --session exits when the id is missing. --session-id opens it or creates it.
		args = append(args, "--session-id", c.SessionID)
	}
	if c.Model != "" && c.Model != "default" {
		args = append(args, "--model", c.Model)
	}
	if c.Reasoning != "" && c.Reasoning != "default" {
		args = append(args, "--thinking", c.Reasoning)
	}
	for _, d := range filterAbsDirs(c.ExtraAllowedDirs) {
		args = append(args, "--append-system-prompt", d)
	}
	return args
}

// BuildInvocationArgs returns argv for the resolved agent definition.
func BuildInvocationArgs(def *AgentDef, c BuildArgsContext) []string {
	if def == nil || def.build == nil {
		return nil
	}
	c.Caps = Capabilities(def.ID)
	return def.build(def, c)
}

// CLIExample returns a representative shell command for manual CLI testing.
// Stdin-based agents use the `echo "..." | bin args` form; others embed the prompt in argv.
func CLIExample(def *AgentDef) string {
	if def == nil || def.build == nil {
		return ""
	}
	var argCtx BuildArgsContext
	if !def.PromptViaStdin {
		argCtx.Prompt = "Hello"
	}
	// Pick the first non-default fallback model so --model appears in the example.
	for _, m := range def.FallbackModels {
		if m.ID != "" && m.ID != "default" {
			argCtx.Model = m.ID
			break
		}
	}
	argCtx.Caps = Capabilities(def.ID)
	// RPC stdin is JSONL commands. A raw prompt is rejected as invalid JSON.
	// JSON mode is the one-shot form of the same event stream.
	if def.StreamFormat == StreamPiRPC {
		argv := []string{def.Bin, "--mode", "json", "--no-session", "-p", "Hello"}
		if argCtx.Model != "" {
			argv = append(argv, "--model", argCtx.Model)
		}
		return shellJoin(argv)
	}
	argv := def.build(def, argCtx)
	parts := make([]string, 0, 1+len(argv))
	parts = append(parts, def.Bin)
	parts = append(parts, argv...)
	cmd := shellJoin(parts)
	if def.StreamFormat == StreamClaudeJSON {
		return `echo '{"type":"user","message":{"role":"user","content":"Hello"}}' | ` + cmd
	}
	if def.PromptViaStdin {
		return `echo "Hello" | ` + cmd
	}
	return cmd
}

// shellJoin joins args into a single command string, quoting args that contain
// spaces or shell-special characters.
func shellJoin(args []string) string {
	var sb strings.Builder
	for i, a := range args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if needsQuote(a) {
			sb.WriteByte('"')
			sb.WriteString(strings.ReplaceAll(a, `"`, `\"`))
			sb.WriteByte('"')
		} else {
			sb.WriteString(a)
		}
	}
	return sb.String()
}

func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '"' || c == '\'' || c == '\\' ||
			c == '$' || c == '`' || c == '!' || c == '&' || c == '|' ||
			c == ';' || c == '<' || c == '>' || c == '(' || c == ')' {
			return true
		}
	}
	return false
}
