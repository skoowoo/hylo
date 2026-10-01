package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var table bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show live status of the running server",
		Long: `Query the running Hylo server for live operational status.

Displays metrics such as the total number of notes tracked in the metadata
database and the number of notes currently indexed in the full-text search
engine. Additional data points will be included as the server exposes them.

The server must be running. Start it with: hylo start server`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			status, err := c.Status()
			if err != nil {
				return err
			}
			if table {
				return printStatus(status)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(status)
		},
	}

	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in human-readable format")

	return cmd
}

func printStatus(s *client.StatusResponse) error {
	rows := []struct{ k, v string }{
		{"vault.notes", fmt.Sprintf("%d", s.Notes)},
		{"vault.knowledge", fmt.Sprintf("%d", s.KnowledgeNotes)},
		{"vault.short_days", fmt.Sprintf("%d", s.ShortDays)},
		{"search.indexed", fmt.Sprintf("%d", s.Indexed)},
	}

	width := 0
	for _, r := range rows {
		if len(r.k) > width {
			width = len(r.k)
		}
	}
	for _, r := range rows {
		fmt.Printf("%-*s  %s\n", width, r.k, r.v)
	}
	return nil
}
