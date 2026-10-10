// "[[" note-name completion. The lookup is injected by the host, which owns the
// vault API; this file only knows the syntax.
import { autocompletion } from '@codemirror/autocomplete';
import { EditorView } from '@codemirror/view';
import { resolveAncestor, nameIs } from './tree-utils.js';

const OPEN_RE = /\[\[[^[\]|#\n]*$/;

// Replaces the rest of an existing target too, so completing inside
// "[[fo|o]]" or "[[foo#h|alias]]" doesn't leave the old tail behind.
export function applyTarget(value) {
  return (view, completion, from, to) => {
    const after = view.state.sliceDoc(to, Math.min(view.state.doc.length, to + 256));
    const rest = /^[^[\]|#\n]*/.exec(after)[0];
    const tail = after.slice(rest.length);
    let end = to;
    let insert = value + ']]';
    let caret = from + insert.length;
    if (tail.startsWith(']]') || tail.startsWith('|') || tail.startsWith('#')) {
      end = to + rest.length;
      insert = value;
      caret = from + value.length + (tail.startsWith(']]') ? 2 : 1);
    }
    view.dispatch({ changes: { from, to: end, insert }, selection: { anchor: caret }, userEvent: 'input.complete' });
  };
}

function source(search) {
  return async (ctx) => {
    const m = ctx.matchBefore(OPEN_RE);
    if (!m) return null;
    // "![[" embeds an image, not a note.
    if (m.from > 0 && ctx.state.sliceDoc(m.from - 1, m.from) === '!') return null;
    if (resolveAncestor(ctx.state, ctx.pos, nameIs(['InlineCode', 'FencedCode', 'CodeBlock']), -1)) return null;
    const abort = new AbortController();
    ctx.addEventListener('abort', () => abort.abort(), { onDocChange: true });
    let items;
    try {
      items = await search(m.text.slice(2), abort.signal);
    } catch {
      return null;
    }
    if (ctx.aborted || !items || !items.length) return null;
    return {
      from: m.from + 2,
      // Ranking comes from the server; client-side fuzzy filtering would fight it.
      filter: false,
      options: items.map((it) => ({ label: it.label, detail: it.detail, apply: applyTarget(it.value || it.label) })),
    };
  };
}

const completionTheme = EditorView.theme({
  '.cm-tooltip.cm-tooltip-autocomplete': {
    padding: '0.25rem',
    background: 'var(--surface-soft, #ffffff)',
    border: 'var(--bd-w, 1px) solid var(--line-edge, rgba(24,24,27,0.16))',
    borderRadius: 'var(--r-md, 10px)',
    boxShadow: 'var(--shadow-sm, 0 4px 16px) var(--shadow-color, rgba(0,0,0,0.16))',
    fontFamily: 'var(--font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif)',
  },
  '.cm-tooltip.cm-tooltip-autocomplete > ul': { maxHeight: '16rem', minWidth: '14rem', fontFamily: 'inherit' },
  '.cm-tooltip.cm-tooltip-autocomplete > ul > li': {
    padding: '0.35rem 0.55rem',
    borderRadius: 'var(--r-sm, 6px)',
    color: 'var(--fg, #1a1a1a)',
    fontSize: 'var(--text-sm, 0.8125rem)',
    lineHeight: '1.4',
  },
  '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]': {
    background: 'var(--tint-soft, rgba(94,106,210,0.1))',
    color: 'var(--accent-text, #5e6ad2)',
  },
  '.cm-completionDetail': { marginLeft: '0.75em', fontStyle: 'normal', color: 'var(--muted, #6d7080)' },
});

/**
 * @param {(query: string, signal: AbortSignal) => Promise<Array<{label: string, value?: string, detail?: string}>>} search
 *   `value` is what goes between the brackets; defaults to `label`.
 */
export function wikiLinkCompletion(search) {
  return [autocompletion({ override: [source(search)], icons: false }), completionTheme];
}
