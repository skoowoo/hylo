// Package pulse gives the main bot rounds of its own: it wakes at irregular
// times, shaped by a daily budget and what the user is doing, looks around in
// a session of its own, and only when something is worth it hands a brief to
// its chat, where it does the work and tells the user. Triggers are separate.
package pulse

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/prompt"
	"github.com/hardhacker/hylo/internal/storage"
)

// RoundFunc runs a round's request in a session of its own and returns how it ended.
type RoundFunc func(ctx context.Context, m *mate.Mate, request, model string) mate.RunResult

// Pulse paces the main bot's rounds. It implements plugin.Plugin for its lifetime.
type Pulse struct {
	store  *mate.Store
	desk   *desktop.Tracker // nil without a store; reports nothing while proactive.desktop is off
	cfg    config.ProactiveConfig
	hours  day
	gap    time.Duration
	logger *slog.Logger

	round    RoundFunc
	wake     mate.WakeFunc
	idle     func(mateID string) bool
	pushable func(mateID string) []string
	tasks    func(from, to time.Time) ([]storage.Task, error)

	now  func() time.Time
	roll func() float64

	changes changes

	mu      sync.Mutex
	running bool
	ctx     context.Context
	// dueAfter holds due agenda items back after a round for them failed.
	dueAfter time.Time
	// last is the chance of the latest minute, for the dev page.
	last Odds
	// looked holds the findings of leads that look now and then.
	looked map[string]looked
}

// Odds is how the latest minute was weighed.
type Odds struct {
	At      time.Time `json:"at"`
	Chance  float64   `json:"chance"`
	Boost   float64   `json:"boost"`
	Signals string    `json:"signals"`
	Reach   string    `json:"reach,omitempty"` // where a message would find the user
	Skip    string    `json:"skip,omitempty"`  // why no round could start
}

// Deps are what rounds run on.
type Deps struct {
	Store *mate.Store
	Desk  *desktop.Tracker
	Round RoundFunc
	// Wake hands a brief to the bot's chat.
	Wake mate.WakeFunc
	// Idle says whether the bot's chat is quiet enough for a round.
	Idle func(mateID string) bool
	// Pushable lists the bot's connected apps that can reach the user's phone; may be nil.
	Pushable func(mateID string) []string
	// Tasks lists open tasks in notes last changed in [from, to); may be nil.
	Tasks func(from, to time.Time) ([]storage.Task, error)
	// ChangesFile keeps the vault's recent changes across restarts; empty keeps them in memory.
	ChangesFile string
}

// New makes a Pulse.
func New(d Deps, cfg config.ProactiveConfig, logger *slog.Logger) (*Pulse, error) {
	start, end, err := cfg.Hours()
	if err != nil {
		return nil, fmt.Errorf("proactive.active_hours: %w", err)
	}
	gap, err := cfg.Gap()
	if err != nil {
		return nil, fmt.Errorf("proactive.min_gap: %w", err)
	}
	p := &Pulse{
		store: d.Store, desk: d.Desk, cfg: cfg, hours: day{start, end}, gap: gap, logger: logger,
		round: d.Round, wake: d.Wake, idle: d.Idle, pushable: d.Pushable, tasks: d.Tasks,
		now: time.Now, roll: rand.Float64, ctx: context.Background(), looked: map[string]looked{},
	}
	for _, name := range cfg.Mute {
		if !slices.ContainsFunc(leads, func(l lead) bool { return l.nudge && l.name == name }) {
			logger.Warn("pulse: proactive.mute names no nudge", "name", name)
		}
	}
	if err := p.changes.load(d.ChangesFile, p.now()); err != nil {
		logger.Warn("pulse: load changes", "err", err)
	}
	return p, nil
}

func (p *Pulse) Name() string { return "pulse" }

func (p *Pulse) Start(ctx context.Context) error {
	p.mu.Lock()
	p.ctx = ctx
	p.mu.Unlock()
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return p.changes.save()
		case <-t.C:
			p.tick()
			if err := p.changes.save(); err != nil {
				p.logger.Warn("pulse: save changes", "err", err)
			}
		}
	}
}

func (p *Pulse) Stop() error { return p.changes.save() }

// Notify keeps the notes that change, for the signals and the digest.
func (p *Pulse) Notify(e plugin.Event) { p.changes.add(e, p.now()) }

var _ plugin.Plugin = (*Pulse)(nil)

// roundTimeout ends a round that keeps exploring; the prompt says so.
var roundTimeout = 5 * time.Minute

// dueRetry is how long due agenda items wait after a round for them failed.
var dueRetry = 10 * time.Minute

// ErrBusy refuses a round while another is running.
var ErrBusy = errors.New("a round is already running")

// tick weighs this minute and starts a round if it comes up.
func (p *Pulse) tick() {
	m, why, due := p.weigh()
	if m == nil {
		return
	}
	_ = p.begin(m, why, due)
}

