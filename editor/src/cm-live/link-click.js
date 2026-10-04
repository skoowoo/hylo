// Obsidian-style: hover/click/edit are independent. Click opens; edit only
// when caret reaches the link by other means (decorateLink reveals raw).
//
// Edge cases:
//   1. Drag: travel >= CLICK_DRAG_THRESHOLD → don't open (not "selection empty"
//      — hand tremor alone can make a 1-char selection).
//   2. Already editing: selectionTouchesRange on pre-click selection → don't open.
//   3. Boundary: isStrictlyInside (exclusive); selectionTouchesRange is inclusive for reveal.
//   4. mouseup: raw capture-phase — CM6 MouseSelection preventDefault races domEventHandlers.
//
// Position via caretPositionFromPoint — more reliable than posAtCoords near block decos.
import { EditorView, ViewPlugin } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { selectionTouchesRange } from './selection.js';
import { setSuppressedLinkReveal, suppressedLinkRevealField } from './link-reveal-suppression.js';

function posAtEvent(view, e) {
  if (document.caretPositionFromPoint) {
    const caret = document.caretPositionFromPoint(e.clientX, e.clientY);
    if (caret) return view.posAtDOM(caret.offsetNode, caret.offset);
  } else if (document.caretRangeFromPoint) {
    const range = document.caretRangeFromPoint(e.clientX, e.clientY);
    if (range) return view.posAtDOM(range.startContainer, range.startOffset);
  }
  return null;
}

// Bare GFM autolinks lack a scheme; window.open needs an absolute URL.
function normalizeAutolinkUrl(raw) {
  if (/^[a-z][-\w+.]*:/i.test(raw)) return raw;
  if (/^www\./i.test(raw)) return 'https://' + raw;
  if (raw.includes('@')) return 'mailto:' + raw;
  return raw;
}

// from/to = full span (reveal); textFrom/textTo = visible text (click hit-test).
function linkNodeAt(view, pos) {
  let result = null;
  syntaxTree(view.state).iterate({
    from: pos,
    to: pos,
    enter(node) {
      if (node.name === 'Link') {
        const marks = node.node.getChildren('LinkMark');
        const urlNode = node.node.getChild('URL');
        if (marks.length >= 2 && urlNode) {
          result = {
            url: view.state.doc.sliceString(urlNode.from, urlNode.to),
            from: node.from,
            to: node.to,
            textFrom: marks[0].to,
            textTo: marks[1].from,
          };
        }
      } else if (node.name === 'Autolink') {
        const urlNode = node.node.getChild('URL');
        if (urlNode) {
          result = {
            url: view.state.doc.sliceString(urlNode.from, urlNode.to),
            from: node.from,
            to: node.to,
            textFrom: urlNode.from,
            textTo: urlNode.to,
          };
        }
      } else if (node.name === 'URL' && result === null) {
        // Bare GFM only — Autolink already handled "<...>" above.
        const url = normalizeAutolinkUrl(view.state.doc.sliceString(node.from, node.to));
        result = { url, from: node.from, to: node.to, textFrom: node.from, textTo: node.to };
      }
    },
  });
  return result;
}

// Matches goldmark AutoHeadingID (mdhtml.go); Unicode-aware for CJK.
function slugify(text) {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\-_ ]+/gu, '')
    .replace(/\s+/g, '-');
}

const HEADING_LEAD_RE = /^#{1,6}\s+/;

// Editor headings have no DOM id — scroll within the view instead of window.open.
function jumpToHeading(view, rawTarget) {
  let target;
  try {
    target = decodeURIComponent(rawTarget);
  } catch {
    target = rawTarget;
  }
  const targetSlug = slugify(target);
  if (!targetSlug) return false;
  const doc = view.state.doc;
  let found = null;
  syntaxTree(view.state).iterate({
    enter(node) {
      if (found) return false;
      if (!/^ATXHeading[1-6]$/.test(node.name)) return;
      const line = doc.lineAt(node.from);
      const headingSlug = slugify(line.text.replace(HEADING_LEAD_RE, ''));
      if (headingSlug === targetSlug) found = line.from;
    },
  });
  if (found == null) return false;
  view.dispatch({
    selection: { anchor: found },
    effects: EditorView.scrollIntoView(found, { y: 'start', yMargin: 40 }),
  });
  view.focus();
  return true;
}

