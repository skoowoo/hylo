package pulse

import (
	"fmt"
	"math"
	"time"
)

// day is the active window that holds a moment, as configured hours.
type day struct {
	start, end int // minutes after local midnight; end <= start runs past midnight
}

// window returns the active span holding now; ok is false while the bot sleeps.
func (d day) window(now time.Time) (from, to time.Time, ok bool) {
	length := d.end - d.start
	if length <= 0 {
		length += 24 * 60
	}
	mid := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// A window that began yesterday may still be open.
	for _, base := range []time.Time{mid, mid.AddDate(0, 0, -1)} {
		from = base.Add(time.Duration(d.start) * time.Minute)
		to = from.Add(time.Duration(length) * time.Minute)
		if !now.Before(from) && now.Before(to) {
			return from, to, true
		}
	}
	return time.Time{}, time.Time{}, false
}

// chance is the probability of a round in this minute: the rounds left
// spread over the minutes left, raised by what's going on.
func chance(left int, until time.Duration, boost float64) float64 {
	if left <= 0 {
		return 0
	}
	minutes := math.Max(until.Minutes(), 1)
	return math.Min(1, float64(left)/minutes*(1+boost))
}

// Signal tuning. A new short counts most, being a thought on its own; then a
// note written, then one the user read. All fade over signalFade. The user
// coming back after awayGap counts until a round sees it.
const (
	shortWeight  = 0.6
	writeWeight  = 0.5
	readWeight   = 0.2
	signalFade   = 45 * time.Minute
	signalWindow = 3 * time.Hour
	maxBoost     = 3.0
	awayGap      = 4 * time.Hour
	backWithin   = 30 * time.Minute
	returnWeight = 1.5
)

// signalLookback is how far back the signals need changes and the user's activity.
const signalLookback = awayGap + backWithin

func fade(weight float64, at, now time.Time) float64 {
	return weight * math.Exp(-now.Sub(at).Minutes()/signalFade.Minutes())
}

// span reads a duration the way people say it: "2h 10m", "45m".
func span(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
