// Block widgets for registered fence cards. A StateField because CM6 refuses block decorations
// from a ViewPlugin (see live-preview.js). While a selection's head is inside, the source stays
// plain and a large card's preview lives in the floating pip.js panel instead of the document flow.
import { StateField } from '@codemirror/state';
import { Decoration, EditorView, WidgetType } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { headInBlock, revealToggled, selectionMatters } from '../selection.js';
import { cardEntry, cardFence } from './registry.js';
import { LEAF_BLOCKS } from '../tree-utils.js';
import { mountCard, returnEditorFocus } from './host.js';
import { cardPip } from './pip.js';
import { heldBlocks, blockHold, blockSpacer } from '../block-hold.js';

// A card scrolled out of the viewport loses its widget; kept rendered here, it
// comes back as it was instead of remounting blank and redrawing.
const parked = new Map();
const PARK_MAX = 6;
let holding = 0;
const passed = [];

// A replace↔widget swap with an eq() widget reuses the DOM yet still destroys the
// old instance, so a parked DOM still in the page is in use: never handed out or destroyed.
function park(key, dom) {
  const prev = parked.get(key);
  parked.delete(key);
  // Two cards with the same source share a key; the one pushed out is gone for good.
  if (prev && prev !== dom) drop(prev);
  parked.set(key, dom);
  if (!holding) trim();
}

function drop(dom) {
  if (dom.isConnected || !dom._card) return;
  dom._card.destroy();
  dom._card = null;
}

function trim() {
  while (parked.size > PARK_MAX) {
    const [oldest, old] = parked.entries().next().value;
    parked.delete(oldest);
    drop(old);
  }
}

function unpark(key) {
  const dom = parked.get(key);
  if (!dom) return null;
  parked.delete(key);
  return dom.isConnected ? null : dom;
}

// Runs fn with every card it scrolls away kept, so the cards in view come back
// as they were however many it passes (scroll.js measuring the note). Cards it
// only passes, never mounted, are dropped rather than mounted detached.
export function keepingParkedCards(fn) {
  holding++;
  try {
    return fn();
  } finally {
    if (!--holding) {
      for (const dom of passed.splice(0)) drop(dom);
      trim();
    }
  }
}

class CardWidget extends WidgetType {
  constructor(info, source) {
    super();
    this.info = info;
    this.source = source;
  }

  eq(other) {
    return other.info === this.info && other.source === this.source;
  }

  // External edits (undo, sync) feed the live renderer instead of remounting it,
  // which would drop its view state and flash an empty slot for a frame.
  updateDOM(dom, view, prev) {
    if (!dom._card || !prev || cardEntry(prev.info) !== cardEntry(this.info)) return false;
    dom._card.update(this.source);
    return true;
  }

  get estimatedHeight() {
    return cardEntry(this.info).minHeight + 20;
  }

  get key() {
    return this.info + '\n' + this.source;
  }

  toDOM(view) {
    const kept = unpark(this.key);
    if (kept) return kept;
    const wrap = document.createElement('div');
    wrap.className = 'cm-card-widget';
    // A press would drop the DOM selection inside the card, and CM's next DOM flush reads it as
    // the caret entering the fence. Capture, since renderers stop propagation. Only Edit opens the source.
    wrap.addEventListener('mousedown', (e) => e.preventDefault(), true);
    const h = mountCard(this.info, this.source, {
      onEdit: () => {
        view.dispatch({ selection: { anchor: editPos(view.state.doc, view.posAtDOM(wrap)) }, scrollIntoView: true });
        view.focus();
      },
      returnFocus: (el) => returnEditorFocus(view, el),
    });
    wrap._card = h;
    wrap.appendChild(h.el);
    return wrap;
  }

  destroy(dom) {
    if (!dom._card) return;
    if (holding && !dom._card.mounted) passed.push(dom);
    else park(this.key, dom);
  }

  ignoreEvent() {
    return true;
  }
}

// The end of the body's first line, where the content starts; the opener line when the body is empty.
function editPos(doc, fenceStart) {
  const open = doc.lineAt(fenceStart);
  if (open.number >= doc.lines) return open.to;
  const next = doc.line(open.number + 1);
  return /^\s*(`{3,}|~{3,})\s*$/.test(next.text) ? open.to : next.to;
}

function collect(state) {
  const found = [];
  syntaxTree(state).iterate({
    enter(node) {
      if (node.name === 'FencedCode') {
        const f = cardFence(state.doc, node.node);
        if (f) found.push(f);
        return false;
      }
      if (LEAF_BLOCKS.has(node.name)) return false;
    },
  });
  return found;
}

function build(state, found) {
  const ranges = [];
  for (const f of found) {
    if (headInBlock(state, f.from, f.to)) {
      ranges.push(blockSpacer(f.to));
      continue;
    }
    const widget = new CardWidget(f.info, f.body);
    ranges.push(Decoration.replace({ widget, block: true }).range(f.from, f.to));
  }
  return Decoration.set(ranges, true);
}

export const cardState = StateField.define({
  create(state) {
    const found = collect(state);
    return { found, decorations: build(state, found), tree: syntaxTree(state) };
  },
  update(value, tr) {
    const tree = syntaxTree(tr.state);
    const treeChanged = tree !== value.tree;
    if (!tr.docChanged && !selectionMatters(tr) && !treeChanged && !revealToggled(tr.startState, tr.state)) return value;
    const found = tr.docChanged || treeChanged ? collect(tr.state) : value.found;
    return { found, decorations: build(tr.state, found), tree };
  },
  provide: (f) => EditorView.decorations.from(f, (v) => v.decorations),
});

// While the caret is inside a card's source, block-hold.js keeps the card's height.
const heldCards = heldBlocks.of((state) => state.field(cardState).found.map((f) => ({
  from: f.from,
  to: f.to,
  key: f.info + '\n' + f.body,
  editing: headInBlock(state, f.from, f.to),
  estimate: cardEntry(f.info).minHeight + 20,
})));

export function fenceCardField() {
  return [cardState, heldCards, blockHold, cardPip(cardState)];
}
