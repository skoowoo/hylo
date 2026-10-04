// Viewport-scoped tree walk → decorators.js. Publishes decorations + atomicRanges.
import { ViewPlugin, Decoration, EditorView } from '@codemirror/view';
import { StateEffect } from '@codemirror/state';
import { syntaxTree } from '@codemirror/language';
import { nodeDecorators } from './decorators.js';
import { setFrontmatterCollapsed } from './frontmatter-collapse.js';
import { readingModeToggled } from './selection.js';

// Dispatched by app wiring (content_pane.js) once it has (re)computed which
// [[wikilink]] targets exist — e.g. after the async /api/notes/exist batch
// resolves, or after a vault delete elsewhere touches this note's links —
// so the next rebuild re-reads options.isWikiLinkBroken even though nothing
// in the document itself changed. Same seam as setFrontmatterCollapsed below.
export const wikiLinksRevalidated = StateEffect.define();

class LivePreviewPlugin {
  constructor(view, options) {
    this.options = options || {};
    this.decorations = Decoration.none;
    this.atomicRanges = Decoration.none;
    // Survives selection/viewport rebuilds; cleared on doc change.
    this.tableCache = new Map();
    // Tree used for the last rebuild — see the `treeChanged` check below.
    this.tree = syntaxTree(view.state);
    this.rebuild(view);
  }

  update(update) {
    if (update.docChanged) this.tableCache.clear();
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
    // The "Metadata" header toggle dispatches a bare effect — no doc change,
    // no selection change, so it wouldn't otherwise trigger a rebuild and
    // decorateFrontmatter's collapsed-block branch would never run.
    const collapseToggled = update.transactions.some((tr) => tr.effects.some((e) => e.is(setFrontmatterCollapsed)));
    const linksRevalidated = update.transactions.some((tr) => tr.effects.some((e) => e.is(wikiLinksRevalidated)));
    if (treeChanged || update.docChanged || update.viewportChanged || update.selectionSet || collapseToggled || linksRevalidated || readingModeToggled(update.startState, update.state)) {
      this.tree = tree;
      this.rebuild(update.view);
    }
  }

  rebuild(view) {
    const decos = [];
    const atomics = [];
    const tree = syntaxTree(view.state);
    for (const { from, to } of view.visibleRanges) {
      tree.iterate({
        from,
        to,
        enter: (node) => {
          const decorate = nodeDecorators[node.name];
          if (decorate) decorate(node, view, decos, atomics, this.options, this.tableCache);
        },
      });
    }
    this.decorations = Decoration.set(decos, true);
    this.atomicRanges = Decoration.set(atomics, true);
  }
}

export const livePreviewPlugin = ViewPlugin.fromClass(LivePreviewPlugin, {
  decorations: (v) => v.decorations,
});

export function livePreviewAtomicRanges() {
  return EditorView.atomicRanges.of((view) => {
    const plugin = view.plugin(livePreviewPlugin);
    return plugin ? plugin.atomicRanges : Decoration.none;
  });
}
