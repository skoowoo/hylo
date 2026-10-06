package notes

import (
	"fmt"
	"io"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/storage"
)

// ResolvePath maps a vault path or a bare filename to exactly one vault path.
// knowledgeOnly rejects notes that are not knowledge notes.
func ResolvePath(c *client.Client, arg string, knowledgeOnly bool) (string, error) {
	if strings.Contains(arg, "/") {
		p := arg
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		note, err := c.StatNote(p)
		if err != nil {
			return "", err
		}
		if knowledgeOnly && note.Kind != storage.KindKnowledge {
			return "", fmt.Errorf("%q is not a knowledge note", note.PathString())
		}
		return note.PathString(), nil
	}

	name := EnsureMarkdownExt(arg)
	res, err := c.ResolveNoteName(name)
	if err != nil {
		return "", err
	}
	matches := res.Matches
	if knowledgeOnly {
		matches = nil
		for _, n := range res.Matches {
			if n.Kind == storage.KindKnowledge {
				matches = append(matches, n)
			}
		}
	}
	switch len(matches) {
	case 0:
		if knowledgeOnly && len(res.Matches) > 0 {
			return "", fmt.Errorf("no knowledge note named %q", name)
		}
		return "", fmt.Errorf("no note found: %q", name)
	case 1:
		return matches[0].PathString(), nil
	default:
		paths := make([]string, len(matches))
		for i, n := range matches {
			paths[i] = n.PathString()
		}
		return "", fmt.Errorf("multiple notes named %q; use a full path: %s", name, strings.Join(paths, ", "))
	}
}

// Resolve lists every vault location of a note filename; it is an error if there is none.
func Resolve(c *client.Client, name string) (client.NoteResolveResponse, error) {
	name = EnsureMarkdownExt(name)
	res, err := c.ResolveNoteName(name)
	if err != nil {
		return res, err
	}
	if res.Count == 0 {
		return res, fmt.Errorf("no note found with name %q", name)
	}
	return res, nil
}

// Read returns the resolved path and full content of a note.
func Read(c *client.Client, arg string, knowledgeOnly bool) (path, content string, err error) {
	if path, err = ResolvePath(c, arg, knowledgeOnly); err != nil {
		return "", "", err
	}
	rc, err := c.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	return path, string(b), err
}
