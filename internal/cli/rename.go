package cli

import (
	"fmt"
	"github.com/hardhacker/hylo/internal/notes"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <path> <new-name>",
		Short: "Rename a note's filename, keeping it in the same directory",
		Long: `Rename the note at <path> to <new-name>, without moving it (use "hylo move" for that).

<path> is vault-absolute, starting with "/" (e.g. /journal/today.md).
<new-name> is a bare filename with no path separators; ".md" is appended if omitted.

Renaming is only allowed for ordinary notes in regular folders — short/knowledge/index
notes and notes under an underscore-prefixed directory (e.g. "/_shorts") cannot be
renamed, since other subsystems assume their filenames stay stable.

Fixing up [[wikilinks]] elsewhere in the vault that point to the old name happens
asynchronously after this command returns; it prints the rename job id so you can
check on it with "hylo rename-status <job-id>".`,
		Example:      `  hylo rename /journal/draft.md final.md`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRename(args[0], args[1])
		},
	}
	return cmd
}

func runRename(path, newName string) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	newPath, jobID, err := notes.Rename(c, path, newName)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "renamed %q to %q (rename job %d)\n", path, newPath, jobID)
	return nil
}

func newRenameStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "rename-status <job-id>",
		Short:        "Check the progress of a vault-wide rename reference sweep",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRenameStatus(args[0])
		},
	}
	return cmd
}

func runRenameStatus(jobIDStr string) error {
	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("job-id %q must be a number", jobIDStr)
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	st, err := c.RenameStatus(jobID)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "status: %s (%d/%d notes scanned, %d updated)\n", st.Status, st.Done, st.Total, st.UpdatedCount)
	if st.Error != "" {
		fmt.Fprintf(os.Stdout, "error: %s\n", st.Error)
	}
	return nil
}
