package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	return buildSearchCommand(
		"Search notes",
		`Search notes by filename, content, or tag. Output is JSON by default; use --table for a table view.

Query syntax:
  word             match files containing "word"
  word1 word2      match either word (OR)
  "exact phrase"   phrase match`,
		`  hylo search "meeting notes"
  hylo search april --field name
  hylo search "TODO" --field content --limit 5
  hylo search "golang" --field tag`,
		false,
	)
}

func buildSearchCommand(short, long, example string, knowledgeOnly bool) *cobra.Command {
	var (
		limit int
		table bool
		field string
	)

	cmd := &cobra.Command{
		Use:          "search <query>",
		Short:        short,
		Long:         long,
		Example:      example,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(args[0], field, limit, table, knowledgeOnly)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 20, "maximum number of results")
	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	cmd.Flags().StringVarP(&field, "field", "f", "", "search field: name, content, tag (default: all)")

	return cmd
}

func runSearch(query, field string, limit int, table, knowledgeOnly bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	out, err := notes.Search(c, notes.SearchQuery{Query: query, Field: field, Limit: limit, KnowledgeOnly: knowledgeOnly})
	if err != nil {
		return err
	}
	if table {
		return printSearchTable(out)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printSearchTable(out notes.SearchResult) error {
	if len(out.Results) == 0 {
		fmt.Printf("No results found for query: %s\n", out.Query)
		return nil
	}

	fmt.Printf("Query: %s (Total: %d)\n\n", out.Query, out.Total)

	cols := []Column{
		{Header: "NAME", MaxWidth: 80},
		{Header: "DIR", MaxWidth: 60},
		{Header: "KIND", MaxWidth: 9},
		{Header: "UPDATED"},
		{Header: "SCORE"},
		{Header: "LINES"},
	}
	rows := make([][]string, len(out.Results))
	for i, e := range out.Results {
		lines := "-"
		if len(e.HitLines) > 0 {
			lines = fmt.Sprintf("%d", len(e.HitLines))
		}
		rows[i] = []string{e.Name, e.Dir, e.Kind, e.UpdatedAt, fmt.Sprintf("%.3f", e.Score), lines}
	}
	PrintTable(cols, rows)
	return nil
}
