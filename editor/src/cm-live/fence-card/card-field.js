// Block widgets for registered fence cards. A StateField because CM6 refuses block decorations
// from a ViewPlugin (see live-preview.js). While the caret is inside, the source stays plain
// and the preview lives in the floating pip.js panel instead of the document flow.
import { StateField } from '@codemirror/state';
import { Decoration, EditorView, ViewPlugin, WidgetType } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { selectionTouchesRange, readingModeToggled } from '../selection.js';
import { cardEntry, cardFence } from './registry.js';
import { NEVER_CONTAINS_HR } from '../horizontal-rule-field.js';
import { mountCard } from './host.js';
import { cardPip } from './pip.js';

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

  toDOM(view) {
    const wrap = document.createElement('div');
    wrap.className = 'cm-card-widget';
    const h = mountCard(this.info, this.source, {
      onEdit: () => {
        const pos = view.posAtDOM(wrap);
        view.dispatch({ selection: { anchor: pos }, scrollIntoView: true });
        view.focus();
      },
    });
    wrap._card = h;
    wrap.appendChild(h.el);
    return wrap;
  }

  // A replace↔widget swap with an eq() widget reuses the DOM yet still destroys the old
  // instance, so teardown waits to see whether the DOM survived the update.
  destroy(dom) {
    queueMicrotask(() => {
      if (dom.isConnected || !dom._card) return;
      dom._card.destroy();
      dom._card = null;
    });
  }

  ignoreEvent() {
    return true;
  }
}

// Holds the card's height under the source while it's being edited, so the
// text below doesn't jump when the card turns into a few lines of source
// (or back). Sized by cardSpacer below; shrinks to nothing once the source
// outgrows the card.
class CardSpacerWidget extends WidgetType {
  eq() {
    return true;
  }

  toDOM() {
    const el = document.createElement('div');
    el.className = 'cm-card-spacer';
    return el;
  }

  ignoreEvent() {
    return false;
  }
}

// Fences sit where the HR walk would stop descending, except FencedCode itself.
function collect(state) {
  const found = [];
  syntaxTree(state).iterate({
    enter(node) {
      if (node.name === 'FencedCode') {
        const f = cardFence(state.doc, node.node);
        if (f) found.push(f);
        return false;
      }
      if (NEVER_CONTAINS_HR.has(node.name)) return false;
    },
  });
  return found;
}

function build(state, found) {
  const ranges = [];
  for (const f of found) {
    if (selectionTouchesRange(state, f.from, f.to)) {
      ranges.push(Decoration.widget({ widget: new CardSpacerWidget(), block: true, side: 1 }).range(f.to));
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
    if (!tr.docChanged && !tr.selection && !treeChanged && !readingModeToggled(tr.startState, tr.state)) return value;
    const found = tr.docChanged || treeChanged ? collect(tr.state) : value.found;
    return { found, decorations: build(tr.state, found), tree };
  },
  provide: (f) => EditorView.decorations.from(f, (v) => v.decorations),
});

const cardKey = (f) => f.info + '\n' + f.body;

// Remembers each card's rendered height (from the height map, so it's the
// measured one) and, while a card is being edited, sets the spacer to that
// height minus the source's.
const cardSpacer = ViewPlugin.fromClass(class {
  constructor(view) {
    this.heights = new Map();
    this.editing = null; // { from, target }
    this.update({ view, state: view.state, changes: null });
  }

  update(u) {
    const view = u.view;
    const { found } = u.state.field(cardState);
    if (this.editing && u.changes) this.editing.from = u.changes.mapPos(this.editing.from);
    let active = null;
    for (const f of found) {
      if (selectionTouchesRange(u.state, f.from, f.to)) {
        active = f;
        continue;
      }
      const block = view.lineBlockAt(f.from);
      if (block.from === f.from && block.to === f.to && block.height > 0) this.heights.set(cardKey(f), block.height);
    }
    if (!active) {
      this.editing = null;
      return;
    }
    // The target is fixed when editing starts; typing changes the key but not the card's slot.
    if (!this.editing || this.editing.from !== active.from) {
      const known = this.heights.get(cardKey(active));
      this.editing = { from: active.from, target: known ?? cardEntry(active.info).minHeight + 20 };
    }
    const { target } = this.editing;
    view.requestMeasure({
      key: this,
      read: () => {
        const lineEl = (pos) => {
          const { node } = view.domAtPos(pos);
          return (node.nodeType === 1 ? node : node.parentElement).closest('.cm-line');
        };
        const first = lineEl(active.from);
        const last = lineEl(active.to);
        if (!first || !last) return null;
        return last.getBoundingClientRect().bottom - first.getBoundingClientRect().top;
      },
      write: (sourceHeight) => {
        const spacer = view.contentDOM.querySelector('.cm-card-spacer');
        if (spacer && sourceHeight != null) spacer.style.height = Math.max(0, Math.round(target - sourceHeight)) + 'px';
      },
    });
  }
});

export function fenceCardField() {
  return [cardState, cardSpacer, cardPip(cardState)];
}
