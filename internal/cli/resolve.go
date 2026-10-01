package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
			name := args[0]
			if filepath.Ext(name) == "" {
				name += ".md"
			}

			c, err := openClient()
			if err != nil {
				return err
			}

			result, err := c.ResolveNoteName(name)
			if err != nil {
				return err
			}

			if result.Count == 0 {
				return fmt.Errorf("no note found with name %q", name)
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
