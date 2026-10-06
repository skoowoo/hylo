package cli

import (
	"encoding/json"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
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
		Short: "List notes",
		Long: `List notes, sorted by most recently updated. Output is JSON by default; use --table for a table view.

Pass a vault-absolute directory path (e.g. /journal) to scope the listing to that directory.`,
		Example: `  hylo list
  hylo list --latest 7
  hylo list --kind raw
  hylo list --start 2026-01-01 --end 2026-01-31
  hylo list --limit 20`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := notes.ListQuery{
				Limit: limit,
				Time:  notes.TimeFilter{Start: start, End: end, Latest: latest},
				Kind:  kind,
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
	cmd.Flags().StringVarP(&kind, "kind", "k", "", "filter by note kind: raw, short, knowledge, index")

	return cmd
}

func runList(q notes.ListQuery, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	entries, err := notes.List(c, q)
	if err != nil {
		return err
	}
	return printNotes(entries, table)
}

func printNotes(entries []notes.NoteEntry, table bool) error {
	if table {
		return printNoteTable(entries)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func printNoteTable(entries []notes.NoteEntry) error {
	if len(entries) == 0 {
		return nil
	}
	cols := []Column{
		{Header: "NAME", MaxWidth: 80},
		{Header: "DIR", MaxWidth: 60},
		{Header: "SIZE"},
		{Header: "KIND"},
		{Header: "UPDATED"},
		{Header: "INDEXED"},
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		indexed := "false"
		if e.Indexed {
			indexed = "true"
		}
		rows[i] = []string{e.Name, e.Dir, e.Size, e.Kind, e.UpdatedAt, indexed}
	}
	PrintTable(cols, rows)
	return nil
}
