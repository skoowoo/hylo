// Viewport-scoped tree walk → decorators.js.
import { ViewPlugin, Decoration } from '@codemirror/view';
import { StateEffect, EditorState } from '@codemirror/state';
import { syntaxTree } from '@codemirror/language';
import { nodeDecorators, isHairlineLine } from './decorators.js';
import { drawnRanges } from './decoration-helpers.js';
import { readingModeToggled } from './selection.js';
import { revealedWidgetField } from './widget-reveal.js';
import { suppressedLinkRevealField } from './link-reveal-suppression.js';

// Dispatched by app wiring (content_pane.js) once it has (re)computed which
// [[wikilink]] targets exist — e.g. after the async /api/notes/exist batch
// resolves, or after a vault delete elsewhere touches this note's links —
// so the next rebuild re-reads options.isWikiLinkBroken even though nothing
// in the document itself changed.
export const wikiLinksRevalidated = StateEffect.define();

// State besides the selection that decides what shows raw. Compared by value,
// so a change rebuilds however it was dispatched.
const REVEAL_FIELDS = [revealedWidgetField, suppressedLinkRevealField];

function revealChanged(a, b) {
  return readingModeToggled(a, b) || REVEAL_FIELDS.some((f) => a.field(f, false) !== b.field(f, false));
}

class LivePreviewPlugin {
  constructor(view, options) {
    this.options = options || {};
    this.decorations = Decoration.none;
    // Per-node layout (table columns, list marker widths, link definitions).
    // Survives selection/viewport rebuilds; cleared when doc or tree changes.
    this.layoutCache = new Map();
    // Tree used for the last rebuild — see the `treeChanged` check below.
    this.tree = syntaxTree(view.state);
    this.rebuild(view);
  }

  update(update) {
    // @codemirror/language parses a fresh EditorState only up to
    // Work.InitViewport (3000 chars) synchronously; the rest lands later via
    // parseWorker's background/idle-time work, landed through a *separate*
    // dispatch (Language.setState) that has docChanged: false and
    // viewportChanged: false — so on a document longer than that, the first
    // rebuild() above decorates an incomplete tree and, without this check,
    // nothing ever asks for a second look: decorations past ~3000 chars
    // stay raw forever, not just for one frame. treeChanged is exactly how
    // CM6's own built-in TreeHighlighter (syntax-highlighting) stays correct
    // through the same background-parse handoff — same fix, same reason.
    const tree = syntaxTree(update.state);
    const treeChanged = tree != this.tree;
    // A grown tree can add rows/definitions the cache was built without.
    if (update.docChanged || treeChanged) this.layoutCache.clear();
    const revalidated = update.transactions.some((tr) => tr.effects.some((e) => e.is(wikiLinksRevalidated)));
    if (treeChanged || update.docChanged || update.viewportChanged || update.selectionSet || revalidated || revealChanged(update.startState, update.state)) {
      this.tree = tree;
      this.rebuild(update.view);
    }
  }

  rebuild(view) {
    const decos = [];
    const tree = syntaxTree(view.state);
    const seenMarks = new Set();
    const enter = (node) => {
      const decorate = nodeDecorators[node.name];
      if (!decorate) return;
      if (node.name === 'ListMark') {
        if (seenMarks.has(node.from)) return;
        seenMarks.add(node.from);
      }
      return decorate(node, view, decos, this.options, this.layoutCache);
    };
    for (const { from, to } of drawnRanges(view)) {
      // Continuation lines take their indent from the item's marker, which may be above the viewport.
      for (let n = tree.resolveInner(from, 1); n; n = n.parent) {
        const mark = n.name === 'ListItem' && n.getChild('ListMark');
        if (mark && mark.from < from) enter(mark);
      }
      tree.iterate({ from, to, enter });
    }
    this.decorations = Decoration.set(decos, true);
  }
}

export const livePreviewPlugin = ViewPlugin.fromClass(LivePreviewPlugin, {
  decorations: (v) => v.decorations,
});

// A hairline row (isHairlineLine) can't show a caret, and revealing its source
// would move every line below. Carry the caret on in the direction it moved:
// a one-character step crossed the line break, a longer one keeps its column.
export const skipHairlines = EditorState.transactionFilter.of((tr) => {
  if (!tr.selection || tr.docChanged) return tr;
  const sel = tr.newSelection.main;
  if (!sel.empty) return tr;
  const state = tr.startState;
  const prev = state.selection.main.head;
  const line = state.doc.lineAt(sel.head);
  if (line.number === state.doc.lineAt(prev).number || !isHairlineLine(state, line.number)) return tr;
  const forward = sel.head >= prev;
  const n = line.number + (forward ? 1 : -1);
  if (n < 1 || n > state.doc.lines) return tr;
  const target = state.doc.line(n);
  const anchor = Math.abs(sel.head - prev) <= 1
    ? (forward ? target.from : target.to)
    : Math.min(target.from + (prev - state.doc.lineAt(prev).from), target.to);
  return [tr, { selection: { anchor }, sequential: true }];
});
