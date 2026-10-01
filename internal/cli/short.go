package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/spf13/cobra"
)

func newShortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "short",
		Short:        "Create and manage short notes",
		Long:         `Operate on short notes — quick captures appended to a daily file.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newShortCreateCmd(), newShortListCmd())
	return cmd
}

func newShortCreateCmd() *cobra.Command {
	var (
		content string
		dir     string
	)

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
			return runShortCreate(content, dir)
		},
	}

	cmd.Flags().StringVarP(&content, "content", "c", "", "inline content for the short note")
	cmd.Flags().StringVar(&dir, "dir", "", `shorts directory override (default "_shorts")`)

	return cmd
}

func runShortCreate(content, dir string) error {
	data, err := resolveAppendContent(content)
	if err != nil {
		return err
	}
	text := string(data)
	if len([]rune(text)) == 0 {
		return fmt.Errorf("content must not be empty")
	}

	c, err := openClient()
	if err != nil {
		return err
	}

	note, err := c.CreateShort(text, dir)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "saved short note to %q\n", note.PathString())
	return nil
}

func newShortListCmd() *cobra.Command {
	var (
		table  bool
		limit  int
		start  string
		end    string
		latest int
		dir    string
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
			if latest > 0 && (start != "" || end != "") {
				return errLatestWithStartEnd()
			}
			opts := client.ShortListOptions{Dir: dir, Limit: limit}
			if err := applyShortTimeFilters(&opts, latest, start, end); err != nil {
				return err
			}
			return runShortList(opts, table)
		},
	}

	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit number of entries (0 = no limit)")
	cmd.Flags().StringVar(&start, "start", "", "filter entries created on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "filter entries created before this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&latest, "latest", 0, "filter entries created within the last N days")
	cmd.Flags().StringVar(&dir, "dir", "", `shorts directory (default "_shorts")`)

	return cmd
}

func applyShortTimeFilters(opts *client.ShortListOptions, latest int, start, end string) error {
	if latest > 0 {
		opts.After = time.Now().AddDate(0, 0, -latest)
	}
	if start != "" {
		t, err := time.Parse(time.DateOnly, start)
		if err != nil {
			return fmt.Errorf("invalid --start date (use YYYY-MM-DD): %w", err)
		}
		opts.After = t
	}
	if end != "" {
		t, err := time.Parse(time.DateOnly, end)
		if err != nil {
			return fmt.Errorf("invalid --end date (use YYYY-MM-DD): %w", err)
		}
		opts.Before = t
	}
	return nil
}

func runShortList(opts client.ShortListOptions, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	entries, err := c.ListShorts(opts)
	if err != nil {
		return err
	}
	if table {
		return printShortTable(entries)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func printShortTable(entries []client.ShortEntry) error {
	if len(entries) == 0 {
		return nil
	}
	cols := []Column{
		{Header: "CREATED", MaxWidth: 20},
		{Header: "DAILY", MaxWidth: 40},
		{Header: "CONTENT", MaxWidth: 80},
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		preview := strings.ReplaceAll(e.Content, "\n", " ")
		rows[i] = []string{e.CreatedAt.Format("2006-01-02 15:04:05"), e.DailyPath, preview}
	}
	PrintTable(cols, rows)
	return nil
}
