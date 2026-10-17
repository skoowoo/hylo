package plugin

import (
	"slices"
	"sync"
	"testing"
	"time"
)

type delivered struct {
	mu  sync.Mutex
	got []string
}

func (d *delivered) add(e Event) {
	d.mu.Lock()
	d.got = append(d.got, string(e.Type)+" "+e.Path+" "+e.Content)
	d.mu.Unlock()
}

func (d *delivered) snapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.got)
}

func write(path, version string) Event {
	return Event{Type: EventVaultWrite, Path: path, Content: version}
}

func TestCoalescerDeliversOnlyTheLastWriteAfterAPause(t *testing.T) {
	d := &delivered{}
	c := NewCoalescer(Window{Quiet: 60 * time.Millisecond, MaxWait: time.Second}, d.add)
	for _, v := range []string{"v1", "v2", "v3"} {
		c.Add(write("/a.md", v))
		time.Sleep(20 * time.Millisecond)
	}
	c.Add(write("/b.md", "b1"))
	if got := d.snapshot(); len(got) != 0 {
		t.Fatalf("writes delivered before the pause: %q", got)
	}
	time.Sleep(150 * time.Millisecond)
	got := d.snapshot()
	slices.Sort(got)
	if want := []string{"vault_write /a.md v3", "vault_write /b.md b1"}; !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCoalescerMaxWaitBoundsAnEndlessSession(t *testing.T) {
	d := &delivered{}
	c := NewCoalescer(Window{Quiet: 50 * time.Millisecond, MaxWait: 120 * time.Millisecond}, d.add)
	deadline := time.Now().Add(300 * time.Millisecond)
	for i := 0; time.Now().Before(deadline); i++ {
		c.Add(write("/a.md", "v"))
		time.Sleep(15 * time.Millisecond)
	}
	if n := len(d.snapshot()); n < 2 || n > 3 {
		t.Fatalf("a 300ms session with a 120ms max wait delivered %d writes", n)
	}
	c.Stop()
}

func TestCoalescerHeldCreateAbsorbsWritesOrVanishesWithDelete(t *testing.T) {
	d := &delivered{}
	c := NewCoalescer(Window{Quiet: 50 * time.Millisecond, MaxWait: time.Second, HoldCreates: true}, d.add)

	c.Add(Event{Type: EventVaultCreate, Path: "/new.md", Content: "h"})
	c.Add(write("/new.md", "hello"))
	c.Add(write("/new.md", "hello world"))
	c.Add(Event{Type: EventVaultCreate, Path: "/gone.md"})
	c.Add(write("/gone.md", "x"))
	c.Add(Event{Type: EventVaultDelete, Path: "/gone.md"})

	time.Sleep(150 * time.Millisecond)
	if got, want := d.snapshot(), []string{"vault_create /new.md hello world"}; !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCoalescerKeepsOrderAroundCreateAndDelete(t *testing.T) {
	d := &delivered{}
	c := NewCoalescer(Window{Quiet: time.Hour, MaxWait: time.Hour}, d.add)

	c.Add(write("/a.md", "v1"))
	c.Add(Event{Type: EventVaultDelete, Path: "/a.md"})
	c.Add(write("/b.md", "v1"))
	c.Add(Event{Type: EventFSCreate, Path: "/b.md"})
	c.Add(Event{Type: EventVaultPin, Path: "/c.md"})

	want := []string{
		"vault_delete /a.md ",
		"vault_write /b.md v1",
		"fs_create /b.md ",
		"vault_pin /c.md ",
	}
	if got := d.snapshot(); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	c.Stop()
}
