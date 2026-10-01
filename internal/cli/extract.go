package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/util"
	"github.com/spf13/cobra"
	"github.com/yuin/goldmark/ast"
)

func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "extract",
		Short:        "Extract structured data from notes",
		SilenceUsage: true,
	}

	var indent string
	outlineCmd := &cobra.Command{
		Use:   "outline <path-or-name>",
		Short: "Print the heading outline of a note",
		Example: `  hylo extract outline /journal/2026/today.md
  hylo extract outline today.md`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOutline(args[0], indent)
		},
	}
	outlineCmd.Flags().StringVar(&indent, "indent", "  ", "string used for each level of indentation")

	sectionCmd := &cobra.Command{
		Use:   "section <path-or-name> <heading>",
		Short: "Extract a named section from a note",
		Long: `Extract a named section from a note.

<heading> is matched case-insensitively. The section ends at the next heading of the same or higher level.`,
		Example: `  hylo extract section /journal/today.md "Goals"
  hylo extract section today.md "## meeting notes"`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSection(args[0], args[1])
		},
	}

	codeCmd := &cobra.Command{
		Use:          "code <path-or-name>",
		Short:        "Extract all fenced code blocks from a markdown note",
		Example:      "  hylo extract code /notes/recipe.md\n  hylo extract code snippet.md",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlocks(args[0], "code")
		},
	}

	linkCmd := &cobra.Command{
		Use:   "link <path-or-name>",
		Short: "Extract all links from a note",
		Example:      "  hylo extract link /notes/research.md\n  hylo extract link research.md",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinks(args[0])
		},
	}

	listCmd := &cobra.Command{
		Use:          "list <path-or-name>",
		Short:        "Extract all lists from a markdown note",
		Example:      "  hylo extract list /notes/todo.md\n  hylo extract list todo.md",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlocks(args[0], "list")
		},
	}

	var (
		segHead  int
		segTail  int
		segStart int
		segEnd   int
	)
	segmentCmd := &cobra.Command{
		Use:   "segment <path-or-name>",
		Short: "Extract a line range from a note",
		Long: `Extract lines from a note by range.

Use --head N, --tail N, or --start/--end for a specific line range (1-based, inclusive).`,
		Example: `  hylo extract segment /notes/today.md --head 10
  hylo extract segment today.md --tail 5
  hylo extract segment today.md --start 3 --end 12`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSegment(args[0], segHead, segTail, segStart, segEnd)
		},
	}
	segmentCmd.Flags().IntVar(&segHead, "head", 0, "print first N lines")
	segmentCmd.Flags().IntVar(&segTail, "tail", 0, "print last N lines")
	segmentCmd.Flags().IntVar(&segStart, "start", 0, "first line to print (1-based, inclusive)")
	segmentCmd.Flags().IntVar(&segEnd, "end", 0, "last line to print (1-based, inclusive)")

	tagCmd := &cobra.Command{
		Use:   "tag <path-or-name>",
		Short: "Print tags from a note's front matter",
		Example: `  hylo extract tag /notes/article.md
  hylo extract tag article.md`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtractTags(args[0])
		},
	}

	cmd.AddCommand(outlineCmd)
	cmd.AddCommand(sectionCmd)
	cmd.AddCommand(codeCmd)
	cmd.AddCommand(linkCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(segmentCmd)
	cmd.AddCommand(tagCmd)
	return cmd
}

// ── subcommand runners ────────────────────────────────────────────────────────

func runOutline(notePath, indent string) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	rc, err := c.ReadFile(notePath)
	if err != nil {
		return err
	}
	defer rc.Close()
	source, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	headings := mdParseHeadings(source)
	if len(headings) == 0 {
		fmt.Println("(no headings found)")
		return nil
	}

	minLevel := headings[0].level
	for _, h := range headings[1:] {
		if h.level < minLevel {
			minLevel = h.level
		}
	}
	for _, h := range headings {
		pad := strings.Repeat(indent, h.level-minLevel)
		fmt.Printf("%s%s %s\n", pad, strings.Repeat("#", h.level), h.text)
	}
	return nil
}

func runSection(pathOrName, query string) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	rc, err := c.ReadFile(pathOrName)
	if err != nil {
		return err
	}
	defer rc.Close()
	source, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	section := mdParseSection(source, query)
	if section == nil {
		return fmt.Errorf("no heading matching %q found", query)
	}
	fmt.Print(string(section))
	return nil
}

