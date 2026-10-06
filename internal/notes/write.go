package notes

import (
	"fmt"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// Create writes a new note. readContent runs only after the path checks pass, so a CLI
// reading stdin does not block on a path that would be rejected anyway.
func Create(c *client.Client, p string, force bool, readContent func() ([]byte, error)) (lines int, err error) {
	if err := CheckMarkdownPath(p); err != nil {
		return 0, err
	}
	if !force {
		if err := checkNoConflict(c, p); err != nil {
			return 0, err
		}
	}
	data, err := readContent()
	if err != nil {
		return 0, err
	}
	if err := CheckText(data); err != nil {
		return 0, err
	}
	if err := c.WriteFile(p, data); err != nil {
		return 0, fmt.Errorf("create %q: %w", p, err)
	}
	return util.CountLines(data), nil
}

func checkNoConflict(c *client.Client, notePath string) error {
	exists, err := c.Exists(notePath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%q already exists (use force to overwrite)", notePath)
	}
	p, ok := storage.ParsePath(notePath)
	if !ok {
		return fmt.Errorf("path %q must be absolute (start with \"/\")", notePath)
	}
	res, err := c.ResolveNoteName(p.Base())
	if err != nil {
		return err
	}
	if res.Count > 0 {
		return fmt.Errorf("a note named %q already exists in the vault (use force to create anyway)", p.Base())
	}
	return nil
}

// Append adds data to the end of a note, or after heading when given.
func Append(c *client.Client, p, heading string, data []byte) (int, error) {
	if err := CheckMarkdownPath(p); err != nil {
		return 0, err
	}
	if err := CheckText(data); err != nil {
		return 0, err
	}
	if err := c.AppendFile(p, data, heading); err != nil {
		return 0, fmt.Errorf("append to %q: %w", p, err)
	}
	return util.CountLines(data), nil
}

// Prepend adds data to the start of a note, or before heading when given.
func Prepend(c *client.Client, p, heading string, data []byte) (int, error) {
	if err := CheckMarkdownPath(p); err != nil {
		return 0, err
	}
	if err := CheckText(data); err != nil {
		return 0, err
	}
	if err := c.PrependFile(p, data, heading); err != nil {
		return 0, fmt.Errorf("prepend to %q: %w", p, err)
	}
	return util.CountLines(data), nil
}

func Delete(c *client.Client, p string) error {
	if err := CheckAbs("path", p); err != nil {
		return err
	}
	return c.DeleteNote(p)
}

// DeleteKnowledge deletes p only if it is a knowledge note.
func DeleteKnowledge(c *client.Client, p string) error {
	if err := CheckAbs("path", p); err != nil {
		return err
	}
	note, err := c.StatNote(p)
	if err != nil {
		return err
	}
	if note.Kind != storage.KindKnowledge {
		return fmt.Errorf("%q is not a knowledge note", note.PathString())
	}
	return c.DeleteNote(p)
}

func Move(c *client.Client, p, newDir string) (string, error) {
	if err := CheckAbs("path", p); err != nil {
		return "", err
	}
	if err := CheckAbs("new-dir", newDir); err != nil {
		return "", err
	}
	return c.MoveNote(p, newDir)
}

// Rename returns the new path and the id of the background reference-update job.
func Rename(c *client.Client, p, newName string) (string, int64, error) {
	if err := CheckAbs("path", p); err != nil {
		return "", 0, err
	}
	if strings.ContainsAny(newName, "/\\") {
		return "", 0, fmt.Errorf("new-name %q must be a filename only (no path separators)", newName)
	}
	return c.RenameNote(p, newName)
}
