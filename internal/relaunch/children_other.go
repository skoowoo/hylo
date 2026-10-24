//go:build unix && !darwin && !linux

package relaunch

// children is unknown here; reapChildren still collects those that exit in time.
func children() []int { return nil }
