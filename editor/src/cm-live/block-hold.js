// A block drawn as a widget (fence card, frontmatter card) turns into a few
// lines of source while it's being edited. A spacer under the source holds the
// widget's measured height, so the text below stays put both ways; it shrinks
// to nothing once the source outgrows the widget.
import { Facet } from '@codemirror/state';
import { ViewPlugin, WidgetType, Decoration } from '@codemirror/view';

// Each source: (state) => [{ from, to, key, editing, estimate }], whole lines.
// `key` names what the widget shows; `estimate` stands in before it's measured.
export const heldBlocks = Facet.define();

class SpacerWidget extends WidgetType {
  eq() {
    return true;
  }

  toDOM() {
    const el = document.createElement('div');
    el.className = 'cm-block-spacer';
    return el;
  }

  ignoreEvent() {
    return false;
  }
}

const spacer = Decoration.widget({ widget: new SpacerWidget(), block: true, side: 1 });

// Goes in the block field's own decorations, right after the source being edited.
export function blockSpacer(pos) {
  return spacer.range(pos);
}

export const blockHold = ViewPlugin.fromClass(class {
  constructor(view) {
    this.heights = new Map();
    this.editing = null; // { from, target }
    this.update({ view, state: view.state, changes: null });
  }

  update(u) {
    const view = u.view;
    if (this.editing && u.changes) this.editing.from = u.changes.mapPos(this.editing.from);
    let active = null;
    for (const source of u.state.facet(heldBlocks)) {
      for (const b of source(u.state)) {
        if (b.editing) {
          active = b;
          continue;
        }
        const block = view.lineBlockAt(b.from);
        if (block.from === b.from && block.to === b.to && block.height > 0) this.heights.set(b.key, block.height);
      }
    }
    if (!active) {
      this.editing = null;
      return;
    }
    // Fixed when editing starts: typing changes the key, not the widget's slot.
    if (!this.editing || this.editing.from !== active.from) {
      this.editing = { from: active.from, target: this.heights.get(active.key) ?? active.estimate };
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
        // The spacer is the block widget placed right after the source's last line.
        const spacer = last.nextElementSibling;
        if (!spacer || !spacer.classList.contains('cm-block-spacer')) return null;
        return { spacer, source: last.getBoundingClientRect().bottom - first.getBoundingClientRect().top };
      },
      write: (m) => {
        if (m) m.spacer.style.height = Math.max(0, target - m.source) + 'px';
      },
    });
  }
});
