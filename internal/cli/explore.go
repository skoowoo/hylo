package cli

import "github.com/spf13/cobra"

func newActivityCmd() *cobra.Command {
	var since, actor string
	var limit int
	cmd := &cobra.Command{
		Use:          "activity",
		Short:        "List who read and wrote which notes, newest first",
		Example:      `  hylo activity --since 6h --actor user`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			entries, err := c.Activity(since, actor, limit)
			if err != nil {
				return err
			}
			return printJSON(entries)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", `how far back: a duration like "6h" or "3d", or local "YYYY-MM-DD HH:MM"; default a day`)
	cmd.Flags().StringVar(&actor, "actor", "", `only this actor: "user", or a bot id`)
	cmd.Flags().IntVar(&limit, "limit", 100, "at most this many entries")
	return cmd
}

func newScreenTimeCmd() *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:          "screen-time",
		Short:        "Show where your time at the computer went, by app",
		Long:         "From the desktop app's presence reports (proactive.desktop): whether you're at the computer now, time per category and app, and the latest stretches.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			st, err := c.ScreenTime(since)
			if err != nil {
				return err
			}
			return printJSON(st)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", `from when: a duration like "6h" or "3d", or local "YYYY-MM-DD HH:MM"; default today`)
	return cmd
}