function openExternal(url) {
  window.open(url, '_blank', 'noopener,noreferrer');
}

// Exclusive of boundaries — see header point 3.
function isStrictlyInside(pos, from, to) {
  return pos > from && pos < to;
}

// CSS :hover can't distinguish edge vs interior; match click hit-test.
const HOVER_HIT_CLASS = 'cm-lp-link-hit';

// Matches CM6 MouseSelection "not yet a drag"; ignores hand-tremor jitter.
const CLICK_DRAG_THRESHOLD = 10;

export function linkClickHandler() {
  let pending = null;
  let hitEl = null;

  const mouseupPlugin = ViewPlugin.fromClass(
    class {
      constructor(view) {
        this.view = view;
        this.onMouseUp = (e) => {
          const p = pending;
          pending = null;
          if (!p || e.button !== 0) {
            if (p) this.view.dispatch({ effects: setSuppressedLinkReveal.of(null) });
            return;
          }
          // Macrotask: CM6 finalizes caret placement on this same mouseup.
          setTimeout(() => {
            // Clear suppression + open in one callback to avoid reveal flash.
            this.view.dispatch({ effects: setSuppressedLinkReveal.of(null) });
            const traveled = Math.hypot(e.clientX - p.startX, e.clientY - p.startY);
            if (traveled >= CLICK_DRAG_THRESHOLD) return;
            // Collapse tremor-induced 1-char selection to a caret.
            if (!this.view.state.selection.main.empty) {
              this.view.dispatch({ selection: { anchor: this.view.state.selection.main.head } });
            }
            if (p.kind === 'stabilize') {
              // Boundary click — caret only, no open.
            } else if (p.kind === 'anchor') {
              jumpToHeading(this.view, p.target);
            } else {
              openExternal(p.url);
            }
          }, 0);
        };
        view.dom.addEventListener('mouseup', this.onMouseUp, true);
      }
      destroy() {
        this.view.dom.removeEventListener('mouseup', this.onMouseUp, true);
      }
    },
  );

  const mouseHandlers = EditorView.domEventHandlers({
    mousemove(e, view) {
      const target = e.target.closest && e.target.closest('.cm-lp-link');
      if (hitEl && hitEl !== target) {
        hitEl.classList.remove(HOVER_HIT_CLASS);
        hitEl = null;
      }
      if (!target) return false;
      const pos = posAtEvent(view, e);
      const node = pos == null ? null : linkNodeAt(view, pos);
      const inside = !!node && isStrictlyInside(pos, node.textFrom, node.textTo);
      target.classList.toggle(HOVER_HIT_CLASS, inside);
      hitEl = inside ? target : null;
      return false;
    },
    mousedown(e, view) {
      pending = null;
      if (e.button !== 0) return false;

      // Frontmatter URLs are Decoration.mark only — no Link syntax node.
      const fmLink = e.target.closest && e.target.closest('.cm-lp-fm-link');
      if (fmLink) {
        pending = { kind: 'fm', url: fmLink.textContent.trim(), startX: e.clientX, startY: e.clientY };
        return false;
      }

      const pos = posAtEvent(view, e);
      if (pos == null) return false;
      const node = linkNodeAt(view, pos);
      if (!node) return false;
      if (!isStrictlyInside(pos, node.textFrom, node.textTo)) {
        // Boundary: stabilize tremor selection, don't open.
        pending = { kind: 'stabilize', startX: e.clientX, startY: e.clientY };
        return false;
      }
      // Already editing raw markdown — leave alone.
      if (selectionTouchesRange(view.state, node.from, node.to)) return false;
      pending = node.url.startsWith('#')
        ? { kind: 'anchor', target: node.url.slice(1), startX: e.clientX, startY: e.clientY }
        : { kind: 'url', url: node.url, startX: e.clientX, startY: e.clientY };
      // Hold reveal until mouseup decides click vs drag.
      view.dispatch({ effects: setSuppressedLinkReveal.of({ from: node.from, to: node.to }) });
      return false;
    },
  });

  return [mouseHandlers, mouseupPlugin, suppressedLinkRevealField];
}
