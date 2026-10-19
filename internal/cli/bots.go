package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func newBotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "bot",
		Short:        "List, show, create, change or delete agent bots",
		SilenceUsage: true,
	}

	var listTable bool
	listCmd := &cobra.Command{
		Use:          "list",
		Short:        "List agent bots with how many triggers each has",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			bots, err := c.Bots()
			if err != nil {
				return err
			}
			if !listTable {
				return printJSON(bots)
			}
			rows := make([][]string, len(bots))
			for i, b := range bots {
				rows[i] = []string{b.ID, b.Name, fmt.Sprintf("%d", b.TriggerCount), fmt.Sprintf("%t", b.Enabled)}
			}
			PrintTable([]Column{{Header: "ID"}, {Header: "NAME", MaxWidth: 40}, {Header: "TRIGGERS"}, {Header: "ENABLED"}}, rows)
			return nil
		},
	}
	listCmd.Flags().BoolVarP(&listTable, "table", "t", false, "output in table format")

	getCmd := &cobra.Command{
		Use:          "get <bot-id>",
		Short:        "Show a bot with its triggers",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			b, err := c.Bot(args[0])
			if err != nil {
				return err
			}
			return printJSON(b)
		},
	}

	var description, systemPrompt string
	createCmd := &cobra.Command{
		Use:          "create <name>",
		Short:        "Create an agent bot",
		Long:         `Creates a bot on the same agent and model as your first bot, working in a vault folder of its own named after it. Add triggers to it with "hylo trigger add".`,
		Example:      `  hylo bot create News --description "Morning news digests"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			b, err := notes.BotCreate(c, args[0], description, systemPrompt)
			if err != nil {
				return err
			}
			return printJSON(b)
		},
	}
	createCmd.Flags().StringVar(&description, "description", "", "one line on what the bot does")
	createCmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "standing instructions for every run of this bot")

	var change notes.BotChange
	botUpdateCmd := &cobra.Command{
		Use:          "update <bot-id>",
		Short:        "Change a bot's name, description or system prompt, or turn it on or off",
		Example:      `  hylo bot update <bot-id> --name "Morning News" --enabled=false`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			fl := cmd.Flags()
			if fl.Changed("description") {
				change.Description = &description
			}
			if fl.Changed("system-prompt") {
				change.SystemPrompt = &systemPrompt
			}
			if fl.Changed("enabled") {
				on, _ := fl.GetBool("enabled")
				change.Enabled = &on
			}
			b, err := notes.BotUpdate(c, args[0], change)
			if err != nil {
				return err
			}
			return printJSON(b)
		},
	}
	botUpdateCmd.Flags().StringVar(&change.Name, "name", "", "new name")
	botUpdateCmd.Flags().StringVar(&description, "description", "", "new description")
	botUpdateCmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "new system prompt, replacing the old one")
	botUpdateCmd.Flags().Bool("enabled", true, "turn the bot on (true) or off (false)")

	botDeleteCmd := &cobra.Command{
		Use:          "delete <bot-id>",
		Short:        "Delete a bot with its triggers and chats",
		Long:         `Deletes the bot, its triggers and its chats; its vault folder stays. There is no undo.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			if err := notes.BotDelete(c, args[0]); err != nil {
				return err
			}
			fmt.Println("Bot deleted.")
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd, createCmd, botUpdateCmd, botDeleteCmd)
	return cmd
}

func newTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "trigger",
		Short:        "Add, change, delete or run a bot's triggers",
		SilenceUsage: true,
	}

	var spec notes.TriggerSpec
	addCmd := &cobra.Command{
		Use:   "add <bot-id>",
		Short: "Add a trigger to a bot",
		Long:  `Adds a trigger that runs the bot on a schedule, or when the given events happen. Its output goes to your inbox.`,
		Example: `  hylo trigger add <bot-id> --schedule "daily 08:00" --prompt "Use the hackernews skill to get today's news."
  hylo trigger add <bot-id> --events note_created --path-prefixes "/Web Clips/" --prompt "Summarize {Path}"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			id, err := notes.TriggerAdd(c, args[0], spec)
			if err != nil {
				return err
			}
			return printJSON(map[string]string{"trigger_id": id})
		},
	}
	addCmd.Flags().StringVar(&spec.Name, "name", "", "short label for the trigger")
	addCmd.Flags().StringVar(&spec.Prompt, "prompt", "", "what the bot is asked to do each time the trigger fires")
	addCmd.Flags().StringVar(&spec.Schedule, "schedule", "", `"every 1h", "daily 08:00" or "weekly mon,wed 09:00", server local time`)
	addCmd.Flags().StringSliceVar(&spec.Events, "events", nil, "event types to fire on instead of a schedule, e.g. note_created")
	addCmd.Flags().StringSliceVar(&spec.PathPrefixes, "path-prefixes", nil, "for note events: only notes under these folders")

	var change notes.TriggerChange
	updateCmd := &cobra.Command{
		Use:          "update <bot-id> <trigger-id>",
		Short:        "Change a trigger's label, prompt or schedule, or turn it on or off",
		Example:      `  hylo trigger update <bot-id> <trigger-id> --schedule "daily 09:00"`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			if f := cmd.Flags().Lookup("enabled"); f.Changed {
				on := strings.EqualFold(f.Value.String(), "true")
				change.Enabled = &on
			}
			if err := notes.TriggerUpdate(c, args[0], args[1], change); err != nil {
				return err
			}
			fmt.Println("Trigger updated.")
			return nil
		},
	}
	updateCmd.Flags().StringVar(&change.Name, "name", "", "new label")
	updateCmd.Flags().StringVar(&change.Prompt, "prompt", "", "new prompt")
	updateCmd.Flags().StringVar(&change.Schedule, "schedule", "", "new schedule for a scheduled trigger")
	updateCmd.Flags().Bool("enabled", true, "turn the trigger on (true) or off (false)")

	deleteCmd := &cobra.Command{
		Use:          "delete <bot-id> <trigger-id>",
		Short:        "Remove a trigger from a bot",
		Long:         `Removes the trigger from the bot; there is no undo.`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			if err := notes.TriggerDelete(c, args[0], args[1]); err != nil {
				return err
			}
			fmt.Println("Trigger deleted.")
			return nil
		},
	}

	runCmd := &cobra.Command{
		Use:          "run <bot-id> <trigger-id>",
		Short:        "Run a scheduled trigger once now; the result goes to your inbox",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			if err := c.RunTrigger(args[0], args[1]); err != nil {
				return err
			}
			fmt.Println("Started; the result arrives in your inbox when it's done.")
			return nil
		},
	}

	cmd.AddCommand(addCmd, updateCmd, deleteCmd, runCmd)
	return cmd
}
