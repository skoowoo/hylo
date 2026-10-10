// Package mcp exposes the hylo CLI commands as MCP tools over streamable HTTP.
package mcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/hardhacker/hylo/internal/build"
	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/editorbus"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `Tools mirror the hylo CLI. Paths are vault-absolute and start with "/" (e.g. /journal/today.md). ` +
	`Read-style tools also accept a bare filename; if several notes share it, use the full path.`

// Option customizes NewHandler.
type Option func(*options)

type options struct{ editor *editorbus.Bus }

// WithEditorBus registers the editor tools, which drive the user's live editor over bus.
func WithEditorBus(b *editorbus.Bus) Option { return func(o *options) { o.editor = b } }

// NewHandler returns the MCP endpoint handler. c talks to the vault in-process.
func NewHandler(c *client.Client, opts ...Option) http.Handler {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	srv := sdk.NewServer(&sdk.Implementation{
		Name:    "hylo",
		Version: build.Get().Version,
	}, &sdk.ServerOptions{Instructions: instructions})

	registerNoteTools(srv, c)
	registerKnowledgeTools(srv, c)
	registerExtractTools(srv, c)
	registerTagTools(srv, c)
	registerShortTools(srv, c)
	if o.editor != nil {
		registerEditorTools(srv, c, o.editor)
	}

	// Stateless is required to serve the 2026-07-28 protocol version.
	h := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, &sdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
	// The spec requires rejecting foreign Origins (DNS rebinding); non-browser clients send none.
	return http.NewCrossOriginProtection().Handler(withRunHeader(h))
}

// RunParam is the endpoint query parameter naming the agent run a client belongs to.
const RunParam = "run"

const runHeader = "X-Hylo-Run"

// withRunHeader exposes ?run= to tools, which only see request headers. Agents with a
// shared config file send the header themselves, filled from an env var that a run sets;
// outside a run it arrives empty or unexpanded, which means unknown.
func withRunHeader(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if run := r.URL.Query().Get(RunParam); run != "" {
			r.Header.Set(runHeader, run)
		} else if v := r.Header.Get(runHeader); strings.Contains(v, "${") {
			r.Header.Del(runHeader)
		}
		h.ServeHTTP(w, r)
	})
}

type runKey struct{}

// runFrom returns the agent run that issued the current tool call, or "".
func runFrom(ctx context.Context) string {
	run, _ := ctx.Value(runKey{}).(string)
	return run
}

// addTool registers a tool with a typed result; the SDK derives the output schema and
// returns it as structured content plus its JSON text.
func addTool[In, Out any](s *sdk.Server, name, desc string, readOnly bool, h func(ctx context.Context, in In) (Out, error)) {
	t := &sdk.Tool{Name: name, Description: desc}
	if readOnly {
		t.Annotations = &sdk.ToolAnnotations{ReadOnlyHint: true}
	}
	sdk.AddTool(s, t, func(ctx context.Context, req *sdk.CallToolRequest, in In) (*sdk.CallToolResult, Out, error) {
		if req != nil && req.Extra != nil && req.Extra.Header != nil {
			ctx = context.WithValue(ctx, runKey{}, req.Extra.Header.Get(runHeader))
		}
		out, err := h(ctx, in)
		return nil, out, err
	})
}
