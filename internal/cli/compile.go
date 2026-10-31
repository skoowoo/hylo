package cli

import (
	"github.com/spf13/cobra"
)

func newCompileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compile <path-or-name>",
		Short: "Compile a note into the knowledge base",
		Long: `Asks your Hylo bot to compile the note into structured knowledge under /_knowledge.
The bot does it as a turn in its chat once you've paused chatting with it, and replies there. A note that's already compiled is left as is.`,
		Example: `  hylo compile "/Projects/Q3 plan.md"
  hylo compile "Q3 plan.md"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			res, err := c.Compile(args[0])
			if err != nil {
				return err
			}
			return printJSON(res)
		},
	}
}
