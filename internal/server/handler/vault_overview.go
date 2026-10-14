package handler

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hardhacker/hylo/internal/storage"
)

const (
	overviewRecent  = 8
	overviewOlder   = 4
	overviewFolders = 6
	// A note counts as older once it's been this long since it changed.
	overviewOlderAge = 14 * 24 * time.Hour
)

// VaultOverview is the glimpse of a vault an agent gets before its first
// words: the desktop's onboarding hello and the first chat's guide.
type VaultOverview struct {
	Name     string           `json:"name"`
	Notes    int              `json:"notes"`
	Obsidian bool             `json:"obsidian"`
	Folders  []OverviewFolder `json:"folders"`
	Recent   []OverviewNote   `json:"recent"`
	// Older is a spread of notes from further back, so a first look can link old and new.
	Older       []OverviewNote `json:"older"`
	WritingTime string         `json:"writingTime,omitempty"`
}

type OverviewFolder struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type OverviewNote struct {
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Preview string    `json:"preview"`
	Updated time.Time `json:"updated"`
}

// buildVaultOverview samples the user's own writing; seed varies which older notes come along.
func buildVaultOverview(v *storage.Vault, seed int) (VaultOverview, error) {
	o := VaultOverview{Name: filepath.Base(v.Root()), Folders: []OverviewFolder{}, Recent: []OverviewNote{}, Older: []OverviewNote{}}
	if fi, err := os.Stat(filepath.Join(v.Root(), ".obsidian")); err == nil && fi.IsDir() {
		o.Obsidian = true
	}
	total, err := v.CountNotes()
	if err != nil || total == 0 {
		return o, err
	}
	o.Notes = total
	if dirs, err := v.ListAllDirs(); err == nil {
		sort.SliceStable(dirs, func(i, j int) bool { return dirs[i].Count > dirs[j].Count })
		for _, d := range dirs {
			if d.Dir == "/" || strings.Contains(d.Dir, "/_") || strings.Contains(d.Dir, "/.") {
				continue
			}
			o.Folders = append(o.Folders, OverviewFolder{Name: strings.TrimPrefix(d.Dir, "/"), Count: d.Count})
			if len(o.Folders) == overviewFolders {
				break
			}
		}
	}
	all, _ := v.ListAllNotes(storage.ListOptions{SortByTime: true, ExcludeKinds: []storage.Kind{storage.KindKnowledge, storage.KindIndex}})
	var mine []storage.Note
	for _, n := range all {
		if n.Kind == storage.KindShort || !strings.HasPrefix(n.Dir, "/_") {
			mine = append(mine, n)
		}
	}
	for i, n := range mine {
		if i == overviewRecent {
			break
		}
		o.Recent = append(o.Recent, overviewNote(n))
	}
	var older []storage.Note
	for _, n := range mine[min(len(mine), overviewRecent):] {
		if time.Since(n.UpdatedAt) > overviewOlderAge {
			older = append(older, n)
		}
	}
	for _, n := range spread(older, overviewOlder, seed) {
		o.Older = append(o.Older, overviewNote(n))
	}
	o.WritingTime = writingTime(mine)
	return o, nil
}

func overviewNote(n storage.Note) OverviewNote {
	p := n.PathString()
	return OverviewNote{Path: p, Title: strings.TrimSuffix(path.Base(p), ".md"), Preview: excerpt(n.Preview.Text), Updated: n.UpdatedAt}
}

// spread takes k notes evenly across ns, so they span its whole time range.
func spread(ns []storage.Note, k, seed int) []storage.Note {
	if len(ns) <= k {
		return ns
	}
	step := len(ns) / k
	off := seed % step
	if off < 0 {
		off = -off
	}
	out := make([]storage.Note, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, ns[off+i*step])
	}
	return out
}

var dayParts = [4]string{"in the morning", "in the afternoon", "in the evening", "late at night"}

// writingTime names the part of the day most of their writing lands in. Each
// hour of each day counts once, so a sync or import that touches many files
// at once doesn't pass for a habit.
func writingTime(ns []storage.Note) string {
	hours := map[string]bool{}
	days := map[string]bool{}
	var count [4]int
	for _, n := range ns {
		t := n.UpdatedAt.Local()
		h := t.Format("2006-01-02T15")
		if hours[h] {
			continue
		}
		hours[h] = true
		days[t.Format("2006-01-02")] = true
		count[partOfDay(t.Hour())]++
	}
	if len(days) < 5 || len(hours) < 8 {
		return ""
	}
	best := 0
	for i := range count {
		if count[i] > count[best] {
			best = i
		}
	}
	if count[best]*2 <= len(hours) {
		return ""
	}
	return dayParts[best]
}

func partOfDay(h int) int {
	switch {
	case h >= 5 && h < 12:
		return 0
	case h >= 12 && h < 18:
		return 1
	case h >= 18 && h < 23:
		return 2
	}
	return 3
}

// NewVaultOverview serves GET /api/vault/overview.
func NewVaultOverview(v *storage.Vault) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o, err := buildVaultOverview(v, 0)
		if err != nil {
			writeVaultError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, o)
	}
}

const introExcerpt = 120

// vaultOverview is what the intro guide is built from. It goes into the prompt
// so the agent starts typing without exploring the vault first.
func vaultOverview(v *storage.Vault) string {
	if v == nil {
		return ""
	}
	o, err := buildVaultOverview(v, 0)
	if err != nil {
		return ""
	}
	if o.Notes == 0 {
		return "Their vault is empty: they are starting fresh."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "- %d notes", o.Notes)
	if o.Obsidian {
		sb.WriteString(" (also their Obsidian vault)")
	}
	var names []string
	for _, f := range o.Folders {
		names = append(names, fmt.Sprintf("%s (%d)", f.Name, f.Count))
	}
	if len(names) > 0 {
		sb.WriteString(", in folders " + strings.Join(names, ", "))
	}
	sb.WriteString(".\n- Most recently edited:")
	for _, n := range o.Recent {
		fmt.Fprintf(&sb, "\n  - %s", n.Path)
		if n.Preview != "" {
			fmt.Fprintf(&sb, ": %q", n.Preview)
		}
	}
	return sb.String()
}

func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= introExcerpt {
		return s
	}
	return string([]rune(s)[:introExcerpt]) + "…"
}
