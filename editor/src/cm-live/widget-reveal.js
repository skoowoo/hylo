import { StateField, StateEffect, Prec } from '@codemirror/state';
import { keymap, BlockType } from '@codemirror/view';
import { resolveAncestor } from './tree-utils.js';
import { livePreviewPlugin } from './live-preview.js';

// Backspace right after a wikilink widget would delete its hidden "]".
// The first press shows the source instead; the next one edits it.
// Images are atomic and delete as one unit.

const WIDGET_NODES = new Set(['WikiLink']);

export const revealWidget = StateEffect.define();

export const revealedWidgetField = StateField.define({
  create: () => null,
  update(value, tr) {
    for (const e of tr.effects) if (e.is(revealWidget)) return e.value;
    return tr.docChanged || tr.selection ? null : value;
  },
});

export function isWidgetRevealed(state, from, to) {
  const r = state.field(revealedWidgetField, false);
  return !!r && r.from === from && r.to === to;
}

function renderedAsWidget(view, from, to) {
  const plugin = view.plugin(livePreviewPlugin);
  if (!plugin) return false;
  let found = false;
  plugin.decorations.between(from, to, (f, t, deco) => {
    if (f === from && t === to && deco.spec.widget) found = true;
  });
  return found;
}

export function revealWidgetBeforeCaret(view) {
  const { state } = view;
  if (state.readOnly || state.selection.ranges.length > 1) return false;
  const sel = state.selection.main;
  if (!sel.empty) return false;
  const node = resolveAncestor(state, sel.head, (n) => WIDGET_NODES.has(n.name) && n.to === sel.head, -1);
  if (!node || isWidgetRevealed(state, node.from, node.to) || !renderedAsWidget(view, node.from, node.to)) return false;
  view.dispatch({ effects: revealWidget.of({ from: node.from, to: node.to }) });
  return true;
}

// Backspace at the start of the line below a rendered block — a card (fence card,
// frontmatter) or a rule — would join it onto the closing fence or the "---" and break
// the block. Step onto it instead, which shows its source; frontmatter's caret filter
// keeps the caret out, so there the press does nothing.
export function revealBlockBeforeCaret(view) {
  const { state } = view;
  if (state.readOnly || state.selection.ranges.length > 1) return false;
  const sel = state.selection.main;
  if (!sel.empty || sel.head === 0 || state.doc.lineAt(sel.head).from !== sel.head) return false;
  const above = sel.head - 1;
  const block = view.lineBlockAt(above);
  const card = block.type === BlockType.WidgetRange && block.to === above;
  if (!card && !resolveAncestor(state, above, (n) => n.name === 'HorizontalRule', -1)) return false;
  view.dispatch({ selection: { anchor: above }, scrollIntoView: true });
  return true;
}

export const widgetReveal = [
  revealedWidgetField,
  Prec.high(keymap.of([{ key: 'Backspace', run: (view) => revealBlockBeforeCaret(view) || revealWidgetBeforeCaret(view) }])),
];
