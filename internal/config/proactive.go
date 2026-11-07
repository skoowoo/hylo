package config

import (
	"fmt"
	"strings"
	"time"
)

// Hours parses ActiveHours into minutes after local midnight. end <= start
// means the window runs past midnight; "24:00" ends at midnight.
func (p ProactiveConfig) Hours() (start, end int, err error) {
	a, b, ok := strings.Cut(strings.TrimSpace(p.ActiveHours), "-")
	if !ok {
		return 0, 0, fmt.Errorf("want HH:MM-HH:MM, got %q", p.ActiveHours)
	}
	if start, err = clockMinutes(a); err != nil {
		return 0, 0, err
	}
	if end, err = clockMinutes(b); err != nil {
		return 0, 0, err
	}
	if start == 24*60 {
		return 0, 0, fmt.Errorf("start must be before 24:00")
	}
	return start, end, nil
}

// Gap parses MinGap; empty is no gap.
func (p ProactiveConfig) Gap() (time.Duration, error) {
	if strings.TrimSpace(p.MinGap) == "" {
		return 0, nil
	}
	return time.ParseDuration(p.MinGap)
}

func clockMinutes(s string) (int, error) {
	var h, m int
	s = strings.TrimSpace(s)
	if n, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil || n != 2 || len(s) != 5 {
		return 0, fmt.Errorf("want HH:MM, got %q", s)
	}
	if h < 0 || m < 0 || m > 59 || h > 24 || (h == 24 && m != 0) {
		return 0, fmt.Errorf("%q is not a time of day", s)
	}
	return h*60 + m, nil
}