func runBlocks(pathOrName, blockType string) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	rc, err := c.ReadFile(pathOrName)
	if err != nil {
		return err
	}
	defer rc.Close()
	source, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	switch blockType {
	case "code":
		blocks := mdParseCodeBlocks(source)
		if len(blocks) == 0 {
			fmt.Println("(no code blocks found)")
			return nil
		}
		for i, b := range blocks {
			if i > 0 {
				fmt.Println()
			}
			lang := b.lang
			if lang == "" {
				lang = "text"
			}
			fmt.Printf("[%d] language: %s\n```%s\n%s```\n", i+1, lang, b.lang, b.content)
		}
	case "list":
		lists := mdParseLists(source)
		if len(lists) == 0 {
			fmt.Println("(no lists found)")
			return nil
		}
		for i, l := range lists {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("[%d]\n%s", i+1, l)
		}
	default:
		return fmt.Errorf("unknown block type %q — use: code, list", blockType)
	}
	return nil
}

func runLinks(pathOrName string) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	rc, err := c.ReadFile(pathOrName)
	if err != nil {
		return err
	}
	defer rc.Close()
	source, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	links := client.ParseLinks(source)
	if len(links) == 0 {
		fmt.Println("(no links found)")
		return nil
	}
	for i, l := range links {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(l.Format())
	}
	return nil
}

func runExtractTags(pathOrName string) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	rc, err := c.ReadFile(pathOrName)
	if err != nil {
		return err
	}
	defer rc.Close()
	source, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	fm, _ := util.ParseFrontmatter(source)
	if !fm.HasMeta() {
		fmt.Println("(no YAML front matter found)")
		return nil
	}
	if len(fm.Tags) == 0 {
		fmt.Println("(no tags in front matter)")
		return nil
	}
	for _, t := range fm.Tags {
		fmt.Println(t)
	}
	return nil
}

func runSegment(pathOrName string, head, tail, start, end int) error {
	// validate flag combinations
	flagCount := 0
	if head > 0 {
		flagCount++
	}
	if tail > 0 {
		flagCount++
	}
	if start > 0 || end > 0 {
		flagCount++
	}
	if flagCount == 0 {
		return fmt.Errorf("specify one of --head, --tail, or --start/--end")
	}
	if flagCount > 1 {
		return fmt.Errorf("--head, --tail, and --start/--end are mutually exclusive")
	}
	if (start > 0) != (end > 0) {
		return fmt.Errorf("--start and --end must be used together")
	}
	if start > 0 && start > end {
		return fmt.Errorf("--start (%d) must not be greater than --end (%d)", start, end)
	}

	c, err := openClient()
	if err != nil {
		return err
	}
	rc, err := c.ReadFile(pathOrName)
	if err != nil {
		return err
	}
	defer rc.Close()
	source, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	lines := strings.Split(string(source), "\n")
	// Trim trailing empty element caused by a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)

	var lo, hi int // 0-based, hi exclusive
	switch {
	case head > 0:
		lo = 0
		hi = head
	case tail > 0:
		lo = total - tail
		hi = total
	default: // start/end
		lo = start - 1
		hi = end
	}
	if lo < 0 {
		lo = 0
	}
	if hi > total {
		hi = total
	}
	if lo >= hi {
		fmt.Println("(no lines in range)")
		return nil
	}
	fmt.Print(strings.Join(lines[lo:hi], "\n"))
	fmt.Println()
	return nil
}

// ── markdown parsing helpers (CLI-specific: headings, sections, code, lists) ──
// Link extraction uses client.ParseLinks instead of a local implementation.

type heading struct {
	level int
	text  string
}

type codeBlock struct {
	lang    string
	content string
}

// mdParseHeadings returns all ATX headings in source using the goldmark AST.
// Headings inside fenced code blocks are correctly excluded.
func mdParseHeadings(source []byte) []heading {
	doc := client.MDParse(source)
	var out []heading
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Kind() != ast.KindHeading {
			return ast.WalkContinue, nil
		}
		h := n.(*ast.Heading)
		out = append(out, heading{level: h.Level, text: client.MDInlineText(h, source)})
		return ast.WalkSkipChildren, nil
	})
	return out
}

