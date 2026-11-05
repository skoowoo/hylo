package storage

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hardhacker/hylo/internal/util"
)

const (
	sampleDefaultEach  = 2
	sampleMaxEach      = 8
	sampleDefaultOlder = 90
	sampleExcerptRunes = 240
)

// SampleItem is one stop on a random walk through the vault.
type SampleItem struct {
	Kind   string `json:"kind"` // raw, knowledge, or short
	Path   string `json:"path"`
	Title  string `json:"title,omitempty"`
	Domain string `json:"domain,omitempty"` // knowledge index the unit was drawn from
	// Perspective is a knowledge unit's own, external or mixed; empty when unset.
	Perspective string    `json:"perspective,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Excerpt     string    `json:"excerpt,omitempty"`
	ID          string    `json:"id,omitempty"` // short entry id
}

// SampleOptions controls Vault.Sample. A nil Rand is seeded from the global source.
type SampleOptions struct {
	// Each is how many items to draw from each stratum. 0 means 2. Values above 8 are capped.
	Each int
	// OlderThan is how many days back a draw must reach. 0 means 90.
	// A negative value keeps the whole history. A stratum with nothing
	// that old is drawn from all of it.
	OlderThan int
	// Now is the clock. Zero means time.Now.
	Now time.Time
	// Rand makes the draw repeatable in tests.
	Rand *rand.Rand
}

type sampleCand struct {
	kind    string
	path    string
	title   string
	created time.Time
	preview string
	domains []domainRef
	domain  string
}

type domainRef struct {
	key, label string
}

// Sample draws a random walk through the vault.
//
// Each call returns up to Each items from each of three strata: ordinary
// notes, knowledge units, and short entries. System folders and index files
// are left out. Within a stratum the draw is spread across when things were
// written, and knowledge units are spread across domains. A unit is dated by
// its earliest source note, so a fresh compile of an old note still counts as old.
func (g *Vault) Sample(opts SampleOptions) ([]SampleItem, error) {
	each, before, rng := sampleBounds(opts)
	g.mu.RLock()
	raw, know, days, err := g.loadSample()
	g.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	raw = preferOld(raw, before)
	know = preferOld(know, before)
	days = preferOld(days, before)

	var items []SampleItem
	for _, c := range drawSpread(raw, each, rng) {
		items = append(items, candItem(c))
	}
	for _, c := range drawSpread(know, each, rng) {
		it := candItem(c)
		it.Perspective = g.perspective(c.path)
		items = append(items, it)
	}
	items = append(items, g.materializeShorts(days, each, rng)...)
	slices.SortFunc(items, func(a, b SampleItem) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	if items == nil {
		items = []SampleItem{}
	}
	return items, nil
}

func sampleBounds(opts SampleOptions) (int, time.Time, *rand.Rand) {
	each := opts.Each
	if each < 1 {
		each = sampleDefaultEach
	}
	if each > sampleMaxEach {
		each = sampleMaxEach
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	var before time.Time
	if days := opts.OlderThan; days >= 0 {
		if days == 0 {
			days = sampleDefaultOlder
		}
		before = now.AddDate(0, 0, -days)
	}
	rng := opts.Rand
	if rng == nil {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	return each, before, rng
}

// preferOld keeps candidates written before the cutoff. An empty result
// means the stratum has no past yet, so the whole stratum stands.
func preferOld(cands []sampleCand, before time.Time) []sampleCand {
	if before.IsZero() {
		return cands
	}
	old := make([]sampleCand, 0, len(cands))
	for _, c := range cands {
		if c.created.Before(before) {
			old = append(old, c)
		}
	}
	if len(old) == 0 {
		return cands
	}
	return old
}

func (g *Vault) loadSample() (raw, know, days []sampleCand, err error) {
	rows, err := g.db.Query(`SELECT dir, name, created_at, kind, title, preview FROM notes`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("storage: sample notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var dir, name, kind, title, preview string
		var createdNs int64
		if err := rows.Scan(&dir, &name, &createdNs, &kind, &title, &preview); err != nil {
			return nil, nil, nil, fmt.Errorf("storage: sample notes: %w", err)
		}
		c := sampleCand{
			path:    JoinPath(dir, name),
			title:   title,
			created: time.Unix(0, createdNs),
			preview: preview,
		}
		switch Kind(kind) {
		case KindKnowledge:
			c.kind = string(KindKnowledge)
			know = append(know, c)
		case KindIndex:
		case KindShort:
			if day, ok := shortDayTime(name); ok {
				c.kind = string(KindShort)
				c.created = day
				days = append(days, c)
			}
		default:
			if skipSampleDir(dir) {
				continue
			}
			c.kind = "raw"
			raw = append(raw, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("storage: sample notes: %w", err)
	}
	if err := g.attachKnowledge(know); err != nil {
		return nil, nil, nil, err
	}
	return raw, know, days, nil
}

// attachKnowledge dates a unit by its earliest source note and records the
// indexes that list it. The compile clock would hide an old note that was
// recompiled last week.
func (g *Vault) attachKnowledge(cands []sampleCand) error {
	if len(cands) == 0 {
		return nil
	}
	byPath := make(map[string]*sampleCand, len(cands))
	for i := range cands {
		byPath[cands[i].path] = &cands[i]
	}
	if err := eachRow(g.db, `
		SELECT d.knowledge_dir, d.knowledge_name, MIN(n.created_at)
		FROM knowledge_deps d
		JOIN notes n ON n.dir = d.source_dir AND n.name = d.source_name
		GROUP BY d.knowledge_dir, d.knowledge_name`, func(rows *sql.Rows) error {
		var dir, name string
		var ns int64
		if err := rows.Scan(&dir, &name, &ns); err != nil {
			return err
		}
		if c := byPath[JoinPath(dir, name)]; c != nil {
			if src := time.Unix(0, ns); src.Before(c.created) {
				c.created = src
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("storage: sample knowledge dates: %w", err)
	}
	if err := eachRow(g.db, `
		SELECT d.knowledge_dir, d.knowledge_name, d.index_dir, d.index_name, COALESCE(i.title, '')
		FROM index_deps d
		LEFT JOIN notes i ON i.dir = d.index_dir AND i.name = d.index_name`, func(rows *sql.Rows) error {
		var kDir, kName, iDir, iName, title string
		if err := rows.Scan(&kDir, &kName, &iDir, &iName, &title); err != nil {
			return err
		}
		c := byPath[JoinPath(kDir, kName)]
		if c == nil {
			return nil
		}
		key := JoinPath(iDir, iName)
		for _, d := range c.domains {
			if d.key == key {
				return nil
			}
		}
		if title == "" {
			title = strings.TrimSuffix(iName, ".md")
		}
		c.domains = append(c.domains, domainRef{key: key, label: title})
		return nil
	}); err != nil {
		return fmt.Errorf("storage: sample knowledge domains: %w", err)
	}
	return nil
}

func eachRow(db *sql.DB, query string, scan func(*sql.Rows) error) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (g *Vault) materializeShorts(days []sampleCand, each int, rng *rand.Rand) []SampleItem {
	chosen := drawSpread(days, each, rng)
	used := map[string]bool{}
	var out []SampleItem
	take := func(c sampleCand) {
		if used[c.path] || len(out) >= each {
			return
		}
		used[c.path] = true
		if item, ok := g.oneShort(c.path, rng); ok {
			out = append(out, item)
		}
	}
	for _, c := range chosen {
		take(c)
	}
	if len(out) < each {
		rest := slices.Clone(days)
		rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
		for _, c := range rest {
			take(c)
		}
	}
	return out
}

// perspective reads a unit's frontmatter, since the index doesn't keep it.
func (g *Vault) perspective(path string) string {
	p, ok := ParsePath(path)
	if !ok {
		return ""
	}
	g.mu.RLock()
	raw, err := g.readNoteLocked(p)
	g.mu.RUnlock()
	if err != nil {
		return ""
	}
	fm, _ := util.ParseFrontmatter(raw)
	for _, e := range fm.All {
		if e.Key == "perspective" {
			return e.Value
		}
	}
	return ""
}

func (g *Vault) oneShort(dayPath string, rng *rand.Rand) (SampleItem, bool) {
	p, ok := ParsePath(dayPath)
	if !ok {
		return SampleItem{}, false
	}
	segs, ok := g.readShortDay(Note{Dir: p.Dir(), Name: p.Base()})
	if !ok {
		return SampleItem{}, false
	}
	var entries []ShortEntry
	for _, seg := range segs {
		if seg.ID != "" && strings.TrimSpace(seg.Content) != "" {
			entries = append(entries, seg.ShortEntry)
		}
	}
	if len(entries) == 0 {
		return SampleItem{}, false
	}
	e := entries[rng.IntN(len(entries))]
	return SampleItem{
		Kind:      string(KindShort),
		Path:      dayPath,
		CreatedAt: e.CreatedAt,
		Excerpt:   trimRunes(e.Content, sampleExcerptRunes),
		ID:        e.ID,
	}, true
}

// drawSpread picks up to each candidates, one from each slice of the
// written-time span, preferring a knowledge domain not yet drawn.
func drawSpread(in []sampleCand, each int, rng *rand.Rand) []sampleCand {
	if len(in) == 0 || each < 1 {
		return nil
	}
	cands := slices.Clone(in)
	slices.SortFunc(cands, func(a, b sampleCand) int { return a.created.Compare(b.created) })
	if each > len(cands) {
		each = len(cands)
	}
	bands := timeBands(cands, each)
	usedPath := map[string]bool{}
	usedDomain := map[string]bool{}
	var out []sampleCand
	for _, band := range bands {
		c, ok := pickCand(band, usedPath, usedDomain, rng)
		if !ok {
			c, ok = pickCand(cands, usedPath, usedDomain, rng)
		}
		if !ok {
			continue
		}
		usedPath[c.path] = true
		key := ""
		if len(c.domains) > 0 {
			d := chooseDomain(c, usedDomain, rng)
			c.domain = d.label
			key = d.key
		}
		usedDomain[key] = true
		out = append(out, c)
	}
	return out
}

func timeBands(cands []sampleCand, each int) [][]sampleCand {
	bands := make([][]sampleCand, each)
	lo, hi := cands[0].created, cands[len(cands)-1].created
	span := hi.Sub(lo)
	if span <= 0 {
		for i := range bands {
			bands[i] = cands
		}
		return bands
	}
	for _, c := range cands {
		i := int(float64(each) * (float64(c.created.Sub(lo)) / float64(span)))
		if i >= each {
			i = each - 1
		}
		if i < 0 {
			i = 0
		}
		bands[i] = append(bands[i], c)
	}
	return bands
}

func pickCand(pool []sampleCand, usedPath, usedDomain map[string]bool, rng *rand.Rand) (sampleCand, bool) {
	var open, fresh []sampleCand
	for _, c := range pool {
		if usedPath[c.path] {
			continue
		}
		open = append(open, c)
		if domainFresh(c, usedDomain) {
			fresh = append(fresh, c)
		}
	}
	choices := open
	if len(fresh) > 0 {
		choices = fresh
	}
	if len(choices) == 0 {
		return sampleCand{}, false
	}
	return choices[rng.IntN(len(choices))], true
}

func domainFresh(c sampleCand, used map[string]bool) bool {
	if len(c.domains) == 0 {
		return !used[""]
	}
	for _, d := range c.domains {
		if !used[d.key] {
			return true
		}
	}
	return false
}

func chooseDomain(c sampleCand, used map[string]bool, rng *rand.Rand) domainRef {
	var fresh []domainRef
	for _, d := range c.domains {
		if !used[d.key] {
			fresh = append(fresh, d)
		}
	}
	pool := c.domains
	if len(fresh) > 0 {
		pool = fresh
	}
	return pool[rng.IntN(len(pool))]
}

func candItem(c sampleCand) SampleItem {
	return SampleItem{
		Kind:      c.kind,
		Path:      c.path,
		Title:     c.title,
		Domain:    c.domain,
		CreatedAt: c.created,
		Excerpt:   trimRunes(unmarshalPreview(c.preview).Text, sampleExcerptRunes),
	}
}

func skipSampleDir(dir string) bool {
	for _, seg := range strings.Split(dir, "/") {
		if seg == "" {
			continue
		}
		if seg[0] == '_' || seg[0] == '.' {
			return true
		}
	}
	return false
}

func shortDayTime(name string) (time.Time, bool) {
	t, err := time.ParseInLocation(time.DateOnly, strings.TrimSuffix(name, ".md"), time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func trimRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
