package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/editorbus"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func editorSession(t *testing.T, bus *editorbus.Bus) (*sdk.ClientSession, *[]string) {
	t.Helper()
	return editorSessionAt(t, bus, "")
}

// editorSessionAt connects with query appended to the endpoint, as agent runs do.
func editorSessionAt(t *testing.T, bus *editorbus.Bus, query string) (*sdk.ClientSession, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var writes []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/vault/read", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("# T\n")) })
	mux.HandleFunc("POST /api/vault/write", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Content string }
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		writes = append(writes, body.Content)
		mu.Unlock()
		w.Write([]byte(`{}`))
	})
	ts := httptest.NewServer(NewHandler(client.NewInProcess(mux), WithEditorBus(bus)))
	t.Cleanup(ts.Close)
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil).
		Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: ts.URL + query}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, &writes
}

// Without an editor the tool fails rather than writing the file some other way.
func TestEditorTypeNeedsAnEditor(t *testing.T) {
	cs, writes := editorSession(t, editorbus.New())
	if _, isErr := call(t, cs, "editor_type", map[string]any{"path": "/a.md", "content": "hello"}); !isErr {
		t.Fatal("editor_type without an editor should fail")
	}
	if len(*writes) != 0 {
		t.Fatalf("file written: %v", *writes)
	}
}

// fakeEditor serves bus over HTTP and answers every op it receives with respond.
func fakeEditor(t *testing.T, bus *editorbus.Bus, respond func(editorbus.Op) editorbus.Result) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			bus.AckPOST(w, r)
			return
		}
		bus.StreamSSE(w, r)
	}))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var op editorbus.Op
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &op) != nil || op.ID == "" {
				continue
			}
			res := respond(op)
			res.ID = op.ID
			ack, _ := json.Marshal(res)
			http.Post(srv.URL, "application/json", strings.NewReader(string(ack)))
		}
	}()
}

func TestEditorTypeThroughEditor(t *testing.T) {
	bus := editorbus.New()
	cs, writes := editorSession(t, bus)
	fakeEditor(t, bus, func(op editorbus.Op) editorbus.Result {
		return editorbus.Result{OK: true, Chars: len(op.Content)}
	})

	m, isErr := call(t, cs, "editor_type", map[string]any{"path": "/a.md", "content": "hello"})
	if isErr || m["chars"] != float64(5) {
		t.Fatalf("m=%v err=%v", m, isErr)
	}
	if len(*writes) != 0 {
		t.Fatalf("file written despite editor: %v", *writes)
	}
}

func TestEditorRead(t *testing.T) {
	bus := editorbus.New()
	cs, _ := editorSession(t, bus)
	fakeEditor(t, bus, func(op editorbus.Op) editorbus.Result {
		if op.Kind != editorbus.KindRead {
			t.Errorf("kind = %q", op.Kind)
		}
		return editorbus.Result{OK: true, Data: json.RawMessage(
			`{"blocks":[{"id":"b1","kind":"heading","text":"# T"}],"selection":{"text":"T","blocks":["b1"]}}`)}
	})
	m, isErr := call(t, cs, "editor_read", map[string]any{"path": "/a.md"})
	if isErr {
		t.Fatalf("m=%v", m)
	}
	blocks := m["blocks"].([]any)
	if len(blocks) != 1 || blocks[0].(map[string]any)["id"] != "b1" || m["selection"].(map[string]any)["text"] != "T" {
		t.Fatalf("m=%v", m)
	}
}

// Without a path the page reads the note it has open and names it.
func TestEditorReadOpenNote(t *testing.T) {
	bus := editorbus.New()
	cs, _ := editorSession(t, bus)
	fakeEditor(t, bus, func(op editorbus.Op) editorbus.Result {
		if op.Path != "" {
			t.Errorf("path = %q", op.Path)
		}
		return editorbus.Result{OK: true, Path: "/open.md", Data: json.RawMessage(`{"blocks":[]}`)}
	})
	m, isErr := call(t, cs, "editor_read", map[string]any{})
	if isErr || m["path"] != "/open.md" {
		t.Fatalf("m=%v err=%v", m, isErr)
	}
}

