package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "apps",
		Short:        "List the user's connected apps, and use their tools",
		Long:         "Apps are the user's accounts on outside platforms, connected in Settings → Apps. An app with tools (like Podwise) is used through these commands; an app with a skill (like WeChat Reading) through its skill. A bot may use the apps the user turned on for it.",
		SilenceUsage: true,
	}

	listCmd := &cobra.Command{
		Use:          "list",
		Short:        "List every app and whether it is connected",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			list, err := c.Apps()
			if err != nil {
				return err
			}
			return printJSON(list)
		},
	}

	toolsCmd := &cobra.Command{
		Use:          "tools <app>",
		Short:        "List a connected app's tools and their input schemas",
		Example:      "  hylo apps tools podwise",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			tools, err := c.AppTools(agentRun(), args[0])
			if err != nil {
				return err
			}
			return printJSON(tools)
		},
	}

	var callArgs string
	callCmd := &cobra.Command{
		Use:          "call <app> <tool>",
		Short:        "Run a tool of a connected app",
		Example:      `  hylo apps call podwise list_my_episodes --args '{"limit": 10}'`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if callArgs != "" && !json.Valid([]byte(callArgs)) {
				return fmt.Errorf("--args is not valid JSON")
			}
			c, err := openClient()
			if err != nil {
				return err
			}
			res, err := c.AppCall(agentRun(), args[0], args[1], json.RawMessage(callArgs))
			if err != nil {
				return err
			}
			return printJSON(res)
		},
	}
	callCmd.Flags().StringVar(&callArgs, "args", "", "the tool's arguments, a JSON object")

	cmd.AddCommand(listCmd, toolsCmd, callCmd)
	return cmd
}
