package notes

import (
	"errors"
	"sync"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/mate"
)

type CapTrigger struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Events       []string `json:"events"`
	Schedule     string   `json:"schedule,omitempty"`
	PathPrefixes []string `json:"path_prefixes,omitempty"`
	Enabled      bool     `json:"enabled"`
}

type CapBot struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Cwd         string       `json:"cwd"`
	Main        bool         `json:"main,omitempty"`
	Enabled     bool         `json:"enabled"`
	Skills      []string     `json:"skills"`
	Triggers    []CapTrigger `json:"triggers"`
}

type CapSkill struct {
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}

type CapEvent struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Capabilities struct {
	Bots []CapBot `json:"bots"`
	// Catalog holds only skills not added yet; added ones show under the bots they're on for.
	Catalog []CapSkill `json:"catalog"`
	Unused  []string   `json:"unused_skills"`
	Events  []CapEvent `json:"events"`
}

// GetCapabilities is everything a routing decision needs, in one call instead of four.
func GetCapabilities(c *client.Client) (Capabilities, error) {
	out := Capabilities{Bots: []CapBot{}, Catalog: []CapSkill{}, Unused: []string{}, Events: []CapEvent{}}
	var (
		list    []mate.Mate
		skills  []client.SkillInfo
		catalog []client.CatalogEntry
		wg      sync.WaitGroup
		errs    [3]error
	)
	wg.Add(3)
	go func() { defer wg.Done(); list, errs[0] = c.Bots() }()
	go func() { defer wg.Done(); skills, errs[1] = c.SkillsList() }()
	go func() { defer wg.Done(); catalog, errs[2] = c.SkillsCatalog() }()
	wg.Wait()
	if err := errors.Join(errs[:]...); err != nil {
		return out, err
	}

	// The list carries only a trigger count.
	full := make([]*mate.Mate, len(list))
	botErrs := make([]error, len(list))
	for i, m := range list {
		wg.Add(1)
		go func() { defer wg.Done(); full[i], botErrs[i] = c.Bot(m.ID) }()
	}
	wg.Wait()
	if err := errors.Join(botErrs...); err != nil {
		return out, err
	}

	on := map[string][]string{}
	for _, s := range skills {
		// Built-ins sit on the main bot implicitly; listing them would bury the user's own.
		if s.Default {
			continue
		}
		if len(s.Bots) == 0 {
			out.Unused = append(out.Unused, s.Name)
		}
		for _, b := range s.Bots {
			on[b.ID] = append(on[b.ID], s.Name)
		}
	}
	for _, f := range full {
		b := CapBot{ID: f.ID, Name: f.Name, Description: f.Description, Cwd: f.Cwd, Main: f.Main,
			Enabled: f.Enabled, Skills: on[f.ID], Triggers: []CapTrigger{}}
		if b.Skills == nil {
			b.Skills = []string{}
		}
		for _, t := range f.Triggers {
			b.Triggers = append(b.Triggers, CapTrigger{ID: t.ID, Name: t.Name, Events: t.EventTypes,
				Schedule: t.Schedule, PathPrefixes: t.PathPrefixes, Enabled: t.Enabled})
		}
		out.Bots = append(out.Bots, b)
	}
	for _, e := range catalog {
		if !e.Added {
			out.Catalog = append(out.Catalog, CapSkill{Name: e.Name, Repo: e.Repo, Category: e.Category, Description: e.Description})
		}
	}
	for _, e := range mate.BuiltinEvents {
		out.Events = append(out.Events, CapEvent{Type: string(e.Type), Description: e.Description})
	}
	return out, nil
}
