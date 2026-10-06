// Package mcp exposes the hylo CLI commands as MCP tools over streamable HTTP.
package mcp

import (
	"context"
	"net/http"

	"github.com/hardhacker/hylo/internal/build"
	"github.com/hardhacker/hylo/internal/client"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `Tools mirror the hylo CLI. Paths are vault-absolute and start with "/" (e.g. /journal/today.md). ` +
	`Read-style tools also accept a bare filename; if several notes share it, use the full path.`

// NewHandler returns the MCP endpoint handler. c talks to the vault in-process.
func NewHandler(c *client.Client) http.Handler {
	srv := sdk.NewServer(&sdk.Implementation{
		Name:    "hylo",
		Version: build.Get().Version,
	}, &sdk.ServerOptions{Instructions: instructions})

	registerNoteTools(srv, c)
	registerKnowledgeTools(srv, c)
	registerExtractTools(srv, c)
	registerTagTools(srv, c)
	registerShortTools(srv, c)

	// Stateless is required to serve the 2026-07-28 protocol version.
	h := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, &sdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
	// The spec requires rejecting foreign Origins (DNS rebinding); non-browser clients send none.
	return http.NewCrossOriginProtection().Handler(h)
}

// addTool registers a tool with a typed result; the SDK derives the output schema and
// returns it as structured content plus its JSON text.
func addTool[In, Out any](s *sdk.Server, name, desc string, readOnly bool, h func(ctx context.Context, in In) (Out, error)) {
	t := &sdk.Tool{Name: name, Description: desc}
	if readOnly {
		t.Annotations = &sdk.ToolAnnotations{ReadOnlyHint: true}
	}
	sdk.AddTool(s, t, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, Out, error) {
		out, err := h(ctx, in)
		return nil, out, err
	})
}
