package mate

import (
	"fmt"
	"strings"
)

// TriggerHasScheduled reports whether t listens for scheduled events.
func TriggerHasScheduled(t MateTrigger) bool {
	for _, et := range t.EventTypes {
		if et == string(MateEventScheduled) {
			return true
		}
	}
	return false
}

// ValidateTriggers checks all triggers before persistence.
func ValidateTriggers(triggers []MateTrigger) error {
	for i, t := range triggers {
		if err := ValidateTrigger(t); err != nil {
			return fmt.Errorf("trigger %d: %w", i+1, err)
		}
	}
	return nil
}

// ValidateTrigger checks schedule/event consistency for one trigger.
func ValidateTrigger(t MateTrigger) error {
	hasScheduled := TriggerHasScheduled(t)
	schedule := strings.TrimSpace(t.Schedule)

	if hasScheduled {
		if len(t.EventTypes) != 1 {
			return fmt.Errorf("scheduled trigger cannot share event types with vault events")
		}
		if schedule == "" {
			return fmt.Errorf("schedule is required for scheduled triggers")
		}
		if _, err := ParseSchedule(schedule); err != nil {
			return err
		}
		return nil
	}
	if schedule != "" {
		return fmt.Errorf("schedule is only allowed when event type is scheduled")
	}
	for _, p := range t.PathPrefixes {
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("path prefix %q must start with /", p)
		}
	}
	return nil
}
