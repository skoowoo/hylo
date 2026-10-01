package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
	var (
		listLimit   int
		listTable   bool
		countTable  bool
		deleteTable bool
	)

	cmd := &cobra.Command{
		Use:   "tag",
		Short: "List tags, count tag usage, or delete by tag",
		Long:  `List tag usage across notes, count how many notes use a tag, or remove notes from the index by tag. To search notes by tag, use: hylo search <query> --field tag`,

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
		Short:        "Remove all notes with this tag from the search index",
		Long:         `Searches the index for notes that have the given tag and removes them from the search index. The note files on disk are not affected.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTagDelete(args[0], deleteTable)
		},
	}
	deleteCmd.Flags().BoolVarP(&deleteTable, "table", "t", false, "output in table format")

	cmd.AddCommand(listCmd, countCmd, deleteCmd)
	return cmd
}

func runTagList(limit int, table bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	resp, err := c.TagList(limit)
	if err != nil {
		return err
	}
	if table {
		if len(resp.Tags) == 0 {
			fmt.Println("No tags in index.")
			return nil
		}
		fmt.Printf("Tags: %d\n\n", len(resp.Tags))
		cols := []Column{
			{Header: "TAG", MaxWidth: 80},
			{Header: "COUNT"},
		}
		rows := make([][]string, len(resp.Tags))
		for i, t := range resp.Tags {
			rows[i] = []string{t.Tag, fmt.Sprintf("%d", t.Count)}
		}
		PrintTable(cols, rows)
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	tags := resp.Tags
	if tags == nil {
		tags = []client.TagStat{}
	}
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
	if table {
		cols := []Column{
			{Header: "TAG", MaxWidth: 80},
			{Header: "DELETED"},
		}
		PrintTable(cols, [][]string{{resp.Tag, fmt.Sprintf("%d", resp.Deleted)}})
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
