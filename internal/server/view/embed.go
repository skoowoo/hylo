package view

import _ "embed"

//go:embed assets/info_dialog.css
var infoDialogCSS string

//go:embed assets/info_dialog.html
var infoDialogHTML string

//go:embed assets/info_dialog.js
var infoDialogJS string

//go:embed assets/base.css
var baseCSS string

//go:embed assets/confirm_dialog.css
var confirmDialogCSS string

//go:embed assets/confirm_dialog.html
var confirmDialogHTML string

//go:embed assets/confirm_dialog.js
var confirmDialogJS string

// frontmatterDialogCSS/HTML/JS form the "Edit metadata" dialog opened from
// the CM6 live-preview editor's frontmatter header (content_pane.js) — a plain
// YAML textarea + Save/Cancel, same window.hyloOverlay shell as
// confirm_dialog/short_dialog.

//go:embed assets/frontmatter_dialog.css
var frontmatterDialogCSS string

//go:embed assets/frontmatter_dialog.html
var frontmatterDialogHTML string

//go:embed assets/frontmatter_dialog.js
var frontmatterDialogJS string

//go:embed assets/content_pane.css
var contentPaneCSS string

//go:embed assets/content_pane.html
var contentPaneHTML string

//go:embed assets/path_ac.js
var pathAcScript string

//go:embed assets/chat_reduce.js
var chatReduceJS string

//go:embed assets/chat_text.js
var chatTextJS string

//go:embed assets/chat_kernel.js
var chatKernelJS string

//go:embed assets/chat_view.js
var chatViewJS string

//go:embed assets/list_tighten.js
var listTightenJS string

//go:embed assets/tab_fit.js
var tabFitJS string

//go:embed assets/graph_layout.js
var graphLayoutJS string

//go:embed assets/editor_session.js
var editorSessionJS string

//go:embed assets/content_pane.js
var contentPaneScript string

//go:embed assets/agent_chat.css
var agentChatCSS string

//go:embed assets/home.css
var homeCSS string

//go:embed assets/home.html
var homeMainHTML string

//go:embed assets/home.js
var homeJS string

//go:embed assets/images.css
var imagesCSS string

//go:embed assets/images_grid.html
var imagesGridHTML string

//go:embed assets/shorts.css
var shortsCSS string

//go:embed assets/shorts_stream.html
var shortsStreamHTML string

//go:embed assets/note_frontmatter.css
var noteFrontmatterCSS string

//go:embed assets/note_shared_prose.css
var noteSharedProseCSS string

//go:embed assets/note_editor_prose.css
var noteEditorProseCSS string

//go:embed assets/note_fonts.html
var noteFontsHTML string

//go:embed assets/note_shared.js
var noteSharedJSBody string

//go:embed assets/graph.css
var graphCSS string

//go:embed assets/cselect.css
var cselectCSS string


//go:embed assets/home_sidebar.html
var homeSidebarPartsHTML string
