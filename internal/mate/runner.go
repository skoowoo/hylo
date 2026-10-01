package mate

import (
	"context"
	"log/slog"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
)

// RunResult carries the outcome of a completed trigger agent run.
type RunResult struct {
	Success     bool
	Duration    time.Duration
	LastMessage string
	EventType   MateEventType // the MateEvent type that triggered this run
}

// RunDoneHook is called when a trigger run reaches a terminal state.
type RunDoneHook func(m *Mate, result RunResult)

// RunFunc is called by the Runner to fire a background agent run.
// onDone must be called exactly once when the run reaches a terminal state.
type RunFunc func(ctx context.Context, m *Mate, convID, prompt string, ev MateEvent, onDone func(RunResult))

// Runner implements plugin.Plugin and fires mate triggers in response to vault events
// and configured schedules.
type Runner struct {
	store    *Store
	runFn    atomic.Value // stores RunFunc
	doneHook atomic.Value // stores RunDoneHook
	logger   *slog.Logger
	sem      chan struct{} // concurrency limiter
}

// NewRunner creates a Runner. Call SetRunFunc before the vault watcher fires events.
func NewRunner(store *Store, logger *slog.Logger) *Runner {
	return &Runner{
		store:  store,
		logger: logger,
		sem:    make(chan struct{}, 3),
	}
}

// SetRunFunc wires in the function that fires an agent run for a trigger match.
func (r *Runner) SetRunFunc(fn RunFunc) {
	r.runFn.Store(fn)
}

// SetRunDoneHook registers a hook called when a trigger run reaches a terminal state.
func (r *Runner) SetRunDoneHook(fn RunDoneHook) {
	r.doneHook.Store(fn)
}

func (r *Runner) Name() string { return "mate_runner" }

func (r *Runner) Start(ctx context.Context) error {
	go r.runScheduler(ctx)
	<-ctx.Done()
	return nil
}

func (r *Runner) Stop() error { return nil }

func (r *Runner) Notify(e plugin.Event) {
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
		if matched.SourceMateID != "" && t.MateID == matched.SourceMateID {
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

func isWechatSessionCommand(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "/new" || t == "/clear"
}

func (r *Runner) handleWechatSessionReset(mateID string, ev MateEvent) {
	if _, err := r.store.CreateConversation(mateID, ""); err != nil {
		r.logger.Warn("mate_runner: reset wechat session", "err", err)
		if ev.Reply != nil {
			ev.Reply(context.Background(), plugin.ReplyResult{Text: "对话重置失败，请稍后再试", Status: "failed"}) //nolint:errcheck
		}
		return
	}
	if ev.Reply != nil {
		ev.Reply(context.Background(), plugin.ReplyResult{Text: "新对话已开启", Status: "succeeded"}) //nolint:errcheck
	}
}

func (r *Runner) fireTrigger(t MateTrigger, me MateEvent) {
	isChat := me.Type == MateEventWechatMessage || me.Type == MateEventDiscordMessage
	if isChat && isWechatSessionCommand(me.Content) {
		go r.handleWechatSessionReset(t.MateID, me)
		return
	}

	fn, _ := r.runFn.Load().(RunFunc)
	if fn == nil {
		return
	}
	go func(tr MateTrigger, ev MateEvent) {
		select {
		case r.sem <- struct{}{}:
		default:
			r.logger.Warn("mate_runner: semaphore full, skipping trigger", "triggerID", tr.ID)
			return
		}
		defer func() { <-r.sem }()

		m, err := r.store.GetMate(tr.MateID)
		if err != nil || m == nil {
			return
		}

		var convID string
		if ev.Reply != nil {
			convID, err = r.store.GetOrCreateActiveChatConvID(m.ID)
		} else {
			convID, err = r.store.GetOrCreateDefaultTriggerConv(m.ID)
		}
		if err != nil {
			r.logger.Warn("mate_runner: get trigger conv", "err", err)
			return
		}

		prompt := renderPrompt(tr.Prompt, ev)
		fields := []any{"mate", m.Name, "trigger", tr.ID, "event", ev.Type, "conv", convID}
		if ev.Path != "" {
			fields = append(fields, "path", ev.Path)
		}
		r.logger.Info("mate_runner: trigger fired", fields...)

		start := time.Now()
		dh, _ := r.doneHook.Load().(RunDoneHook)
		onDone := func(result RunResult) {
			result.Duration = time.Since(start)
			result.EventType = ev.Type
			if dh != nil {
				dh(m, result)
			}
			if result.Success {
				// Notify other mates that this run succeeded.
				// Path carries the source mate name; PathPrefixes on agent_run_completed triggers filters by mate name.
				// SourceMateID prevents the same mate from triggering itself.
				r.dispatchMateEvents([]MateEvent{{
					Type:         MateEventAgentRunCompleted,
					Path:         m.Name,
					Content:      result.LastMessage,
					FiredAt:      time.Now(),
					SourceMateID: m.ID,
				}})
			}
		}
		fn(context.Background(), m, convID, prompt, ev, onDone)
	}(t, me)
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
// Events without a path (scheduled, wechat_message) always pass. An empty prefix list passes all.
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
	name := path.Base(me.Path)
	name = strings.TrimSuffix(name, ".md")
	r := tmpl
	r = strings.ReplaceAll(r, "{Path}", me.Path)
	r = strings.ReplaceAll(r, "{Name}", name)
	r = strings.ReplaceAll(r, "{Content}", me.Content)
	r = strings.ReplaceAll(r, "{WechatUserID}", me.WechatUserID)
	r = strings.ReplaceAll(r, "{DiscordChannelID}", me.DiscordChannelID)
	r = strings.ReplaceAll(r, "{DiscordUserID}", me.DiscordUserID)
	r = strings.ReplaceAll(r, "{Now}", fired.Format(time.RFC3339))
	r = strings.ReplaceAll(r, "{Date}", fired.Format("2006-01-02"))
	r = strings.ReplaceAll(r, "{Time}", fired.Format("15:04"))
	return r
}
