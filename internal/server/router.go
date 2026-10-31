package server

import (
	"log/slog"
	"net/http"

	"github.com/hardhacker/hylo/internal/channels"
	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/editorbus"
	"github.com/hardhacker/hylo/internal/live"
	"github.com/hardhacker/hylo/internal/mate"
	hylomcp "github.com/hardhacker/hylo/internal/mcp"
	"github.com/hardhacker/hylo/internal/plugins/compile"
	"github.com/hardhacker/hylo/internal/plugins/gitsync"
	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/server/handler"
	"github.com/hardhacker/hylo/internal/server/middleware"
	"github.com/hardhacker/hylo/internal/server/static"
	"github.com/hardhacker/hylo/internal/server/view"
	"github.com/hardhacker/hylo/internal/skills"
	"github.com/hardhacker/hylo/internal/storage"
)

// newRouter builds and returns the application HTTP router.
func newRouter(
	logger *slog.Logger,
	cfg *config.Config,
	vault *storage.Vault,
	cfgFileLoaded string,
	gitPlugin *gitsync.Plugin,
	searchPlugin *search.Plugin,
	compilePlugin *compile.Plugin,
	mateStore *mate.Store,
	mateRunner *mate.Runner,
	skillsMgr *skills.Manager,
	feed *live.Feed,
	apiKey string,
	channelHub *channels.Hub,
	proxy config.Proxy,
	ah *handler.AgentAPI,
	restart func()) http.Handler {
	mux := http.NewServeMux()

	h := handler.New(logger, cfg)
	cfgHTTP := handler.NewConfigHTTP(logger, cfg, cfgFileLoaded)
	gh := handler.NewVault(vault)
	sh := search.NewHandler(searchPlugin)
	vh := view.NewView(vault, searchPlugin, cfg)

	editorBus := editorbus.New()
	ah.SetEditorBus(editorBus)
	if channelHub != nil {
		channelHub.SetChatter(ah)
	}

	// Triggers wake bots in their chats. A wake that ends with something for
	// the user goes out on the live feed, to pages and the desktop notifier.
	if mateRunner != nil {
		ah.SetMateRunner(mateRunner)
		mateRunner.SetWakeFunc(ah.Wake)
	}
	ah.SetWakeEnd(func(e handler.WakeEnd) {
		feed.Publish(live.Change{Topic: live.TopicChat, Op: "wake", Data: e})
	})

	mux.Handle("GET /api/live", feed)
	mux.HandleFunc("GET /api/agents", ah.AgentsGET)
	mux.HandleFunc("POST /api/chat", ah.ChatPOST)
	mux.HandleFunc("POST /api/chat/uploads", ah.ChatUploadPOST)
	mux.HandleFunc("GET /api/chat/uploads/{id}", ah.ChatUploadGET)
	mux.HandleFunc("POST /api/runs", ah.RunsPOST)
	mux.HandleFunc("GET /api/runs/active", ah.RunsActiveGET)
	mux.HandleFunc("GET /api/runs/by-ref", ah.RunByRefGET)
	mux.HandleFunc("GET /api/runs/{id}/events", ah.RunEventsGET)
	mux.HandleFunc("GET /api/runs/{id}", ah.RunGET)
	mux.HandleFunc("POST /api/runs/{id}/cancel", ah.RunCancelPOST)
	mux.HandleFunc("POST /api/setup/hello", ah.SetupHelloPOST)
	mux.HandleFunc("POST /api/setup/bot", ah.SetupBotPOST)

	if mateStore != nil {
		ch := handler.NewConversationAPI(mateStore)
		mux.HandleFunc("GET /api/conversations", ch.ConversationsGET)
		mux.HandleFunc("POST /api/conversations", ch.ConversationsPOST)
		mux.HandleFunc("GET /api/conversations/{id}", ch.ConversationGET)
		mux.HandleFunc("DELETE /api/conversations/{id}", ch.ConversationDELETE)
		mux.HandleFunc("PATCH /api/conversations/{id}/title", ch.ConversationTitlePATCH)

		vh.SetMates(mateStore)
		mh := handler.NewMateAPI(logger, mateStore, skillsMgr)
		mux.HandleFunc("GET /api/mate-events", mh.MateEventsGET)

		mux.HandleFunc("GET /api/mates", mh.MatesGET)
		mux.HandleFunc("POST /api/mates/reorder", mh.MatesReorderPOST)
		mux.HandleFunc("POST /api/mates", mh.MatesPOST)
		mux.HandleFunc("GET /api/mates/{id}", mh.MateGET)
		mux.HandleFunc("PUT /api/mates/{id}", mh.MatePUT)
		mux.HandleFunc("DELETE /api/mates/{id}", mh.MateDELETE)
		mux.HandleFunc("POST /api/mates/{id}/seen", mh.MateSeenPOST)
		mux.HandleFunc("POST /api/mates/{id}/triggers/{tid}/run", ah.TriggerRunPOST)

		chHTTP := handler.NewChannelHTTP(logger, mateStore, proxy)
		mux.HandleFunc("GET /api/mates/{id}/channels", chHTTP.List)
		mux.HandleFunc("DELETE /api/mates/{id}/channels/{kind}", chHTTP.Delete)
		mux.HandleFunc("POST /api/mates/{id}/channels/discord", chHTTP.DiscordConnect)
		mux.HandleFunc("POST /api/mates/{id}/channels/telegram/pair/start", chHTTP.TelegramStart)
		mux.HandleFunc("GET /api/mates/{id}/channels/telegram/pair/status", chHTTP.TelegramStatus)
		mux.HandleFunc("POST /api/mates/{id}/channels/wechat/login/start", chHTTP.WechatStart)
		mux.HandleFunc("GET /api/mates/{id}/channels/wechat/login/status", chHTTP.WechatStatus)
	}

	skh := handler.NewSkillsHTTP(skillsMgr, mateStore)
	mux.HandleFunc("GET /api/skills", skh.List)
	mux.HandleFunc("GET /api/skills/catalog", skh.Catalog)
	mux.HandleFunc("POST /api/skills/add", skh.Add)
	mux.HandleFunc("DELETE /api/skills/{name}", skh.Remove)
	mux.HandleFunc("POST /api/skills/{name}/enable", skh.Enable)
	mux.HandleFunc("POST /api/skills/{name}/disable", skh.Disable)

	// System routes
	mux.HandleFunc("POST /healthz", h.HealthCheck)
	mux.HandleFunc("POST /version", h.Version)
	mux.HandleFunc("POST /api/status", handler.NewStatus(vault, searchPlugin).Status)

	mux.HandleFunc("GET /api/config", cfgHTTP.Get)
	mux.HandleFunc("GET /api/config/schema", cfgHTTP.Schema)
	mux.HandleFunc("PATCH /api/config", cfgHTTP.Patch)
	mux.HandleFunc("POST /api/server/restart", handler.NewRestartHTTP(cfgFileLoaded, restart).Post)

	// MCP endpoint; tools call back into mux in-process, so auth is enforced once at the edge.
	mux.HandleFunc("GET /api/editor/agent", editorBus.StreamSSE)
	mux.HandleFunc("POST /api/editor/agent/ack", editorBus.AckPOST)
	mux.HandleFunc("POST /api/editor/agent/focus", editorBus.FocusPOST)
	mux.Handle("/mcp", hylomcp.NewHandler(client.NewInProcess(mux), hylomcp.WithEditorBus(editorBus)))

	// Website routes
	mux.HandleFunc("GET /notes/search", vh.SearchFragment)
	mux.HandleFunc("GET /home", vh.Home)
	mux.HandleFunc("GET /dev/icons", vh.IconSpecimen)
	mux.HandleFunc("GET /home/refresh", vh.HomeRefresh)
	mux.HandleFunc("GET /home/section", vh.HomeSection)
	mux.HandleFunc("GET /home/section/more", vh.HomeSectionMore)
	mux.HandleFunc("GET /shorts/stream", vh.ShortsStream)
	mux.HandleFunc("GET /shorts/entry", vh.ShortsEntry)
	mux.HandleFunc("GET /shorts/tags", vh.ShortsTags)
	mux.HandleFunc("GET /images/grid", vh.ImagesGrid)
	mux.HandleFunc("GET /api/graph/data", vh.KnowledgeGraphData)
	mux.HandleFunc("POST /api/graph/rebuild", vh.KnowledgeGraphRebuild)
	mux.HandleFunc("GET /notes/fragment", vh.NoteFragment)

	mux.Handle("POST /api/notes/resolve", handler.NewNoteResolve(vault))
	mux.Handle("POST /api/notes/exist", handler.NewNoteExist(vault))
	mux.Handle("POST /api/notes/info", handler.NewNoteInfo(vault))
	mux.Handle("POST /api/notes/meta", handler.NewNoteMeta(vault))
	embeds, embedProgress := handler.NewEmbedHandlers(vault, proxy)
	mux.Handle("POST /api/embeds", embeds)
	mux.Handle("POST /api/embeds/progress", embedProgress)

	// Vault REST
	mux.HandleFunc("POST /api/vault/read", gh.Read)
	mux.HandleFunc("POST /api/vault/stat", gh.Stat)
	mux.HandleFunc("POST /api/vault/list", gh.List)
	mux.HandleFunc("POST /api/vault/list-dirs", gh.ListDirs)
	mux.HandleFunc("POST /api/vault/write", gh.Write)
	mux.HandleFunc("POST /api/vault/create-untitled", gh.CreateUntitled)
	mux.HandleFunc("POST /api/vault/delete", gh.Delete)
	mux.HandleFunc("POST /api/vault/move", gh.Move)
	mux.HandleFunc("POST /api/vault/rename", gh.Rename)
	mux.HandleFunc("POST /api/tag/delete", gh.TagDelete)
	mux.HandleFunc("POST /api/tag/rename", gh.TagRename)
	mux.HandleFunc("GET /api/vault/check-name", gh.CheckName)
	mux.Handle("GET /api/vault/overview", handler.NewVaultOverview(vault))
	mux.HandleFunc("GET /api/vault/rename-status", gh.RenameStatus)
	mux.HandleFunc("POST /api/vault/upload-image", gh.UploadImage)
	mux.HandleFunc("POST /api/vault/pin", gh.Pin)
	mux.HandleFunc("POST /api/vault/shorts", gh.Short)
	mux.HandleFunc("POST /api/vault/shorts/list", gh.ShortList)
	mux.HandleFunc("POST /api/vault/shorts/get", gh.ShortGet)
	mux.HandleFunc("POST /api/vault/shorts/update", gh.ShortUpdate)
	mux.HandleFunc("POST /api/vault/shorts/delete", gh.ShortDelete)
	mux.HandleFunc("GET "+storage.AssetsDir+"/", gh.ServeAsset)
	mux.HandleFunc("GET /api/images/serve", gh.ServeImageByName)
	mux.HandleFunc("GET /api/images/at", gh.ServeImageAt)
	mux.HandleFunc("POST /api/images/delete", gh.DeleteGalleryImage)
	mux.Handle("GET /static/", http.StripPrefix("/static/", static.Handler()))

	// Plugin routes
	mux.HandleFunc("POST /api/search", sh.Search)
	mux.HandleFunc("POST /api/tag/list", sh.TagList)
	mux.HandleFunc("POST /api/tag/count", sh.TagCount)

	if gitPlugin != nil {
		gsh := gitsync.NewHandler(gitPlugin)
		mux.HandleFunc("POST /api/git/sync", gsh.Sync)
	}
	if compilePlugin != nil {
		dh := compile.NewHandler(vault, compilePlugin)
		mux.HandleFunc("POST /api/compile/trigger", dh.Trigger)
	}

	return middleware.Chain(mux,
		middleware.Recoverer(logger),
		middleware.Logger(logger),
		middleware.Authenticator(apiKey, logger, view.RenderLogin),
	)
}
