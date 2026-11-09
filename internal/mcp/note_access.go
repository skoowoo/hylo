package mcp

import (
	"path"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/hardhacker/hylo/internal/storage"
)

// noteRef is how a tool's input names the note it touches; each resolves the
// argument with the same function the tool itself calls.
type noteRef int

const (
	noNote      noteRef = iota
	byPath              // "path": vault-absolute, as the write tools require
	byName              // "path": a vault path or a unique bare filename (notes.ResolvePath)
	byKnowledge         // byName among knowledge units only
	byLatest            // "path": a vault path or a bare filename, latest wins (notes.ResolveLatest)
	byMove              // the note "path" lands at in "new_dir"
	byRename            // the note "path" becomes as "new_name"
	byShortID           // "id": a short entry, whose day file is the note
	byNewShort          // a new short, in today's day file
)

type noteTool struct {
	ref   noteRef
	write bool
}

// noteTools says what each tool does to a note, so a bot run's note activity
// is exact for hylo's own tools rather than guessed from their names. Every
// tool is listed (TestEveryToolDeclaresItsNoteAccess); compile only queues a
// run, whose own calls are recorded.
var noteTools = map[string]noteTool{
	"read": {byName, false}, "knowledge_read": {byKnowledge, false}, "editor_read": {byPath, false},
	"extract_outline": {byLatest, false}, "extract_section": {byLatest, false}, "extract_code": {byLatest, false},
	"extract_link": {byLatest, false}, "extract_list": {byLatest, false}, "extract_segment": {byLatest, false},
	"extract_tag": {byLatest, false}, "short_get": {byShortID, false},

	"create": {byPath, true}, "append": {byPath, true}, "prepend": {byPath, true}, "delete": {byPath, true},
	"knowledge_delete": {byPath, true}, "editor_type": {byPath, true}, "editor_rewrite": {byPath, true},
	"move": {byMove, true}, "rename": {byRename, true},
	"short_create": {byNewShort, true}, "short_update": {byShortID, true}, "short_delete": {byShortID, true},

	"capabilities": {}, "status": {}, "list": {}, "sample": {}, "search": {}, "resolve": {}, "compile": {}, "rename_status": {},
	"knowledge_list": {}, "knowledge_search": {}, "knowledge_list_indexes": {},
	"tag_list": {}, "tag_count": {}, "short_list": {}, "image_upload": {},
	"bot_list": {}, "bot_get": {}, "bot_create": {}, "bot_update": {}, "bot_delete": {},
	"agenda_list": {}, "agenda_add": {}, "agenda_done": {},
	"apps_list": {}, "apps_tools": {}, "apps_call": {},
	"bot_chat": {}, "activity": {}, "screen_time": {},
	"trigger_add": {}, "trigger_update": {}, "trigger_delete": {}, "trigger_run": {},
	"skills_list": {}, "skills_catalog": {}, "skills_add": {}, "skills_enable": {}, "skills_disable": {}, "skills_remove": {},
}

// NoteAccess is a note a tool call reads or writes, as a vault path.
type NoteAccess struct {
	Path  string
	Write bool
}

// ToolName strips the prefix an agent CLI puts on one of hylo's MCP tools
// (mcp__hylo__read, hylo_read, hylo-read); ok is false for any other tool.
func ToolName(name string) (string, bool) {
	for _, p := range []string{"mcp__hylo__", "hylo__", "hylo_", "hylo-", "hylo."} {
		if rest, found := strings.CutPrefix(name, p); found {
			_, known := noteTools[rest]
			return rest, known
		}
	}
	return "", false
}

// NotesTouched names the note a call to tool touches, resolved as the tool
// resolves it, so a call the tool would reject records nothing; ok is false
// for a tool hylo doesn't have.
func NotesTouched(l notes.NoteLookup, tool string, input map[string]any) ([]NoteAccess, bool) {
	nt, ok := noteTools[tool]
	if !ok {
		return nil, false
	}
	str := func(k string) string { s, _ := input[k].(string); return strings.TrimSpace(s) }
	abs := func(k string) string {
		if p := str(k); strings.HasPrefix(p, "/") {
			return path.Clean(p)
		}
		return ""
	}
	var p string
	switch nt.ref {
	case byPath:
		p = abs("path")
	case byName, byKnowledge:
		if arg := str("path"); arg != "" {
			p, _ = notes.ResolvePath(l, arg, nt.ref == byKnowledge)
		}
	case byLatest:
		if arg := str("path"); arg != "" {
			p, _ = notes.ResolveLatest(l, arg)
		}
	case byMove:
		if from, dir := abs("path"), abs("new_dir"); from != "" && dir != "" {
			p = path.Join(dir, path.Base(from))
		}
	case byRename:
		if from, name := abs("path"), str("new_name"); from != "" && name != "" && !strings.ContainsAny(name, `/\`) {
			p = path.Join(path.Dir(from), notes.EnsureMarkdownExt(name))
		}
	case byShortID:
		if sp, err := storage.ShortPathFromID(str("id")); err == nil {
			p = sp.String()
		}
	case byNewShort:
		p = storage.ShortDayPath(time.Now()).String()
	}
	if p == "" {
		return nil, true
	}
	return []NoteAccess{{Path: p, Write: nt.write}}, true
}

// commandValueFlags are the CLI flags that take a value, so the word after one
// is not a positional argument. TestCommandValueFlagsMatchTheCLI keeps it in
// step with the commands.
var commandValueFlags = map[string]bool{
	"-c": true, "--content": true, "-f": true, "--file": true, "--field": true, "--hash": true, "--indent": true,
	"--head": true, "--tail": true, "--start": true, "--end": true, "--latest": true, "-l": true, "--limit": true,
	"--each": true, "--older-than": true,
	"-k": true, "--kind": true, "--name": true, "--description": true, "--system-prompt": true,
	"--prompt": true, "--schedule": true, "--events": true, "--path-prefixes": true,
	"--category": true, "--skill": true, "--bot-id": true, "--pid-file": true, "--images": true,
	"--due": true, "--before": true, "--since": true, "--actor": true, "--args": true,
}

// CommandValueFlags reports whether flag (e.g. "-c", "--content") takes a value.
func CommandValueFlags(flag string) bool { return commandValueFlags[flag] }

// CommandNotesTouched is NotesTouched for the CLI: args are the shell words
// after "hylo", e.g. ["extract", "section", "/a.md", "Intro"].
func CommandNotesTouched(l notes.NoteLookup, args []string) ([]NoteAccess, bool) {
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if commandValueFlags[a] {
				i++
			}
		default:
			pos = append(pos, a)
		}
	}
	for _, n := range []int{2, 1} {
		if len(pos) < n {
			continue
		}
		tool := strings.ReplaceAll(strings.Join(pos[:n], "_"), "-", "_")
		nt, ok := noteTools[tool]
		if !ok {
			continue
		}
		rest := append(pos[n:], "", "")
		input := map[string]any{"path": rest[0]}
		switch nt.ref {
		case byMove:
			input["new_dir"] = rest[1]
		case byRename:
			input["new_name"] = rest[1]
		case byShortID:
			input = map[string]any{"id": rest[0]}
		}
		return NotesTouched(l, tool, input)
	}
	return nil, false
}
