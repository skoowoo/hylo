package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		table  bool
		limit  int
		start  string
		end    string
		latest int
		kind   string
	)

	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List notes",
		Long: `List notes, sorted by most recently updated. Output is JSON by default; use --table for a table view.

Pass a vault-absolute directory path (e.g. /journal) to scope the listing to that directory.`,
		Example: `  hylo list
  hylo list --latest 7
  hylo list --kind raw
  hylo list --start 2026-01-01 --end 2026-01-31
  hylo list --limit 20`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if latest > 0 && (start != "" || end != "") {
				return errLatestWithStartEnd()
			}
			opts := storage.ListOptions{
				SortByTime: true,
				Limit:      limit,
			}
			if err := applyListTimeFilters(&opts, latest, start, end); err != nil {
				return err
			}
			if len(args) == 0 {
				return runListAll(opts, kind, table)
			}
			return runListDir(args[0], opts, kind, table)
		},
	}

	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit number of results (0 = no limit)")
	cmd.Flags().StringVar(&start, "start", "", "filter notes updated on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "filter notes updated before this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&latest, "latest", 0, "filter notes updated within the last N days")
	cmd.Flags().StringVarP(&kind, "kind", "k", "", "filter by note kind: raw, short, knowledge, index")

	return cmd
}

func errLatestWithStartEnd() error {
	return fmt.Errorf("cannot combine --latest with --start/--end")
}

func applyListTimeFilters(opts *storage.ListOptions, latest int, start, end string) error {
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

func applyKindFilter(opts *storage.ListOptions, kind string) error {
	switch kind {
	case "":
		// no filter
	case "raw":
		opts.ExcludeKinds = []storage.Kind{storage.KindShort, storage.KindKnowledge, storage.KindIndex}
	case "short":
		opts.OnlyKinds = []storage.Kind{storage.KindShort}
	case "knowledge":
		opts.OnlyKinds = []storage.Kind{storage.KindKnowledge}
	case "index":
		opts.OnlyKinds = []storage.Kind{storage.KindIndex}
	default:
		return fmt.Errorf("unknown kind %q: must be raw, short, knowledge, or index", kind)
	}
	return nil
}

func runListAll(opts storage.ListOptions, kind string, table bool) error {
	if err := applyKindFilter(&opts, kind); err != nil {
		return err
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	notes, err := c.ListAllNotes(opts)
	if err != nil {
		return err
	}
	return printNotes(notes, table)
}

func runListDir(dir string, opts storage.ListOptions, kind string, table bool) error {
	if err := applyKindFilter(&opts, kind); err != nil {
		return err
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	notes, err := c.ListDir(dir, opts)
	if err != nil {
		return err
	}
	return printNotes(notes, table)
}

func noteKind(k storage.Kind) string {
	if k == "" {
		return "raw"
	}
	return string(k)
}

func printNotes(notes []storage.Note, table bool) error {
	entries := make([]listEntry, len(notes))
	for i, n := range notes {
		entries[i] = listEntry{
			Name:      n.Name,
			Dir:       n.Dir,
			Size:      util.FormatSize(n.Size),
			UpdatedAt: util.FormatTime(n.UpdatedAt),
			Indexed:   n.Indexed,
			Kind:      noteKind(n.Kind),
		}
	}
	if table {
		return printNoteTable(entries)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

type listEntry struct {
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	Size      string `json:"size"`
	UpdatedAt string `json:"updated_at"`
	Indexed   bool   `json:"indexed,omitempty"`
	Kind      string `json:"kind"`
}

func printNoteTable(entries []listEntry) error {
	if len(entries) == 0 {
		return nil
	}
	cols := []Column{
		{Header: "NAME", MaxWidth: 80},
		{Header: "DIR", MaxWidth: 60},
		{Header: "SIZE"},
		{Header: "KIND"},
		{Header: "UPDATED"},
		{Header: "INDEXED"},
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		indexed := "false"
		if e.Indexed {
			indexed = "true"
		}
		rows[i] = []string{e.Name, e.Dir, e.Size, e.Kind, e.UpdatedAt, indexed}
	}
	PrintTable(cols, rows)
	return nil
}
