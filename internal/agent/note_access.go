package agent

import (
	"encoding/json"
	"path"
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
	// Skim marks a read that only listed, searched or counted the note: it
	// shows on the run's card but is not reading the note.
	Skim bool
}

// KnownNotes names, as vault paths, the notes a call touches for tools whose
// meaning is known exactly (hylo's own MCP tools and CLI); ok is false for any
// other tool, which is then judged from its name and input.
type KnownNotes interface {
	Tool(name string, input map[string]any) ([]NoteAccess, bool)
	Command(args []string) ([]NoteAccess, bool)
}

// NoteAccessTracker turns the tool_use events any backend's stream parser
// emits (claude_stream.go, json_event_stream.go, copilot_stream.go,
// pi_session.go all relay the same {type, name, input} shape)
// into a deduplicated list of notes touched during one run. This is the only
// place path-extraction rules live — parsers just need to make sure a tool
// call actually produces a tool_use event; they don't need to know what a
// "note" is.
type NoteAccessTracker struct {
	cwd       string
	vaultRoot string
	known     KnownNotes
	seen      map[string]int
	order     []NoteAccess
}

// known may be nil: every tool is then judged from its name and input.
func NewNoteAccessTracker(cwd, vaultRoot string, known KnownNotes) *NoteAccessTracker {
	return &NoteAccessTracker{cwd: cwd, vaultRoot: vaultRoot, known: known, seen: make(map[string]int)}
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
	for _, c := range extractNoteCandidates(name, m["input"], t.known) {
		full := t.resolve(c)
		if full == "" {
			continue
		}
		if idx, ok := t.seen[full]; ok {
			e := &t.order[idx]
			if !c.skim {
				e.Skim = false
			}
			if e.Action == NoteActionRead && c.action == NoteActionWrite {
				e.Action = NoteActionWrite
				e.Tool = name
				added = append(added, *e)
			}
			continue
		}
		entry := NoteAccess{Path: full, Action: c.action, Tool: name, At: time.Now(), Skim: c.skim}
		t.seen[full] = len(t.order)
		t.order = append(t.order, entry)
		added = append(added, entry)
	}
	return added
}

// resolve maps a path argument (a vault path, or a filesystem path absolute or
// relative to cwd) to a vault-relative note path ("/journal/april.md"), or ""
// when it falls outside the vault, inside internal/upload directories, or
// isn't a .md file.
func (t *NoteAccessTracker) resolve(c noteCandidate) string {
	raw := strings.TrimSpace(c.path)
	if raw == "" {
		return ""
	}
	// Glob patterns ("*.md", "**/*.md", "daily-*.md") aren't a specific note
	// the agent touched — only concrete filenames count.
	if strings.ContainsAny(raw, "*?") {
		return ""
	}
	if c.vault {
		return vaultNotePath(strings.TrimPrefix(path.Clean("/"+raw), "/"))
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
	if err != nil {
		return ""
	}
	return vaultNotePath(filepath.ToSlash(rel))
}

func vaultNotePath(rel string) string {
	if rel == "" || rel == "." || strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, ".hylo/") {
		return ""
	}
	if !strings.EqualFold(path.Ext(rel), ".md") {
		return ""
	}
	return "/" + rel
}

type noteCandidate struct {
	path   string
	action string
	vault  bool // path is already vault-absolute, not a filesystem path
	skim   bool // a read that is a NoteAccess.Skim
}

func knownCandidates(accs []NoteAccess) []noteCandidate {
	out := make([]noteCandidate, 0, len(accs))
	for _, a := range accs {
		out = append(out, noteCandidate{path: a.Path, action: a.Action, vault: true})
	}
	return out
}

var (
	notePathKeyRe      = regexp.MustCompile(`(?i)^(file_?path|filepath|path|notebook_?path|target_?file|file)$`)
	notePathArrayKeyRe = regexp.MustCompile(`(?i)^(paths|files|file_paths)$`)
	noteReadVerbRe     = regexp.MustCompile(`(?i)read|view|cat|get|glob|grep|search|fetch|list`)
	noteSkimVerbRe     = regexp.MustCompile(`(?i)glob|grep|search|list|^ls$|find`)
	noteWriteVerbRe    = regexp.MustCompile(`(?i)write|edit|create|patch|replace|delete|remove|append|save|multiedit|move|rename`)
	noteShellToolRe    = regexp.MustCompile(`(?i)bash|shell|exec|command|terminal`)
)

