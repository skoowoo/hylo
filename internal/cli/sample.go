package cli

import (
	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newSampleCmd() *cobra.Command {
	var (
		table     bool
		each      int
		olderThan int
	)
	cmd := &cobra.Command{
		Use:   "sample",
		Short: "Draw a random walk through the vault",
		Long: `Draw a random walk through ordinary notes, knowledge units, and short entries.

The draw is spread across when things were written, not when they were last edited, and knowledge units are spread across domains. A unit is dated by its earliest source note.

--older-than 0 (the default) reaches back 90 days. A negative value keeps the whole history. A kind with nothing that old is drawn from all of it.`,
		Example: `  hylo sample
  hylo sample --each 3
  hylo sample --older-than -1`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			items, err := notes.Sample(c, notes.SampleQuery{Each: each, OlderThan: olderThan})
			if err != nil {
				return err
			}
			if table {
				return printSampleTable(items)
			}
			return printJSON(items)
		},
	}
	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	cmd.Flags().IntVar(&each, "each", 0, "how many to draw from each kind (0 = 2, max 8)")
	cmd.Flags().IntVar(&olderThan, "older-than", 0, "days back the draw must reach (0 = 90, negative = whole history)")
	return cmd
}

func printSampleTable(items []notes.SampleItem) error {
	if len(items) == 0 {
		return nil
	}
	cols := []Column{
		{Header: "KIND"},
		{Header: "DATE"},
		{Header: "DOMAIN", MaxWidth: 24},
		{Header: "PATH", MaxWidth: 48},
		{Header: "EXCERPT", MaxWidth: 60},
	}
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = []string{it.Kind, it.CreatedAt, it.Domain, it.Path, it.Excerpt}
	}
	PrintTable(cols, rows)
	return nil
}
