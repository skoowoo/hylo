package handler

import (
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"

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
