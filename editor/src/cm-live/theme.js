// Ported from note_editor_prose.css (+ shared prose/frontmatter rules).
// Values are old literals or deliberate, commented deviations.
//
// CM6 notes:
//   1. No <h1>/<li> — only .cm-line; DOM-adjacency rules live in decorators.js.
//   2. Blank lines are real — don't port inter-block margin (would double-gap).
import { EditorView } from '@codemirror/view';
import { HighlightStyle } from '@codemirror/language';
import { tags } from '@lezer/highlight';

export const livePreviewTheme = EditorView.theme({
  '.cm-content': {
    fontFamily: 'var(--font-sans, "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif)',
    fontSize: 'var(--text-body, 1rem)',
    color: 'var(--prose-body, #374151)',
    lineHeight: '1.75',
    // !important: content_pane.css `#content-pane-edit-area .cm-content { padding: 0 }` otherwise wins.
    // Top is 0, not 24px — .cm-scroller already adds 2.5rem (content_pane.css),
    // and source mode (no livePreviewTheme) only gets that one gap, so a top
    // value here just double-pads live-preview on top of it.
    paddingTop: '0 !important',
    paddingBottom: '24px !important',
  },

  // ── Headings ────────────────────────────────────────────────────────────
  // Obsidian's actual Minimal-theme scale (pulled from live devtools output,
  // not guessed): --h1-size:1.125em, h2:1.05em, h3:1em (== body!), h4:0.9em,
  // h5/h6:0.85em (below body, propped up by small-caps). That's far flatter
  // than a first pass at "gentle" assumed — landing here is a deliberate
  // middle ground: keep h4-h6 at-or-above body size (no small-caps trick to
  // fall back on) while still compressing top-to-bottom much closer than
  // before. No dedicated display face any more (Cal Sans dropped app-wide —
  // headings inherit .cm-content's font-family, same Inter stack as body).
  // All six levels share one weight (600); same scale in .prose
  // (note_shared_prose.css).
  // Explicit lineHeight: large fonts without it leave hit-box taller than glyphs
  // → click lands on next line. Padding not margin — CM6 ResizeObserver
  // ignores margin. !important: content_pane.css zeros .cm-line padding via
  // ID selector.
  '.cm-lp-heading': { fontWeight: '600' },
  '.cm-lp-h1': {
    fontSize: '1.3em',
    lineHeight: '1.3',
    letterSpacing: '-0.012em',
    color: 'var(--h1, #111111)',
    paddingTop: '1.5em !important',
    paddingBottom: '0.65em !important',
  },
  '.cm-lp-h2': {
    fontSize: '1.2em',
    lineHeight: '1.35',
    letterSpacing: '-0.01em',
    color: 'var(--h2, #1f2937)',
    paddingTop: '1.75em !important',
    paddingBottom: '0.65em !important',
  },
  '.cm-lp-h3': {
    fontSize: '1.1em',
    lineHeight: '1.4',
    letterSpacing: '-0.006em',
    color: 'var(--h3, #374151)',
    paddingTop: '1.5em !important',
    paddingBottom: '0.5em !important',
  },
  '.cm-lp-h4': {
    fontSize: '1.05em',
    lineHeight: '1.5',
    letterSpacing: '-0.003em',
    color: 'var(--h4, #6b7280)',
    paddingTop: '1.25em !important',
    paddingBottom: '0.5em !important',
  },
  // h5/h6 keep h4's ink (lighter fails AA at body size) and step down by size/case.
  '.cm-lp-h5': {
    fontSize: '1em',
    lineHeight: '1.6',
    color: 'var(--h5, #6b7280)',
    paddingTop: '1.25em !important',
    paddingBottom: '0.5em !important',
  },
  '.cm-lp-h6': {
    fontSize: '0.875em',
    lineHeight: '1.8',
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
    color: 'var(--h6, #6b7280)',
    paddingTop: '1.25em !important',
    paddingBottom: '0.5em !important',
  },
  // Revealed "#" marker (caret on the line) — muted regardless of heading
  // level so it reads as syntax, not part of the heading text.
  '.cm-lp-heading-mark': { color: 'var(--muted-soft, rgba(60,60,67,0.35))', fontWeight: '400' },
  // Box for hangRange: right-aligned so the syntax ends where the text begins.
  '.cm-lp-hang': { display: 'inline-block', textAlign: 'right', whiteSpace: 'pre', fontStyle: 'normal' },
  '.cm-lp-quote-mark': { color: 'var(--muted-soft, rgba(60,60,67,0.35))', paddingRight: '0.45rem', boxSizing: 'border-box' },
  // Setext underline once its text is hidden.
  '.cm-lp-hidden-line': { fontSize: '1px', lineHeight: '1px', padding: '0 !important' },

  '.cm-lp-strong': { fontWeight: '600', color: 'var(--prose-strong, #111111)' },
  '.cm-lp-em': { fontStyle: 'italic', color: 'var(--prose-em, #374151)' },
  // CJK has no italic face — the browser would shear the glyphs. Chinese
  // typography marks emphasis with dots under the characters instead.
  '.cm-lp-em-cjk': {
    fontStyle: 'normal',
    textEmphasis: 'filled dot',
    WebkitTextEmphasis: 'filled dot',
    textEmphasisPosition: 'under right',
    WebkitTextEmphasisPosition: 'under right',
  },
  '.cm-lp-strike': { textDecoration: 'line-through', color: 'var(--muted, #6d7080)' },
  '.cm-lp-code': {
    fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
    fontSize: '0.85em',
    color: 'var(--code-tx, #374151)',
    background: 'var(--th-bg, rgba(24,24,27,0.04))',
    borderRadius: 'var(--r-xs, 4px)',
    padding: '0.15em 0.42em',
  },

  // ── Blockquote ──────────────────────────────────────────────────────────
  // !important: content_pane.css zeros .cm-line padding.
  // borderRadius:0 — global reset would round the inset bar into brackets.
  // Not italic: most quotes here are CJK, which has no italic face.
  '.cm-lp-quote': {
    paddingTop: '0.35rem !important',
    paddingBottom: '0.35rem !important',
    paddingLeft: '1.5rem !important',
    fontSize: '1em',
    lineHeight: '1.75',
    color: 'var(--bq-tx, rgba(55,65,81,0.88))',
    position: 'relative',
    borderRadius: '0',
  },
  // One 2px bar per nesting level, 1.5rem apart. Gradients, not inset
  // box-shadows: an offset inset shadow fills everything up to the offset.
  '.cm-line.cm-lp-quote': { background: 'linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 0 0 / 2px 100% no-repeat' },
  // Nested: +1.5rem pad + one more bar per level (cap 4).
  '.cm-lp-quote-d2': { paddingLeft: '3rem !important' },
  '.cm-line.cm-lp-quote-d2': { background: 'linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 0 0 / 2px 100% no-repeat, linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 1.5rem 0 / 2px 100% no-repeat' },
  '.cm-lp-quote-d3': { paddingLeft: '4.5rem !important' },
  '.cm-line.cm-lp-quote-d3': { background: 'linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 0 0 / 2px 100% no-repeat, linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 1.5rem 0 / 2px 100% no-repeat, linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 3.0rem 0 / 2px 100% no-repeat' },
  '.cm-lp-quote-d4': { paddingLeft: '6rem !important' },
  '.cm-line.cm-lp-quote-d4': { background: 'linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 0 0 / 2px 100% no-repeat, linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 1.5rem 0 / 2px 100% no-repeat, linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 3.0rem 0 / 2px 100% no-repeat, linear-gradient(var(--bq-bd, rgba(94,106,210,0.55)), var(--bq-bd, rgba(94,106,210,0.55))) 4.5rem 0 / 2px 100% no-repeat' },

  // ── Links — the whole visible text is the hit target (link-click.js).
  // Color carries the link now, not an underline — no textDecoration.
  '.cm-lp-link': {
    color: 'var(--accent-text, #5e6ad2)',
    fontWeight: '400',
    cursor: 'pointer',
  },
  '.cm-lp-link:not(.cm-lp-link-editing):hover': {
    background: 'var(--tint-soft, rgba(94,106,210,0.06))',
    color: 'var(--accent-hov, #4c56c8)',
    borderRadius: '0', // defeat global 8px reset
  },
  // Caret on the link: it's being edited, not followed.
  '.cm-lp-link-editing': { cursor: 'text' },
  '.cm-lp-link-url': { color: 'var(--accent-text, #5e6ad2)', opacity: '0.7' },
  '.cm-lp-linkref': { color: 'var(--muted, #6d7080)' },

  // ── Source shown as-is: HTML, comments, indented code.
  '.cm-lp-html': {
    fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
    fontSize: '0.85em',
    color: 'var(--muted, #6d7080)',
  },
  '.cm-lp-html-comment': { color: 'var(--muted-soft, rgba(60,60,67,0.45))' },
  '.cm-lp-htmlblock': {
    fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
    fontSize: 'var(--text-base, 0.875rem)',
    color: 'var(--muted, #6d7080)',
  },
  '.cm-lp-codeblock-indented.cm-lp-codeblock-first': { paddingTop: '0.6rem !important' },
  '.cm-lp-codeblock-indented.cm-lp-codeblock-last': { paddingBottom: '0.6rem !important' },

  // ── Lists ───────────────────────────────────────────────────────────────
  '.cm-lp-list-line': {
    // Was 1.7 — nearly identical to surrounding paragraph text's 1.75, so a
    // list never actually read as more compact than prose no matter how far
    // the padding above was cut; this is the value that actually needed to
    // move. 1.4 matches how mainstream editors (Notion, Typora) tighten
    // list-item line-height specifically, distinct from paragraph text.
    lineHeight: '1.4 !important',
    paddingTop: '0.1em !important',
    paddingBottom: '0.1em !important',
  },
  // A blank "loose list" line between two items — collapsed to a hairline
  // (same trick as the frontmatter/table hairline widgets) so cm-lp-list-line's
  // own padding is what controls the visible gap, not the blank line's
  // default full-text-line height.
  '.cm-lp-list-gap': { fontSize: '1px', lineHeight: '1px', padding: '0 !important' },
  // Main text color, not brand — these read as prose structure (like a
  // paragraph's own text), not an accent/call-to-action.
  // Fixed-width column the line's negative text-indent hangs the marker in.
  '.cm-lp-list-marker': { display: 'inline-block', textIndent: '0', whiteSpace: 'nowrap', verticalAlign: 'baseline' },
  '.cm-lp-list-mark-ol': { color: 'var(--prose-body, #374151)', fontVariantNumeric: 'tabular-nums' },
  '.cm-lp-bullet': { color: 'var(--prose-body, #374151)' },
  '.cm-lp-list-mark-raw': { color: 'var(--muted-soft, rgba(60,60,67,0.35))' },

  // ── Task checkboxes — real <input>, same look as old ::before SVG.
  '.cm-lp-task-checkbox': {
    appearance: 'none',
    WebkitAppearance: 'none',
    display: 'inline-block',
    width: '14px',
    height: '14px',
    verticalAlign: 'middle',
    margin: '-0.15em 0 0 0',
    border: '1.5px solid var(--prose-body, #374151)',
    borderRadius: 'var(--r-xs, 4px)',
    background: 'transparent',
    cursor: 'pointer',
    position: 'relative',
  },
  // Checked = main-text fill, not brand. The checkmark can't just be a
  // fixed white SVG on that fill (fill is near-white in dark mode, which
  // would hide a white check) — instead it's a ::after masked to the
  // checkmark shape with background-color: var(--bg), so it's always the
  // page background color, guaranteed to contrast against the fill
  // (main-text vs background is the one pair every theme keeps opposite).
  '.cm-lp-task-checkbox:checked': {
    background: 'var(--prose-body, #374151)',
    borderColor: 'var(--prose-body, #374151)',
  },
  '.cm-lp-task-checkbox:checked::after': {
    content: '""',
    position: 'absolute',
    inset: '0',
    backgroundColor: 'var(--bg, #ffffff)',
    WebkitMaskImage:
      "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 12'%3E%3Cpolyline points='2,6 5,9 10,3' fill='none' stroke='black' stroke-width='1.8' stroke-linecap='square' stroke-linejoin='miter'/%3E%3C/svg%3E\")",
    maskImage:
      "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 12'%3E%3Cpolyline points='2,6 5,9 10,3' fill='none' stroke='black' stroke-width='1.8' stroke-linecap='square' stroke-linejoin='miter'/%3E%3C/svg%3E\")",
    WebkitMaskSize: '100% 100%',
    maskSize: '100% 100%',
    WebkitMaskRepeat: 'no-repeat',
    maskRepeat: 'no-repeat',
  },

  // ── HR — padding not margin (ResizeObserver); reduced vs old 2em (blank
  // line already adds ~1.75em). margin:0 clears <hr> UA default.
  '.cm-lp-hr': {
    border: 'none',
    margin: '0',
    borderRadius: '0',
    borderTop: '1px solid var(--line-edge, rgba(24,24,27,0.16))',
    padding: '0.5em 0',
  },
  '.cm-lp-hr.cm-lp-hr-tight-bottom': {
    paddingBottom: '0',
  },

  // ── Fenced code — per-line chrome; borderRadius:0 so middle lines don't
  // pick up global 8px (would scallop the box edge).
  '.cm-lp-codeblock': {
    fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
    fontSize: 'var(--text-base, 0.875rem)',
    lineHeight: '1.65',
    color: 'var(--pre-tx, #111111)',
    background: 'var(--th-bg, rgba(24,24,27,0.04))',
    borderRadius: '0',
    paddingLeft: '1.5rem !important',
    paddingRight: '1.5rem !important',
  },
  '.cm-lp-codeblock-first': {
    position: 'relative', // anchors the absolute cm-lp-code-header
    borderTopLeftRadius: 'var(--r-sm, 6px)',
    borderTopRightRadius: 'var(--r-sm, 6px)',
  },
  '.cm-lp-codeblock-last': {
    borderBottomLeftRadius: 'var(--r-sm, 6px)',
    borderBottomRightRadius: 'var(--r-sm, 6px)',
  },
  // Fence lines are the block's top/bottom padding. Same height raw or
  // rendered (only the font is small), so revealing ``` never shifts text.
  '.cm-lp-codeblock-fence': { fontSize: '0.75rem', color: 'var(--muted, #6d7080)' },
  '.cm-lp-codeblock-first.cm-lp-codeblock-fence': { lineHeight: '2rem' },
  '.cm-lp-codeblock-last.cm-lp-codeblock-fence': { lineHeight: '1.25rem' },
  '.cm-lp-code-lang-edit': { color: 'var(--syn-keyword, #5a4a78)' },
  // flow-root keeps the card's margins inside the widget box: collapsed through,
  // CM's height measure misses them and every line below drifts.
  '.cm-card-widget': { display: 'flow-root' },
  '.cm-lp-code-header': {
    position: 'absolute',
    top: '0',
    right: '0.75rem',
    height: '2rem',
    display: 'inline-flex',
    alignItems: 'center',
    gap: '0.4rem',
  },
  '.cm-lp-code-lang': {
    color: 'var(--muted, #6d7080)',
    fontSize: '0.6875rem',
    lineHeight: '1',
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  },
  '.cm-lp-code-copy': {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '22px',
    height: '22px',
    padding: '0',
    border: 'none',
    background: 'transparent',
    color: 'var(--muted, #6d7080)',
    borderRadius: 'var(--r-sm, 6px)',
    cursor: 'pointer',
    opacity: '0.55',
    transition: 'opacity 0.15s ease, background-color 0.15s ease',
  },
  '.cm-lp-code-copy:hover': { opacity: '1', background: 'var(--icon-hov, rgba(0,0,0,0.06))', color: 'var(--fg, #1a1a1a)' },
  '.cm-lp-code-copy.is-done': { opacity: '1', color: 'var(--accent-text, #5e6ad2)' },
  '.cm-lp-code-copy svg': { width: '13px', height: '13px', display: 'block' },

  // ── Wikilink ────────────────────────────────────────────────────────────
  // Color carries the link now, not an underline — same as .cm-lp-link.
  '.cm-lp-wikilink': {
    display: 'inline',
    color: 'var(--accent-text, #5e6ad2)',
    fontWeight: '400',
    cursor: 'pointer',
    userSelect: 'none',
  },
  '.cm-lp-wikilink::before': {
    content: '""',
    display: 'inline-block',
    width: '11px',
    height: '13px',
    marginRight: '0.2em',
    verticalAlign: 'middle',
    marginTop: '-0.1em',
    backgroundColor: 'var(--muted, #6d7080)',
    WebkitMaskImage:
      'url("data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' viewBox=\'0 0 24 24\' fill=\'none\' stroke=\'black\' stroke-width=\'2\' stroke-linecap=\'round\' stroke-linejoin=\'round\'%3E%3Cpath d=\'M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z\'/%3E%3Cpath d=\'M14 2v4a2 2 0 0 0 2 2h4\'/%3E%3C/svg%3E")',
    maskImage:
      'url("data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' viewBox=\'0 0 24 24\' fill=\'none\' stroke=\'black\' stroke-width=\'2\' stroke-linecap=\'round\' stroke-linejoin=\'round\'%3E%3Cpath d=\'M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z\'/%3E%3Cpath d=\'M14 2v4a2 2 0 0 0 2 2h4\'/%3E%3C/svg%3E")',
    WebkitMaskSize: '100% 100%',
    maskSize: '100% 100%',
    WebkitMaskRepeat: 'no-repeat',
    maskRepeat: 'no-repeat',
    borderRadius: '0',
  },
  '.cm-lp-wikilink:hover': {
    background: 'var(--tint-soft, rgba(94,106,210,0.06))',
    color: 'var(--accent-hov, #4c56c8)',
    borderRadius: '0',
  },
  '.cm-lp-wikilink-raw': { color: 'var(--accent-text, #5e6ad2)' },
  // Target note no longer exists — struck through, muted, not interactive
  // (mirrors .wikilink-broken in note_shared_prose.css for the read view).
  '.cm-lp-wikilink-broken': {
    textDecoration: 'line-through',
    textDecorationColor: 'var(--muted, #6d7080)',
    color: 'var(--muted, #6d7080)',
    opacity: '0.55',
    cursor: 'text',
    userSelect: 'none',
  },
  '.cm-lp-wikilink-broken::before': { backgroundColor: 'var(--muted, #6d7080)' },
  '.cm-lp-wikilink-broken:hover': { background: 'transparent', color: 'var(--muted, #6d7080)' },

  // ── Images ──────────────────────────────────────────────────────────────
  '.cm-lp-wikiimage': { display: 'inline-block', verticalAlign: 'middle', maxWidth: '100%', cursor: 'text', position: 'relative' },
  '.cm-lp-image-zoom': {
    position: 'absolute',
    top: '8px',
    right: '8px',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '26px',
    height: '26px',
    padding: '0',
    border: 'none',
    borderRadius: 'var(--r-sm, 6px)',
    background: 'rgba(0,0,0,0.45)',
    color: '#fff',
    cursor: 'zoom-in',
    opacity: '0',
    transition: 'opacity 0.15s ease',
  },
  '.cm-lp-wikiimage:hover .cm-lp-image-zoom': { opacity: '1' },
  '.cm-lp-image-zoom svg': { width: '14px', height: '14px', display: 'block' },
  '.cm-lp-image-broken': {
    display: 'inline-flex',
    alignItems: 'center',
    gap: '0.4em',
    padding: '0.35em 0.7em',
    border: '1px dashed var(--tbl-bd, rgba(24,24,27,0.2))',
    borderRadius: 'var(--r-sm, 6px)',
    color: 'var(--muted, #6d7080)',
    fontSize: 'var(--text-sm, 0.8125rem)',
    lineHeight: '1.5',
  },
  '.cm-lp-image-broken svg': { width: '14px', height: '14px', flexShrink: '0' },
  '.cm-lp-wikiimage img': {
    display: 'block',
    maxWidth: '100%',
    maxHeight: '360px',
    border: '1px solid var(--tbl-bd, rgba(24,24,27,0.12))',
    borderRadius: 'var(--r-sm, 6px)',
  },
  '.cm-lp-wikiimage-raw': { color: 'var(--link, #7c3aed)' },

  // ── Frontmatter header — block:true widget (frontmatter-collapse.js),
  // a direct child of .cm-content like any .cm-line, so it lines up at the
  // same left edge with no extra CSS.
  '.cm-lp-fm-header': {
    display: 'flex',
    alignItems: 'center',
    gap: '0.4em',
    cursor: 'pointer',
    userSelect: 'none',
    padding: '0.2rem 0 0.15rem',
    fontSize: 'var(--text-xs, 0.75rem)',
    fontWeight: '600',
    letterSpacing: '0.07em',
    textTransform: 'uppercase',
    color: 'var(--muted, rgba(60,60,67,0.6))',
  },
  '.cm-lp-fm-header:hover': { color: 'var(--fg, #1a1a1a)' },
  '.cm-lp-fm-header-chevron': {
    display: 'inline-flex',
    transition: 'transform 0.15s ease',
  },
  '.cm-lp-fm-header-chevron svg': { width: '12px', height: '12px' },
  '.cm-lp-fm-header-collapsed .cm-lp-fm-header-chevron': { transform: 'rotate(-90deg)' },
  '.cm-lp-fm-header-edit': {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    marginLeft: 'auto',
    padding: '3px',
    border: 'none',
    background: 'transparent',
    color: 'inherit',
    borderRadius: 'var(--r-sm, 6px)',
    cursor: 'pointer',
    opacity: '1',
    transition: 'opacity 0.15s ease',
  },
  // Stays in the DOM (widgets.js) so the row doesn't reflow when this
  // appears — opacity + pointer-events, not display:none, so it fades and
  // an invisible button can't eat a stray click.
  '.cm-lp-fm-header-edit-hidden': { opacity: '0', pointerEvents: 'none' },
  // The caret can't enter the block any more, so hover is the usual way in.
  '.cm-lp-fm-header:hover .cm-lp-fm-header-edit-hidden': { opacity: '1', pointerEvents: 'auto' },
  '.cm-lp-fm-header-edit:hover': { background: 'var(--icon-hov, rgba(0,0,0,0.06))', color: 'var(--fg, #1a1a1a)' },
  '.cm-lp-fm-header-edit svg': { width: '13px', height: '13px', display: 'block' },

  // ── Frontmatter — no card chrome (no bg, no border) — keys sit flush at
  // the same left edge as regular markdown body text (.cm-content has no
  // left padding), not inset like the old bordered-card design was.
  '.cm-lp-fm-line': {
    display: 'block',
    paddingLeft: '0 !important',
    paddingRight: '0.875rem !important',
    paddingTop: '0.15rem !important',
    paddingBottom: '0.15rem !important',
    fontSize: 'var(--text-base, 0.875rem)',
    lineHeight: '1.6',
  },
  // first/last no longer need extra top/bottom padding — that was clearance
  // for the old bordered card's rounded corners, which are gone.
  '.cm-lp-fm-label': {
    display: 'inline-block',
    fontSize: 'var(--text-2xs, 0.6875rem)',
    verticalAlign: 'middle',
    whiteSpace: 'nowrap', // overflow, don't wrap ":" if ch estimate is short
  },
  // Per-field type icon (text/tag/list/number), inserted before the label —
  // fixed width so it doesn't affect frontmatterLabelWidthCh's ch alignment.
  '.cm-lp-fm-icon': {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '14px',
    marginRight: '0.5em',
    verticalAlign: 'middle',
    color: 'var(--muted, rgba(60,60,67,0.55))',
    flexShrink: '0',
  },
  '.cm-lp-fm-icon svg': { width: '13px', height: '13px', display: 'block' },
  // Block-list items' invisible icon+label stand-in (FrontmatterListIndentWidget)
  // — visibility:hidden keeps the layout box (and hence the width) without
  // painting anything, so list-item text lines up under the value column.
  '.cm-lp-fm-list-indent': { visibility: 'hidden' },
  '.cm-lp-fm-key': {
    fontWeight: '500',
    letterSpacing: '0.07em',
    textTransform: 'uppercase',
    color: 'var(--muted, rgba(60,60,67,0.75))',
  },
  '.cm-lp-fm-colon': { color: 'var(--muted, rgba(60,60,67,0.5))' },
  '.cm-lp-fm-val': { color: 'var(--muted, rgba(60,60,67,0.75))' },
  '.cm-lp-fm-link': {
    color: 'var(--link, #2563eb)',
    textDecoration: 'underline',
    textDecorationColor: 'var(--link-ul, rgba(37,99,235,0.35))',
    textDecorationThickness: '1px',
    textUnderlineOffset: '2px',
    cursor: 'pointer',
  },
  '.cm-lp-fm-link:hover': {
    background: 'var(--tint-soft, rgba(94,106,210,0.06))',
    textDecorationColor: 'var(--link-ul-hov, #4c56c8)',
    borderRadius: '0',
  },
  '.cm-lp-fm-tag': {
    margin: '0.1em 0.25em 0.1em 0',
    padding: '0.05em 0.5em',
    borderRadius: 'var(--r-full, 999px)',
    background: 'var(--cnt-bg, rgba(24,24,27,0.055))',
    color: 'var(--muted, rgba(60,60,67,0.85))',
  },
  // Decorative only (not wired to delete-on-click) — matches the
  // "removable chip" look of Obsidian's Properties panel. Not on
  // .cm-lp-fm-list-item: these rows aren't removable chips, just plain
  // list values, so the "×" read as a stray/broken delete affordance.
  '.cm-lp-fm-tag::after': { content: '"×"', marginLeft: '0.4em', opacity: '0.5' },
  '.cm-lp-fm-more': {
    marginLeft: '0.6em',
    padding: '0.05em 0.5em',
    borderRadius: 'var(--r-full, 999px)',
    background: 'var(--cnt-bg, rgba(24,24,27,0.055))',
    color: 'var(--muted, rgba(60,60,67,0.75))',
    fontSize: '0.92em',
    cursor: 'pointer',
  },
  '.cm-lp-fm-more:hover': { color: 'var(--fg, #1a1a1a)' },
  // On its own row (block-list overflow) it's the first thing on the line,
  // right after the invisible indent spacer — no inline neighbor to space
  // away from, so drop the margin that left-aligned it out from under the
  // value column.
  '.cm-lp-fm-more-own-line': { marginLeft: '0' },
  // Shrink line font so CM6 widgetBuffers collapse with the hairline
  // widget, and kill .cm-lp-fm-line's own !important padding too — that
  // survives font/line-height shrinking otherwise, fine for one hairline
  // but a truncated array/list can stack many of them (every hidden item
  // is its own collapsed line) and the per-row padding adds up into a
  // real gap.
  '.cm-lp-fm-collapsed': {
    fontSize: '1px',
    lineHeight: '1px',
    paddingTop: '0 !important',
    paddingBottom: '0 !important',
  },
  '.cm-lp-fm-collapsed-widget': { display: 'inline-block', height: '0' },

  // ── Tables — a row is one markdown source line; cells are Decoration.mark
  // spans on it, so there's no real <table> to lean on. flex covers the two
  // things a real <table> would give for free:
  //   1. align-items:stretch (the default) equalizes every cell's height to
  //      the tallest one in the row, so a cell that wraps to 2+ lines
  //      doesn't leave its siblings' borders drawn at their own shorter
  //      height (which read as a broken/staggered grid).
  //   2. Per-cell flex-grow + min-width (set inline in decorators.js,
  //      driven by tableColumnWeights/tableColumnMinEm) instead of a flat
  //      width:X% — a width tied purely to "this column's share of every
  //      column's combined weight" lets one long-text column inflate the
  //      total and squeeze a short label column below the width its own
  //      content needs (it'd wrap one character per line despite the row
  //      having plenty of spare width overall). min-width guarantees every
  //      column at least enough room for its own content; only space left
  //      over after that is split by weight.
  '.cm-lp-table-row': { display: 'flex', alignItems: 'stretch' },
  '.cm-lp-table-cell': {
    boxSizing: 'border-box',
    color: 'var(--td-tx, #374151)',
    fontSize: 'var(--text-base, 0.875rem)',
    padding: '0.45rem 0.75rem',
    borderBottom: '1px solid var(--tbl-bd, rgba(24,24,27,0.12))',
    borderRight: '1px solid var(--tbl-bd, rgba(24,24,27,0.12))',
    borderRadius: '0',
  },
  '.cm-lp-table-cell-first': { borderLeft: '1px solid var(--tbl-bd, rgba(24,24,27,0.12))' },
  '.cm-lp-table-row-header .cm-lp-table-cell': {
    background: 'var(--th-bg, rgba(24,24,27,0.04))',
    color: 'var(--th-tx, #9ca3af)',
    fontWeight: '500',
    borderTop: '1px solid var(--tbl-bd, rgba(24,24,27,0.12))',
  },
  '.cm-lp-table-row-header .cm-lp-table-cell-first': { borderTopLeftRadius: 'var(--r-sm, 6px)' },
  '.cm-lp-table-row-header .cm-lp-table-cell-last': { borderTopRightRadius: 'var(--r-sm, 6px)' },
  // Keep last-row bottom border — no outer <table> to close the box.
  '.cm-lp-table-row-last .cm-lp-table-cell-first': { borderBottomLeftRadius: 'var(--r-sm, 6px)' },
  '.cm-lp-table-row-last .cm-lp-table-cell-last': { borderBottomRightRadius: 'var(--r-sm, 6px)' },
  '.cm-lp-table-row-top .cm-lp-table-cell': { borderTop: '1px solid var(--tbl-bd, rgba(24,24,27,0.12))' },
  '.cm-lp-table-row-top .cm-lp-table-cell-first': { borderTopLeftRadius: 'var(--r-sm, 6px)' },
  '.cm-lp-table-row-top .cm-lp-table-cell-last': { borderTopRightRadius: 'var(--r-sm, 6px)' },
  // Row under the caret: plain source, sized like the cells so the switch doesn't jolt.
  '.cm-lp-table-raw': {
    fontSize: 'var(--text-base, 0.875rem)',
    color: 'var(--td-tx, #374151)',
    paddingTop: '0.45rem !important',
    paddingBottom: '0.45rem !important',
  },
  '.cm-lp-table-raw-delim, .cm-lp-table-pipe': { color: 'var(--muted-soft, rgba(60,60,67,0.35))' },
  // Hairline: shrink line font + inline-block widget (not block — splits buffers).
  '.cm-lp-table-delim': { fontSize: '1px', lineHeight: '1px' },
  '.cm-lp-table-delim-widget': { display: 'inline-block', height: '0' },
});

