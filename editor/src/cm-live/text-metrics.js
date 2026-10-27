// CM estimates the height of every unrendered line from a text sample: the
// first short plain line it has rendered, else a fixed probe line of its own.
// In a proportional face each short line gives a different char width, and a
// changed sample re-estimates the whole height map, so scrolling onto a new
// short line moved everything off-screen (and the scrollbar with it). A mark
// on every short line keeps CM on its probe, so the estimate never changes.
import { ViewPlugin, Decoration } from '@codemirror/view';
import { Prec } from '@codemirror/state';
import { drawnRanges } from './decoration-helpers.js';

const SAMPLE_MAX = 20;
const guard = Decoration.mark({ class: 'cm-sample-guard' });

function build(view) {
  const ranges = [];
  const { doc } = view.state;
  for (const { from, to } of drawnRanges(view)) {
    for (let pos = from; pos <= to;) {
      const line = doc.lineAt(pos);
      if (line.length > 0 && line.length <= SAMPLE_MAX) ranges.push(guard.range(line.from, line.to));
      pos = line.to + 1;
    }
  }
  return Decoration.set(ranges, true);
}

// Highest precedence nests the mark inside every other one, so it never wraps
// a table row's cells (flex items) or a hung marker's anchor.
export const stableTextMetrics = Prec.highest(ViewPlugin.fromClass(class {
  constructor(view) {
    this.decorations = build(view);
    this.stale = false;
  }

  update(u) {
    // No new wrapper may close around an IME composition (WebKit drops or
    // misplaces the marked text); the marks only follow the text until it ends.
    if (u.view.compositionStarted) {
      this.decorations = this.decorations.map(u.changes);
      this.stale = true;
    } else if (this.stale || u.docChanged || u.viewportChanged || u.selectionSet) {
      this.decorations = build(u.view);
      this.stale = false;
    }
  }
}, { decorations: (v) => v.decorations }));
