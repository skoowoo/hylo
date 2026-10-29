package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/util"
)

// A daily short file is /_shorts/YYYY-MM-DD.md: `kind: short` frontmatter,
// then entries oldest-first, each "###### Short Note: <stamp>\n\n<body>",
// separated by a thematic-break line.
const (
	shortHeadingPrefix = "###### Short Note: "
	shortStampLayout   = "2006-01-02 15:04:05"
)

var (
	shortDelimRe    = regexp.MustCompile(`\r?\n(?:---|\*\*\*)[ \t]*\r?\n`)
	shortIDRe       = regexp.MustCompile(`^(.+)-(\d{6})(?:-(\d+))?$`)
	shortKindLineRe = regexp.MustCompile(`(?m)^kind:.*$`)
)

// shortSegment is one non-blank chunk of a daily file. Chunks without a valid
// heading (hand-written text) have an empty ID; they are never listed but are
// kept so edits splice around them instead of dropping them.
type shortSegment struct {
	ShortEntry
	from, to int
}

func parseShortDay(raw []byte, p Path) []shortSegment {
	_, body := util.ParseFrontmatter(raw)
	start := len(raw) - len(body)
	stem := strings.TrimSuffix(p.Base(), ".md")

	var segs []shortSegment
	seen := map[string]int{}
	add := func(from, to int) {
		chunk := string(raw[from:to])
		trimmed := strings.TrimSpace(chunk)
		if trimmed == "" {
			return
		}
		from += strings.Index(chunk, trimmed)
		seg := shortSegment{from: from, to: from + len(trimmed)}
		head, rest, _ := strings.Cut(trimmed, "\n")
		if stamp, ok := strings.CutPrefix(strings.TrimSpace(head), shortHeadingPrefix); ok {
			if t, err := time.ParseInLocation(shortStampLayout, strings.TrimSpace(stamp), time.Local); err == nil {
				id := stem + "-" + t.Format("150405")
				seen[id]++
				if n := seen[id]; n > 1 {
					id += "-" + strconv.Itoa(n)
				}
				content := strings.TrimSpace(rest)
				seg.ShortEntry = ShortEntry{ID: id, Content: content, Hash: shortHash(content), CreatedAt: t, Path: p.String()}
			}
		}
		segs = append(segs, seg)
	}
	prev := start
	for _, m := range shortDelimRe.FindAllIndex(raw[start:], -1) {
		add(prev, start+m[0])
		prev = start + m[1]
	}
	add(prev, len(raw))
	return segs
}

func shortHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:6])
}

func shortDayPath(t time.Time) Path {
	return Path(ShortDir + "/" + t.Format("2006-01-02") + ".md")
}

func shortPathFromID(id string) (Path, error) {
	m := shortIDRe.FindStringSubmatch(id)
	if m == nil || strings.Contains(m[1], "/") {
		return "", fmt.Errorf("invalid short id %q: %w", id, ErrNotFound)
	}
	return Path(ShortDir + "/" + m[1] + ".md"), nil
}

func shortChunk(t time.Time, content string) string {
	return shortHeadingPrefix + t.Format(shortStampLayout) + "\n\n" + content
}

// shortSanitize blanks thematic-break lines so content can't split its entry.
func shortSanitize(content []byte) (string, error) {
	if !util.IsValidText(content) {
		return "", ErrBinaryContent
	}
	lines := strings.Split(string(content), "\n")
	for i, l := range lines {
		if t := strings.TrimSpace(l); t == "---" || t == "***" {
			lines[i] = ""
		}
	}
	out := strings.TrimSpace(strings.Join(lines, "\n"))
	if out == "" {
		return "", ErrEmptyContent
	}
	return out, nil
}

