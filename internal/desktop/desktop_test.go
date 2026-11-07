package desktop

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
)

func tracker(t *testing.T) (*Tracker, *mate.Store) {
	t.Helper()
	s, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return New(s, true), s
}

var xcode = Sample{App: "Xcode"}

func TestReportsBecomeSessions(t *testing.T) {
	tr, s := tracker(t)
	t0 := time.Now().Add(-time.Hour)
	for i := 0; i < 4; i++ {
		if err := tr.Report(xcode, t0.Add(time.Duration(i)*30*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	tr.Report(Sample{App: "Safari"}, t0.Add(2*time.Minute))
	tr.Report(Sample{App: "Safari", Idle: 600}, t0.Add(150*time.Second)) // idle: away
	tr.Report(xcode, t0.Add(30*time.Minute))

	ss, err := s.DesktopSessions(t0, time.Now())
	if err != nil || len(ss) != 3 {
		t.Fatalf("sessions = %+v, %v", ss, err)
	}
	if ss[0].App != "Xcode" || ss[0].End.Sub(ss[0].Start) != 90*time.Second || ss[1].App != "Safari" {
		t.Fatalf("sessions = %+v", ss)
	}
	st := tr.Now(t0.Add(30*time.Minute + time.Second))
	if !st.Present || st.App != "Xcode" || !st.BackAt.IsZero() {
		t.Fatalf("a half hour away is not a return: %+v", st)
	}
	if apps := Totals(ss); apps[0].App != "Xcode" || len(apps) != 2 {
		t.Fatalf("totals = %+v", apps)
	}
}

func TestShortGapDoesNotEraseReturn(t *testing.T) {
	tr, _ := tracker(t)
	back := time.Now().Add(-20 * time.Minute).Truncate(time.Second)
	if err := tr.Report(xcode, back.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := tr.Report(xcode, back); err != nil {
		t.Fatal(err)
	}
	st := tr.Now(back)
	if !st.Present || !st.BackAt.Equal(back) || st.AwayFor != 3*time.Hour {
		t.Fatalf("return = %+v", st)
	}
	// A lock, then a couple of minutes later: still the same return.
	if err := tr.Report(Sample{App: "Xcode", Locked: true}, back.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tr.Report(xcode, back.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// And a dropped report, which is just another short gap.
	if err := tr.Report(xcode, back.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	st = tr.Now(back.Add(5 * time.Minute))
	if !st.Present || !st.BackAt.Equal(back) || st.AwayFor != 3*time.Hour {
		t.Fatalf("a short gap replaced the return: %+v", st)
	}
	// Leaving again for long enough is a new return.
	again := back.Add(5*time.Minute + 2*time.Hour)
	if err := tr.Report(xcode, again); err != nil {
		t.Fatal(err)
	}
	st = tr.Now(again)
	if !st.BackAt.Equal(again) || st.AwayFor != 2*time.Hour {
		t.Fatalf("a new long absence is not a return: %+v", st)
	}
}

func TestNowAwayAndUnknown(t *testing.T) {
	tr, s := tracker(t)
	now := time.Now()
	if st := tr.Now(now); st.Known {
		t.Fatalf("never reported, yet known: %+v", st)
	}
	tr.Report(xcode, now.Add(-10*time.Minute))
	if st := tr.Now(now); !st.Known || st.Present || !st.AwaySince.Equal(now.Add(-10*time.Minute)) {
		t.Fatalf("silent desktop should read as away: %+v", st)
	}
	tr.Report(Sample{App: "Xcode", Locked: true}, now)
	if st := tr.Now(now); st.Present {
		t.Fatalf("locked yet present: %+v", st)
	}
	// A restart remembers when the user was last seen.
	if st := New(s, true).Now(now); !st.Known || st.Present {
		t.Fatalf("after restart: %+v", st)
	}
	off := New(s, false)
	if off.Report(xcode, now) != nil || off.Now(now).Known {
		t.Fatal("a disabled tracker keeps reports")
	}
}
