package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <path>",
		Short: "Delete a note",
		Long: `Delete the note at <path>.

<path> is a vault-absolute path starting with "/" (e.g. /journal/today.md).`,
		Example: `  hylo delete /journal/today.md
  hylo delete /note.md`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(args[0])
		},
	}
	return cmd
}

func runDelete(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path %q must be absolute (start with \"/\")", path)
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	if err := c.DeleteNote(path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "deleted %q\n", path)
	return nil
}
