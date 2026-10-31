package mcp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/editorbus"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type editorTypeReq struct {
	Path    string `json:"path,omitempty" jsonschema:"vault-absolute markdown path, created empty if missing; omit for the note open in the user's editor"`
	Content string `json:"content" jsonschema:"markdown to type into the note"`
	At      string `json:"at,omitempty" jsonschema:"where to type: end (default) or cursor"`
	Heading string `json:"heading,omitempty" jsonschema:"type at the end of this section instead (case-insensitive heading text)"`
	Speed   string `json:"speed,omitempty" jsonschema:"slow, normal (default), fast or instant; long content types faster to finish in time"`
}

type editorTypeOut struct {
	Path        string `json:"path"`
	Chars       int    `json:"chars"`
	Interrupted bool   `json:"interrupted,omitempty"`
	Message     string `json:"message,omitempty"`
}

type editorReadReq struct {
	Path string `json:"path,omitempty" jsonschema:"vault-absolute path of an existing note; omit for the note open in the user's editor"`
}

type editorBlock struct {
	ID   string `json:"id" jsonschema:"block id, derived from the block's text: it stays valid until that block is edited"`
	Kind string `json:"kind" jsonschema:"heading, paragraph, list, code, quote, table, rule ..."`
	Text string `json:"text"`
}

type editorSelection struct {
	Text   string   `json:"text"`
	Blocks []string `json:"blocks" jsonschema:"ids of the blocks the selection touches"`
}

type editorReadOut struct {
	Path      string           `json:"path"`
	Blocks    []editorBlock    `json:"blocks"`
	Selection *editorSelection `json:"selection,omitempty" jsonschema:"text the user currently has selected in the editor, if any"`
}

type editorRewriteReq struct {
	Path     string `json:"path,omitempty" jsonschema:"vault-absolute path of an existing note; omit for the note open in the user's editor"`
	Content  string `json:"content" jsonschema:"the new markdown for the region; empty deletes it"`
	Target   string `json:"target,omitempty" jsonschema:"selection (what the user has selected), or a block id or an inclusive range of ids from editor_read such as b4k2x7 or b4k2x7-b9x0m3"`
	FromText string `json:"from_text,omitempty" jsonschema:"instead of target: exact text where the region starts; must occur once in the note"`
	ToText   string `json:"to_text,omitempty" jsonschema:"with from_text: exact text where the region ends; the region runs from from_text through it"`
	Speed    string `json:"speed,omitempty" jsonschema:"slow, normal (default), fast or instant; long content types faster to finish in time"`
}

type editorRewriteOut struct {
	Path        string `json:"path"`
	Chars       int    `json:"chars"`
	Interrupted bool   `json:"interrupted,omitempty"`
	Message     string `json:"message,omitempty"`
}

// errNoEditor and errUnwatched guarantee nothing was written, so writing the note another way is safe;
// the others don't, and a write racing an editor still typing would duplicate text.
var (
	errNoEditor    = errors.New("no editor is open in Hylo, so nothing was written. If the user asked to watch the writing, ask them to open the app and retry; otherwise write the note with the note tools")
	errEditorGone  = errors.New("the editor disconnected while working on the note; part of it may be written. Call editor_read once the user reopens it; do not write the note with other tools instead")
	errUnwatched   = errors.New("this turn started on its own, not with the user watching, so nothing was written in the editor. Write the note with the note tools")
	errEditorStuck = errors.New("the editor stopped responding and was told to stop; part of the text may be written. Call editor_read and continue from what is there; do not write the note with other tools instead")
)

// requireNote fails early for a missing note, so the editor never pops an error dialog for it.
// No path means the note open in the editor, which the page resolves.
func requireNote(c *client.Client, path string) error {
	if path == "" {
		return nil
	}
	if err := notes.CheckMarkdownPath(path); err != nil {
		return err
	}
	exists, err := c.Exists(path)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("note does not exist: " + path)
	}
	return nil
}

func dispatch(ctx context.Context, bus *editorbus.Bus, op editorbus.Op) (editorbus.Result, error) {
	op.Run = runFrom(ctx)
	res, err := bus.Dispatch(ctx, op)
	switch {
	case errors.Is(err, editorbus.ErrNoEditor):
		return res, errNoEditor
	case errors.Is(err, editorbus.ErrUnwatched):
		return res, errUnwatched
	case errors.Is(err, editorbus.ErrEditorGone):
		return res, errEditorGone
	case errors.Is(err, editorbus.ErrEditorStuck):
		return res, errEditorStuck
	case err != nil:
		return res, err
	case !res.OK && !res.Interrupted:
		if res.Error == "" {
			res.Error = "the editor failed without a reason"
		}
		return res, errors.New(res.Error)
	}
	return res, nil
}

