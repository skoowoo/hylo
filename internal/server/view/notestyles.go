package view

// noteSharedCSS = frontmatter CSS + prose typography + neo note overrides.
// noteEditorCSS = empty today (note_editor_prose.css) — the ProseMirror/
// Milkdown editor it used to style was replaced by the CM6 live-preview
// editor, which gets its styling from a runtime EditorView.theme() instead
// (see editor/src/cm-live/theme.js). Kept as a wired-but-empty
// embed rather than removed so this concatenation point still exists if
// editor-only CSS comes up again.
// Both are assembled from embedded asset files.
var noteSharedCSS = noteFrontmatterCSS + noteSharedProseCSS
var noteEditorCSS = noteEditorProseCSS

// noteSharedJS is a complete <script> block shared by all note-rendering pages.
var noteSharedJS = "<script>\n" + noteSharedJSBody + "</script>"