// Code-fence tokens only — omit markdown structural tags (decorators.js owns those).
// Tokyo Night palette via --syn-* (shared_tokens.go): dark on bare :root, Tokyo
// Night Light under [data-theme="light"]. Fallbacks are the light values, per
// this file's var()-fallback convention above.
export const codeHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: 'var(--syn-keyword, #5a4a78)' },
  // No labelName — fence CodeInfo is decorateFencedCode's job.
  { tag: [tags.atom, tags.bool], color: 'var(--syn-const, #965027)' },
  { tag: [tags.literal, tags.inserted], color: 'var(--syn-const, #965027)' },
  // Empty rule: block tags.url inheriting --syn-const from literal (cm-lp-link owns it).
  { tag: tags.url },
  { tag: [tags.string, tags.deleted], color: 'var(--syn-string, #385f0d)' },
  { tag: [tags.regexp, tags.escape, tags.special(tags.string)], color: 'var(--syn-escape, #0f4b6e)' },
  { tag: tags.definition(tags.variableName), color: 'var(--syn-def, #34548a)' },
  { tag: tags.local(tags.variableName), color: 'var(--syn-param, #8f5e15)' },
  { tag: [tags.typeName, tags.namespace], color: 'var(--syn-type, #166775)' },
  { tag: tags.className, color: 'var(--syn-class, #33635c)' },
  { tag: [tags.special(tags.variableName), tags.standard(tags.variableName), tags.macroName], color: 'var(--syn-builtin, #8c4351)' },
  { tag: [tags.propertyName, tags.attributeName], color: 'var(--syn-property, #33635c)' },
  { tag: tags.number, color: 'var(--syn-const, #965027)' },
  { tag: tags.tagName, color: 'var(--syn-keyword, #5a4a78)' },
  { tag: tags.comment, color: 'var(--syn-comment, #848cb1)', fontStyle: 'italic' },
  { tag: tags.invalid, color: 'var(--syn-invalid, #c53b53)' },
]);