// mdParseSection extracts the raw source bytes for the first heading whose
// text contains query (case-insensitive). The section spans from the heading
// line through all content until the next heading of equal or higher level,
// or end of file.
func mdParseSection(source []byte, query string) []byte {
	doc := client.MDParse(source)
	query = strings.ToLower(strings.TrimLeft(strings.TrimSpace(query), "# \t"))

	type hpos struct {
		level int
		start int // byte offset of the heading line start
	}

	var positions []hpos
	matchIdx := -1

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok {
			continue
		}
		inner := mdNodeInnerStart(h, source)
		if inner < 0 {
			continue
		}
		lineStart := mdLineStart(source, inner)
		if matchIdx < 0 && strings.Contains(strings.ToLower(client.MDInlineText(h, source)), query) {
			matchIdx = len(positions)
		}
		positions = append(positions, hpos{level: h.Level, start: lineStart})
	}

	if matchIdx < 0 {
		return nil
	}

	secStart := positions[matchIdx].start
	secLevel := positions[matchIdx].level
	secEnd := len(source)
	for i := matchIdx + 1; i < len(positions); i++ {
		if positions[i].level <= secLevel {
			secEnd = positions[i].start
			break
		}
	}

	return source[secStart:secEnd]
}

// mdParseCodeBlocks extracts all fenced code blocks from source.
func mdParseCodeBlocks(source []byte) []codeBlock {
	doc := client.MDParse(source)
	var out []codeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fcb, ok := n.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}
		lang := ""
		if fcb.Info != nil {
			raw := strings.TrimSpace(string(fcb.Info.Segment.Value(source)))
			if i := strings.IndexByte(raw, ' '); i >= 0 {
				lang = raw[:i]
			} else {
				lang = raw
			}
		}
		var sb strings.Builder
		for i := 0; i < fcb.Lines().Len(); i++ {
			seg := fcb.Lines().At(i)
			sb.Write(seg.Value(source))
		}
		out = append(out, codeBlock{lang: lang, content: sb.String()})
		return ast.WalkContinue, nil
	})
	return out
}

// mdParseLists extracts top-level lists from source as raw markdown strings.
// Nested lists are not reported separately.
func mdParseLists(source []byte) []string {
	doc := client.MDParse(source)
	var out []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Kind() != ast.KindList {
			return ast.WalkContinue, nil
		}
		if s := mdRawSource(n, source); s != "" {
			out = append(out, s)
		}
		return ast.WalkSkipChildren, nil
	})
	return out
}

// ── low-level AST / source helpers ───────────────────────────────────────────

// mdNodeInnerStart returns the minimum byte offset of any content within n,
// searching both Lines() segments and ast.Text inline segments. Returns -1
// when n contains no content.
func mdNodeInnerStart(n ast.Node, source []byte) int {
	min := -1
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if child.Type() == ast.TypeBlock {
			if lines := child.Lines(); lines != nil {
				for i := 0; i < lines.Len(); i++ {
					if s := lines.At(i).Start; min < 0 || s < min {
						min = s
					}
				}
			}
		}
		if t, ok := child.(*ast.Text); ok {
			if s := t.Segment.Start; min < 0 || s < min {
				min = s
			}
		}
		return ast.WalkContinue, nil
	})
	return min
}

// mdNodeInnerStop returns the maximum byte offset (exclusive) of any content
// within n. Returns 0 when n contains no content.
func mdNodeInnerStop(n ast.Node, source []byte) int {
	max := 0
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if child.Type() == ast.TypeBlock {
			if lines := child.Lines(); lines != nil {
				for i := 0; i < lines.Len(); i++ {
					if s := lines.At(i).Stop; s > max {
						max = s
					}
				}
			}
		}
		if t, ok := child.(*ast.Text); ok {
			if s := t.Segment.Stop; s > max {
				max = s
			}
		}
		return ast.WalkContinue, nil
	})
	return max
}

// mdLineStart returns the byte offset of the start of the source line
// containing pos (scanning backward for a preceding '\n').
func mdLineStart(source []byte, pos int) int {
	for pos > 0 && source[pos-1] != '\n' {
		pos--
	}
	return pos
}

// mdLineEnd returns the byte offset one past the end of the source line
// containing pos (past '\n', or EOF).
func mdLineEnd(source []byte, pos int) int {
	for pos < len(source) && source[pos] != '\n' {
		pos++
	}
	if pos < len(source) {
		pos++ // past '\n'
	}
	return pos
}

// mdRawSource extracts the verbatim markdown source for block node n by
// finding the full line span from its first to its last content byte.
// For container blocks (blockquote, list) this preserves markers like "> " or "- ".
func mdRawSource(n ast.Node, source []byte) string {
	start := mdNodeInnerStart(n, source)
	stop := mdNodeInnerStop(n, source)
	if start < 0 || stop == 0 || start >= stop {
		return ""
	}
	lineStart := mdLineStart(source, start)
	lineEnd := mdLineEnd(source, stop-1)
	if lineEnd > len(source) {
		lineEnd = len(source)
	}
	return string(source[lineStart:lineEnd])
}
