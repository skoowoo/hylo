package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/util"
	"github.com/spf13/cobra"
)

type searchScope int

const (
	searchScopeAny       searchScope = iota // no filter
	searchScopeKnowledge                    // knowledge notes only
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
		searchScopeAny,
	)
}

func buildSearchCommand(short, long, example string, scope searchScope) *cobra.Command {
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
			switch field {
			case "", "name", "content", "tag":
			default:
				return fmt.Errorf("unknown --field %q: must be name, content, or tag (default: all)", field)
			}
			return runSearch(args[0], field, limit, table, scope)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 20, "maximum number of results")
	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	cmd.Flags().StringVarP(&field, "field", "f", "", "search field: name, content, tag (default: all)")

	return cmd
}

type searchOutput struct {
	Query   string        `json:"query"`
	Total   int           `json:"total"`
	Results []searchEntry `json:"results"`
}

type searchEntry struct {
	Name      string  `json:"name"`
	Dir       string  `json:"dir"`
	Kind      string  `json:"kind"`
	UpdatedAt string  `json:"updated_at"`
	Score     float64 `json:"score"`
	HitLines  []int   `json:"hit_lines,omitempty"`
}

func runSearch(query, field string, limit int, table bool, scope searchScope) error {
	c, err := openClient()
	if err != nil {
		return err
	}

	var fetchLimit int
	if scope == searchScopeAny {
		fetchLimit = limit
	} else {
		fetchLimit = limit * 25
		if fetchLimit < 100 {
			fetchLimit = 100
		}
		if fetchLimit > 3000 {
			fetchLimit = 3000
		}
	}

	resp, err := c.Search(query, field, fetchLimit)
	if err != nil {
		return err
	}

	var hits []client.SearchResult
	for _, hit := range resp.Results {
		if scope == searchScopeKnowledge && hit.Kind != "knowledge" && hit.Kind != "index" {
			continue
		}
		hits = append(hits, hit)
		if len(hits) >= limit {
			break
		}
	}

	out := searchOutput{
		Query:   resp.Query,
		Total:   len(hits),
		Results: make([]searchEntry, 0, len(hits)),
	}
	for _, hit := range hits {
		out.Results = append(out.Results, searchEntry{
			Name:      hit.Name,
			Dir:       hit.Dir,
			Kind:      hit.Kind,
			UpdatedAt: util.FormatTime(hit.UpdatedAt),
			Score:     math.Round(hit.Score*1000) / 1000,
			HitLines:  hit.Lines,
		})
	}

	if table {
		return printSearchTable(out)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printSearchTable(out searchOutput) error {
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
