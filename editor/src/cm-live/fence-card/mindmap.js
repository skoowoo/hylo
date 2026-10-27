import { HyloMarkmap } from './mindmap-view.js';
import { zoomIdentity, zoomTransform } from 'd3-zoom';
import { parseOutline } from './mindmap-tree.js';

// Widgets are rebuilt when scrolled out of view; folds and pan/zoom survive by source text.
const viewCache = new Map();
const CACHE_MAX = 100;
// Only user-driven changes (fold, fit) animate; initial draw, edits, resize and theme swaps snap.
const ANIM_MS = 140;

function remember(source, state) {
  viewCache.delete(source);
  viewCache.set(source, state);
  if (viewCache.size > CACHE_MAX) viewCache.delete(viewCache.keys().next().value);
}

// Fold keys follow the text path, not the index path, so inserting a sibling doesn't
// move a fold onto its neighbour. Duplicate sibling texts are told apart by occurrence.
function annotate(node, folds, key = '') {
  node.payload = { k: key, fold: folds.has(key) ? 1 : 0 };
  const seen = new Map();
  node.children.forEach((c) => {
    const n = seen.get(c.content) || 0;
    seen.set(c.content, n + 1);
    annotate(c, folds, `${key}/${c.content}#${n}`);
  });
  return node;
}

function collectFolds(data, out = new Set()) {
  if (!data) return out;
  if (data.payload && data.payload.fold) out.add(data.payload.k);
  (data.children || []).forEach((c) => collectFolds(c, out));
  return out;
}

export function mount(el, source, ctx = {}) {
  const mode = ctx.mode || 'card';
  const persist = mode !== 'full';
  // Pan/zoom is per-viewport: the pip's would misplace the full-width card, so it only carries it along.
  const ownsView = mode === 'card';
  const onError = ctx.onError || ((e) => console.error(e));
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  el.replaceChildren(svg);

  const cached = persist ? viewCache.get(source) : null;
  let folds = cached ? cached.folds : new Set();
  let transform = cached ? cached.transform : null;
  let touched = ownsView && !!transform;
  let current = source;

  const mm = HyloMarkmap.create(svg, {
    lineWidth: () => 1.5,
    pan: false,
    duration: ANIM_MS,
    maxWidth: 260,
    paddingX: 8,
    spacingVertical: 8,
    spacingHorizontal: 40,
    initialExpandLevel: -1,
    maxInitialScale: 1,
  });
  // Plain wheel must keep scrolling the note; only ⌘/Ctrl+wheel zooms.
  mm.zoom.filter((e) => (e.type === 'wheel' ? e.ctrlKey || e.metaKey : !e.button));

  // Counted so overlapping snaps don't restore the animation under each other.
  let snapping = 0;
  const instant = async (fn) => {
    snapping++;
    mm.options.duration = 0;
    try {
      return await fn();
    } finally {
      if (--snapping === 0) mm.options.duration = ANIM_MS;
    }
  };

  const save = () => {
    if (!persist) return;
    folds = collectFolds(mm.state.data);
    if (ownsView) {
      const t = zoomTransform(svg);
      transform = touched ? { x: t.x, y: t.y, k: t.k } : null;
    }
    remember(current, { folds, transform });
  };

  mm.zoom.on('zoom.keep', (e) => {
    if (!e.sourceEvent) return;
    touched = true;
    save();
  });

  const origToggle = mm.toggleNode.bind(mm);
  mm.toggleNode = async (...args) => {
    await origToggle(...args);
    save();
  };

  const place = () => {
    if (ownsView && transform) {
      mm.svg.call(mm.zoom.transform, zoomIdentity.translate(transform.x, transform.y).scale(transform.k));
    } else {
      return instant(() => mm.fit());
    }
  };

  const render = (refit) => instant(() => mm.setData(annotate(parseOutline(current), folds)))
    .then(() => refit && place())
    .catch(onError);
  render(true);

  // The initial render already fits; refit only when the box actually changes size.
  let size = `${el.clientWidth}x${el.clientHeight}`;
  const ro = new ResizeObserver(() => {
    // Detached while parked (card-field.js), it measures 0×0; nothing to fit.
    if (!el.clientWidth) return;
    const next = `${el.clientWidth}x${el.clientHeight}`;
    if (next === size) return;
    size = next;
    if (!touched) instant(() => mm.fit());
  });
  ro.observe(el);

  return {
    update(next) {
      if (next === current) return;
      // Intermediate sources from typing would otherwise pile up in the cache.
      if (persist) viewCache.delete(current);
      current = next;
      save();
      // An untouched view follows the content, so a map typed line by line ends up fitted.
      return render(!touched);
    },
    fit() {
      touched = false;
      save();
      return mm.fit();
    },
    zoom(factor) {
      touched = true;
      return mm.rescale(factor).then(save);
    },
    destroy() {
      ro.disconnect();
      mm.destroy();
      el.replaceChildren();
    },
  };
}