func TestEditorRewriteForwardsTarget(t *testing.T) {
	bus := editorbus.New()
	cs, writes := editorSession(t, bus)
	got := make(chan editorbus.Op, 1)
	fakeEditor(t, bus, func(op editorbus.Op) editorbus.Result {
		got <- op
		return editorbus.Result{OK: true, Chars: 3}
	})
	m, isErr := call(t, cs, "editor_rewrite", map[string]any{"path": "/a.md", "content": "new", "target": "b4k2x7"})
	if isErr || m["chars"] != float64(3) {
		t.Fatalf("m=%v err=%v", m, isErr)
	}
	op := <-got
	if op.Kind != editorbus.KindRewrite || op.Target != "b4k2x7" || op.Content != "new" {
		t.Fatalf("op = %+v", op)
	}
	if len(*writes) != 0 {
		t.Fatalf("file written: %v", *writes)
	}
}

func TestEditorRewriteReportsEditorError(t *testing.T) {
	bus := editorbus.New()
	cs, _ := editorSession(t, bus)
	fakeEditor(t, bus, func(editorbus.Op) editorbus.Result {
		return editorbus.Result{Error: "the note changed since it was read; call editor_read again"}
	})
	if _, isErr := call(t, cs, "editor_rewrite", map[string]any{"path": "/a.md", "content": "x", "target": "b4k2x7"}); !isErr {
		t.Fatal("expected the editor's error to surface")
	}
}

func TestEditorReadAndRewriteNeedAnEditor(t *testing.T) {
	cs, writes := editorSession(t, editorbus.New())
	if _, isErr := call(t, cs, "editor_read", map[string]any{"path": "/a.md"}); !isErr {
		t.Error("editor_read without an editor should fail")
	}
	if _, isErr := call(t, cs, "editor_rewrite", map[string]any{"path": "/a.md", "content": "x", "target": "selection"}); !isErr {
		t.Error("editor_rewrite without an editor should fail")
	}
	if len(*writes) != 0 {
		t.Fatalf("file written: %v", *writes)
	}
}

func TestEditorTypeRejectsBadInput(t *testing.T) {
	cs, _ := editorSession(t, editorbus.New())
	if _, isErr := call(t, cs, "editor_type", map[string]any{"path": "rel.md", "content": "x"}); !isErr {
		t.Error("relative path should fail")
	}
	if _, isErr := call(t, cs, "editor_type", map[string]any{"path": "/a.txt", "content": "x"}); !isErr {
		t.Error("non-markdown path should fail")
	}
}

// Cancelling another run leaves this run's typing alone; cancelling this run stops it.
func TestEditorTypeIsCanceledWithItsRun(t *testing.T) {
	bus := editorbus.New()
	cs, _ := editorSessionAt(t, bus, "?run=r1")
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	fakeEditor(t, bus, func(editorbus.Op) editorbus.Result {
		<-release
		return editorbus.Result{OK: true}
	})
	done := make(chan map[string]any, 1)
	go func() {
		m, _ := call(t, cs, "editor_type", map[string]any{"path": "/a.md", "content": "hello"})
		done <- m
	}()
	for i := 0; i < 100 && !bus.Busy(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	bus.CancelRun("r2")
	select {
	case m := <-done:
		t.Fatalf("canceled by another run: %v", m)
	case <-time.After(100 * time.Millisecond):
	}
	bus.CancelRun("r1")
	select {
	case m := <-done:
		if m["interrupted"] != true {
			t.Fatalf("m=%v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run cancel did not stop typing")
	}
}

func TestRunHeaderFromSharedConfig(t *testing.T) {
	var got []string
	h := withRunHeader(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get(runHeader))
	}))
	for _, c := range []struct{ query, header string }{
		{"?run=q", "h"}, {"", "h"}, {"", "${HYLO_RUN_ID}"}, {"", "${env:HYLO_RUN_ID}"}, {"", ""},
	} {
		r := httptest.NewRequest(http.MethodPost, "/mcp"+c.query, nil)
		if c.header != "" {
			r.Header.Set(runHeader, c.header)
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	if strings.Join(got, ",") != "q,h,,," {
		t.Fatalf("runs = %q", got)
	}
}
