package mate

import (
	"context"
	"errors"
	"log/slog"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/prompt"
)

// RunResult is how a chat turn ended.
type RunResult struct {
	Success     bool
	LastMessage string
}

// Runner implements plugin.Plugin and wakes bots whose triggers match vault
// events or come due.
type Runner struct {
	store  *Store
	wakeFn atomic.Value // stores WakeFunc
	logger *slog.Logger
	settle *plugin.Coalescer
}

// NewRunner creates a Runner. Call SetWakeFunc before the vault watcher fires events.
func NewRunner(store *Store, logger *slog.Logger) *Runner {
	r := &Runner{store: store, logger: logger}
	// note_created and note_updated wait for the edit session to end instead
	// of waking a bot on the first keystroke and every autosave.
	r.settle = plugin.NewCoalescer(plugin.AgentWindow, r.dispatch)
	return r
}

// SetWakeFunc wires in what runs a wake in a bot's chat.
func (r *Runner) SetWakeFunc(fn WakeFunc) {
	r.wakeFn.Store(fn)
}

func (r *Runner) Name() string { return "mate_runner" }

func (r *Runner) Start(ctx context.Context) error {
	go r.runScheduler(ctx)
	<-ctx.Done()
	return nil
}

func (r *Runner) Stop() error {
	r.settle.Stop()
	return nil
}

func (r *Runner) Notify(e plugin.Event) {
	r.settle.Add(e)
}

func (r *Runner) dispatch(e plugin.Event) {
	mateEvents := Translate(e)
	if len(mateEvents) == 0 {
		return
	}
	r.dispatchMateEvents(mateEvents)
}

func (r *Runner) dispatchMateEvents(mateEvents []MateEvent) {
	triggers, err := r.store.ListAllEnabledTriggers()
	if err != nil {
		r.logger.Warn("mate_runner: list triggers", "err", err)
		return
	}
	for _, t := range triggers {
		if TriggerHasScheduled(t) {
			continue
		}
		matched, ok := matchMateEvent(t.EventTypes, mateEvents)
		if !ok {
			continue
		}
		if !matchPathPrefixes(t.PathPrefixes, matched) {
			continue
		}
		r.fireTrigger(t, matched)
	}
}

func (r *Runner) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tickScheduled()
		}
	}
}

func (r *Runner) tickScheduled() {
	triggers, err := r.store.ListScheduledTriggers()
	if err != nil {
		r.logger.Warn("mate_runner: list scheduled triggers", "err", err)
		return
	}
	now := time.Now()
	for _, t := range triggers {
		if !TriggerHasScheduled(t) {
			continue
		}
		sched, err := ParseSchedule(t.Schedule)
		if err != nil {
			r.logger.Warn("mate_runner: invalid schedule", "triggerID", t.ID, "err", err)
			continue
		}
		if !sched.Due(t.LastFiredAt, now) {
			continue
		}
		if err := r.store.UpdateTriggerLastFiredAt(t.ID, now); err != nil {
			r.logger.Warn("mate_runner: update last_fired_at", "triggerID", t.ID, "err", err)
			continue
		}
		t.LastFiredAt = now
		me := MateEvent{Type: MateEventScheduled, FiredAt: now}
		r.logger.Info("mate_runner: schedule fired", "triggerID", t.ID, "schedule", t.Schedule)
		r.fireTrigger(t, me)
	}
}

func (r *Runner) fireTrigger(t MateTrigger, me MateEvent) error {
	fn, _ := r.wakeFn.Load().(WakeFunc)
	if fn == nil {
		return errors.New("mate runner: wake unavailable")
	}
	m, err := r.store.TurnBot(t.MateID, t.ID)
	if err != nil {
		return err
	}
	w := Wake{
		Event: me.Type, TriggerID: t.ID, Prompt: renderPrompt(t.Prompt, me),
		Key: t.ID + "\x00" + me.Path, Path: me.Path,
	}
	fields := []any{"mate", m.Name, "trigger", t.ID, "event", me.Type}
	if me.Path != "" {
		fields = append(fields, "path", me.Path)
	}
	r.logger.Info("mate_runner: trigger fired", fields...)
	return fn(m, w)
}

// FireNow fires t once, off its schedule, the way a scheduled fire would.
func (r *Runner) FireNow(t MateTrigger) error {
	return r.fireTrigger(t, MateEvent{Type: MateEventScheduled, FiredAt: time.Now()})
}

// matchMateEvent returns the first MateEvent whose type appears in triggerTypes.
func matchMateEvent(triggerTypes []string, events []MateEvent) (MateEvent, bool) {
	if len(triggerTypes) == 0 {
		if len(events) > 0 {
			return events[0], true
		}
		return MateEvent{}, false
	}
	for _, tt := range triggerTypes {
		for _, me := range events {
			if tt == string(me.Type) {
				return me, true
			}
		}
	}
	return MateEvent{}, false
}

// matchPathPrefixes returns true when the event should pass through the trigger's path filter.
// Events without a path (scheduled) always pass. An empty prefix list passes all.
func matchPathPrefixes(prefixes []string, me MateEvent) bool {
	if len(prefixes) == 0 || me.Path == "" {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(me.Path, p) {
			return true
		}
	}
	return false
}

func renderPrompt(tmpl string, me MateEvent) string {
	fired := me.FiredAt
	if fired.IsZero() {
		fired = time.Now()
	}
	if tmpl == "" {
		if me.Path != "" {
			return me.Path
		}
		return fired.Format(time.RFC3339)
	}
	return prompt.Fill(tmpl, map[string]string{
		"Path":    me.Path,
		"Name":    strings.TrimSuffix(path.Base(me.Path), ".md"),
		"Content": me.Content,
		"Now":     fired.Format(time.RFC3339),
		"Date":    fired.Format("2006-01-02"),
		"Time":    fired.Format("15:04"),
	})
}
