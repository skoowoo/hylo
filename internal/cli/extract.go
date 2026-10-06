package cli

import (
	"fmt"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
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
		Use:          "link <path-or-name>",
		Short:        "Extract all links from a note",
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

// withSource opens the client, loads the note's markdown and hands it to fn.
func withSource(pathOrName string, fn func(source []byte) error) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	source, err := notes.ReadSource(c, pathOrName)
	if err != nil {
		return err
	}
	return fn(source)
}

func runOutline(notePath, indent string) error {
	return withSource(notePath, func(source []byte) error {
		headings := client.ParseHeadings(source)
		if len(headings) == 0 {
			fmt.Println("(no headings found)")
			return nil
		}
		minLevel := headings[0].Level
		for _, h := range headings[1:] {
			minLevel = min(minLevel, h.Level)
		}
		for _, h := range headings {
			fmt.Printf("%s%s %s\n", strings.Repeat(indent, h.Level-minLevel), strings.Repeat("#", h.Level), h.Text)
		}
		return nil
	})
}

func runSection(pathOrName, query string) error {
	return withSource(pathOrName, func(source []byte) error {
		section, err := notes.Section(source, query)
		if err != nil {
			return err
		}
		fmt.Print(section)
		return nil
	})
}

func runBlocks(pathOrName, blockType string) error {
	return withSource(pathOrName, func(source []byte) error {
		switch blockType {
		case "code":
			blocks := client.ParseCodeBlocks(source)
			if len(blocks) == 0 {
				fmt.Println("(no code blocks found)")
				return nil
			}
			for i, b := range blocks {
				if i > 0 {
					fmt.Println()
				}
				lang := b.Lang
				if lang == "" {
					lang = "text"
				}
				fmt.Printf("[%d] language: %s\n```%s\n%s```\n", i+1, lang, b.Lang, b.Content)
			}
		case "list":
			lists := client.ParseLists(source)
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
	})
}

func runLinks(pathOrName string) error {
	return withSource(pathOrName, func(source []byte) error {
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
	})
}

func runExtractTags(pathOrName string) error {
	return withSource(pathOrName, func(source []byte) error {
		hasFM, tags := notes.Tags(source)
		if !hasFM {
			fmt.Println("(no YAML front matter found)")
			return nil
		}
		if len(tags) == 0 {
			fmt.Println("(no tags in front matter)")
			return nil
		}
		for _, t := range tags {
			fmt.Println(t)
		}
		return nil
	})
}

func runSegment(pathOrName string, head, tail, start, end int) error {
	spec := notes.SegmentSpec{Head: head, Tail: tail, Start: start, End: end}
	if err := spec.Validate(); err != nil {
		return err
	}
	return withSource(pathOrName, func(source []byte) error {
		lines, _ := notes.Segment(string(source), spec)
		if len(lines) == 0 {
			fmt.Println("(no lines in range)")
			return nil
		}
		fmt.Print(strings.Join(lines, "\n"))
		fmt.Println()
		return nil
	})
}
