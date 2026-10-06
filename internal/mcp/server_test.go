package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/client"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestSession(t *testing.T) *sdk.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"notes":3,"indexed":2,"knowledge_notes":1,"short_days":0}`))
	})
	mux.HandleFunc("POST /api/vault/stat", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"dir":"/","name":"a.md"}`))
	})
	mux.HandleFunc("POST /api/vault/read", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("# Title\n\nline1\nline2\n\n## Sub\n\nbody\n"))
	})
	ts := httptest.NewServer(NewHandler(client.NewInProcess(mux)))
	t.Cleanup(ts.Close)

	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		return nil, true
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("%s: structured content is %T", name, res.StructuredContent)
	}
	return m, false
}

func TestToolsAlignWithCLI(t *testing.T) {
	cs := newTestSession(t)

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
		if tool.OutputSchema == nil {
			t.Errorf("tool %q has no output schema", tool.Name)
		}
	}
	for _, name := range []string{
		"status", "list", "read", "search", "create", "append", "prepend", "delete", "move",
		"rename", "rename_status", "resolve", "knowledge_list", "knowledge_read",
		"knowledge_search", "knowledge_delete", "knowledge_list_indexes",
		"extract_outline", "extract_section", "extract_code", "extract_link", "extract_list",
		"extract_segment", "extract_tag", "tag_list", "tag_count", "tag_delete",
		"short_create", "short_list",
	} {
		if !got[name] {
			t.Errorf("missing tool %q", name)
		}
	}
}

func TestCallTools(t *testing.T) {
	cs := newTestSession(t)

	if m, isErr := call(t, cs, "status", nil); isErr || m["notes"] != float64(3) {
		t.Errorf("status: %v err=%v", m, isErr)
	}
	if m, _ := call(t, cs, "read", map[string]any{"path": "/a.md"}); !strings.Contains(m["content"].(string), "## Sub") || m["path"] != "/a.md" {
		t.Errorf("read: %v", m)
	}
	m, _ := call(t, cs, "extract_outline", map[string]any{"path": "/a.md"})
	hs := m["headings"].([]any)
	if len(hs) != 2 || hs[1].(map[string]any)["text"] != "Sub" || hs[1].(map[string]any)["level"] != float64(2) {
		t.Errorf("outline: %v", m)
	}
	if m, _ := call(t, cs, "extract_segment", map[string]any{"path": "/a.md", "head": 3}); m["content"] != "# Title\n\nline1" || m["total_lines"] != float64(8) {
		t.Errorf("segment: %v", m)
	}
	if m, _ := call(t, cs, "extract_code", map[string]any{"path": "/a.md"}); len(m["blocks"].([]any)) != 0 {
		t.Errorf("code: %v", m)
	}
	if _, isErr := call(t, cs, "extract_segment", map[string]any{"path": "/a.md"}); !isErr {
		t.Error("segment without range should fail")
	}
	if _, isErr := call(t, cs, "delete", map[string]any{"path": "relative.md"}); !isErr {
		t.Error("delete with relative path should fail")
	}
}
