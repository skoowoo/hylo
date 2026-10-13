package storage

import (
	"testing"

	"github.com/hardhacker/hylo/internal/plugin"
)

// Pins and images never reach the watcher, so the vault has to say so itself.
func TestPinAndImageEmitEvents(t *testing.T) {
	v := newTestVault(t)
	p := Path("/a.md")
	if err := v.WriteNote(p, []byte("body"), ""); err != nil {
		t.Fatal(err)
	}
	var got []plugin.Event
	v.SetEventHook(func(e plugin.Event) { got = append(got, e) })

	if err := v.SetNotePinned(p, true); err != nil {
		t.Fatal(err)
	}
	img, err := v.SaveImageBytes(testPNG, ".png")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.DeleteImage(img.Dir, img.Name); err != nil {
		t.Fatal(err)
	}
	want := []plugin.Event{
		{Type: plugin.EventVaultPin, Path: "/a.md"},
		{Type: plugin.EventVaultImage, Path: img.Dir + "/" + img.Name},
		{Type: plugin.EventVaultImage, Path: img.Dir + "/" + img.Name},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Type != want[i].Type || got[i].Path != want[i].Path {
			t.Fatalf("event %d: got %+v", i, got[i])
		}
	}
}
