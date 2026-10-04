package server

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/inbox"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/plugins/compile"
	"github.com/hardhacker/hylo/internal/plugins/gitsync"
	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/server/handler"
	"github.com/hardhacker/hylo/internal/server/middleware"
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
	apiKey string) http.Handler {
	mux := http.NewServeMux()

	h := handler.New(logger, cfg)
	cfgHTTP := handler.NewConfigHTTP(logger, cfg, cfgFileLoaded)
	gh := handler.NewVault(vault)
	sh := search.NewHandler(searchPlugin)
	vh := view.NewView(vault, searchPlugin, cfg)

	agentHub := agent.NewHub()
	agentCache := agent.NewAgentCache(0, nil)
	agentCache.WarmUp()
	ah := handler.NewAgentAPI(logger, cfg, vault, agentHub, mateStore, agentCache)

	// Inbox: generic, source-agnostic message store. mate trigger runs are the
	// first producer (wired below); other producers can call inboxStore.Create
	// directly without depending on the mate package. Shares mate.db's
	// connection rather than opening its own file.
	var inboxStore *inbox.Store
	if mateStore != nil {
		is, err := inbox.Open(mateStore.DB())
		if err != nil {
			logger.Warn("inbox store unavailable", "err", err)
		} else {
			inboxStore = is
		}
	}

	// Wire mate runner → agent API so trigger runs use the same execution path.
	// Trigger run completions become inbox messages; inboxStore.Create fans out
	// to /api/inbox/notifications on its own, so no separate push is needed here.
	if mateRunner != nil {
		ah.SetMateRunner(mateRunner)
		mateRunner.SetRunFunc(ah.FireTriggerRun)
		mateRunner.SetRetryFunc(ah.FireTriggerRetry)
		mateRunner.SetRunDoneHook(func(m *mate.Mate, result mate.RunResult) {
			if result.EventType == mate.MateEventWechatMessage || result.EventType == mate.MateEventDiscordMessage {
				return
			}
			if inboxStore == nil {
				return
			}
			if strings.TrimSpace(result.LastMessage) == "" {
				return
			}
			level := inbox.LevelSuccess
			if !result.Success {
				level = inbox.LevelError
			}
			title := m.Name
			if h1 := firstMarkdownH1(result.LastMessage); h1 != "" {
				title = m.Name + ": " + h1
			}
			if _, err := inboxStore.Create(inbox.Message{
				Source: "mate_trigger",
				Level:  level,
				Title:  title,
				Body:   result.LastMessage,
				Metadata: map[string]any{
					"mateId":     m.ID,
					"eventType":  string(result.EventType),
					"durationMs": result.Duration.Milliseconds(),
				},
			}); err != nil {
				logger.Warn("inbox: create message", "err", err)
			}
		})
	}

	if inboxStore != nil {
		ih := handler.NewInboxAPI(inboxStore)
		mux.HandleFunc("GET /api/inbox", ih.InboxGET)
		mux.HandleFunc("GET /api/inbox/unread-count", ih.InboxUnreadCountGET)
		mux.HandleFunc("POST /api/inbox/read-all", ih.InboxReadAllPOST)
		mux.HandleFunc("POST /api/inbox/{id}/read", ih.InboxReadPOST)
		mux.HandleFunc("DELETE /api/inbox/{id}", ih.InboxDELETE)
		mux.HandleFunc("GET /api/inbox/notifications", inboxStore.Notifications().StreamSSE)
	}

	mux.HandleFunc("GET /api/agents", ah.AgentsGET)
	mux.HandleFunc("POST /api/chat", ah.ChatPOST)
	mux.HandleFunc("POST /api/runs", ah.RunsPOST)
	mux.HandleFunc("GET /api/runs/active", ah.RunsActiveGET)
	mux.HandleFunc("GET /api/runs/by-ref", ah.RunByRefGET)
	mux.HandleFunc("GET /api/runs/{id}/events", ah.RunEventsGET)
	mux.HandleFunc("GET /api/runs/{id}", ah.RunGET)
	mux.HandleFunc("POST /api/runs/{id}/cancel", ah.RunCancelPOST)
	mux.HandleFunc("POST /api/runs/retry-trigger", ah.RunsRetryTriggerPOST)

	if mateStore != nil {
		ch := handler.NewConversationAPI(mateStore)
		mux.HandleFunc("GET /api/conversations", ch.ConversationsGET)
		mux.HandleFunc("POST /api/conversations", ch.ConversationsPOST)
		mux.HandleFunc("GET /api/conversations/{id}", ch.ConversationGET)
		mux.HandleFunc("DELETE /api/conversations/{id}", ch.ConversationDELETE)
		mux.HandleFunc("PATCH /api/conversations/{id}/title", ch.ConversationTitlePATCH)

		mh := handler.NewMateAPI(logger, mateStore)
		mux.HandleFunc("GET /api/mate-events", mh.MateEventsGET)

		mux.HandleFunc("GET /api/mates", mh.MatesGET)
		mux.HandleFunc("POST /api/mates/reorder", mh.MatesReorderPOST)
		mux.HandleFunc("POST /api/mates", mh.MatesPOST)
		mux.HandleFunc("GET /api/mates/{id}", mh.MateGET)
		mux.HandleFunc("PUT /api/mates/{id}", mh.MatePUT)
		mux.HandleFunc("DELETE /api/mates/{id}", mh.MateDELETE)
	}

	skh := handler.NewSkillsHTTP(skillsMgr)
	mux.HandleFunc("GET /api/skills", skh.List)
	mux.HandleFunc("POST /api/skills/install", skh.Install)
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

	wxHTTP := handler.NewWechatHTTP(logger, cfg, cfgFileLoaded)
	mux.HandleFunc("GET /api/wechat/status", wxHTTP.Status)
	mux.HandleFunc("POST /api/wechat/login/start", wxHTTP.LoginStart)
	mux.HandleFunc("GET /api/wechat/login/status", wxHTTP.LoginStatus)
	mux.HandleFunc("POST /api/wechat/logout", wxHTTP.Logout)

	// Website routes
	mux.HandleFunc("GET /notes/search", vh.SearchFragment)
	mux.HandleFunc("GET /home", vh.Home)
	mux.HandleFunc("GET /home/refresh", vh.HomeRefresh)
	mux.HandleFunc("GET /home/section", vh.HomeSection)
	mux.HandleFunc("GET /home/section/more", vh.HomeSectionMore)
	mux.HandleFunc("GET /shorts/stream", vh.ShortsStream)
	mux.HandleFunc("GET /images/grid", vh.ImagesGrid)
	mux.HandleFunc("GET /api/graph/data", vh.KnowledgeGraphData)
	mux.HandleFunc("POST /api/graph/rebuild", vh.KnowledgeGraphRebuild)
	mux.HandleFunc("GET /notes/fragment", vh.NoteFragment)
	mux.HandleFunc("GET /quick-capture", vh.QuickCapture)

	mux.Handle("POST /api/notes/resolve", handler.NewNoteResolve(vault))
	mux.Handle("POST /api/notes/exist", handler.NewNoteExist(vault))
	mux.Handle("POST /api/notes/info", handler.NewNoteInfo(vault))

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
	mux.HandleFunc("GET /api/vault/check-name", gh.CheckName)
	mux.HandleFunc("GET /api/vault/rename-status", gh.RenameStatus)
	mux.HandleFunc("POST /api/vault/upload-image", gh.UploadImage)
	mux.HandleFunc("POST /api/vault/pin", gh.Pin)
	mux.HandleFunc("POST /api/vault/shorts", gh.Short)
	mux.HandleFunc("POST /api/vault/shorts/list", gh.ShortList)
	mux.HandleFunc("GET /_assets/", gh.ServeAsset)
	mux.HandleFunc("GET /api/images/serve", gh.ServeImageByName)
	mux.HandleFunc("GET /api/images/at", gh.ServeImageAt)
	mux.HandleFunc("POST /api/images/delete", gh.DeleteGalleryImage)
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))

	// Plugin routes
	mux.HandleFunc("POST /api/search", sh.Search)
	mux.HandleFunc("POST /api/tag/list", sh.TagList)
	mux.HandleFunc("POST /api/tag/count", sh.TagCount)
	mux.HandleFunc("POST /api/tag/delete", sh.TagDelete)

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
		middleware.Authenticator(apiKey, logger),
	)
}

// markdownH1Pattern matches the first ATX H1 heading ("# ...", not "##...")
// on its own line, multiline mode so ^/$ match at line boundaries.
var markdownH1Pattern = regexp.MustCompile(`(?m)^#[ \t]+(.+?)[ \t]*$`)

// firstMarkdownH1 returns the text of the first H1 heading in body, so an
// inbox message's title can read "<mate name>: <H1>" instead of just the
// mate name. Returns "" if body has no H1.
func firstMarkdownH1(body string) string {
	m := markdownH1Pattern.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}