func findShort(segs []shortSegment, id string) int {
	for i, s := range segs {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// AppendShort appends content as a new entry to today's daily file. A stamp
// already used in that file is bumped by a second so entry IDs stay unique.
func (g *Vault) AppendShort(content []byte) (ShortEntry, error) {
	body, err := shortSanitize(content)
	if err != nil {
		return ShortEntry{}, err
	}

	g.mu.Lock()
	now := time.Now().Truncate(time.Second)
	var p Path
	var existing []byte
	for {
		p = shortDayPath(now)
		existing, err = g.readNoteLocked(p)
		if err != nil && !errors.Is(err, ErrNotFound) {
			g.mu.Unlock()
			return ShortEntry{}, err
		}
		taken := false
		for _, s := range parseShortDay(existing, p) {
			if s.ID != "" && s.CreatedAt.Equal(now) {
				taken = true
				break
			}
		}
		if !taken {
			break
		}
		now = now.Add(time.Second)
	}

	head := strings.TrimRight(shortFrontmatter(existing, now), " \t\r\n")
	sep := "\n\n---\n\n"
	if _, rest := util.ParseFrontmatter([]byte(head)); strings.TrimSpace(string(rest)) == "" {
		sep = "\n\n"
	}
	merged := head + sep + shortChunk(now, body) + "\n"

	isNew, modTime, err := g.writeNoteLocked(p, []byte(merged), Note{Kind: KindShort})
	g.mu.Unlock()
	if err != nil {
		return ShortEntry{}, err
	}

	var entry ShortEntry
	segs := parseShortDay([]byte(merged), p)
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i].ID != "" && segs[i].CreatedAt.Equal(now) {
			entry = segs[i].ShortEntry
			break
		}
	}
	if entry.ID == "" {
		return ShortEntry{}, fmt.Errorf("short: appended entry not found in %s", p)
	}

	typ := plugin.EventVaultWrite
	if isNew {
		typ = plugin.EventVaultCreate
	}
	g.emitShort(typ, p, modTime, entry.ID, entry.Hash)
	g.emitEvent(plugin.Event{
		Type:    plugin.EventVaultShortAppend,
		Path:    p.String(),
		Time:    modTime,
		Content: body,
	})
	return entry, nil
}

// shortFrontmatter gives an existing daily file (or a new, empty one) the
// `kind: short` frontmatter a rescan reads its kind from, editing only that
// line so the rest of a hand-edited block is kept.
func shortFrontmatter(existing []byte, now time.Time) string {
	if util.IsShortNote(existing) {
		return string(existing)
	}
	end := util.FrontmatterEnd(existing)
	if end == 0 {
		return util.FormatShortFrontmatter(now) + strings.TrimSpace(string(existing))
	}
	block := string(existing[:end])
	if shortKindLineRe.MatchString(block) {
		block = shortKindLineRe.ReplaceAllString(block, "kind: short")
	} else {
		block = "---\nkind: short\n" + strings.TrimPrefix(block, "---\n")
	}
	return block + string(existing[end:])
}

// emitShort reports a write to p that touched one entry; hash is "" once it's gone.
func (g *Vault) emitShort(typ plugin.EventType, p Path, t time.Time, id, hash string) {
	g.emitEvent(plugin.Event{Type: typ, Path: p.String(), Time: t, ShortID: id, ShortHash: hash})
}

// GetShort returns the entry with the given id.
func (g *Vault) GetShort(id string) (ShortEntry, error) {
	p, err := shortPathFromID(id)
	if err != nil {
		return ShortEntry{}, err
	}
	g.mu.RLock()
	raw, err := g.readNoteLocked(p)
	g.mu.RUnlock()
	if err != nil {
		return ShortEntry{}, err
	}
	segs := parseShortDay(raw, p)
	i := findShort(segs, id)
	if i < 0 {
		return ShortEntry{}, fmt.Errorf("short %q: %w", id, ErrNotFound)
	}
	return segs[i].ShortEntry, nil
}

// UpdateShort replaces the entry's content, keeping its stamp and position.
// A non-empty hash must match the entry's current hash.
func (g *Vault) UpdateShort(id, hash string, content []byte) (ShortEntry, error) {
	body, err := shortSanitize(content)
	if err != nil {
		return ShortEntry{}, err
	}
	var out ShortEntry
	err = g.spliceShort(id, hash, shortHash(body), func(raw []byte, segs []shortSegment, i int) []byte {
		s := segs[i]
		out = s.ShortEntry
		out.Content, out.Hash = body, shortHash(body)
		return append(append(append([]byte{}, raw[:s.from]...), shortChunk(s.CreatedAt, body)...), raw[s.to:]...)
	})
	return out, err
}

