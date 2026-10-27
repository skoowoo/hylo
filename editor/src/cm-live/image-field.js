// ![alt](url) / ![[file]] as picture widgets. A StateField, not part of the
// live-preview plugin: CM builds its height map from state decorations only,
// so this is how an image line it hasn't drawn gets the picture's height
// (ImageWidget.estimatedHeight) instead of one line of text. Atomic, so
// selection and Backspace treat the picture as one object; a caret inside
// (double-click) shows the source, which decorators.js colors.
import { StateField, StateEffect } from '@codemirror/state';
import { Decoration, EditorView, ViewPlugin } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { selectionInsideRange, selectionCoversRange, readingModeToggled } from './selection.js';
import { imageSpan } from './image-nodes.js';
import { wikiNodeInner } from './wiki-syntax.js';
import { ImageWidget } from './widgets.js';
import { imageSize, probeImageSizes, onImageSize, setColumnWidth } from './image-sizes.js';

// Blocks whose text is never markdown, so never an image.
const VERBATIM = new Set(['FencedCode', 'CodeBlock', 'HTMLBlock', 'CommentBlock', 'Frontmatter', 'LinkReference']);

// Spans the wrapping link too, so a caret in its URL keeps the source shown.
export function imageEditing(state, node) {
  const span = imageSpan(node);
  return selectionInsideRange(state, span.from, span.to);
}

// [{ node, from, to, src, label, editOffset }]; from/to cover what the picture replaces.
function collect(state, options) {
  const doc = state.doc;
  const out = [];
  syntaxTree(state).iterate({
    enter(n) {
      if (VERBATIM.has(n.name)) return false;
      if (n.name === 'Image') {
        const marks = n.node.getChildren('LinkMark');
        const url = n.node.getChild('URL');
        if (marks.length < 2 || !url) return false;
        const href = doc.sliceString(url.from, url.to);
        const alt = doc.sliceString(marks[0].to, marks[1].from);
        // Relative paths name files in the vault, not URLs on the app's own page.
        const src = options.resolveMarkdownImageSrc ? options.resolveMarkdownImageSrc(href) : href;
        out.push({ node: n.node, ...imageSpan(n.node), src, label: alt || href, editOffset: 2 });
        return false;
      }
      if (n.name === 'WikiImage') {
        const name = wikiNodeInner(n.node, doc);
        if (name) out.push({ node: n.node, ...imageSpan(n.node), src: options.resolveImageSrc ? options.resolveImageSrc(name) : name, label: name, editOffset: 3 });
        return false;
      }
    },
  });
  return out;
}

function build(state, images) {
  const ranges = [];
  for (const img of images) {
    if (imageEditing(state, img.node)) continue;
    const widget = new ImageWidget(img.src, img.label, img.editOffset, selectionCoversRange(state, img.from, img.to), imageSize(img.src));
    ranges.push(Decoration.replace({ widget }).range(img.from, img.to));
  }
  return Decoration.set(ranges, true);
}

const sizesLearned = StateEffect.define();

// Captures what collect() resolves: the whole inner text, as wikiNodeInner gives it.
const WIKI_IMAGE_RE = /!\[\[([^\]\n]+)\]\]/g;
const MD_IMAGE_RE = /!\[[^\]\n]*\]\(<?([^)\s>]+)/g;

// CM keeps a line's first height estimate until the line is drawn, so a size
// learned after the note's state is built only lands when it scrolls into view.
// The host awaits this (capped) before building the state; sizes persist, so
// it costs each image once.
export function preloadImageSizes(text, options = {}) {
  const srcs = [];
  for (const m of text.matchAll(WIKI_IMAGE_RE)) srcs.push(options.resolveImageSrc ? options.resolveImageSrc(m[1]) : m[1]);
  for (const m of text.matchAll(MD_IMAGE_RE)) srcs.push(options.resolveMarkdownImageSrc ? options.resolveMarkdownImageSrc(m[1]) : m[1]);
  return probeImageSizes(srcs);
}

export function imageField(options = {}) {
  const field = StateField.define({
    create(state) {
      const images = collect(state, options);
      return { images, decorations: build(state, images), tree: syntaxTree(state) };
    },
    update(value, tr) {
      const tree = syntaxTree(tr.state);
      const rescan = tr.docChanged || tree !== value.tree;
      const learned = tr.effects.some((e) => e.is(sizesLearned));
      if (!rescan && !tr.selection && !learned && !readingModeToggled(tr.startState, tr.state)) return value;
      const images = rescan ? collect(tr.state, options) : value.images;
      return { images, decorations: build(tr.state, images), tree };
    },
    provide: (f) => [
      EditorView.decorations.from(f, (v) => v.decorations),
      EditorView.atomicRanges.of((view) => view.state.field(f).decorations),
    ],
  });

  // Learns the sizes of the note's images, nearest the viewport first, and
  // rebuilds once per frame as they come in; tracks the column they fit.
  const sizer = ViewPlugin.fromClass(class {
    constructor(view) {
      this.view = view;
      this.raf = 0;
      this.probed = null;
      this.off = onImageSize(() => this.learned());
      this.probe();
      this.measureColumn();
    }

    update(u) {
      if (u.state.field(field).images !== u.startState.field(field).images) this.probe();
      if (u.geometryChanged) this.measureColumn();
    }

    // A rescan (every edit) rarely changes which images there are.
    probe() {
      const images = this.view.state.field(field).images;
      const key = images.map((img) => img.src).join('\n');
      if (key === this.probed) return;
      this.probed = key;
      const at = this.view.viewport.from;
      const nearest = [...images].sort((a, b) => Math.abs(a.from - at) - Math.abs(b.from - at));
      probeImageSizes(nearest.map((img) => img.src));
    }

    measureColumn() {
      this.view.requestMeasure({ key: this, read: (view) => view.contentDOM.clientWidth, write: setColumnWidth });
    }

    learned() {
      if (this.raf) return;
      this.raf = requestAnimationFrame(() => {
        this.raf = 0;
        this.view.dispatch({ effects: sizesLearned.of(null) });
      });
    }

    destroy() {
      this.off();
      cancelAnimationFrame(this.raf);
    }
  });

  return [field, sizer];
}
