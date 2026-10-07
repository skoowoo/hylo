package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/client"
	hylomcp "github.com/hardhacker/hylo/internal/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Commands that manage the local install rather than the vault; deliberately not MCP tools.
var notMCPTools = map[string]bool{
	"init": true, "start_server": true, "auth_show": true, "auth_rotate": true,
	"skills_add": true, "skills_list": true, "skills_remove": true,
}

// Flags that exist only on the CLI because of how it prints or reads input. Keyed by tool; "*" applies to all.
var cliOnlyFlags = map[string][]string{
	"*":               {"table"},
	"create":          {"file"},   // MCP never reads server-side files
	"extract_outline": {"indent"}, // MCP returns heading levels instead of indented text
	"resolve":         {"json"},   // MCP always returns full metadata
}

func mcpToolParams(t *testing.T) map[string]map[string]bool {
	t.Helper()
	ts := httptest.NewServer(hylomcp.NewHandler(client.NewInProcess(http.NotFoundHandler())))
	defer ts.Close()

	cs, err := sdk.NewClient(&sdk.Implementation{Name: "align", Version: "0"}, nil).
		Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	out := map[string]map[string]bool{}
	for _, tool := range res.Tools {
		b, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		_ = json.Unmarshal(b, &schema)
		out[tool.Name] = map[string]bool{}
		for p := range schema.Properties {
			out[tool.Name][p] = true
		}
	}
	return out
}

func toolName(c *cobra.Command) string {
	parts := strings.Fields(c.CommandPath())[1:] // drop "hylo"
	return strings.ReplaceAll(strings.Join(parts, "_"), "-", "_")
}

// cliParams returns the parameter names a CLI command takes: positional args from Use, plus flags.
func cliParams(c *cobra.Command, tool string) map[string]bool {
	params := map[string]bool{}
	norm := func(s string) string {
		s = strings.ReplaceAll(strings.Trim(s, "<>[]"), "-", "_")
		if s == "path_or_name" {
			return "path"
		}
		return s
	}
	for _, tok := range strings.Fields(c.Use)[1:] {
		params[norm(tok)] = true
	}

	skip := map[string]bool{"help": true}
	for _, f := range append(cliOnlyFlags["*"], cliOnlyFlags[tool]...) {
		skip[f] = true
	}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if !skip[f.Name] {
			params[strings.ReplaceAll(f.Name, "-", "_")] = true
		}
	})
	return params
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestMCPToolsMatchCLI(t *testing.T) {
	tools := mcpToolParams(t)
	seen := map[string]bool{}

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			if sub.HasSubCommands() {
				walk(sub)
				continue
			}
			name := toolName(sub)
			if notMCPTools[name] {
				continue
			}
			got, ok := tools[name]
			if !ok {
				t.Errorf("CLI command %q has no MCP tool %q", sub.CommandPath(), name)
				continue
			}
			seen[name] = true
			want := cliParams(sub, name)
			if strings.Join(keys(want), ",") != strings.Join(keys(got), ",") {
				t.Errorf("%s: CLI takes %v but MCP tool takes %v", sub.CommandPath(), keys(want), keys(got))
			}
		}
	}
	walk(rootCmd)

	for name := range tools {
		if !seen[name] {
			t.Errorf("MCP tool %q has no matching CLI command", name)
		}
	}
	for name := range notMCPTools {
		if _, ok := tools[name]; ok {
			t.Errorf("%q is allowlisted as CLI-only but exists as an MCP tool", name)
		}
	}
}