// weigh decides whether a round starts now: a due agenda item always starts
// one; otherwise the budget, the gap and the user's activity set the odds.
func (p *Pulse) weigh() (m *mate.Mate, why string, due []mate.AgendaItem) {
	now := p.now()
	odds := Odds{At: now}
	defer func() {
		p.mu.Lock()
		p.last = odds
		p.mu.Unlock()
	}()
	m, err := p.mainBot()
	if err != nil || m == nil {
		odds.Skip = "no enabled main bot"
		return nil, "", nil
	}
	from, to, awake := p.hours.window(now)
	switch {
	case !awake:
		odds.Skip = "asleep"
		return nil, "", nil
	case p.isRunning():
		odds.Skip = "a round is running"
		return nil, "", nil
	case !p.idle(m.ID):
		odds.Skip = "the chat is in use"
		return nil, "", nil
	}
	if due, err = p.store.DueAgenda(m.ID, now); err != nil {
		p.logger.Warn("pulse: due agenda", "err", err)
	}
	if len(due) > 0 {
		p.mu.Lock()
		after := p.dueAfter
		p.mu.Unlock()
		if now.Before(after) {
			odds.Skip = "the last round for due agenda items failed; retrying at " + after.Format("15:04")
			return nil, "", nil
		}
		var texts []string
		for _, it := range due {
			texts = append(texts, fmt.Sprintf("%q", it.Text))
		}
		return m, "Agenda items came due: " + strings.Join(texts, ", ") + ".", due
	}
	n, last, err := p.store.PulseCount(m.ID, from)
	if err != nil {
		p.logger.Warn("pulse: count rounds", "err", err)
		return nil, "", nil
	}
	if gap := now.Sub(last); gap < p.gap {
		odds.Skip = fmt.Sprintf("last round %s ago", gap.Round(time.Minute))
		return nil, "", nil
	}
	settled, err := p.store.PulseSettled(m.ID)
	if err != nil {
		p.logger.Warn("pulse: settled", "err", err)
	}
	s := p.scene(m, settled, now)
	sv := p.survey(s)
	odds.Boost, odds.Signals = sv.boost, sv.why()
	odds.Chance = chance(p.cfg.WakesPerDay-n, to.Sub(now), odds.Boost)
	odds.Reach = s.reach.Where
	if f, ok := reachOdds[s.reach.Where]; ok {
		odds.Chance *= f
	}
	if odds.Chance == 0 {
		odds.Skip = "no rounds left today"
		return nil, "", nil
	}
	if p.roll() >= odds.Chance {
		return nil, "", nil
	}
	why = "Nothing in particular woke you; you look around now and then."
	if odds.Signals != "" {
		why = "You woke because " + odds.Signals + "."
	}
	return m, why, nil
}

// RunNow starts a round at once, outside the odds, as the dev page asks.
func (p *Pulse) RunNow() error {
	m, err := p.mainBot()
	if err != nil {
		return err
	}
	if m == nil {
		return errors.New("there is no enabled main bot")
	}
	return p.begin(m, "The user started this round by hand, to see how you look around.", nil)
}

// begin starts a round in the background.
func (p *Pulse) begin(m *mate.Mate, why string, due []mate.AgendaItem) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return ErrBusy
	}
	p.running = true
	ctx := p.ctx
	p.mu.Unlock()
	go func() {
		defer func() {
			p.mu.Lock()
			p.running = false
			p.mu.Unlock()
		}()
		p.run(ctx, m, why, due)
	}()
	return nil
}

func (p *Pulse) isRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// run does one round: look around, then stay silent or hand a brief to the chat.
func (p *Pulse) run(ctx context.Context, m *mate.Mate, why string, due []mate.AgendaItem) {
	now := p.now()
	_, last, _ := p.store.PulseCount(m.ID, now)
	since, _ := p.store.PulseSettled(m.ID)
	pr, err := p.store.StartPulse(m.ID, why, now)
	if err != nil {
		p.logger.Warn("pulse: start", "err", err)
		return
	}
	p.logger.Info("pulse: round", "mate", m.Name, "why", why)

	digest, nudges := p.digest(m, last, since, now)
	rctx, cancel := context.WithTimeout(ctx, roundTimeout)
	res := p.round(rctx, m, prompt.PulseRound(why, digest), p.cfg.Model)
	timedOut := errors.Is(rctx.Err(), context.DeadlineExceeded)
	cancel()
	pr.EndedAt, pr.RunID, pr.Wrote = p.now(), res.RunID, res.Wrote
	reply := strings.TrimSpace(res.LastMessage)
	silent := prompt.IsSilent(reply)
	if silent {
		reply = strings.TrimSpace(strings.TrimSuffix(reply, prompt.SilentReply))
	}
	if timedOut {
		res.Success, silent = false, false
		reply = strings.TrimSpace("Timed out after " + roundTimeout.String() + ". " + reply)
	}
	switch {
	case !res.Success && len(res.Wrote) == 0:
		pr.Decision, pr.Summary = mate.PulseFailed, reply
	case silent && len(res.Wrote) == 0:
		pr.Decision, pr.Summary = mate.PulseSilent, reply
	default:
		// Whatever a round wrote, the user hears about.
		pr.Decision = mate.PulseSpeak
		pr.Brief, pr.Nudged = nudged(reply, nudges)
	}
	if err := p.store.EndPulse(pr); err != nil {
		p.logger.Warn("pulse: end", "err", err)
	}
	p.settleDue(due, pr.Decision != mate.PulseFailed)
	p.logger.Info("pulse: decided", "mate", m.Name, "decision", pr.Decision, "wrote", len(pr.Wrote))
	if pr.Decision != mate.PulseSpeak {
		return
	}
	brief := pr.Brief
	if brief == "" {
		brief = "Your rounds ended before you wrote a brief."
	}
	phone := p.reach(m.ID, p.desk.Now(p.now()), p.now()).onPhone()
	// No key: each brief is its own news, and its delivery is told back by Ref.
	w := mate.Wake{Event: mate.MateEventPulse, Prompt: prompt.PulseWake(brief, pr.Wrote, phone, len(pr.Nudged) > 0), Ref: pr.ID}
	if err := p.wake(m, w); err != nil {
		p.logger.Warn("pulse: hand brief to chat", "err", err)
		_ = p.store.SetPulseDelivery(pr.ID, "dropped")
	}
}

