package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/editorbus"
	"github.com/hardhacker/hylo/internal/notes"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type editorTypeReq struct {
	Path    string `json:"path" jsonschema:"vault-absolute markdown path; created empty if missing"`
	Content string `json:"content" jsonschema:"markdown to type into the note"`
	At      string `json:"at,omitempty" jsonschema:"where to type: end (default) or cursor"`
	Heading string `json:"heading,omitempty" jsonschema:"type at the end of this section instead (case-insensitive heading text)"`
	Speed   string `json:"speed,omitempty" jsonschema:"slow, normal (default), fast or instant"`
}

type editorTypeOut struct {
	Path        string `json:"path"`
	Chars       int    `json:"chars"`
	Interrupted bool   `json:"interrupted,omitempty"`
	Message     string `json:"message,omitempty"`
}

type editorReadReq struct {
	Path string `json:"path" jsonschema:"vault-absolute path of an existing note"`
}

type editorBlock struct {
	ID   string `json:"id" jsonschema:"block id such as b3, valid until the note changes"`
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
	Path     string `json:"path" jsonschema:"vault-absolute path of an existing note"`
	Content  string `json:"content" jsonschema:"the new markdown for the region; empty deletes it"`
	Target   string `json:"target,omitempty" jsonschema:"selection (what the user has selected), or a block id or range from editor_read such as b3 or b3-b5"`
	Expect   string `json:"expect,omitempty" jsonschema:"required with a block target: the first words of the region, copied from editor_read, to detect a changed note"`
	FromText string `json:"from_text,omitempty" jsonschema:"instead of target: exact text where the region starts; must occur once in the note"`
	ToText   string `json:"to_text,omitempty" jsonschema:"with from_text: exact text where the region ends; the region runs from from_text through it"`
	Speed    string `json:"speed,omitempty" jsonschema:"slow, normal (default), fast or instant"`
}

type editorRewriteOut struct {
	Path        string `json:"path"`
	Chars       int    `json:"chars"`
	Interrupted bool   `json:"interrupted,omitempty"`
	Message     string `json:"message,omitempty"`
}

// The editor tools never fall back to writing the file: the user asked to watch the
// edit happen, and a write racing an editor that is still typing would duplicate text.
var (
	errNoEditor   = errors.New("no editor is open in Hylo; ask the user to open the app, then retry. Do not write the note with other tools instead")
	errEditorGone = errors.New("the editor disconnected while working on the note; part of it may be written. Call editor_read once the user reopens it; do not write the note with other tools instead")
)

// requireNote fails early for a missing note, so the editor never pops an error dialog for it.
func requireNote(c *client.Client, path string) error {
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
	case errors.Is(err, editorbus.ErrEditorGone):
		return res, errEditorGone
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
			"Use it instead of append when the user asks to write in the editor or wants to watch the writing happen. "+
			"Needs the Hylo app open; when it fails, report that to the user instead of writing the note another way.", additive,
		func(ctx context.Context, in editorTypeReq) (editorTypeOut, error) {
			out := editorTypeOut{Path: in.Path}
			if err := notes.CheckMarkdownPath(in.Path); err != nil {
				return out, err
			}
			if err := notes.CheckText([]byte(in.Content)); err != nil {
				return out, err
			}
			if !bus.Connected() {
				return out, errNoEditor
			}
			if exists, err := c.Exists(in.Path); err == nil && !exists {
				if _, err := notes.Create(c, in.Path, false, func() ([]byte, error) { return nil, nil }); err != nil {
					return out, err
				}
			}

			res, err := dispatch(ctx, bus, editorbus.Op{
				Kind: editorbus.KindType, Path: in.Path, Content: in.Content, At: in.At, Heading: in.Heading, Speed: in.Speed,
			})
			if err != nil {
				return out, err
			}
			out.Chars, out.Interrupted = res.Chars, res.Interrupted
			if res.Interrupted {
				out.Message = "the user interrupted typing; do not retype the rest unless asked"
			}
			return out, nil
		})

	addTool(s, "editor_read",
		"Read a note as it is in the user's open editor, unsaved edits included, split into numbered blocks, "+
			"plus the text the user has selected. Call it before editor_rewrite to see what the user sees.", readOnly,
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
			out.Path = in.Path
			return out, nil
		})

	addTool(s, "editor_rewrite",
		"Rewrite one region of a note in the user's open editor the way a person would: the region is highlighted, "+
			"deleted, and the new text is typed in its place. Use it when asked to redo or improve part of what is "+
			"already written. Point at the region with target (selection, or block ids from editor_read) or with "+
			"from_text/to_text. Empty content deletes the region.", destructive,
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
				Target: in.Target, Expect: in.Expect, FromText: in.FromText, ToText: in.ToText,
			})
			if err != nil {
				return out, err
			}
			out.Chars, out.Interrupted = res.Chars, res.Interrupted
			if res.Interrupted {
				out.Message = "the user interrupted; the region may be partly rewritten. Call editor_read before doing anything else"
			}
			return out, nil
		})
}
