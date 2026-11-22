package mate

import (
	"fmt"
	"strings"
	"time"
)

// ProactiveConfig paces the main bot's own rounds (internal/pulse): it wakes
// at irregular times, looks around, and only speaks when something is worth it.
type ProactiveConfig struct {
	Enabled bool `json:"enabled"`
	// WakesPerDay caps the rounds in one active day; most end in silence.
	WakesPerDay int `json:"wakes_per_day"`
	// ActiveHours is when the bot is awake, "HH:MM-HH:MM" local time; the end may pass midnight.
	ActiveHours string `json:"active_hours"`
	// MinGap is the least time between two rounds (Go duration).
	MinGap string `json:"min_gap"`
}

// DefaultProactiveConfig returns the default settings for the main bot's proactive rounds.
func DefaultProactiveConfig() ProactiveConfig {
	return ProactiveConfig{
		Enabled:     true,
		WakesPerDay: 24,
		ActiveHours: "08:00-24:00",
		MinGap:      "20m",
	}
}

// Validate checks ProactiveConfig fields.
func (p ProactiveConfig) Validate() error {
	if p.Enabled || p.ActiveHours != "" {
		if _, _, err := p.Hours(); err != nil {
			return fmt.Errorf("active_hours: %w", err)
		}
	}
	if gap, err := p.Gap(); err != nil {
		return fmt.Errorf("min_gap: %w", err)
	} else if gap < 0 {
		return fmt.Errorf("min_gap: must be >= 0")
	}
	if p.WakesPerDay < 0 {
		return fmt.Errorf("wakes_per_day: must be >= 0")
	}
	return nil
}

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
