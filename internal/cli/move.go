package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <path> <new-dir>",
		Short: "Move a note to another directory",
		Long: `Move the note at <path> into <new-dir>, keeping its filename.

<path> and <new-dir> are vault-absolute, starting with "/" (e.g. /journal/today.md, /archive).`,
		Example: `  hylo move /journal/today.md /archive
  hylo move /note.md /`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMove(args[0], args[1])
		},
	}
	return cmd
}

func runMove(path, newDir string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path %q must be absolute (start with \"/\")", path)
	}
	if !strings.HasPrefix(newDir, "/") {
		return fmt.Errorf("new-dir %q must be absolute (start with \"/\")", newDir)
	}
	c, err := openClient()
	if err != nil {
		return err
	}
	newPath, err := c.MoveNote(path, newDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "moved %q to %q\n", path, newPath)
	return nil
}
