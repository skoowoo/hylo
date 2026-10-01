package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/storage"
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
			return runRead(args[0])
		},
	}
	return cmd
}

func runRead(arg string) error {
	c, err := openClient()
	if err != nil {
		return err
	}

	if strings.Contains(arg, "/") {
		p := arg
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		note, err := c.StatNote(p)
		if err != nil {
			return err
		}
		return streamReadFile(c, note.PathString())
	}

	name := ensureMarkdownExt(strings.TrimSpace(arg))
	resp, err := c.ResolveNoteName(name)
	if err != nil {
		return err
	}
	switch len(resp.Matches) {
	case 0:
		return fmt.Errorf("no note found: %q", name)
	case 1:
		return streamReadFile(c, resp.Matches[0].PathString())
	default:
		paths := make([]string, len(resp.Matches))
		for i, n := range resp.Matches {
			paths[i] = n.PathString()
		}
		return fmt.Errorf("multiple notes named %q; use a full path: %s", name, strings.Join(paths, ", "))
	}
}

// runScopedRead reads a note filtered by knowledge/raw type; used by "hylo knowledge read".
func runScopedRead(arg string, wantKnowledge bool) error {
	c, err := openClient()
	if err != nil {
		return err
	}
	if strings.Contains(arg, "/") {
		p := arg
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		note, err := c.StatNote(p)
		if err != nil {
			return err
		}
		isKnowledge := note.Kind == storage.KindKnowledge
		if wantKnowledge && !isKnowledge {
			return fmt.Errorf("%q is not a knowledge note", note.PathString())
		}
		if !wantKnowledge && isKnowledge {
			return fmt.Errorf("%q is a knowledge note; use \"hylo knowledge read\" instead", note.PathString())
		}
		return streamReadFile(c, note.PathString())
	}

	name := ensureMarkdownExt(strings.TrimSpace(arg))
	resp, err := c.ResolveNoteName(name)
	if err != nil {
		return err
	}
	var picks []storage.Note
	for _, n := range resp.Matches {
		if (n.Kind == storage.KindKnowledge) == wantKnowledge {
			picks = append(picks, n)
		}
	}
	switch len(picks) {
	case 0:
		if len(resp.Matches) == 0 {
			return fmt.Errorf("no note found: %q", name)
		}
		if wantKnowledge {
			return fmt.Errorf("no knowledge note named %q", name)
		}
		return fmt.Errorf("no raw note named %q", name)
	case 1:
		return streamReadFile(c, picks[0].PathString())
	default:
		paths := make([]string, len(picks))
		for i, n := range picks {
			paths[i] = n.PathString()
		}
		kind := "raw"
		if wantKnowledge {
			kind = "knowledge"
		}
		return fmt.Errorf("multiple %s notes named %q; use a full path: %s", kind, name, strings.Join(paths, ", "))
	}
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

func ensureMarkdownExt(name string) string {
	if path.Ext(name) != "" {
		return name
	}
	return name + ".md"
}
