package view

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/storage"
)

const activityWeeks = 53

// Levels step up past 1, 3 and 6 notes' worth of writing. A write counts as
// three reads, so the steps are 3, 9 and 18.
var activityFixedSteps = [3]int{3, 9, 18}

// activityScore weighs a day for the map: writing a note counts as much as
// reading three.
func activityScore(c mate.ActivityCount) int { return 3*c.Write + c.Read }

// activityMap is the Activity section's heatmap: a year of weeks, newest first, each Monday
// first. Each cell carries a level per actor so switching All/You/Bots
// is CSS only.
type activityMap struct {
	Weeks   []activityWeek
	Summary [3]string // all, user, bots
	Share   []activityShare
}

type activityWeek struct {
	Month string // set on the first week of a month
	Days  []activityCell
}

type activityCell struct {
	Future     bool
	Any        bool
	Selected   bool
	All, User  int // levels 0–4
	Bots       int
	Title, URL string
}

// activityFilter is the head chip of a filtered list.
type activityFilter struct {
	Label, ClearURL string
}

func activityDayURL(day time.Time) string {
	return "/home/section?type=activity&day=" + url.QueryEscape(day.Format(time.DateOnly))
}

// activityData loads the year up to today, keeping only notes that still exist.
func (vh *ViewHandler) activityData(today time.Time) (map[time.Time]map[string]*mate.NoteDay, map[string]storage.Note, time.Time, error) {
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	from := monday.AddDate(0, 0, -7*(activityWeeks-1))
	rows, err := vh.mates.ListNoteActivity(from, today.AddDate(0, 0, 1))
	if err != nil {
		return nil, nil, from, err
	}
	seen := map[string]bool{}
	var paths []storage.Path
	for _, r := range rows {
		if !seen[r.Path] {
			seen[r.Path] = true
			paths = append(paths, storage.Path(r.Path))
		}
	}
	notes, err := vh.vault.GetNotesByPaths(paths)
	if err != nil {
		return nil, nil, from, err
	}
	byPath := make(map[string]storage.Note, len(notes))
	for _, n := range notes {
		byPath[n.PathString()] = n
	}
	keep := func(p string) bool { _, ok := byPath[p]; return ok }
	return mate.NoteDays(rows, keep), byPath, from, nil
}

func (vh *ViewHandler) buildActivityMap(byDay map[time.Time]map[string]*mate.NoteDay, from, today, selected time.Time, names map[string]string) activityMap {
	days := mate.ActivityDays(byDay, from, today)
	scores := [3][]int{}
	for _, d := range days {
		scores[0] = append(scores[0], activityScore(d.All))
		scores[1] = append(scores[1], activityScore(d.User))
		scores[2] = append(scores[2], activityScore(d.Bots))
	}
	levels := [3][]int{activityLevels(scores[0]), activityLevels(scores[1]), activityLevels(scores[2])}

	var m activityMap
	var week *activityWeek
	for i, d := range days {
		if d.Date.Weekday() == time.Monday || week == nil {
			m.Weeks = append(m.Weeks, activityWeek{})
			week = &m.Weeks[len(m.Weeks)-1]
			if len(m.Weeks) > 1 && d.Date.Day() <= 7 {
				week.Month = d.Date.Format("Jan")
			}
		}
		c := activityCell{Any: d.All.Total() > 0, All: levels[0][i], User: levels[1][i], Bots: levels[2][i], Title: activityTitle(d, names)}
		if c.Any {
			c.Selected = d.Date.Equal(selected)
			c.URL = activityDayURL(d.Date)
			if c.Selected {
				c.URL = "/home/section?type=activity"
			}
		}
		week.Days = append(week.Days, c)
	}
	for len(week.Days) < 7 {
		week.Days = append(week.Days, activityCell{Future: true})
	}
	// Newest first, so a narrow pane wraps (and hides) the oldest weeks.
	slices.Reverse(m.Weeks)

	m.Summary = vh.activitySummary(scores, today)
	m.Share = activityShareRows(days, today.AddDate(0, 0, 1-activityShareDays), names)
	return m
}

func activityLevels(scores []int) []int {
	out := make([]int, len(scores))
	for i, c := range scores {
		if c == 0 {
			continue
		}
		out[i] = 1
		for _, s := range activityFixedSteps {
			if c > s {
				out[i]++
			}
		}
	}
	return out
}

func activityTitle(d mate.ActivityDay, names map[string]string) string {
	date := d.Date.Format("Mon, Jan 2")
	if d.All.Total() == 0 {
		return date + " · No activity"
	}
	lines := []string{fmt.Sprintf("%s · %s", date, plural(d.All.Total(), "note"))}
	if line := activityCountLine("You", d.User, "edited", "viewed"); line != "" {
		lines = append(lines, line)
	}
	bots := make([]string, 0, len(d.ByBot))
	for id := range d.ByBot {
		bots = append(bots, id)
	}
	slices.SortFunc(bots, func(a, b string) int {
		if n := d.ByBot[b].Total() - d.ByBot[a].Total(); n != 0 {
			return n
		}
		return strings.Compare(names[a], names[b])
	})
	for _, id := range bots {
		name := names[id]
		if name == "" {
			name = "Removed bot"
		}
		lines = append(lines, activityCountLine(name, d.ByBot[id], "wrote", "read"))
	}
	return strings.Join(lines, "\n")
}

