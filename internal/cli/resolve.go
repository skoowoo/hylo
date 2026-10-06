package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newResolveCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "resolve <name>",
		Short: "Look up vault path(s) for a note filename",
		Long: `Look up every vault path for the given filename.

Pass a filename with or without .md. Use --json for full metadata.`,
		Example: `  hylo resolve today.md
  hylo resolve today
  hylo resolve today --json`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}

			result, err := notes.Resolve(c, args[0])
			if err != nil {
				return err
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result.Matches)
			}

			for _, n := range result.Matches {
				fmt.Println(n.PathString())
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "output full note metadata as JSON")
	return cmd
}
