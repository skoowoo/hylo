package mcp

import (
	"context"
	"encoding/json"

	"github.com/hardhacker/hylo/internal/apps"
	"github.com/hardhacker/hylo/internal/client"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type appsListOut struct {
	Apps []apps.Status `json:"apps"`
}

type appsToolsReq struct {
	App string `json:"app" jsonschema:"app id, from apps_list"`
}

type appsToolsOut struct {
	Tools []apps.Tool `json:"tools"`
}

type appsCallReq struct {
	App  string         `json:"app" jsonschema:"app id, from apps_list"`
	Tool string         `json:"tool" jsonschema:"tool name, from apps_tools"`
	Args map[string]any `json:"args,omitempty" jsonschema:"the tool's arguments, matching its input schema from apps_tools"`
}

// registerAppsTools reach the user's connected apps (internal/apps). An app's
// own tools stay behind apps_tools and apps_call instead of joining this
// server's list, so each run doesn't carry every platform's tool definitions.
func registerAppsTools(s *sdk.Server, c *client.Client) {
	addTool(s, "apps_list", "List the user's apps on outside platforms (podcasts, reading) and whether each is connected.", readOnly,
		func(_ context.Context, _ noReq) (appsListOut, error) {
			list, err := c.Apps()
			return appsListOut{Apps: list}, err
		})

	addTool(s, "apps_tools", "List a connected app's tools with their input schemas. You can use the apps the user turned on for you.", readOnly,
		func(ctx context.Context, in appsToolsReq) (appsToolsOut, error) {
			tools, err := c.AppTools(runFrom(ctx), in.App)
			if tools == nil {
				tools = []apps.Tool{}
			}
			return appsToolsOut{Tools: tools}, err
		})

	addTool(s, "apps_call", "Run a tool of a connected app on the user's own account there. Some tools spend the user's credits there; use those only when the task needs them.", additive,
		func(ctx context.Context, in appsCallReq) (*apps.CallResult, error) {
			var args json.RawMessage
			if in.Args != nil {
				b, err := json.Marshal(in.Args)
				if err != nil {
					return nil, err
				}
				args = b
			}
			return c.AppCall(runFrom(ctx), in.App, in.Tool, args)
		})
}
