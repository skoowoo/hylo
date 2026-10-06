package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newPrependCmd() *cobra.Command {
	var content string

	cmd := &cobra.Command{
		Use:   "prepend <path> [heading]",
		Short: "Prepend content to a note",
		Long: `Insert content at the top of a note, or at the start of a named section.

<path> is a vault-absolute path starting with "/" (e.g. /journal/today.md).
If [heading] is given, content is inserted at the start of that section (case-insensitive match).
Content can be provided via --content or stdin.`,
		Example: `  hylo prepend /journal/today.md --content "## Morning"
  hylo prepend /journal/today.md "## Morning" --content "- woke up early"
  echo "- urgent" | hylo prepend /shopping.md`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			heading := ""
			if len(args) == 2 {
				heading = args[1]
			}
			return runPrepend(args[0], heading, content)
		},
	}

	cmd.Flags().StringVarP(&content, "content", "c", "", "inline content to prepend")

	return cmd
}

func runPrepend(notePath, heading, content string) error {
	if err := notes.CheckMarkdownPath(notePath); err != nil {
		return err
	}
	data, err := resolvePrependContent(content)
	if err != nil {
		return err
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	lines, err := notes.Prepend(c, notePath, heading, data)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "prepended %d lines to %q\n", lines, notePath)
	return nil
}

func resolvePrependContent(content string) ([]byte, error) {
	if content != "" {
		return []byte(content), nil
	}

	if isTerminal(os.Stdin) {
		fmt.Fprintln(os.Stderr, "Reading from stdin — press Ctrl+D when done:")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return data, nil
}
