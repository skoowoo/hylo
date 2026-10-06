package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newAppendCmd() *cobra.Command {
	var content string

	cmd := &cobra.Command{
		Use:   "append <path> [heading]",
		Short: "Append content to a note",
		Long: `Append content to the end of a note, or after a named section.

<path> is a vault-absolute path starting with "/" (e.g. /journal/today.md).
If [heading] is given, content is appended after that section (case-insensitive match).
Content can be provided via --content or stdin.`,
		Example: `  hylo append /journal/today.md --content "## Evening"
  hylo append /journal/today.md "## Morning" --content "- woke up early"
  echo "- buy milk" | hylo append /shopping.md`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			heading := ""
			if len(args) == 2 {
				heading = args[1]
			}
			return runAppend(args[0], heading, content)
		},
	}

	cmd.Flags().StringVarP(&content, "content", "c", "", "inline content to append")

	return cmd
}

func runAppend(notePath, heading, content string) error {
	if err := notes.CheckMarkdownPath(notePath); err != nil {
		return err
	}
	data, err := resolveAppendContent(content)
	if err != nil {
		return err
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	lines, err := notes.Append(c, notePath, heading, data)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "appended %d lines to %q\n", lines, notePath)
	return nil
}

// resolveAppendContent returns content from the inline flag or stdin.
// Local file import is intentionally not supported for append.
func resolveAppendContent(content string) ([]byte, error) {
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