// settleDue marks due items as woken for once a round saw them through; after
// a failed round they come back once dueRetry has passed.
func (p *Pulse) settleDue(due []mate.AgendaItem, ok bool) {
	if len(due) == 0 {
		return
	}
	if !ok {
		p.mu.Lock()
		p.dueAfter = p.now().Add(dueRetry)
		p.mu.Unlock()
		return
	}
	ids := make([]string, len(due))
	for i, it := range due {
		ids[i] = it.ID
	}
	if err := p.store.MarkAgendaFired(ids, p.now()); err != nil {
		p.logger.Warn("pulse: fire agenda", "err", err)
	}
}

// WakeEnded records how the chat turn of round id's brief ended.
func (p *Pulse) WakeEnded(id, status string, silent bool) {
	d := "failed"
	switch {
	case status == "succeeded" && silent:
		d = "silent"
	case status == "succeeded":
		d = "sent"
	case status == "canceled":
		d = "canceled"
	}
	if err := p.store.SetPulseDelivery(id, d); err != nil {
		p.logger.Warn("pulse: delivery", "err", err)
	}
}

// mainBot is the bot rounds are for: the main bot, while it is enabled.
func (p *Pulse) mainBot() (*mate.Mate, error) {
	m, err := p.store.MainBot()
	if err != nil || m == nil || !m.Enabled {
		return nil, err
	}
	return m, nil
}

// nudgedLine ends a brief that raises a nudge: `[nudged: writing]`.
var nudgedLine = regexp.MustCompile(`(?m)^\s*\[nudged:\s*([^\]]*?)\s*\]\s*$`)

// nudged takes the nudged line off a brief and, when it names a live nudge,
// returns it with the items the round saw for it.
func nudged(brief string, live []liveNudge) (string, map[string][]string) {
	var out map[string][]string
	if m := nudgedLine.FindStringSubmatch(brief); m != nil {
		if i := slices.IndexFunc(live, func(n liveNudge) bool { return n.name == m[1] }); i >= 0 {
			out = map[string][]string{m[1]: live[i].items}
		}
	}
	return strings.TrimSpace(nudgedLine.ReplaceAllString(brief, "")), out
}

// Status is the pacing state, for the dev page.
type Status struct {
	Bot         string    `json:"bot"`
	Awake       bool      `json:"awake"`
	DayFrom     time.Time `json:"dayFrom,omitzero"`
	DayTo       time.Time `json:"dayTo,omitzero"`
	Rounds      int       `json:"rounds"`
	PerDay      int       `json:"perDay"`
	LastRound   time.Time `json:"lastRound,omitzero"`
	Running     bool      `json:"running"`
	Odds        Odds      `json:"odds"`
	ActiveHours string    `json:"activeHours"`
	MinGap      string    `json:"minGap"`
	Model       string    `json:"model"`
	// Nudges is where each nudge stands now.
	Nudges []NudgeState `json:"nudges,omitempty"`
}

// Status reports where pacing stands now.
func (p *Pulse) Status() (Status, *mate.Mate) {
	now := p.now()
	p.mu.Lock()
	st := Status{Running: p.running, Odds: p.last, PerDay: p.cfg.WakesPerDay,
		ActiveHours: p.cfg.ActiveHours, MinGap: p.gap.String(), Model: p.cfg.Model}
	p.mu.Unlock()
	m, _ := p.mainBot()
	if m == nil {
		return st, nil
	}
	st.Bot = m.Name
	st.DayFrom, st.DayTo, st.Awake = p.hours.window(now)
	since := st.DayFrom
	if !st.Awake {
		since = now.Add(-24 * time.Hour)
	}
	st.Rounds, st.LastRound, _ = p.store.PulseCount(m.ID, since)
	settled, _ := p.store.PulseSettled(m.ID)
	st.Nudges = p.survey(p.scene(m, settled, now)).states
	return st, m
}
