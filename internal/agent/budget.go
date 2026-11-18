package agent

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// PromptTooLargeError matches the Open Design SSE error shape fields.
type PromptTooLargeError struct {
	Code    string
	Message string
	Bytes   int
	Limit   int
}

func (e *PromptTooLargeError) Error() string { return e.Message }

// CheckPromptArgvBudget mirrors agents.ts checkPromptArgvBudget.
func CheckPromptArgvBudget(def *AgentDef, composed string) *PromptTooLargeError {
	if def == nil || def.MaxPromptArgBytes <= 0 {
		return nil
	}
	n := len([]byte(composed))
	if n <= def.MaxPromptArgBytes {
		return nil
	}
	return &PromptTooLargeError{
		Code: "AGENT_PROMPT_TOO_LARGE",
		Message: fmt.Sprintf(
			`%s requires the prompt as a command-line argument and this run's composed prompt exceeds the safe size (%d > %d bytes). `+
				`Reduce context or pick an adapter with stdin support.`,
			def.Name, n, def.MaxPromptArgBytes,
		),
		Bytes: n,
		Limit: def.MaxPromptArgBytes,
	}
}

const windowsCreateProcessLimit = 32767
const windowsCreateProcessHeadroom = 256

func quoteWindowsCmdShim(value string) string {
	if !strings.ContainsAny(value, " \t\"&<>|^%") {
		return value
	}
	escaped := strings.ReplaceAll(value, `"`, `""`)
	escaped = strings.ReplaceAll(escaped, "%", `"^%"`)
	return `"` + escaped + `"`
}

// quoteWindowsDirectExe mirrors libuv quote_cmd_arg semantics (simplified).
func quoteWindowsDirectExe(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	if !strings.ContainsAny(value, "\\\"") && !strings.Contains(value, "\\") {
		return `"` + value + `"`
	}
	var b strings.Builder
	b.WriteByte('"')
	back := 0
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' {
			back++
			continue
		}
		if value[i] == '"' {
			b.WriteString(strings.Repeat(`\`, 2*back+1))
			b.WriteByte('"')
			back = 0
			continue
		}
		if back > 0 {
			b.WriteString(strings.Repeat(`\`, back))
			back = 0
		}
		b.WriteByte(value[i])
	}
	b.WriteString(strings.Repeat(`\`, 2*back))
	b.WriteByte('"')
	return b.String()
}

func looksLikeWindowsPath(p string) bool {
	if len(p) < 2 {
		return false
	}
	if p[0] >= 'A' && p[0] <= 'z' && p[1] == ':' && (len(p) > 2 && (p[2] == '/' || p[2] == '\\')) {
		return true
	}
	return strings.HasPrefix(p, `\\`)
}

// CheckWindowsCmdShimCommandLineBudget mirrors agents.ts (argv + .cmd shim wrap).
func CheckWindowsCmdShimCommandLineBudget(def *AgentDef, resolvedBin string, args []string) *PromptTooLargeError {
	if def == nil {
		return nil
	}
	if resolvedBin == "" {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(resolvedBin))
	if ext != ".cmd" && ext != ".bat" {
		return nil
	}
	parts := append([]string{resolvedBin}, args...)
	var inner strings.Builder
	for i, p := range parts {
		if i > 0 {
			inner.WriteByte(' ')
		}
		inner.WriteString(quoteWindowsCmdShim(p))
	}
	clen := len("cmd.exe /d /s /c ") + inner.Len() + 2
	limit := windowsCreateProcessLimit - windowsCreateProcessHeadroom
	if clen <= limit {
		return nil
	}
	return &PromptTooLargeError{
		Code: "AGENT_PROMPT_TOO_LARGE",
		Message: fmt.Sprintf(
			`%s on Windows runs through a .cmd shim and this run's prompt would expand past the CreateProcess command-line limit `+
				`after cmd.exe quote-doubling (%d > %d chars).`,
			def.Name, clen, limit,
		),
		Bytes: clen,
		Limit: limit,
	}
}

// CheckWindowsDirectExeCommandLineBudget mirrors agents.ts for direct .exe spawns.
func CheckWindowsDirectExeCommandLineBudget(def *AgentDef, resolvedBin string, args []string) *PromptTooLargeError {
	if def == nil {
		return nil
	}
	if resolvedBin == "" || !looksLikeWindowsPath(resolvedBin) {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(resolvedBin))
	if ext == ".cmd" || ext == ".bat" {
		return nil
	}
	parts := append([]string{resolvedBin}, args...)
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(quoteWindowsDirectExe(p))
	}
	clen := b.Len()
	limit := windowsCreateProcessLimit - windowsCreateProcessHeadroom
	if clen <= limit {
		return nil
	}
	return &PromptTooLargeError{
		Code: "AGENT_PROMPT_TOO_LARGE",
		Message: fmt.Sprintf(
			`%s on Windows builds a CreateProcess command line and this run's prompt would expand past the limit `+
				`after libuv quote-escaping (%d > %d chars).`,
			def.Name, clen, limit,
		),
		Bytes: clen,
		Limit: limit,
	}
}

// RuneLen returns UTF-8 byte length of s (same as len for valid UTF-8).
func RuneLen(s string) int { return utf8.RuneCountInString(s) }
