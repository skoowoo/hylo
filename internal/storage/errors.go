package storage

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is the base sentinel returned when a path does not exist in
	// the vault. Callers should prefer the more specific ErrMetadataNotFound
	// or ErrFileNotFound when the origin matters; all three satisfy
	// errors.Is(err, ErrNotFound).
	ErrNotFound = errors.New("not found")

	// ErrMetadataNotFound is returned when the metadata DB has no record for the
	// requested path (i.e. the note was never registered or has been removed from
	// the index). It wraps ErrNotFound for backward-compatible Is checks.
	ErrMetadataNotFound = fmt.Errorf("metadata record not found: %w", ErrNotFound)

	// ErrFileNotFound is returned when the note exists in the metadata DB but its
	// backing file is absent on disk (stale index row). It wraps ErrNotFound for
	// backward-compatible Is checks.
	ErrFileNotFound = fmt.Errorf("file not found on disk: %w", ErrNotFound)

	// ErrIsDir is returned when a file operation is attempted on a directory.
	ErrIsDir = errors.New("path is a directory")

	// ErrIsFile is returned when a directory operation is attempted on a file.
	ErrIsFile = errors.New("path is a file")

	// ErrInvalidPath is returned when a path escapes the vault root or is otherwise invalid.
	ErrInvalidPath = errors.New("invalid path")

	// ErrUnsupportedType is returned when a file path does not have a markdown extension.
	ErrUnsupportedType = errors.New("only markdown files (.md, .markdown) are supported")

	// ErrBinaryContent is returned when the content to be written is not valid UTF-8 text.
	ErrBinaryContent = errors.New("content must be valid UTF-8 text, not binary data")

	// ErrAlreadyExists is returned when an operation's destination path is
	// already occupied by another note (on disk or in the metadata DB) and
	// the operation would silently overwrite it. Used by MoveNote and
	// RenameNote — for RenameNote the check is vault-wide (any dir), not just
	// the note's own directory, since filenames are expected to be unique
	// across the whole vault.
	ErrAlreadyExists = errors.New("a note already exists at the destination")

	// ErrRenameNotAllowed is returned by RenameNote when the note cannot be
	// safely renamed: its kind is system-managed (short/knowledge/index —
	// each has a subsystem elsewhere that assumes a stable filename), or it
	// lives under a system directory (an underscore-prefixed path segment,
	// e.g. "/_shorts", "/_knowledge" — see dirHasUnderscoreSegment).
	ErrRenameNotAllowed = errors.New("this note cannot be renamed")

	// ErrEmptyContent is returned when a short note would be saved with no text.
	ErrEmptyContent = errors.New("content must not be empty")

	// ErrConflict is returned when a short note changed since the caller read it.
	ErrConflict = errors.New("the note changed since it was read; reload and try again")
)
