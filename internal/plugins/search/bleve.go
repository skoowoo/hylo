package search

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/char/regexp"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unique"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/mapping"
	bleveSearch "github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// nameKeywordAnalyzer is a custom analyzer name that stores the full filename
// as a single lowercase token — used for whole-name prefix and fuzzy queries.
const nameKeywordAnalyzer = "name_keyword"

const (
	vaultInternalDir  = ".hylo"
	vaultIndexDirName = "data.idx"
)

func vaultIndexPath(root string) string {
	return filepath.Join(root, vaultInternalDir, vaultIndexDirName)
}

// BleveIndexer implements the full-text search engine using bleve.
type BleveIndexer struct {
	idx      bleve.Index
	useJieba bool
}

// NewBleveIndexer opens the bleve index at <vaultRoot>/.hylo/data.idx,
// creating it if it does not yet exist.
// useJieba controls whether the jieba CJK tokeniser is enabled when creating
// a new index; it has no effect on an already-existing index.
func NewBleveIndexer(vaultRoot string, useJieba bool) (*BleveIndexer, error) {
	idxPath := vaultIndexPath(vaultRoot)
	idx, err := bleve.Open(idxPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		idx, err = bleve.New(idxPath, buildIndexMapping(useJieba))
	}
	if err != nil {
		return nil, fmt.Errorf("search: open index: %w", err)
	}
	return &BleveIndexer{idx: idx, useJieba: useJieba}, nil
}

// Upsert indexes or re-indexes a single markdown file.
// kind is one of "raw", "knowledge", "index", or "short".
func (b *BleveIndexer) Upsert(path, name, content string, tags []string, updatedAt time.Time, kind string) {
	doc := indexDoc{
		Path:      path,
		Name:      name,
		NameRaw:   name,
		Content:   content,
		Tags:      tags,
		UpdatedAt: updatedAt,
		Kind:      kind,
	}
	if b.useJieba {
		doc.ContentZh = cjkContent(content) // only populated when CJK chars are present
	}
	if err := b.idx.Index(path, doc); err != nil {
		slog.Error("search index: upsert failed", "path", path, "err", err)
	}
}

// Delete removes a single document from the index.
func (b *BleveIndexer) Delete(path string) {
	if err := b.idx.Delete(path); err != nil {
		slog.Error("search index: delete failed", "path", path, "err", err)
	}
}

// DocCount returns the number of documents currently in the search index.
func (b *BleveIndexer) DocCount() (uint64, error) { return b.idx.DocCount() }

const tagField = "tags"

// TagDocCount returns how many indexed notes contain the given YAML front matter tag.
// The tag is matched the same way as type=tag search: trimmed, lowercased, keyword field "tags".
func (b *BleveIndexer) TagDocCount(tag string) (uint64, error) {
	t := strings.TrimSpace(strings.ToLower(tag))
	if t == "" {
		return 0, fmt.Errorf("search index: empty tag")
	}
	q := bleve.NewMatchQuery(t)
	q.SetField(tagField)
	req := bleve.NewSearchRequest(q)
	req.Size = 0
	res, err := b.idx.Search(req)
	if err != nil {
		return 0, fmt.Errorf("search index: tag doc count %q: %w", t, err)
	}
	return res.Total, nil
}

// TagCount is one bucket from a terms facet on the tags field (lowercased term).
type TagCount struct {
	Tag   string `json:"tag"`
	Count uint64 `json:"count"`
}

// TagDistribution returns tag → document counts using a Bleve terms facet on the "tags" field.
// Results are ordered by count descending, then tag string (Bleve facet order).
//
// topN is the maximum number of distinct tag terms returned; if topN <= 0, a default
// of 500 is used. If the index has more distinct tags than topN, remaining mass
// appears in the facet's "other" bucket (not returned here); raise topN if you
// need a full census.
func (b *BleveIndexer) TagDistribution(topN int) ([]TagCount, error) {
	n := topN
	if n <= 0 {
		n = 500
	}
	req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	req.Size = 0
	req.Score = bleve.ScoreNone
	req.AddFacet("tags_facet", bleve.NewFacetRequest(tagField, n))
	res, err := b.idx.Search(req)
	if err != nil {
		return nil, fmt.Errorf("search index: tag distribution: %w", err)
	}
	fr := res.Facets["tags_facet"]
	if fr == nil || fr.Terms == nil {
		return nil, nil
	}
	terms := fr.Terms.Terms()
	out := make([]TagCount, 0, len(terms))
	for _, tf := range terms {
		if tf == nil {
			continue
		}
		out = append(out, TagCount{Tag: tf.Term, Count: uint64(tf.Count)})
	}
	return out, nil
}

