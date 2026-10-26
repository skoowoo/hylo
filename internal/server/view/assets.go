package view

import (
	"embed"
	"strings"

	"github.com/hardhacker/hylo/internal/server/static"
)

//go:embed assets
var assetFS embed.FS

func readAsset(name string) string {
	b, err := assetFS.ReadFile("assets/" + name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func readAssets(names ...string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString(readAsset(n))
	}
	return b.String()
}

// Bundle order is cascade order for CSS; JS files only declare globals until Alpine starts.
func init() {
	static.Bundle("bundle/home.css", appTokensCSS+readAssets(
		"info_dialog.css", "base.css", "cselect.css", "home.css", "images.css", "graph.css", "agent_chat.css",
		"content_pane.css", "note_frontmatter.css", "note_shared_prose.css", "shorts.css", "search_overlay.css",
		"confirm_dialog.css",
		"settings_modal.css", "settings_appearance.css", "settings_server.css", "settings_chatlink.css",
		"settings_agentcli.css", "settings_notifications.css", "settings_shortcuts.css", "settings_agentbots.css",
		"settings_skills.css",
		"servers.css",
	))

	// The editor modules are imported lazily by URL, so the bundle carries their versioned URLs.
	modules := `window.__hyloModules = {"editor": "` + static.URL("editor.js") + `", "fenceCard": "` + static.URL("fence-card.js") + `"};` + "\n"
	static.Bundle("bundle/home.js", modules+readAssets(
		"home_init.js", "live.js", "note_changes.js", "keys.js", "path_ac.js",
		"chat_reduce.js", "chat_text.js", "chat_kernel.js", "chat_view.js",
		"list_tighten.js", "tab_fit.js", "graph_layout.js", "editor_session.js", "editor_agent.js", "content_pane.js",
		"search_overlay.js", "confirm_dialog.js", "info_dialog.js", "servers_store.js",
		"settings_modal.js", "settings_appearance.js", "settings_server.js", "settings_agentcli.js",
		"settings_notifications.js", "settings_shortcuts.js", "settings_agentbots.js", "settings_skills.js",
		"section_cache.js", "home_images.js", "home_graph.js", "home_inbox.js", "home_shorts.js", "tag_cloud.js",
		"home_drag.js", "home.js", "note_shared.js",
	))

	static.Bundle("bundle/login.css", appTokensCSS+readAssets("base.css", "login.css"))
}
