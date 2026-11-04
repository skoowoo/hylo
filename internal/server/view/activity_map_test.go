package view

import (
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/storage"
)

func TestActivityLevels(t *testing.T) {
	if got, want := activityLevels([]int{0, 1, 10, 11, 31, 61}), []int{0, 1, 1, 2, 3, 4}; !slices.Equal(got, want) {
		t.Fatalf("levels = %v, want %v", got, want)
	}
}

func TestActivityScoreWeighsWritesOverReads(t *testing.T) {
	write, reads := activityScore(mate.ActivityCount{Write: 1}), activityScore(mate.ActivityCount{Read: 9})
	if write <= reads || activityScore(mate.ActivityCount{Read: 10}) != write {
		t.Fatalf("one write = %d, nine reads = %d", write, reads)
	}
}

func TestActivityDayListsWhoTouchedEachNote(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []storage.Path{"/a.md", "/b.md", "/gone.md", "/_memory/_state.md"} {
		if err := v.WriteNote(p, []byte("x"), ""); err != nil {
			t.Fatal(err)
		}
	}
	ms, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ms.Close() })
	bot := &mate.Mate{Name: "Mate", Enabled: true}
	if err := ms.CreateMate(bot); err != nil {
		t.Fatal(err)
	}
	conv, _ := ms.CreateConversation(bot.ID, "")
	msg, err := ms.InsertMessage(mate.Message{ConversationID: conv.ID, Role: "assistant", MateID: bot.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := localMidnight(time.Now()).Add(12 * time.Hour)
	if err := ms.InsertMessageNoteAccess(msg.ID, []mate.NoteAccessEntry{
		{Path: "/a.md", Action: mate.NoteAccessRead, At: now.Add(-time.Minute)},
		{Path: "/gone.md", Action: mate.NoteAccessWrite, At: now.Add(-time.Minute)},
		{Path: "/_memory/_state.md", Action: mate.NoteAccessWrite, At: now.Add(-time.Minute)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.RecordUserVisit("/b.md", mate.NoteAccessWrite, now); err != nil {
		t.Fatal(err)
	}
	if err := v.DeleteNote("/gone.md"); err != nil {
		t.Fatal(err)
	}

	vh := NewView(v, nil, nil)
	vh.SetMates(ms)
	today := now.Format(time.DateOnly)
	html, err := vh.renderHomeSectionHTML(httptest.NewRequest("GET", "/home/section?type=activity&day="+today, nil))
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	for _, want := range []string{
		`class="act-map"`, `class="act-cell is-selected"`, `class="badge badge--accent home-list-filter"`,
		`data-note-path="/a.md"`, `data-bot="Mate"`, `data-note-path="/b.md"`, `>Edited<`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, `data-note-path="/gone.md"`) {
		t.Error("a deleted note is listed")
	}
	if strings.Contains(out, `data-note-path="/_memory/_state.md"`) {
		t.Error("a system note is listed")
	}
	recent, err := vh.renderHomeSectionHTML(httptest.NewRequest("GET", "/home/section?type=activity", nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(recent), `data-note-path="/_memory/_state.md"`) {
		t.Error("a system note is in the recent list")
	}
	if strings.Count(out, `class="act-week"`) != activityWeeks {
		t.Errorf("weeks = %d, want %d", strings.Count(out, `class="act-week"`), activityWeeks)
	}
	if strings.Index(out, `data-note-path="/b.md"`) > strings.Index(out, `data-note-path="/a.md"`) {
		t.Error("the latest touched note should come first")
	}
}

// Icons are looked up when a section renders, so a misnamed empty-state icon
// only shows up as a 500 on an empty vault.
func TestEverySectionRendersOnAnEmptyVault(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ms, err := mate.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ms.Close() })
	vh := NewView(v, nil, nil)
	vh.SetMates(ms)
	for _, q := range []string{"type=pinned", "type=activity", "type=memory", "type=knowledge", "type=folder&path=%2F", "type=shorts", "type=images"} {
		if _, err := vh.renderHomeSectionHTML(httptest.NewRequest("GET", "/home/section?"+q, nil)); err != nil {
			t.Errorf("%s: %v", q, err)
		}
	}
}

func TestActivityShareSplitsScoreAndFoldsOthers(t *testing.T) {
	today := localMidnight(time.Now())
	byBot := map[string]mate.ActivityCount{"a": {Write: 2}, "b": {Read: 2}, "c": {Read: 1}, "d": {Read: 1}, "gone1": {Read: 6}, "gone2": {Read: 6}}
	days := []mate.ActivityDay{
		{Date: today.AddDate(0, 0, -activityShareDays), User: mate.ActivityCount{Write: 9}},
		{Date: today, User: mate.ActivityCount{Write: 1}, ByBot: byBot},
	}
	names := map[string]string{"a": "A", "b": "B", "c": "C", "d": "D"}
	got := activityShareRows(days, today.AddDate(0, 0, 1-activityShareDays), names)
	var rows []string
	var pcts []int
	for _, r := range got {
		rows = append(rows, r.Name)
		pcts = append(pcts, r.Pct)
	}
	// Two removed bots fold into one row, which outranks you on their sum.
	if want := []string{"A", "Removed bots", "You", "Others"}; !slices.Equal(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
	if want := []int{43, 26, 22, 9}; !slices.Equal(pcts, want) || got[0].Kind != "bot" || got[1].Kind != "other" || got[2].Kind != "user" || got[3].Kind != "other" {
		t.Errorf("share = %+v", got)
	}
	if tiny := activityShareRows([]mate.ActivityDay{{Date: today, User: mate.ActivityCount{Write: 100}, ByBot: map[string]mate.ActivityCount{"a": {Read: 1}}}}, today, names); tiny[1].Pct != 1 {
		t.Errorf("a sliver shows %d%%, want 1%%", tiny[1].Pct)
	}
}