// Close closes the underlying bleve index.
func (b *BleveIndexer) Close() error { return b.idx.Close() }

// SearchOptions controls what fields are searched and how many results to return.
type SearchOptions struct {
	// Type filters search to a specific field:
	//   "name"    — filename search only
	//   "content" — full-text content search only
	//   "tag"     — YAML front matter tags only
	//   ""        — search name, content, and tags (default)
	Type string

	// Limit caps the number of results. 0 defaults to 20.
	Limit int

	// After / Before filter results by UpdatedAt (zero value = unbounded).
	After  time.Time
	Before time.Time

	// ExcludeKnowledge omits knowledge notes (compile plugin output) from results.
	ExcludeKnowledge bool

	// Kind restricts results to a single note kind: "raw", "knowledge", "index", or "short".
	// Empty string means no kind filter.
	Kind string
}

// SearchResult is a single hit from a search query.
type SearchResult struct {
	Dir  string    `json:"dir"`
	Name string    `json:"name"`
	// Kind is the note type: "raw", "knowledge", "index", or "short".
	Kind      string    `json:"kind"`
	UpdatedAt time.Time `json:"updated_at"`
	Score     float64   `json:"score"`
	// NameMatch is true when the hit was produced by a name or name_raw field match
	// (as opposed to a content-only match). Used by callers to surface filename hits first.
	NameMatch bool `json:"name_match"`
	// Lines contains the 1-indexed line numbers where content term hits were found.
	// Only populated for content-field hits; nil for name-only matches.
	Lines []int `json:"lines,omitempty"`
}