func registerEditorTools(s *sdk.Server, c *client.Client, bus *editorbus.Bus) {
	addTool(s, "editor_type",
		"Type markdown into a note inside the user's open editor, character by character, as if they were typing. "+
			"Prefer it over create/append for writing the user is there to see. "+
			"Needs the Hylo app open; if none is, it fails without writing anything.", additive,
		func(ctx context.Context, in editorTypeReq) (editorTypeOut, error) {
			out := editorTypeOut{Path: in.Path}
			if in.Path != "" {
				if err := notes.CheckMarkdownPath(in.Path); err != nil {
					return out, err
				}
			}
			if err := notes.CheckText([]byte(in.Content)); err != nil {
				return out, err
			}
			if !bus.Connected() {
				return out, errNoEditor
			}
			created := false
			if in.Path != "" {
				if exists, err := c.Exists(in.Path); err == nil && !exists {
					if _, err := notes.Create(c, in.Path, false, func() ([]byte, error) { return nil, nil }); err != nil {
						return out, err
					}
					created = true
				}
			}

			res, err := dispatch(ctx, bus, editorbus.Op{
				Kind: editorbus.KindType, Path: in.Path, Content: in.Content, At: in.At, Heading: in.Heading, Speed: in.Speed,
			})
			if err != nil {
				// The editor closed after the check: keep "nothing was written" true.
				if created && errors.Is(err, errNoEditor) {
					_ = notes.Delete(c, in.Path)
				}
				return out, err
			}
			out.Path, out.Chars, out.Interrupted = cmp.Or(res.Path, in.Path), res.Chars, res.Interrupted
			if res.Interrupted {
				out.Message = "the user interrupted typing; do not retype the rest unless asked"
			}
			return out, nil
		})

	addTool(s, "editor_read",
		"Read a note as it is in the user's open editor, unsaved edits included, split into blocks with ids, "+
			"plus the text the user has selected. Without path it reads the note the user has open and returns its path. "+
			"Call it before editor_rewrite to see what the user sees.", readOnly,
		func(ctx context.Context, in editorReadReq) (editorReadOut, error) {
			out := editorReadOut{Path: in.Path}
			if err := requireNote(c, in.Path); err != nil {
				return out, err
			}
			res, err := dispatch(ctx, bus, editorbus.Op{Kind: editorbus.KindRead, Path: in.Path})
			if err != nil {
				return out, err
			}
			if res.Interrupted {
				return out, errors.New("the read was canceled")
			}
			if err := json.Unmarshal(res.Data, &out); err != nil {
				return out, err
			}
			out.Path = cmp.Or(res.Path, in.Path)
			return out, nil
		})

	addTool(s, "editor_rewrite",
		"Rewrite one region of a note in the user's open editor the way a person would: the region is highlighted, "+
			"deleted, and the new text is typed in its place. Use it when asked to redo or improve part of what is "+
			"already written. Point at the region with target (selection, or block ids from editor_read) or with "+
			"from_text/to_text. Empty content deletes the region. Block ids of the blocks it did not touch stay valid.", destructive,
		func(ctx context.Context, in editorRewriteReq) (editorRewriteOut, error) {
			out := editorRewriteOut{Path: in.Path}
			if err := requireNote(c, in.Path); err != nil {
				return out, err
			}
			if err := notes.CheckText([]byte(in.Content)); err != nil {
				return out, err
			}
			res, err := dispatch(ctx, bus, editorbus.Op{
				Kind: editorbus.KindRewrite, Path: in.Path, Content: in.Content, Speed: in.Speed,
				Target: in.Target, FromText: in.FromText, ToText: in.ToText,
			})
			if err != nil {
				return out, err
			}
			out.Path, out.Chars, out.Interrupted = cmp.Or(res.Path, in.Path), res.Chars, res.Interrupted
			if res.Interrupted {
				out.Message = "the user interrupted; the region may be partly rewritten. Call editor_read before doing anything else"
			}
			return out, nil
		})
}
