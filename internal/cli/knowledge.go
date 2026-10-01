package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/spf13/cobra"
)

func newKnowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "knowledge",
		Short:        "List, read, search, and delete knowledge notes",
		Long:         `Commands for knowledge notes — the output of the compile plugin.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(
		newKnowledgeListCmd(),
		newKnowledgeListIndexesCmd(),
		newKnowledgeReadCmd(),
		newKnowledgeSearchCmd(),
		newKnowledgeDeleteCmd(),
	)
	return cmd
}

func newKnowledgeListCmd() *cobra.Command {
	var (
		table  bool
		limit  int
		start  string
		end    string
		latest int
		kind   string
	)

	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List knowledge notes",
		Long: `List knowledge notes, sorted by most recently updated. Output is JSON by default; use --table for a table view.

Pass a vault-absolute directory path (e.g. /_knowledge) to scope the listing to that directory.`,
		Example: `  hylo knowledge list
  hylo knowledge list --kind index
  hylo knowledge list --latest 7
  hylo knowledge list --start 2026-01-01 --end 2026-01-31
  hylo knowledge list --limit 20`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if latest > 0 && (start != "" || end != "") {
				return errLatestWithStartEnd()
			}
			opts := storage.ListOptions{
				SortByTime: true,
				Limit:      limit,
			}
			if err := applyListTimeFilters(&opts, latest, start, end); err != nil {
				return err
			}
			if len(args) == 0 {
				return runKnowledgeListAll(opts, kind, table)
			}
			return runKnowledgeListDir(args[0], opts, kind, table)
		},
	}

	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit number of results (0 = no limit)")
	cmd.Flags().StringVar(&start, "start", "", "filter notes updated on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "filter notes updated before this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&latest, "latest", 0, "filter notes updated within the last N days")
	cmd.Flags().StringVarP(&kind, "kind", "k", "", "filter by note kind: knowledge, index")

	return cmd
}

func newKnowledgeReadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "read <path-or-name>",
		Short: "Print the content of a knowledge note",
		Long: `Print a knowledge note to stdout.

Pass a vault-absolute path (e.g. /_knowledge/summary.md) or a bare filename.
If multiple notes share the same name, use the full vault path.`,
		Example: `  hylo knowledge read "/_knowledge/summary.md"
  hylo knowledge read summary.md`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScopedRead(args[0], true)
		},
	}
}

func newKnowledgeSearchCmd() *cobra.Command {
	return buildSearchCommand(
		"Search knowledge notes by filename or content",
		`Full-text search over knowledge notes only.

Output is JSON by default, or table format with --table.

Query syntax:
  word              match any file containing "word"
  word1 word2       match files containing either word (OR)
  "exact phrase"    phrase search within content`,
		`  hylo knowledge search "summary"
  hylo knowledge search clip --field name
  hylo knowledge search "TODO" --field content --limit 5`,
		searchScopeKnowledge,
	)
}

func newKnowledgeDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <path>",
		Short: "Permanently delete a knowledge note",
		Long: `Delete the knowledge note at <path>.

<path> is a vault-absolute path starting with "/" (e.g. /_knowledge/article.md).`,
		Example:      `  hylo knowledge delete "/_knowledge/article.md"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKnowledgeDelete(args[0])
		},
	}
}

var knowledgeKinds = []storage.Kind{
	storage.KindKnowledge,
	storage.KindIndex,
}

func runKnowledgeListAll(opts storage.ListOptions, kind string, table bool) error {
	kinds, err := resolveKnowledgeKinds(kind)
	if err != nil {
		return err
	}
	if len(kinds) == 0 {
		return printNotes(nil, table)
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	opts.OnlyKinds = kinds
	notes, err := c.ListAllNotes(opts)
	if err != nil {
		return err
	}
	return printNotes(notes, table)
}

func runKnowledgeListDir(dir string, opts storage.ListOptions, kind string, table bool) error {
	kinds, err := resolveKnowledgeKinds(kind)
	if err != nil {
		return err
	}
	if len(kinds) == 0 {
		return printNotes(nil, table)
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	opts.OnlyKinds = kinds
	notes, err := c.ListDir(dir, opts)
	if err != nil {
		return err
	}
	return printNotes(notes, table)
}

// resolveKnowledgeKinds returns the Kind slice for the given kind filter.
// Empty kind returns all knowledge-subsystem kinds. Unknown kind returns an error.
func resolveKnowledgeKinds(kind string) ([]storage.Kind, error) {
	switch kind {
	case "":
		return knowledgeKinds, nil
	case "knowledge":
		return []storage.Kind{storage.KindKnowledge}, nil
	case "index":
		return []storage.Kind{storage.KindIndex}, nil
	default:
		return nil, fmt.Errorf("unknown kind %q: must be knowledge or index", kind)
	}
}

func sortAndLimit(notes []storage.Note, limit int) []storage.Note {
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})
	if limit > 0 && len(notes) > limit {
		return notes[:limit]
	}
	return notes
}


func newKnowledgeListIndexesCmd() *cobra.Command {
	var table bool

	cmd := &cobra.Command{
		Use:          "list-indexes",
		Short:        "List index notes",
		Long:         `List all index notes. Each entry shows the domain and the vault path of the index note. Output is JSON by default; use --table for a table view.`,
		Example:      `  hylo knowledge list-indexes\n  hylo knowledge list-indexes --table`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKnowledgeListIndexes(table)
		},
	}

	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	return cmd
}

type indexEntry struct {
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

func runKnowledgeListIndexes(table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	notes, err := c.ListAllNotes(storage.ListOptions{
		SortByTime: true,
		OnlyKinds:  []storage.Kind{storage.KindIndex},
	})
	if err != nil {
		return err
	}
	entries := make([]indexEntry, len(notes))
	for i, n := range notes {
		entries[i] = indexEntry{
			Domain: n.Title,
			Path:   n.PathString(),
		}
	}
	if table {
		return printIndexTable(entries)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func printIndexTable(entries []indexEntry) error {
	if len(entries) == 0 {
		return nil
	}
	cols := []Column{
		{Header: "DOMAIN", MaxWidth: 60},
		{Header: "PATH", MaxWidth: 80},
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{e.Domain, e.Path}
	}
	PrintTable(cols, rows)
	return nil
}

func runKnowledgeDelete(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path %q must be absolute (start with \"/\")", path)
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	note, err := c.StatNote(path)
	if err != nil {
		return err
	}
	if note.Kind != storage.KindKnowledge {
		return fmt.Errorf("%q is not a knowledge note", note.PathString())
	}
	if err := c.DeleteNote(path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "deleted %q\n", path)
	return nil
}