// DeleteShort removes the entry and its separator; the daily file goes too
// once nothing but frontmatter is left.
func (g *Vault) DeleteShort(id, hash string) error {
	return g.spliceShort(id, hash, "", func(raw []byte, segs []shortSegment, i int) []byte {
		from, to := segs[i].from, segs[i].to
		switch {
		case i > 0:
			from = segs[i-1].to
		case i+1 < len(segs):
			to = segs[i+1].from
		}
		return append(append([]byte{}, raw[:from]...), raw[to:]...)
	})
}

// spliceShort rewrites only the byte span edit touches, so frontmatter,
// hand-written text and other entries are left exactly as they were. after is
// the entry's hash once edited, "" when edit removes it.
func (g *Vault) spliceShort(id, hash, after string, edit func(raw []byte, segs []shortSegment, i int) []byte) error {
	p, err := shortPathFromID(id)
	if err != nil {
		return err
	}

	g.mu.Lock()
	raw, err := g.readNoteLocked(p)
	if err != nil {
		g.mu.Unlock()
		return err
	}
	segs := parseShortDay(raw, p)
	i := findShort(segs, id)
	if i < 0 {
		g.mu.Unlock()
		return fmt.Errorf("short %q: %w", id, ErrNotFound)
	}
	if hash != "" && hash != segs[i].Hash {
		g.mu.Unlock()
		return ErrConflict
	}

	next := edit(raw, segs, i)
	if _, rest := util.ParseFrontmatter(next); strings.TrimSpace(string(rest)) == "" {
		err = g.deleteNoteLocked(p)
		g.mu.Unlock()
		if err != nil {
			return err
		}
		g.emitShort(plugin.EventVaultDelete, p, time.Now(), id, after)
		return nil
	}
	_, modTime, err := g.writeNoteLocked(p, next, Note{Kind: KindShort})
	g.mu.Unlock()
	if err != nil {
		return err
	}
	g.emitShort(plugin.EventVaultWrite, p, modTime, id, after)
	return nil
}

// ListShortEntries returns individual short note entries, newest first.
// Days are picked and ordered by file name, not mtime: editing an old entry
// must not move its day or hide it from a before-cursor page.
func (g *Vault) ListShortEntries(opts ShortListOptions) ([]ShortEntry, error) {
	g.mu.RLock()
	dailyNotes, err := dbListByDir(g.db, ShortDir, ListOptions{OnlyKinds: []Kind{KindShort}})
	g.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	sort.Slice(dailyNotes, func(i, j int) bool { return dailyNotes[i].Name > dailyNotes[j].Name })

	var all []ShortEntry
	for _, daily := range dailyNotes {
		if day, err := time.ParseInLocation("2006-01-02", strings.TrimSuffix(daily.Name, ".md"), time.Local); err == nil {
			if !opts.Before.IsZero() && !day.Before(opts.Before) {
				continue
			}
			if !opts.After.IsZero() && !day.AddDate(0, 0, 1).After(opts.After) {
				continue
			}
		}
		g.mu.RLock()
		raw, readErr := g.readNoteLocked(daily.Path())
		g.mu.RUnlock()
		if readErr != nil {
			continue
		}
		segs := parseShortDay(raw, daily.Path())
		for i := len(segs) - 1; i >= 0; i-- {
			e := segs[i].ShortEntry
			if e.ID == "" {
				continue
			}
			if !opts.After.IsZero() && e.CreatedAt.Before(opts.After) {
				continue
			}
			if !opts.Before.IsZero() && !e.CreatedAt.Before(opts.Before) {
				continue
			}
			all = append(all, e)
			if opts.Limit > 0 && len(all) >= opts.Limit {
				return all, nil
			}
		}
	}
	return all, nil
}
