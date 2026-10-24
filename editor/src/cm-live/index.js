// Live-preview editor extensions. App wiring (image upload, navigation, autosave) is
// injected through `options` by the host, not imported here.
import { markdown } from '@codemirror/lang-markdown';
import { GFM } from '@lezer/markdown';
import { codeLanguages } from './code-languages.js';
import { syntaxHighlighting } from '@codemirror/language';
import { wikiSyntax } from './wiki-syntax.js';
import { frontmatterSyntax } from './frontmatter-syntax.js';
import { livePreviewPlugin, skipListGaps } from './live-preview.js';
import { horizontalRuleField } from './horizontal-rule-field.js';
import { fenceCardField } from './fence-card/card-field.js';
import { frontmatterCardField } from './frontmatter-card.js';
import { livePreviewTheme, codeHighlightStyle } from './theme.js';
import { verticalMotion } from './vertical-motion.js';
import { widgetReveal } from './widget-reveal.js';

export { wikiLinksRevalidated } from './live-preview.js';
export { frontmatterCollapseField } from './frontmatter-card.js';
export { linkClickHandler, jumpToHeading } from './link-click.js';
export { listIndentExtension } from './list-indent.js';
export { selectionFormatMenu } from './format-menu.js';
export { fenceAutoClose } from './fence-close.js';
export { cjkWordSelection } from './cjk-words.js';
export { formatKeymap } from './format-keymap.js';
export { tableKeymap } from './table-keys.js';
export { wikiLinkCompletion } from './wikilink-complete.js';
export { readingExtensions } from './reading.js';
export { initialCursorOffset } from './initial-cursor.js';
export { preloadCodeLanguages } from './code-languages.js';
export { agentCursorExtension, planAgentInsert, planAgentRewrite, readAgentDoc, agentType, foreignEditCount } from './agent-typing.js';

// Shared by live-preview mode and (Phase 4) plain source mode, so toggling
// between them reconfigures a Compartment around the *same* parse instead
// of swapping to a differently-configured language and re-parsing from
// scratch.
// codeLanguages nests a matching grammar inside fenced code so codeHighlightStyle can color it.
export function wikiMarkdownLanguage() {
  return markdown({ extensions: [GFM, wikiSyntax, frontmatterSyntax], codeLanguages });
}

/**
 * @param {object} [options]
 * @param {(filename: string) => string} [options.resolveImageSrc]
 *   Builds the <img src> for a ![[wikiimage]] widget. Defaults to using the
 *   filename verbatim, which only works for test fixtures.
 * @param {(url: string) => string} [options.resolveMarkdownImageSrc]
 *   Builds the <img src> for a ![alt](url) image — e.g. resolving a relative
 *   path against the note's folder. Defaults to the URL verbatim.
 * @param {(target: string, alias: string|null, event: MouseEvent, heading: string|null) => void}
 *   [options.onWikiLinkClick] Called when a [[wikilink]] chip is clicked. `target`
 *   is '' for a same-note "[[#Heading]]" link.
 * @param {(target: string) => boolean} [options.isWikiLinkBroken]
 *   Reports whether a [[wikilink]] target note no longer exists, so the
 *   widget renders struck-through and skips the click handler. Read fresh
 *   on every rebuild — dispatch wikiLinksRevalidated after updating whatever
 *   this closes over (e.g. an existence Set) to force a re-check without a
 *   doc/selection change.
 */
// Torn down when the host switches to source mode. The shared language, the
// collapse flag, and link clicks stay outside this list: a mode toggle must
// not reparse the doc or drop a flag the card still reads.
export function liveDecorations(options) {
  return [
    livePreviewPlugin.of(options),
    skipListGaps,
    widgetReveal,
    verticalMotion,
    horizontalRuleField(),
    fenceCardField(),
    frontmatterCardField(options),
    livePreviewTheme,
    syntaxHighlighting(codeHighlightStyle),
  ];
}