// extractNoteCandidates finds (path, action) pairs in one tool_use's input,
// independent of which backend or tool produced it. Known tools are looked up
// exactly; structured backends (Codex file_change) hand over pre-resolved
// pairs via the reserved "__notes" key; only the rest go by the heuristics below.
func extractNoteCandidates(name string, input any, known KnownNotes) []noteCandidate {
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
	if known != nil {
		if accs, ok := known.Tool(name, inputMap); ok {
			return knownCandidates(accs)
		}
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
	skim := action == NoteActionRead && noteSkimVerbRe.MatchString(name)

	var out []noteCandidate
	for k, v := range inputMap {
		switch {
		case notePathKeyRe.MatchString(k):
			if s, ok := v.(string); ok && s != "" {
				out = append(out, noteCandidate{path: s, action: action, skim: skim})
			}
		case notePathArrayKeyRe.MatchString(k):
			if arr, ok := v.([]any); ok {
				for _, e := range arr {
					if s, ok := e.(string); ok && s != "" {
						out = append(out, noteCandidate{path: s, action: action, skim: skim})
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
			return extractNotesFromShellCommand(cmd, known)
		}
	}
	return nil
}

var (
	mdTokenRe        = regexp.MustCompile(`'([^']*\.md)'|"([^"]*\.md)"|(\S*\.md)`)
	shellWriteVerbRe = regexp.MustCompile(`^(mv|rm|tee|mkdir|touch)$`)
	shellReadVerbRe  = regexp.MustCompile(`^(cat|less|more|head|tail|grep|rg|find|ls|awk|wc|nl|sort|cut|stat)$`)
	shellSkimVerbRe  = regexp.MustCompile(`^(grep|rg|find|ls|wc|stat)$`)
	sedInPlaceRe     = regexp.MustCompile(`^(-[a-zA-Z]*i[a-zA-Z]*|--in-place.*)$`)
	// A real output redirect: `>` / `>>` / `2>` with a target. Rules out
	// `>=` inside awk/test expressions, `->` in text, and `2>/dev/null`.
	shellRedirectRe = regexp.MustCompile(`(?:^|\s)(?:[0-9]|&)?>>?\s*("[^"]*"|'[^']*'|[^\s&=>]\S*)`)
)

// Only what a command demonstrably changes counts as a write: a redirect
// target, the destination of cp, tee/rm/mv/touch/mkdir, or sed -i. Anything
// else (cat, sed -n, awk, source files of cp) is a read.
func extractNotesFromShellCommand(cmd string, known KnownNotes) []noteCandidate {
	var out []noteCandidate
	for _, c := range splitShell(cmd) {
		part, fields := c.raw, c.words
		if len(fields) == 0 {
			continue
		}
		verb := fields[0]
		// The hylo CLI takes vault paths; its commands are known, not guessed.
		if path.Base(verb) == "hylo" {
			if known != nil {
				if accs, ok := known.Command(fields[1:]); ok {
					out = append(out, knownCandidates(accs)...)
				}
			}
			continue
		}
		targets := map[string]bool{}
		for _, m := range shellRedirectRe.FindAllStringSubmatch(part, -1) {
			if t := strings.Trim(m[1], `"'`); t != "/dev/null" {
				targets[t] = true
			}
		}
		verbAction := ""
		switch {
		case shellReadVerbRe.MatchString(verb):
			verbAction = NoteActionRead
		case verb == "sed":
			verbAction = NoteActionRead
			for _, f := range fields[1:] {
				if sedInPlaceRe.MatchString(f) {
					verbAction = NoteActionWrite
				}
			}
		case shellWriteVerbRe.MatchString(verb), verb == "cp":
			verbAction = NoteActionWrite
		}
		var paths []string
		for _, m := range mdTokenRe.FindAllStringSubmatch(part, -1) {
			if p := firstNonEmpty(m[1], m[2], m[3]); p != "" {
				paths = append(paths, p)
			}
		}
		for i, p := range paths {
			action := verbAction
			switch {
			case targets[p]:
				action = NoteActionWrite
			case verb == "cp":
				action = NoteActionRead
				if i == len(paths)-1 {
					action = NoteActionWrite
				}
			}
			if action != "" {
				out = append(out, noteCandidate{path: p, action: action, skim: action == NoteActionRead && shellSkimVerbRe.MatchString(verb)})
			}
		}
	}
	return out
}

type shellCommand struct {
	raw   string   // the command as written
	words []string // its words, quotes removed
}

// splitShell splits a command line at unquoted ;, & and |, so a quoted
// argument ("a; b") stays one word of one command.
func splitShell(cmd string) []shellCommand {
	var out []shellCommand
	var words []string
	var word strings.Builder
	inWord, quote, start := false, rune(0), 0
	endWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endCommand := func(end int) {
		endWord()
		if len(words) > 0 {
			out = append(out, shellCommand{raw: cmd[start:end], words: words})
		}
		words = nil
	}
	escaped := false
	for i, r := range cmd {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			inWord, escaped = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inWord, quote = true, r
		case r == ';' || r == '&' || r == '|':
			endCommand(i)
			start = i + 1
		case r == ' ' || r == '\t' || r == '\n':
			endWord()
		default:
			inWord = true
			word.WriteRune(r)
		}
	}
	endCommand(len(cmd))
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
