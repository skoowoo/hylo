package live

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

func TestNoteChangesFromVaultEvents(t *testing.T) {
	cases := []struct {
		e    plugin.Event
		want Change
	}{
		{plugin.Event{Type: plugin.EventFSCreate, Path: "/a.md"}, Change{Topic: TopicNotes, Op: "create", Path: "/a.md"}},
		{plugin.Event{Type: plugin.EventVaultWrite, Path: "/a.md"}, Change{Topic: TopicNotes, Op: "write", Path: "/a.md"}},
		{plugin.Event{Type: plugin.EventFSDelete, Path: "/dir", IsDir: true}, Change{Topic: TopicNotes, Op: "delete", Path: "/dir", IsDir: true}},
		{plugin.Event{Type: plugin.EventVaultRename, Path: "/b.md", OldPath: "/a.md"}, Change{Topic: TopicNotes, Op: "rename", Path: "/b.md", OldPath: "/a.md"}},
		{plugin.Event{Type: plugin.EventVaultPin, Path: "/a.md"}, Change{Topic: TopicNotes, Op: "pin", Path: "/a.md"}},
		{plugin.Event{Type: plugin.EventVaultImage, Path: "/_assets/x.png"}, Change{Topic: TopicNotes, Op: "image", Path: "/_assets/x.png"}},
		{plugin.Event{Type: plugin.EventVaultWrite, Path: "/_shorts/d.md", ShortID: "d-090000", ShortHash: "h"},
			Change{Topic: TopicNotes, Op: "write", Path: "/_shorts/d.md", Data: ShortRef{ID: "d-090000", Hash: "h"}}},
	}
	for _, c := range cases {
		got, ok := noteChange(c.e)
		if !ok || got != c.want {
			t.Errorf("%s: got %+v", c.e.Type, got)
		}
	}
	if _, ok := noteChange(plugin.Event{Type: plugin.EventCompileRequested}); ok {
		t.Error("compile is not a note change")
	}
	// The daily file's own write already carries the entry.
	if _, ok := noteChange(plugin.Event{Type: plugin.EventVaultShortAppend}); ok {
		t.Error("short append is not a separate note change")
	}
}

func readEvents(t *testing.T, f *Feed, n int, publish func()) []string {
	t.Helper()
	ts := httptest.NewServer(f)
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Scan() // ": live" means subscribed
	publish()
	var got []string
	for len(got) < n && sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "event: ") || strings.HasPrefix(line, "data: ") {
			got = append(got, line)
		}
	}
	return got
}

func TestFeedStreamsChanges(t *testing.T) {
	f := NewFeed()
	got := readEvents(t, f, 4, func() {
		f.Publish(Change{Topic: TopicNotes, Op: "create", Path: "/a.md"})
		f.Notify(TopicBots)()
	})
	want := []string{
		"event: change", `data: {"topic":"notes","op":"create","path":"/a.md"}`,
		"event: change", `data: {"topic":"bots","op":"change"}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

func TestFeedAsksASlowPageToResync(t *testing.T) {
	f := NewFeed()
	s := f.subscribe()
	for i := 0; i < subBuffer+5; i++ {
		f.Publish(Change{Topic: TopicNotes, Op: "write", Path: "/a.md"})
	}
	select {
	case <-s.overflow:
	default:
		t.Fatal("no resync after the buffer filled")
	}
	f.unsubscribe(s)
}
