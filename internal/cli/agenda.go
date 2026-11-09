package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/hardhacker/hylo/internal/agent"
)

// agentRun is the agent run the command comes from, when an agent set it;
// the server then acts for that run's bot. Outside a run it is the user.
func agentRun() string { return os.Getenv(agent.RunEnv) }

func newAgendaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "agenda",
		Short:        "List, add or close the main bot's agenda items",
		Long:         "The main bot's agenda holds what it means to do or follow up on. Its own rounds look at it, and an item with a due time wakes it then.",
		SilenceUsage: true,
	}

	listCmd := &cobra.Command{
		Use:          "list",
		Short:        "List open agenda items",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			items, err := c.Agenda(agentRun())
			if err != nil {
				return err
			}
			return printJSON(items)
		},
	}

	var due string
	addCmd := &cobra.Command{
		Use:          "add <text>",
		Short:        "Add an agenda item",
		Example:      `  hylo agenda add "Ask how the interview went" --due "2026-11-07 18:00"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			it, err := c.AgendaAdd(agentRun(), args[0], due)
			if err != nil {
				return err
			}
			return printJSON(it)
		},
	}
	addCmd.Flags().StringVar(&due, "due", "", `when to wake the bot for it, local time "YYYY-MM-DD HH:MM"`)

	doneCmd := &cobra.Command{
		Use:          "done <id>",
		Short:        "Close an agenda item",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			return c.AgendaDone(agentRun(), args[0])
		},
	}

	cmd.AddCommand(listCmd, addCmd, doneCmd)
	return cmd
}
