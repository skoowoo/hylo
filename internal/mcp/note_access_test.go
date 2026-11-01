package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/editorbus"
	"github.com/hardhacker/hylo/internal/storage"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// A tool missing from noteTools would leave its bot runs' note activity unrecorded.
func TestEveryToolDeclaresItsNoteAccess(t *testing.T) {
	ts := httptest.NewServer(NewHandler(client.NewInProcess(http.NotFoundHandler()), WithEditorBus(editorbus.New())))
	defer ts.Close()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil).
		Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, tool := range res.Tools {
		listed[tool.Name] = true
		if _, ok := noteTools[tool.Name]; !ok {
			t.Errorf("%s: not in noteTools", tool.Name)
		}
	}
	for name := range noteTools {
		if !listed[name] {
			t.Errorf("noteTools has %s, which is no tool", name)
		}
	}
}

// fakeLookup holds notes by vault path; names resolve newest first in list order.
type fakeLookup []storage.Note

func (f fakeLookup) StatNote(p string) (storage.Note, error) {
	for _, n := range f {
		if n.PathString() == p || n.PathString() == p+".md" {
			return n, nil
		}
	}
	return storage.Note{}, storage.ErrNotFound
}

func (f fakeLookup) NotesByName(name string) ([]storage.Note, error) {
	var out []storage.Note
	for _, n := range f {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

var notesFixture = fakeLookup{
	{Dir: "/journal", Name: "today.md"},
	{Dir: "/a", Name: "dup.md"},
	{Dir: "/b", Name: "dup.md"},
	{Dir: "/_knowledge", Name: "dup.md", Kind: storage.KindKnowledge},
}

func TestNotesTouchedResolvesAsTheToolDoes(t *testing.T) {
	read := func(p string) []NoteAccess { return []NoteAccess{{Path: p}} }
	write := func(p string) []NoteAccess { return []NoteAccess{{Path: p, Write: true}} }
	for _, c := range []struct {
		name  string
		input map[string]any
		want  []NoteAccess
	}{
		{"mcp__hylo__read", map[string]any{"path": "/journal/today.md"}, read("/journal/today.md")},
		{"hylo_read", map[string]any{"path": "today"}, read("/journal/today.md")},
		{"hylo_read", map[string]any{"path": "journal/today"}, read("/journal/today.md")},
		{"hylo_read", map[string]any{"path": "dup.md"}, nil}, // ambiguous: the tool fails
		{"hylo_read", map[string]any{"path": "missing.md"}, nil},
		{"hylo_knowledge_read", map[string]any{"path": "dup"}, read("/_knowledge/dup.md")},
		{"hylo_extract_outline", map[string]any{"path": "dup"}, read("/a/dup.md")}, // latest wins
		{"hylo_extract_outline", map[string]any{"path": "x/y"}, read("/x/y.md")},
		{"mcp__hylo__prepend", map[string]any{"path": "/a.md"}, write("/a.md")},
		{"mcp__hylo__create", map[string]any{"path": "ideas/new.md"}, nil}, // must be absolute
		{"mcp__hylo__editor_read", map[string]any{"path": "/a.md"}, read("/a.md")},
		{"mcp__hylo__editor_type", map[string]any{"content": "x"}, nil}, // the note in front of the user
		{"mcp__hylo__move", map[string]any{"path": "/a/n.md", "new_dir": "/b"}, write("/b/n.md")},
		{"mcp__hylo__rename", map[string]any{"path": "/a/n.md", "new_name": "m"}, write("/a/m.md")},
		{"mcp__hylo__short_update", map[string]any{"id": "2026-11-01-101500"}, write("/_shorts/2026-11-01.md")},
		{"mcp__hylo__short_create", map[string]any{"content": "x"}, write(storage.ShortDayPath(time.Now()).String())},
		{"mcp__hylo__search", map[string]any{"query": "x"}, nil},
	} {
		tool, ok := ToolName(c.name)
		if !ok {
			t.Fatalf("%s: not a hylo tool", c.name)
		}
		got, ok := NotesTouched(notesFixture, tool, c.input)
		if !ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %v = %+v, want %+v", c.name, c.input, got, c.want)
		}
	}
	if _, ok := ToolName("Read"); ok {
		t.Error("Read is no hylo tool")
	}
}

func TestCommandNotesTouched(t *testing.T) {
	for _, c := range []struct {
		args []string
		want []NoteAccess
		ok   bool
	}{
		{[]string{"read", "/journal/today.md"}, []NoteAccess{{Path: "/journal/today.md"}}, true},
		{[]string{"extract", "section", "/a.md", "Intro"}, []NoteAccess{{Path: "/a.md"}}, true},
		{[]string{"extract", "segment", "--head", "20", "/a.md"}, []NoteAccess{{Path: "/a.md"}}, true},
		{[]string{"append", "--table", "/a.md"}, []NoteAccess{{Path: "/a.md", Write: true}}, true},
		{[]string{"create", "-c", "hello world", "/a.md"}, []NoteAccess{{Path: "/a.md", Write: true}}, true},
		{[]string{"create", "--content=x", "/a.md"}, []NoteAccess{{Path: "/a.md", Write: true}}, true},
		{[]string{"move", "/a/n.md", "/b"}, []NoteAccess{{Path: "/b/n.md", Write: true}}, true},
		{[]string{"short", "delete", "--hash", "abc", "2026-11-01-101500"}, []NoteAccess{{Path: "/_shorts/2026-11-01.md", Write: true}}, true},
		{[]string{"knowledge", "list-indexes"}, nil, true},
		{[]string{"search", "x"}, nil, true},
		{[]string{"init"}, nil, false},
	} {
		got, ok := CommandNotesTouched(notesFixture, c.args)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("hylo %v = %+v %v, want %+v %v", c.args, got, ok, c.want, c.ok)
		}
	}
}
