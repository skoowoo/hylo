package cli

import (
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
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
	c, err := openClient()
	if err != nil {
		return err
	}
	if err := notes.Delete(c, path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "deleted %q\n", path)
	return nil
}
