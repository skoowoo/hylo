package storage

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Task is an unchecked task item in a note.
type Task struct {
	Path    string    `json:"path"`
	Line    int       `json:"line"` // 1-based
	Text    string    `json:"text"`
	Updated time.Time `json:"updated"` // when its note last changed
}

var openTaskLine = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[ \]\s+(\S.*)$`)

// OpenTasks lists the unchecked tasks in notes and shorts last changed
// within [from, to), the most recently changed notes first. Knowledge,
// indexes and system folders are left out: they aren't the user's to-dos.
func (g *Vault) OpenTasks(from, to time.Time) ([]Task, error) {
	rows, err := g.db.Query(`SELECT dir, name, kind, updated_at FROM notes WHERE updated_at >= ? AND updated_at < ? ORDER BY updated_at DESC`,
		from.UnixNano(), to.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("storage: open tasks: %w", err)
	}
	type cand struct {
		path    Path
		updated time.Time
	}
	var cands []cand
	for rows.Next() {
		var dir, name, kind string
		var ns int64
		if err := rows.Scan(&dir, &name, &kind, &ns); err != nil {
			rows.Close()
			return nil, fmt.Errorf("storage: open tasks: %w", err)
		}
		switch Kind(kind) {
		case KindKnowledge, KindIndex:
			continue
		case KindShort:
		default:
			if skipSampleDir(dir) {
				continue
			}
		}
		if p, ok := ParsePath(JoinPath(dir, name)); ok {
			cands = append(cands, cand{p, time.Unix(0, ns)})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: open tasks: %w", err)
	}

	var out []Task
	for _, c := range cands {
		g.mu.RLock()
		raw, err := g.readNoteLocked(c.path)
		g.mu.RUnlock()
		if err != nil || !strings.Contains(string(raw), "[ ]") {
			continue
		}
		for _, t := range openTasks(string(raw)) {
			t.Path, t.Updated = c.path.String(), c.updated
			out = append(out, t)
		}
	}
	return out, nil
}

func openTasks(md string) []Task {
	var out []Task
	fence := ""
	for i, line := range strings.Split(md, "\n") {
		trim := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trim, fence) {
				fence = ""
			}
			continue
		}
		if i := slices.IndexFunc([]string{"```", "~~~"}, func(f string) bool { return strings.HasPrefix(trim, f) }); i >= 0 {
			fence = trim[:3]
			continue
		}
		if m := openTaskLine.FindStringSubmatch(line); m != nil {
			out = append(out, Task{Line: i + 1, Text: strings.TrimSpace(m[1])})
		}
	}
	return out
}
