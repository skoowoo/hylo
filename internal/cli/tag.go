package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
	var (
		listLimit   int
		listTable   bool
		countTable  bool
		deleteTable bool
		renameTable bool
	)

	cmd := &cobra.Command{
		Use:   "tag",
		Short: "List, count, rename or delete tags",
		Long:  `List tag usage across notes, count how many notes use a tag, or rename or delete a tag in every note. To search notes by tag, use: hylo search <query> --field tag`,

		SilenceUsage: true,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show each tag and how many notes use it",
		Long: `Prints every tag found, with the number of notes that include it.

If you have many different tags, raise --limit to include more of them (0 uses the server default).`,
		Example: `  hylo tag list
  hylo tag list --limit 1000
  hylo tag list --table`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTagList(listLimit, listTable)
		},
	}
	listCmd.Flags().IntVarP(&listLimit, "limit", "l", 0, "cap how many tags are listed (0 = server default)")
	listCmd.Flags().BoolVarP(&listTable, "table", "t", false, "output in table format")

	countCmd := &cobra.Command{
		Use:          "count <tag>",
		Short:        "Print how many notes have this tag",
		Long:         `Counts notes that include the tag. Matching follows the same rules as "hylo tag search".`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTagCount(args[0], countTable)
		},
	}
	countCmd.Flags().BoolVarP(&countTable, "table", "t", false, "output in table format")

	deleteCmd := &cobra.Command{
		Use:          "delete <tag>",
		Short:        "Delete a tag from every note",
		Long:         `Rewrites every note that has the tag: it is dropped from front matter tags, and an inline #tag keeps its word without the '#'. There is no undo.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTagDelete(args[0], deleteTable)
		},
	}
	deleteCmd.Flags().BoolVarP(&deleteTable, "table", "t", false, "output in table format")

	renameCmd := &cobra.Command{
		Use:          "rename <tag> <new-tag>",
		Short:        "Rename a tag in every note",
		Long:         `Rewrites every note that has the tag, in front matter and inline. A note that already has <new-tag> keeps one of them, so this also merges two tags. There is no undo.`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTagRename(args[0], args[1], renameTable)
		},
	}
	renameCmd.Flags().BoolVarP(&renameTable, "table", "t", false, "output in table format")

	cmd.AddCommand(listCmd, countCmd, deleteCmd, renameCmd)
	return cmd
}

func runTagList(limit int, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	tags, err := notes.TagList(c, limit)
	if err != nil {
		return err
	}
	if table {
		if len(tags) == 0 {
			fmt.Println("No tags in index.")
			return nil
		}
		fmt.Printf("Tags: %d\n\n", len(tags))
		cols := []Column{
			{Header: "TAG", MaxWidth: 80},
			{Header: "COUNT"},
		}
		rows := make([][]string, len(tags))
		for i, t := range tags {
			rows[i] = []string{t.Tag, fmt.Sprintf("%d", t.Count)}
		}
		PrintTable(cols, rows)
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(tags)
}

func runTagDelete(tag string, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	resp, err := c.TagDelete(tag)
	if err != nil {
		return err
	}
	return printTagRewrite(resp, table)
}

func runTagRename(tag, to string, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	resp, err := c.TagRename(tag, to)
	if err != nil {
		return err
	}
	return printTagRewrite(resp, table)
}

func printTagRewrite(resp *client.TagRewriteResponse, table bool) error {
	if table {
		cols := []Column{
			{Header: "TAG", MaxWidth: 80},
			{Header: "TO", MaxWidth: 80},
			{Header: "UPDATED"},
		}
		PrintTable(cols, [][]string{{resp.Tag, resp.To, fmt.Sprintf("%d", resp.Updated)}})
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func runTagCount(tag string, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	resp, err := c.TagCount(tag)
	if err != nil {
		return err
	}
	if table {
		cols := []Column{
			{Header: "TAG", MaxWidth: 80},
			{Header: "TOTAL"},
		}
		PrintTable(cols, [][]string{{resp.Tag, fmt.Sprintf("%d", resp.Total)}})
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
