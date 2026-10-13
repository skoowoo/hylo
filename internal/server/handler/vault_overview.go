package handler

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/hardhacker/hylo/internal/storage"
)

const (
	overviewRecent  = 8
	overviewFolders = 6
)

// VaultOverview is the glimpse of a vault an agent gets before its first
// words: the desktop's onboarding hello and the first chat's guide.
type VaultOverview struct {
	Name    string           `json:"name"`
	Notes   int              `json:"notes"`
	Folders []OverviewFolder `json:"folders"`
	Recent  []OverviewNote   `json:"recent"`
}

type OverviewFolder struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type OverviewNote struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Preview string `json:"preview"`
}

func buildVaultOverview(v *storage.Vault) (VaultOverview, error) {
	o := VaultOverview{Name: filepath.Base(v.Root()), Folders: []OverviewFolder{}, Recent: []OverviewNote{}}
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
	notes, _ := v.ListAllNotes(storage.ListOptions{SortByTime: true, Limit: overviewRecent})
	for _, n := range notes {
		p := n.PathString()
		o.Recent = append(o.Recent, OverviewNote{
			Path:    p,
			Title:   strings.TrimSuffix(path.Base(p), ".md"),
			Preview: excerpt(n.Preview.Text),
		})
	}
	return o, nil
}

// NewVaultOverview serves GET /api/vault/overview.
func NewVaultOverview(v *storage.Vault) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o, err := buildVaultOverview(v)
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
	o, err := buildVaultOverview(v)
	if err != nil {
		return ""
	}
	if o.Notes == 0 {
		return "Their vault is empty: they are starting fresh."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "- %d notes", o.Notes)
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
