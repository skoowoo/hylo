package agent

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Note access actions (mirrors mate.NoteAccessEntry.Action; kept independent
// so this package does not need to import mate).
const (
	NoteActionRead  = "read"
	NoteActionWrite = "write"
)

// NoteAccess is one vault note an agent tool call touched, resolved to a
// vault-relative path.
type NoteAccess struct {
	Path   string
	Action string
	Tool   string
	At     time.Time
}

// NoteAccessTracker turns the tool_use events any backend's stream parser
// emits (claude_stream.go, json_event_stream.go, copilot_stream.go,
// pi_session.go, acp_session.go all relay the same {type, name, input} shape)
// into a deduplicated list of notes touched during one run. This is the only
// place path-extraction rules live — parsers just need to make sure a tool
// call actually produces a tool_use event; they don't need to know what a
// "note" is.
type NoteAccessTracker struct {
	cwd       string
	vaultRoot string
	seen      map[string]int
	order     []NoteAccess
}

func NewNoteAccessTracker(cwd, vaultRoot string) *NoteAccessTracker {
	return &NoteAccessTracker{cwd: cwd, vaultRoot: vaultRoot, seen: make(map[string]int)}
}

// Entries returns every note touched so far, in first-seen order.
func (t *NoteAccessTracker) Entries() []NoteAccess { return t.order }

// Feed inspects one relayed agent event and returns the entries it newly
// introduced or upgraded from read to write (empty when the event carried no
// new note access, e.g. text_delta or a tool call outside the vault).
func (t *NoteAccessTracker) Feed(m map[string]any) []NoteAccess {
	if typ, _ := m["type"].(string); typ != "tool_use" {
		return nil
	}
	name, _ := m["name"].(string)
	var added []NoteAccess
	for _, c := range extractNoteCandidates(name, m["input"]) {
		full := t.resolve(c.path)
		if full == "" {
			continue
		}
		if idx, ok := t.seen[full]; ok {
			if t.order[idx].Action == NoteActionRead && c.action == NoteActionWrite {
				t.order[idx].Action = NoteActionWrite
				t.order[idx].Tool = name
				added = append(added, t.order[idx])
			}
			continue
		}
		entry := NoteAccess{Path: full, Action: c.action, Tool: name, At: time.Now()}
		t.seen[full] = len(t.order)
		t.order = append(t.order, entry)
		added = append(added, entry)
	}
	return added
}

// resolve maps a raw path argument (absolute or relative to cwd) to a
// vault-relative note path ("/journal/april.md"), or "" when it falls outside
// the vault, inside internal/upload directories, or isn't a .md file.
func (t *NoteAccessTracker) resolve(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Glob patterns ("*.md", "**/*.md", "daily-*.md") aren't a specific note
	// the agent touched — only concrete filenames count.
	if strings.ContainsAny(raw, "*?") {
		return ""
	}
	abs := raw
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(t.cwd, abs)
	}
	abs = filepath.Clean(abs)
	rootAbs, err := filepath.Abs(t.vaultRoot)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, ".hylo/") || strings.HasPrefix(rel, "_agent_uploads/") {
		return ""
	}
	if !strings.EqualFold(filepath.Ext(rel), ".md") {
		return ""
	}
	return "/" + rel
}

type noteCandidate struct {
	path   string
	action string
}

var (
	notePathKeyRe      = regexp.MustCompile(`(?i)^(file_?path|filepath|path|notebook_?path|target_?file|file)$`)
	notePathArrayKeyRe = regexp.MustCompile(`(?i)^(paths|files|file_paths)$`)
	noteReadVerbRe     = regexp.MustCompile(`(?i)read|view|cat|get|glob|grep|search|fetch|list`)
	noteWriteVerbRe    = regexp.MustCompile(`(?i)write|edit|create|patch|replace|delete|remove|append|save|multiedit|move|rename`)
	noteShellToolRe    = regexp.MustCompile(`(?i)bash|shell|exec|command|terminal`)
)

// extractNoteCandidates finds (path, action) pairs in one tool_use's input,
// independent of which backend or tool produced it. Structured backends (ACP
// tool_call locations, Codex file_change) hand over pre-resolved pairs via the
// reserved "__notes" key, bypassing the heuristics below entirely.
func extractNoteCandidates(name string, input any) []noteCandidate {
	inputMap, _ := input.(map[string]any)
	if inputMap == nil {
		if s, ok := input.(string); ok && s != "" {
			var parsed map[string]any
			if json.Unmarshal([]byte(s), &parsed) == nil {
				inputMap = parsed
			}
		}
	}
	if inputMap == nil {
		return nil
	}

	if raw, ok := inputMap["__notes"].([]any); ok {
		var out []noteCandidate
		for _, r := range raw {
			e, ok := r.(map[string]any)
			if !ok {
				continue
			}
			p, _ := e["path"].(string)
			if p == "" {
				continue
			}
			a, _ := e["action"].(string)
			if a != NoteActionWrite {
				a = NoteActionRead
			}
			out = append(out, noteCandidate{path: p, action: a})
		}
		return out
	}

	action := NoteActionRead
	if noteWriteVerbRe.MatchString(name) {
		action = NoteActionWrite
	} else if noteReadVerbRe.MatchString(name) {
		action = NoteActionRead
	}

	var out []noteCandidate
	for k, v := range inputMap {
		switch {
		case notePathKeyRe.MatchString(k):
			if s, ok := v.(string); ok && s != "" {
				out = append(out, noteCandidate{path: s, action: action})
			}
		case notePathArrayKeyRe.MatchString(k):
			if arr, ok := v.([]any); ok {
				for _, e := range arr {
					if s, ok := e.(string); ok && s != "" {
						out = append(out, noteCandidate{path: s, action: action})
					}
				}
			}
		}
	}
	if len(out) > 0 {
		return out
	}

	// No structured path argument (Bash/shell-style tools): fall back to
	// scanning the command string itself. Best-effort safety net, not a
	// precise parse — unrecognized verbs are skipped rather than guessed.
	if noteShellToolRe.MatchString(name) {
		if cmd, ok := inputMap["command"].(string); ok {
			return extractNotesFromShellCommand(cmd)
		}
	}
	return nil
}

var (
	mdTokenRe        = regexp.MustCompile(`'([^']*\.md)'|"([^"]*\.md)"|(\S*\.md)`)
	shellSplitRe     = regexp.MustCompile(`[;&|]+`)
	shellWriteVerbRe = regexp.MustCompile(`^(mv|cp|rm|tee|sed|mkdir|touch)$`)
	shellReadVerbRe  = regexp.MustCompile(`^(cat|less|more|head|tail|grep|rg|find|ls)$`)
)

func extractNotesFromShellCommand(cmd string) []noteCandidate {
	var out []noteCandidate
	for _, part := range shellSplitRe.Split(cmd, -1) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		verb := ""
		if len(fields) > 0 {
			verb = fields[0]
		}
		var action string
		switch {
		case strings.Contains(part, ">"), shellWriteVerbRe.MatchString(verb):
			action = NoteActionWrite
		case shellReadVerbRe.MatchString(verb):
			action = NoteActionRead
		default:
			continue // unrecognized verb: skip rather than guess
		}
		for _, m := range mdTokenRe.FindAllStringSubmatch(part, -1) {
			p := firstNonEmpty(m[1], m[2], m[3])
			if p != "" {
				out = append(out, noteCandidate{path: p, action: action})
			}
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