// Search executes a full-text or filename search and returns ranked results.
// readFn is called to resolve byte offsets to line numbers for content matches.
func (b *BleveIndexer) Search(queryStr string, opts SearchOptions, readFn func(string) ([]byte, error)) ([]SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	q := buildQuery(queryStr, opts.Type, b.useJieba)

	// Wrap with date range if caller supplied bounds.
	if !opts.After.IsZero() || !opts.Before.IsZero() {
		dateQ := bleve.NewDateRangeQuery(opts.After, opts.Before)
		dateQ.SetField("updated_at")
		q = bleve.NewConjunctionQuery(q, dateQ)
	}

	// Exclude non-raw notes (knowledge and index) from results.
	// The MustNot approach means documents that lack the field (e.g. very old index entries)
	// still pass, which is the right behaviour for normal notes.
	if opts.ExcludeKnowledge {
		knowledgeQ := bleve.NewTermQuery("knowledge")
		knowledgeQ.SetField("kind")
		indexQ := bleve.NewTermQuery("index")
		indexQ.SetField("kind")
		boolQ := bleve.NewBooleanQuery()
		boolQ.AddMust(q)
		boolQ.AddMustNot(bleve.NewDisjunctionQuery(knowledgeQ, indexQ))
		q = boolQ
	}

	// Filter to a specific kind when requested.
	if opts.Kind != "" {
		kindQ := bleve.NewTermQuery(opts.Kind)
		kindQ.SetField("kind")
		boolQ := bleve.NewBooleanQuery()
		boolQ.AddMust(q)
		boolQ.AddMust(kindQ)
		q = boolQ
	}

	req := bleve.NewSearchRequest(q)
	req.Size = limit
	req.Fields = []string{"name", "updated_at", "path", "tags", "kind"}
	req.IncludeLocations = true

	result, err := b.idx.Search(req)
	if err != nil {
		return nil, fmt.Errorf("search index: query %q: %w", queryStr, err)
	}

	hits := make([]SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		docPath := hit.ID
		if v, ok := hit.Fields["path"].(string); ok && strings.TrimSpace(v) != "" {
			docPath = v
		}
		// Vault paths are slash-separated; normalize so PathParts works on every OS.
		docPath = strings.ReplaceAll(docPath, `\`, `/`)

		dir, base := storage.PathParts(docPath)
		sr := SearchResult{
			Dir:       dir,
			Score:     hit.Score,
			NameMatch: len(hit.Locations["name"]) > 0 || len(hit.Locations["name_raw"]) > 0,
		}
		if v, ok := hit.Fields["name"].(string); ok {
			sr.Name = v
		} else {
			sr.Name = base
		}
		if v, ok := hit.Fields["kind"].(string); ok {
			sr.Kind = v
		}
		if v, ok := hit.Fields["updated_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				sr.UpdatedAt = t
			}
		}
		if contentLocs, ok := hit.Locations["content"]; ok && len(contentLocs) > 0 {
			if content, readErr := readFn(docPath); readErr == nil {
				sr.Lines = byteLocsToLines(content, contentLocs)
			}
		}
		hits = append(hits, sr)
	}
	return hits, nil
}

// ── internal types ────────────────────────────────────────────────────────────

// indexDoc is the document shape stored in the bleve index.
type indexDoc struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	NameRaw   string    `json:"name_raw"`       // full filename, single lowercase token for prefix/fuzzy
	Content   string    `json:"content"`        // English analyzer (stemming + stop-words)
	ContentZh string    `json:"content_zh"`     // same text, jieba analyzer for Chinese segmentation
	Tags      []string  `json:"tags,omitempty"` // YAML front matter tags; keyword analyzer per tag
	UpdatedAt time.Time `json:"updated_at"`
	Kind      string    `json:"kind"` // "raw", "knowledge", "index", or "short"
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildIndexMapping(useJieba bool) mapping.IndexMapping {
	im := bleve.NewIndexMapping()

	// 1. Character filter: treat separators as word breaks.
	_ = im.AddCustomCharFilter("filename_sep", map[string]interface{}{
		"type":    regexp.Name,
		"regexp":  `[-_.]`,
		"replace": " ",
	})

	// 2. filename_analyzer: separators + camelCase split + lowercase.
	//    Used for the "name" field — enables token-level matching ("cool"
	//    finds "my-cool-note.md").
	_ = im.AddCustomAnalyzer("filename_analyzer", map[string]interface{}{
		"type":         custom.Name,
		"char_filters": []string{"filename_sep"},
		"tokenizer":    unicode.Name,
		"token_filters": []string{
			camelcase.Name,
			lowercase.Name,
			unique.Name,
		},
	})

	// 3. name_keyword: stores the entire filename as one lowercase token.
	//    Used for the "name_raw" field — enables whole-name prefix queries
	//    ("my-cool" matches "my-cool-note.md") and fuzzy typo correction.
	_ = im.AddCustomAnalyzer(nameKeywordAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"tokenizer":     "single",
		"token_filters": []string{lowercase.Name},
	})

	doc := bleve.NewDocumentMapping()

	// path: vault-relative path for stored fields on hits (document ID is the same string).
	// Indexed as one lowercased token; user search does not query this field yet.
	pathMapping := bleve.NewTextFieldMapping()
	pathMapping.Analyzer = nameKeywordAnalyzer // single lowercase token; no stemming
	doc.AddFieldMappingsAt("path", pathMapping)

	// name: token-level search (separator + camelCase split).
	nameMapping := bleve.NewTextFieldMapping()
	nameMapping.Analyzer = "filename_analyzer"
	doc.AddFieldMappingsAt("name", nameMapping)

	// name_raw: whole-name prefix and fuzzy search.
	nameRawMapping := bleve.NewTextFieldMapping()
	nameRawMapping.Analyzer = nameKeywordAnalyzer
	doc.AddFieldMappingsAt("name_raw", nameRawMapping)

	// tags: each tag is one lowercase token (same analyzer as path / name_raw).
	tagsMapping := bleve.NewTextFieldMapping()
	tagsMapping.Analyzer = nameKeywordAnalyzer
	doc.AddFieldMappingsAt("tags", tagsMapping)

	// content: English full-text search (stemming + stop-words).
	// Store=false: raw text is read from the vault on demand via readFn when
	// resolving byte offsets to line numbers. Stored content is only needed for
	// fragment highlighting, which we do not use.
	contentMapping := bleve.NewTextFieldMapping()
	contentMapping.Analyzer = "en"
	contentMapping.Store = false
	doc.AddFieldMappingsAt("content", contentMapping)

	if useJieba {
		// zh: jieba SearchMode tokenizer + lowercase.
		//    Used for the "content_zh" field — correctly segments Chinese compound
		//    words that would otherwise be treated as one opaque token. English text
		//    passes through jieba unchanged, so mixed-language notes work naturally.
		_ = im.AddCustomAnalyzer("zh", map[string]interface{}{
			"type":          custom.Name,
			"tokenizer":     jiebaTokenizerName,
			"token_filters": []string{lowercase.Name},
		})

		// content_zh: same text re-analyzed with jieba for Chinese segmentation.
		contentZhMapping := bleve.NewTextFieldMapping()
		contentZhMapping.Analyzer = "zh"
		contentZhMapping.Store = false
		doc.AddFieldMappingsAt("content_zh", contentZhMapping)
	}

	doc.AddFieldMappingsAt("updated_at", bleve.NewDateTimeFieldMapping())

	// kind: categorical value stored as a single lowercase keyword token.
	kindMapping := bleve.NewTextFieldMapping()
	kindMapping.Analyzer = nameKeywordAnalyzer
	doc.AddFieldMappingsAt("kind", kindMapping)

	im.DefaultMapping = doc
	return im
}

func buildQuery(queryStr, searchType string, useJieba bool) query.Query {
	lq := strings.ToLower(queryStr)

	// -- name queries --

	// Token-level match: "cool" finds "my-cool-note.md" via filename_analyzer.
	nameMatchQ := bleve.NewMatchQuery(queryStr)
	nameMatchQ.SetField("name")
	nameMatchQ.SetBoost(3.0)

	// Token-level prefix: "co" finds any token starting with "co" in the name.
	nameTokenPrefixQ := bleve.NewPrefixQuery(lq)
	nameTokenPrefixQ.SetField("name")
	nameTokenPrefixQ.SetBoost(2.0)

	// Whole-name prefix: "my-cool" finds "my-cool-note.md" as a raw prefix.
	nameRawPrefixQ := bleve.NewPrefixQuery(lq)
	nameRawPrefixQ.SetField("name_raw")
	nameRawPrefixQ.SetBoost(2.0)

	// Whole-name fuzzy: tolerates 1-character typos in the filename.
	nameFuzzyQ := bleve.NewFuzzyQuery(lq)
	nameFuzzyQ.SetField("name_raw")
	nameFuzzyQ.SetFuzziness(1)
	nameFuzzyQ.SetBoost(1.0)

	nameQ := bleve.NewDisjunctionQuery(nameMatchQ, nameTokenPrefixQ, nameRawPrefixQ, nameFuzzyQ)

	// -- content queries --

	// English: benefits from stop-word filtering and Porter stemming.
	contentEnQ := bleve.NewMatchQuery(queryStr)
	contentEnQ.SetField("content")

	var contentQ query.Query
	if useJieba {
		// Chinese: jieba-segmented field; also handles English queries correctly
		// since jieba passes through ASCII tokens unchanged.
		contentZhQ := bleve.NewMatchQuery(queryStr)
		contentZhQ.SetField("content_zh")
		contentQ = bleve.NewDisjunctionQuery(contentEnQ, contentZhQ)
	} else {
		contentQ = contentEnQ
	}

	// -- tag queries (YAML front matter tags; one token per tag) --
	tagQ := bleve.NewMatchQuery(queryStr)
	tagQ.SetField("tags")

	switch searchType {
	case "name":
		return nameQ
	case "content":
		return contentQ
	case "tag":
		return tagQ
	default:
		// Boost name hits above content hits in mixed results; tags between the two.
		nameQ.SetBoost(1.5)
		tagQ.SetBoost(1.8)
		return bleve.NewDisjunctionQuery(nameQ, contentQ, tagQ)
	}
}

// extractSearchTags returns front matter tags from a full note body using
// util.ParseFrontmatter, or nil when there are no tags.
func extractSearchTags(content []byte) []string {
	fm, _ := util.ParseFrontmatter(content)
	if len(fm.Tags) == 0 {
		return nil
	}
	out := make([]string, len(fm.Tags))
	copy(out, fm.Tags)
	return out
}

func byteLocsToLines(content []byte, termLocs bleveSearch.TermLocationMap) []int {
	seen := make(map[int]struct{})
	for _, locs := range termLocs {
		for _, loc := range locs {
			line := countNewlines(content, loc.Start) + 1
			seen[line] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	lines := make([]int, 0, len(seen))
	for ln := range seen {
		lines = append(lines, ln)
	}
	sort.Ints(lines)
	return lines
}

func countNewlines(content []byte, offset uint64) int {
	if offset > uint64(len(content)) {
		offset = uint64(len(content))
	}
	return bytes.Count(content[:offset], []byte("\n"))
}

func baseName(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}

// containsCJK reports whether s contains at least one CJK character.
// Used to decide whether to populate content_zh so that purely-English
// documents don't accumulate redundant jieba terms in the index.
func containsCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
			(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
			(r >= 0xF900 && r <= 0xFAFF) { // CJK Compatibility Ideographs
			return true
		}
	}
	return false
}

// cjkContent returns s when it contains CJK characters, and "" otherwise.
// Passing "" to bleve produces an empty token stream, leaving content_zh
// unpopulated for that document without touching the inverted index.
func cjkContent(s string) string {
	if containsCJK(s) {
		return s
	}
	return ""
}
