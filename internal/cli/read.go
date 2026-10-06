package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newReadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <path-or-name>",
		Short: "Print a note to stdout",
		Long: `Print a note to stdout.

Pass a vault-absolute path (e.g. /journal/today.md) or a bare filename.
If multiple notes share the same name, use the full vault path.`,
		Example: `  hylo read /journal/today.md
  hylo read today.md`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRead(args[0], false)
		},
	}
	return cmd
}

// runRead prints a note; knowledgeOnly is used by "hylo knowledge read".
func runRead(arg string, knowledgeOnly bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	p, err := notes.ResolvePath(c, arg, knowledgeOnly)
	if err != nil {
		return err
	}
	return streamReadFile(c, p)
}

func streamReadFile(c *client.Client, vaultPath string) error {
	rc, err := c.ReadFile(vaultPath)
	if err != nil {
		return err
	}
	defer rc.Close()
	if _, err := io.Copy(os.Stdout, rc); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	return nil
}
