package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
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
			q := notes.ListQuery{
				Limit:     limit,
				Time:      notes.TimeFilter{Start: start, End: end, Latest: latest},
				Kind:      kind,
				Knowledge: true,
			}
			if len(args) > 0 {
				q.Dir = args[0]
			}
			return runList(q, table)
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
			return runRead(args[0], true)
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
		true,
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

func runKnowledgeListIndexes(table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	entries, err := notes.ListIndexes(c)
	if err != nil {
		return err
	}
	if table {
		return printIndexTable(entries)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func printIndexTable(entries []notes.IndexEntry) error {
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
	c, err := openClient()
	if err != nil {
		return err
	}
	if err := notes.DeleteKnowledge(c, path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "deleted %q\n", path)
	return nil
}
