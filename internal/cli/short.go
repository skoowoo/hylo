package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newShortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "short",
		Short: "Create and manage short notes",
		Long: `Operate on short notes — quick captures appended to a daily file.

Each entry has an id like 2026-10-29-153012 (its day and time), shown by
"short list" and accepted by get, update and delete.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newShortCreateCmd(), newShortListCmd(), newShortGetCmd(), newShortUpdateCmd(), newShortDeleteCmd())
	return cmd
}

func newShortCreateCmd() *cobra.Command {
	var content string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Append a short note to today's daily shorts file",
		Long: `Append a short note entry to today's daily file inside the shorts directory.

Content sources (in priority order):
  1. --content <text>   inline text
  2. stdin              pipe or interactive input`,
		Example: `  hylo short create --content "Quick thought"
  echo "Buy groceries" | hylo short create`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := resolveAppendContent(content)
			if err != nil {
				return err
			}
			c, err := openClient()
			if err != nil {
				return err
			}
			e, err := notes.ShortCreate(c, string(data))
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "saved short note %s to %q\n", e.ID, e.Path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&content, "content", "c", "", "inline content for the short note")

	return cmd
}

func newShortListCmd() *cobra.Command {
	var (
		table  bool
		limit  int
		start  string
		end    string
		latest int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List short note entries",
		Long: `List individual short note entries, newest first.

Each daily shorts file is parsed into individual entries so you see one row per
note rather than one row per day.

Without date filters all entries are returned (subject to --limit).`,
		Example: `  hylo short list
  hylo short list --latest 7
  hylo short list --start 2026-01-01 --end 2026-01-31
  hylo short list --limit 20 --table`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShortList(notes.ShortQuery{
				Limit: limit,
				Time:  notes.TimeFilter{Start: start, End: end, Latest: latest},
			}, table)
		},
	}

	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit number of entries (0 = no limit)")
	cmd.Flags().StringVar(&start, "start", "", "filter entries created on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "filter entries created before this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&latest, "latest", 0, "filter entries created within the last N days")

	return cmd
}

func runShortList(q notes.ShortQuery, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	entries, err := notes.ShortList(c, q)
	if err != nil {
		return err
	}
	if table {
		return printShortTable(entries)
	}
	return printJSON(entries)
}

func newShortGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "get <id>",
		Short:        "Print one short note entry as JSON",
		Example:      `  hylo short get 2026-10-29-153012`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			e, err := c.GetShort(args[0])
			if err != nil {
				return err
			}
			return printJSON(e)
		},
	}
}

func newShortUpdateCmd() *cobra.Command {
	var content, hash string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Replace a short note entry's content",
		Long: `Replace the content of a short note entry, keeping its time and place in
the daily file.

Content sources (in priority order):
  1. --content <text>   inline text
  2. stdin              pipe or interactive input

Pass --hash (from "short get") to fail instead of overwriting a newer edit.`,
		Example: `  hylo short update 2026-10-29-153012 --content "Fixed thought"
  echo "Fixed thought" | hylo short update 2026-10-29-153012`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := resolveAppendContent(content)
			if err != nil {
				return err
			}
			c, err := openClient()
			if err != nil {
				return err
			}
			e, err := notes.ShortUpdate(c, args[0], hash, string(data))
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "updated short note %s in %q\n", e.ID, e.Path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&content, "content", "c", "", "new content for the short note")
	cmd.Flags().StringVar(&hash, "hash", "", "only edit if the entry still has this hash")

	return cmd
}

func newShortDeleteCmd() *cobra.Command {
	var hash string

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a short note entry",
		Long: `Permanently delete one short note entry; there is no undo. The daily file
is removed once its last entry is gone.`,
		Example:      `  hylo short delete 2026-10-29-153012`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			if err := c.DeleteShort(args[0], hash); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "deleted short note %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&hash, "hash", "", "only delete if the entry still has this hash")

	return cmd
}

func printShortTable(entries []client.ShortEntry) error {
	if len(entries) == 0 {
		return nil
	}
	cols := []Column{
		{Header: "ID", MaxWidth: 24},
		{Header: "CREATED", MaxWidth: 20},
		{Header: "CONTENT", MaxWidth: 80},
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		preview := strings.ReplaceAll(e.Content, "\n", " ")
		rows[i] = []string{e.ID, e.CreatedAt.Format("2006-01-02 15:04:05"), preview}
	}
	PrintTable(cols, rows)
	return nil
}
