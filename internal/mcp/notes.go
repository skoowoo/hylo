package mcp

import (
	"context"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type noReq struct{}

type listReq struct {
	Path   string `json:"path,omitempty" jsonschema:"vault-absolute directory to scope the listing, e.g. /journal; omit for the whole vault"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max results, 0 = no limit"`
	Start  string `json:"start,omitempty" jsonschema:"only notes updated on or after this date (YYYY-MM-DD)"`
	End    string `json:"end,omitempty" jsonschema:"only notes updated before this date (YYYY-MM-DD)"`
	Latest int    `json:"latest,omitempty" jsonschema:"only notes updated within the last N days; cannot combine with start/end"`
	Kind   string `json:"kind,omitempty" jsonschema:"filter by kind: raw, short, knowledge, index"`
}

type readReq struct {
	Path string `json:"path" jsonschema:"vault-absolute path (e.g. /journal/today.md) or bare filename"`
}

type searchReq struct {
	Query string `json:"query" jsonschema:"words match either (OR); \"exact phrase\" in quotes matches a phrase"`
	Field string `json:"field,omitempty" jsonschema:"search field: name, content or tag; default all"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results, default 20"`
}

type createReq struct {
	Path    string `json:"path" jsonschema:"vault-absolute markdown path, e.g. /notes/idea.md"`
	Content string `json:"content" jsonschema:"note content"`
	Force   bool   `json:"force,omitempty" jsonschema:"overwrite if the note already exists"`
}

type writeReq struct {
	Path    string `json:"path" jsonschema:"vault-absolute markdown path"`
	Content string `json:"content" jsonschema:"text to insert"`
	Heading string `json:"heading,omitempty" jsonschema:"insert within this section (case-insensitive match) instead of the whole note"`
}

type pathReq struct {
	Path string `json:"path" jsonschema:"vault-absolute path starting with /"`
}

type moveReq struct {
	Path   string `json:"path" jsonschema:"vault-absolute path of the note"`
	NewDir string `json:"new_dir" jsonschema:"vault-absolute destination directory"`
}

type renameReq struct {
	Path    string `json:"path" jsonschema:"vault-absolute path of the note"`
	NewName string `json:"new_name" jsonschema:"new filename only, no path separators"`
}

type renameStatusReq struct {
	JobID int64 `json:"job_id" jsonschema:"job id returned by rename"`
}

type resolveReq struct {
	Name string `json:"name" jsonschema:"note filename, with or without .md"`
}

type pathOut struct {
	Path string `json:"path"`
}

type writeOut struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
}

type moveOut struct {
	Path    string `json:"path"`
	NewPath string `json:"new_path"`
}

type renameOut struct {
	Path    string `json:"path"`
	NewPath string `json:"new_path"`
	JobID   int64  `json:"job_id"`
}

type renameStatusOut struct {
	Status       string `json:"status"`
	Total        int    `json:"total"`
	Done         int    `json:"done"`
	UpdatedCount int    `json:"updated_count"`
	Error        string `json:"error,omitempty"`
}

type notesOut struct {
	Notes []notes.NoteEntry `json:"notes"`
}

type readOut struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type resolveOut struct {
	Name    string            `json:"name"`
	Matches []notes.NoteEntry `json:"matches"`
}

func registerNoteTools(s *sdk.Server, c *client.Client) {
	addTool(s, "status", "Show vault and search index counts.", true,
		func(_ context.Context, _ noReq) (*client.StatusResponse, error) {
			return c.Status()
		})

	addTool(s, "list", "List notes, most recently updated first.", true,
		func(_ context.Context, in listReq) (notesOut, error) {
			entries, err := notes.List(c, notes.ListQuery{
				Dir:   in.Path,
				Limit: in.Limit,
				Time:  notes.TimeFilter{Start: in.Start, End: in.End, Latest: in.Latest},
				Kind:  in.Kind,
			})
			return notesOut{Notes: entries}, err
		})

	addTool(s, "read", "Read a note's full content.", true,
		func(_ context.Context, in readReq) (readOut, error) {
			p, content, err := notes.Read(c, in.Path, false)
			return readOut{Path: p, Content: content}, err
		})

	addTool(s, "search", "Full-text search over notes by filename, content or tag.", true,
		func(_ context.Context, in searchReq) (notes.SearchResult, error) {
			return notes.Search(c, notes.SearchQuery{Query: in.Query, Field: in.Field, Limit: in.Limit})
		})

	addTool(s, "create", "Create a note. Fails if the path or a same-named note exists unless force is set.", false,
		func(_ context.Context, in createReq) (writeOut, error) {
			lines, err := notes.Create(c, in.Path, in.Force, func() ([]byte, error) { return []byte(in.Content), nil })
			return writeOut{Path: in.Path, Lines: lines}, err
		})

	addTool(s, "append", "Append content to the end of a note, or after a named section.", false,
		func(_ context.Context, in writeReq) (writeOut, error) {
			lines, err := notes.Append(c, in.Path, in.Heading, []byte(in.Content))
			return writeOut{Path: in.Path, Lines: lines}, err
		})

	addTool(s, "prepend", "Prepend content to the start of a note, or before a named section.", false,
		func(_ context.Context, in writeReq) (writeOut, error) {
			lines, err := notes.Prepend(c, in.Path, in.Heading, []byte(in.Content))
			return writeOut{Path: in.Path, Lines: lines}, err
		})

	addTool(s, "delete", "Permanently delete a note.", false,
		func(_ context.Context, in pathReq) (pathOut, error) {
			return pathOut{Path: in.Path}, notes.Delete(c, in.Path)
		})

	addTool(s, "move", "Move a note to another directory.", false,
		func(_ context.Context, in moveReq) (moveOut, error) {
			newPath, err := notes.Move(c, in.Path, in.NewDir)
			return moveOut{Path: in.Path, NewPath: newPath}, err
		})

	addTool(s, "rename", "Rename a note and update references vault-wide in the background; poll rename_status with the returned job_id.", false,
		func(_ context.Context, in renameReq) (renameOut, error) {
			newPath, jobID, err := notes.Rename(c, in.Path, in.NewName)
			return renameOut{Path: in.Path, NewPath: newPath, JobID: jobID}, err
		})

	addTool(s, "rename_status", "Check progress of a vault-wide rename reference sweep.", true,
		func(_ context.Context, in renameStatusReq) (renameStatusOut, error) {
			st, err := c.RenameStatus(in.JobID)
			return renameStatusOut{Status: st.Status, Total: st.Total, Done: st.Done, UpdatedCount: st.UpdatedCount, Error: st.Error}, err
		})

	addTool(s, "resolve", "Resolve a note filename to every vault path where it exists.", true,
		func(_ context.Context, in resolveReq) (resolveOut, error) {
			res, err := notes.Resolve(c, in.Name)
			return resolveOut{Name: res.Name, Matches: notes.Entries(res.Matches)}, err
		})
}
