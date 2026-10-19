package cli

import (
	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newCapabilitiesCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "capabilities",
		Short:        "Show every bot with its skills and triggers, catalog skills not added yet, and trigger events",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			out, err := notes.GetCapabilities(c)
			if err != nil {
				return err
			}
			return printJSON(out)
		},
	}
}
