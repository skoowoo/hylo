// Bundled to internal/server/static/editor.js; the host imports only what is exported here.
export { EditorView, keymap } from '@codemirror/view';
export { EditorState, EditorSelection, Compartment } from '@codemirror/state';
export { HighlightStyle, syntaxHighlighting, forceParsing } from '@codemirror/language';
export { tags } from '@lezer/highlight';
export { defaultKeymap, history, historyKeymap, undo as cmUndo, redo as cmRedo } from '@codemirror/commands';
export {
  search, openSearchPanel, closeSearchPanel, findNext, findPrevious, replaceNext,
  replaceAll as cmReplaceAll, SearchQuery, setSearchQuery,
} from '@codemirror/search';
export {
  liveDecorations,
  wikiMarkdownLanguage,
  wikiLinksRevalidated,
  frontmatterCollapseField,
  allowFrontmatterEdit,
  linkClickHandler,
  listIndentExtension,
  readingExtensions,
  initialCursorOffset,
  selectionFormatMenu,
  preloadCodeLanguages,
} from './cm-live/index.js';
