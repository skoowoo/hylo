// Public entry point for the CodeMirror-based live-preview editor (Phase 0
// foundation). Everything under cm-live/ is app-agnostic — no fetch calls,
// no Vaultr routing, no DOM ids from content_pane.js — app wiring (image upload,
// wikilink navigation, autosave, tab state) stays in content_pane.js and is
// injected here through `options`, the same seam __vaultrDE currently uses
// to configure Milkdown's plugins. That separation is what let this be
// built and demoed (see ../../livepreview-demo/) without touching the live
// app, and is what should let Phase 1-4 extend it without re-architecting.
import { markdown } from '@codemirror/lang-markdown';
import { GFM } from '@lezer/markdown';
import { languages } from '@codemirror/language-data';
import { syntaxHighlighting } from '@codemirror/language';
import { wikiSyntax } from './wiki-syntax.js';
import { frontmatterSyntax } from './frontmatter-syntax.js';
import { livePreviewPlugin, livePreviewAtomicRanges } from './live-preview.js';
import { horizontalRuleField } from './horizontal-rule-field.js';
import { frontmatterCollapseField, frontmatterHeaderField } from './frontmatter-collapse.js';
import { frontmatterReadOnly } from './frontmatter-readonly.js';
import { linkClickHandler } from './link-click.js';
import { livePreviewTheme, codeHighlightStyle } from './theme.js';

export { wikiSyntax, wikiNodeInner, splitWikiLinkInner } from './wiki-syntax.js';
export { frontmatterSyntax } from './frontmatter-syntax.js';
export { nodeDecorators } from './decorators.js';
export { livePreviewPlugin, livePreviewAtomicRanges, wikiLinksRevalidated } from './live-preview.js';
export { horizontalRuleField } from './horizontal-rule-field.js';
export { frontmatterCollapseField, frontmatterHeaderField, setFrontmatterCollapsed } from './frontmatter-collapse.js';
export { frontmatterReadOnly, allowFrontmatterEdit } from './frontmatter-readonly.js';
export { linkClickHandler } from './link-click.js';
export { livePreviewTheme, codeHighlightStyle } from './theme.js';
export { listIndentExtension } from './list-indent.js';
export { toggleBold, toggleItalic, toggleStrikethrough } from './inline-format.js';
export { toggleBulletList, toggleOrderedList, toggleTaskList } from './list-format.js';
export { toggleBlockquote } from './quote-format.js';
export { selectionFormatMenu } from './format-menu.js';
export {
  WikiLinkWidget,
  WikiImageWidget,
  TaskCheckboxWidget,
  MarkdownImageWidget,
  BulletWidget,
  HorizontalRuleWidget,
} from './widgets.js';
export { selectionTouchesRange, selectionTouchesLine, selectionInsideRange, readingMode } from './selection.js';
export { readingExtensions } from './reading.js';
export { initialCursorOffset } from './initial-cursor.js';

// Shared by live-preview mode and (Phase 4) plain source mode, so toggling
// between them reconfigures a Compartment around the *same* parse instead
// of swapping to a differently-configured language and re-parsing from
// scratch.
// codeLanguages: languages — @codemirror/lang-markdown's stock hook for
// fenced-code-content parsing. Matches a fence's info string ("```js")
// against @codemirror/language-data's registry and, on a hit, nests that
// language's own grammar inside the FencedCode node instead of leaving its
// content as opaque text — the standard, documented way to get real code
// tokens for syntaxHighlighting(codeHighlightStyle) (theme.js) to color.
export function wikiMarkdownLanguage() {
  return markdown({ extensions: [GFM, wikiSyntax, frontmatterSyntax], codeLanguages: languages });
}

/**
 * @param {object} [options]
 * @param {(filename: string) => string} [options.resolveImageSrc]
 *   Builds the <img src> for a ![[wikiimage]] widget. Defaults to using the
 *   filename verbatim, which only works for demo/test fixtures.
 * @param {(target: string, alias: string|null, event: MouseEvent) => void}
 *   [options.onWikiLinkClick] Called when a [[wikilink]] chip is clicked.
 * @param {(target: string) => boolean} [options.isWikiLinkBroken]
 *   Reports whether a [[wikilink]] target note no longer exists, so the
 *   widget renders struck-through and skips the click handler. Read fresh
 *   on every rebuild — dispatch wikiLinksRevalidated after updating whatever
 *   this closes over (e.g. an existence Set) to force a re-check without a
 *   doc/selection change.
 * @param {(view: import('@codemirror/view').EditorView, from: number, to: number) => void}
 *   [options.onEditFrontmatter] Called with the Frontmatter node's range
 *   when the "Metadata" header's pencil button is clicked — see
 *   frontmatter-collapse.js. Frontmatter is read-only in this view
 *   (frontmatter-readonly.js) otherwise, so this is the only edit path.
 */
// Torn down when the host switches to source mode. The shared language, the
// collapse flag, and link clicks stay outside this list: a mode toggle must
// not reparse the doc or drop a flag the header still reads.
//
// The "Metadata" header is its own StateField. decorateFrontmatter only
// hairlines each YAML line; a block:true row is refused from a ViewPlugin,
// and an earlier multi-line replace crashed CM6's height-map on a full-doc
// replace after an edit.
export function liveDecorations(options) {
  return [
    livePreviewPlugin.of(options),
    livePreviewAtomicRanges(),
    horizontalRuleField(),
    frontmatterReadOnly(),
    frontmatterHeaderField(options),
    livePreviewTheme,
    syntaxHighlighting(codeHighlightStyle),
  ];
}

// Single-mode bundle used by the live-preview demo. The app mounts
// liveDecorations() inside a compartment instead of calling this.
export function livePreviewExtensions(options) {
  return [
    wikiMarkdownLanguage(),
    frontmatterCollapseField,
    linkClickHandler(),
  ].concat(liveDecorations(options));
}