func activityCountLine(who string, c mate.ActivityCount, wrote, read string) string {
	var parts []string
	if c.Write > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", wrote, c.Write))
	}
	if c.Read > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", read, c.Read))
	}
	if len(parts) == 0 {
		return ""
	}
	return who + ": " + strings.Join(parts, ", ")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// activitySummary counts active days, or says since when it has recorded
// while that is under a month and the count would look poor.
func (vh *ViewHandler) activitySummary(scores [3][]int, today time.Time) [3]string {
	first, ok, err := vh.mates.FirstNoteActivity()
	if err != nil || !ok {
		return [3]string{"No activity yet", "No activity yet", "No activity yet"}
	}
	if first.After(today.AddDate(0, 0, -30)) {
		s := "Recording since " + first.Format("Jan 2")
		return [3]string{s, s, s}
	}
	var out [3]string
	for i, cs := range scores {
		n := 0
		for _, c := range cs {
			if c > 0 {
				n++
			}
		}
		out[i] = plural(n, "active day") + " in the past year"
	}
	return out
}

// activityShare is a row of the Who panel beside the map: how the last
// activityShareDays' score splits between you and each bot.
type activityShare struct {
	Name string
	Kind string // user | bot | other (Others, or a removed bot: no avatar colour)
	Pct  int
}

const (
	activityShareDays = 30
	activityShares    = 4 // the last row folds the rest into Others
)

func activityShareRows(days []mate.ActivityDay, since time.Time, names map[string]string) []activityShare {
	user, total := 0, 0
	bots := map[string]int{}
	for _, d := range days {
		if d.Date.Before(since) {
			continue
		}
		user += activityScore(d.User)
		total += activityScore(d.User)
		for id, c := range d.ByBot {
			bots[id] += activityScore(c)
			total += activityScore(c)
		}
	}
	if total == 0 {
		return nil
	}
	type row struct {
		activityShare
		score int
	}
	rows := []row{{activityShare{Name: "You", Kind: "user"}, user}}
	removed := 0
	for id, score := range bots {
		if names[id] == "" {
			removed += score
			continue
		}
		rows = append(rows, row{activityShare{Name: names[id], Kind: "bot"}, score})
	}
	rows = append(rows, row{activityShare{Name: "Removed bots", Kind: "other"}, removed})
	rows = slices.DeleteFunc(rows, func(r row) bool { return r.score == 0 })
	slices.SortFunc(rows, func(a, b row) int {
		if n := b.score - a.score; n != 0 {
			return n
		}
		return strings.Compare(a.Name, b.Name)
	})
	if len(rows) > activityShares {
		rest := 0
		for _, r := range rows[activityShares-1:] {
			rest += r.score
		}
		rows = append(rows[:activityShares-1], row{activityShare{Name: "Others", Kind: "other"}, rest})
	}
	out := make([]activityShare, len(rows))
	for i, r := range rows {
		r.Pct = max(1, (r.score*100+total/2)/total) // active, however little
		out[i] = r.activityShare
	}
	return out
}

// botNames maps mate ids to names; a removed bot is simply missing.
func (vh *ViewHandler) botNames() map[string]string {
	names := map[string]string{}
	if list, err := vh.mates.ListMates(); err == nil {
		for _, m := range list {
			names[m.ID] = m.Name
		}
	}
	return names
}

// activitySection adds the activity map to the section and, with a day picked, swaps
// its list for the notes touched that day, most recent first.
func (vh *ViewHandler) activitySection(data *homeSectionData, day time.Time) error {
	today := localMidnight(time.Now())
	byDay, notes, from, err := vh.activityData(today)
	if err != nil {
		return err
	}
	names := vh.botNames()
	am := vh.buildActivityMap(byDay, from, today, day, names)
	data.Activity = &am
	if day.IsZero() {
		return nil
	}
	touched := mate.NotesOn(byDay, day)
	ordered := make([]storage.Note, 0, len(touched))
	var touches []*noteTouch
	for _, t := range touched {
		ordered = append(ordered, notes[t.Path])
		touches = append(touches, noteTouchFrom(t, names))
	}
	data.Items = vh.noteItemsFromNotes(ordered)
	for i := range data.Items {
		data.Items[i].Touch = touches[i]
	}
	label := day.Format("Jan 2")
	data.Title = label
	data.Filter = &activityFilter{Label: label, ClearURL: "/home/section?type=activity"}
	data.Empty = emptyState{Icon: "recent", Title: "Nothing on " + label}
	return nil
}

// noteTouch is what a day's list shows on a note: who touched it and when.
type noteTouch struct {
	Time string
	User string // "Edited" | "Viewed" | ""
	Bots []touchBot
}

type touchBot struct {
	Name, Title string
}

func noteTouchFrom(t mate.NoteDay, names map[string]string) *noteTouch {
	nt := &noteTouch{Time: t.Last.Format("15:04")}
	switch t.User {
	case mate.NoteAccessWrite:
		nt.User = "Edited"
	case mate.NoteAccessRead:
		nt.User = "Viewed"
	}
	for id, action := range t.ByBot {
		name := names[id]
		if name == "" {
			continue
		}
		verb := "read"
		if action == mate.NoteAccessWrite {
			verb = "wrote"
		}
		nt.Bots = append(nt.Bots, touchBot{Name: name, Title: name + " " + verb + " this"})
	}
	slices.SortFunc(nt.Bots, func(a, b touchBot) int { return strings.Compare(a.Name, b.Name) })
	return nt
}

func localMidnight(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}
