package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var (
		content  string
		fromFile string
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "create <path>",
		Short: "Create a note in the vault",
		Long: `Create a note at <path>.

<path> is a vault-absolute path starting with "/" (e.g. /journal/today.md).
Content is read from --file, --content, or stdin (checked in that order).
Use --force to overwrite an existing note.`,
		Example: `  hylo create /journal/today.md --content "# Today"
  hylo create /journal/recipe.md --file recipe.md
  hylo create /note.md < draft.md
  echo "# Idea" | hylo create /idea.md`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(args[0], content, fromFile, force)
		},
	}

	cmd.Flags().StringVarP(&content, "content", "c", "", "inline content for the note")
	cmd.Flags().StringVarP(&fromFile, "file", "f", "", "local file whose content is copied into the vault")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite if the note already exists")

	return cmd
}

func runCreate(notePath, content, fromFile string, force bool) error {
	if !strings.HasPrefix(notePath, "/") {
		return fmt.Errorf("path %q must be absolute (start with \"/\")", notePath)
	}
	if !util.IsMarkdownPath(notePath) {
		return fmt.Errorf("path %q is not a markdown file (.md or .markdown)", notePath)
	}

	c, err := openClient()
	if err != nil {
		return err
	}

	if !force {
		exists, err := c.Exists(notePath)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%q already exists (use --force to overwrite)", notePath)
		}

		p, ok := storage.ParsePath(notePath)
		if !ok {
			return fmt.Errorf("path %q must be absolute (start with \"/\")", notePath)
		}
		resolved, err := c.ResolveNoteName(p.Base())
		if err != nil {
			return err
		}
		if resolved.Count > 0 {
			return fmt.Errorf("a note named %q already exists in the vault (use --force to create anyway)", p.Base())
		}
	}

	data, err := resolveContent(content, fromFile)
	if err != nil {
		return err
	}

	if !util.IsValidText(data) {
		return fmt.Errorf("source file appears to be binary — only text content can be stored as markdown")
	}

	if err := c.WriteFile(notePath, data); err != nil {
		return fmt.Errorf("create %q: %w", notePath, err)
	}

	fmt.Fprintf(os.Stdout, "created %q (%d lines)\n", notePath, countLines(data))
	return nil
}

// resolveContent returns the bytes to write, choosing between fromFile,
// inline content, or stdin — in that priority order.
//
// When neither --file nor --content is given, stdin is always read.
// If stdin is an interactive terminal a hint is printed to stderr so the
// user knows to type their content and press Ctrl+D (EOF) when done.
func resolveContent(content, fromFile string) ([]byte, error) {
	if fromFile != "" {
		data, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, fmt.Errorf("read source file: %w", err)
		}
		return data, nil
	}

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

// isTerminal reports whether f is connected to an interactive terminal
// (as opposed to a pipe or file redirect).
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func countLines(data []byte) int { return util.CountLines(data) }
