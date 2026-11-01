package notes

import (
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/storage"
)

// NoteLookup is what resolving a note argument needs: *client.Client over
// HTTP, or the server's vault directly, so a bot run's note activity resolves
// a tool's argument exactly as the tool did.
type NoteLookup interface {
	// StatNote takes a vault path; a base name without extension gets ".md".
	StatNote(p string) (storage.Note, error)
	// NotesByName lists the notes with that filename, most recently updated first.
	NotesByName(name string) ([]storage.Note, error)
}

// ResolvePath maps a vault path or a bare filename to exactly one vault path.
// knowledgeOnly rejects notes that are not knowledge notes.
func ResolvePath(l NoteLookup, arg string, knowledgeOnly bool) (string, error) {
	if strings.Contains(arg, "/") {
		note, err := l.StatNote(absPath(arg))
		if err != nil {
			return "", err
		}
		if knowledgeOnly && note.Kind != storage.KindKnowledge {
			return "", fmt.Errorf("%q is not a knowledge note", note.PathString())
		}
		return note.PathString(), nil
	}

	name := EnsureMarkdownExt(arg)
	all, err := l.NotesByName(name)
	if err != nil {
		return "", err
	}
	matches := all
	if knowledgeOnly {
		matches = nil
		for _, n := range all {
			if n.Kind == storage.KindKnowledge {
				matches = append(matches, n)
			}
		}
	}
	switch len(matches) {
	case 0:
		if knowledgeOnly && len(all) > 0 {
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

// ResolveLatest maps a vault path or a bare filename to the note a read of it
// gets: the path itself, or the most recently updated note of that name.
func ResolveLatest(l NoteLookup, arg string) (string, error) {
	if strings.Contains(arg, "/") {
		p := path.Clean(absPath(arg))
		return path.Join(path.Dir(p), EnsureMarkdownExt(path.Base(p))), nil
	}
	name := EnsureMarkdownExt(arg)
	matches, err := l.NotesByName(name)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no note found: %q", name)
	}
	return matches[0].PathString(), nil
}

func absPath(p string) string {
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
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
