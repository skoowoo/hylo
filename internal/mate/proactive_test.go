package mate

import "testing"

func TestProactiveHours(t *testing.T) {
	for _, c := range []struct {
		in         string
		start, end int
		bad        bool
	}{
		{in: "08:00-24:00", start: 480, end: 1440},
		{in: "07:30-01:00", start: 450, end: 60},
		{in: " 09:00 - 18:15 ", start: 540, end: 1095},
		{in: "8:00-24:00", bad: true},
		{in: "24:00-08:00", bad: true},
		{in: "08:00-24:30", bad: true},
		{in: "08:00", bad: true},
	} {
		s, e, err := ProactiveConfig{ActiveHours: c.in}.Hours()
		if c.bad {
			if err == nil {
				t.Errorf("%q: want error", c.in)
			}
			continue
		}
		if err != nil || s != c.start || e != c.end {
			t.Errorf("%q = %d,%d,%v; want %d,%d", c.in, s, e, err, c.start, c.end)
		}
	}
}

func TestValidateProactive(t *testing.T) {
	if err := (ProactiveConfig{ActiveHours: "nope", MinGap: "soon"}).Validate(); err == nil {
		t.Fatal("want errors for bad hours and gap")
	}
	if err := DefaultProactiveConfig().Validate(); err != nil {
		t.Fatalf("default proactive config invalid: %v", err)
	}
	if err := (ProactiveConfig{Enabled: true}).Validate(); err == nil {
		t.Fatal("turned on with no hours, so rounds could never start")
	}
}
