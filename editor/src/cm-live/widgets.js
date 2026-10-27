import { WidgetType, EditorView } from '@codemirror/view';
import { openImageViewer } from './image-viewer.js';
import { imageNodeAt } from './image-nodes.js';
import { imageBox, columnWidth, rememberImageSize, IMAGE_MAX_HEIGHT } from './image-sizes.js';

// Click puts the caret `offset` chars into the source the widget replaces,
// which flips it back to raw markdown for editing.
function editOnMouseDown(dom, view, offset) {
  dom.addEventListener('mousedown', (e) => {
    if (e.button !== 0 || e.defaultPrevented) return;
    e.preventDefault();
    view.dispatch({ selection: { anchor: view.posAtDOM(dom) + offset } });
    view.focus();
  });
}

// Clickable [[wikilink]] chip; onClick injected by content_pane.js (no Hylo routing here).
// broken (from options.isWikiLinkBroken, also app-injected) means the target note
// doesn't exist any more — nothing to navigate to, so it renders struck-through
// and skips the click handler entirely rather than firing onClick into a dead end.
export class WikiLinkWidget extends WidgetType {
  constructor(target, heading, alias, onClick, broken) {
    super();
    this.target = target;
    this.heading = heading;
    this.alias = alias;
    this.onClick = onClick;
    this.broken = !!broken;
  }

  eq(other) {
    return other.target === this.target && other.heading === this.heading && other.alias === this.alias && other.broken === this.broken;
  }

  toDOM(view) {
    const span = document.createElement('span');
    span.className = this.broken ? 'cm-lp-wikilink cm-lp-wikilink-broken' : 'cm-lp-wikilink';
    const ref = this.heading ? (this.target ? this.target + ' › ' : '') + this.heading : this.target;
    span.textContent = this.alias || ref;
    span.title = this.broken ? 'Note not found — click to edit' : ref;
    span.setAttribute('data-wl-value', this.target);
    if (this.onClick && !this.broken) {
      span.addEventListener('mousedown', (e) => {
        e.preventDefault();
        this.onClick(this.target, this.alias, e, this.heading);
      });
    } else {
      // Nothing to open — the likely intent is fixing the target.
      editOnMouseDown(span, view, 2);
    }
    return span;
  }

  // Let our listener run instead of CM6 caret-near-widget default.
  ignoreEvent() {
    return false;
  }
}

const ZOOM_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h6v6"/><path d="M9 21H3v-6"/><path d="m21 3-7 7"/><path d="m3 21 7-7"/></svg>';
const BROKEN_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m2 2 20 20"/><path d="M10.41 10.41a2 2 0 1 1-2.83-2.83"/><path d="M13.5 13.5 6 21"/><path d="M18 12l3 3"/><path d="M3.59 3.59A1.99 1.99 0 0 0 3 5v14a2 2 0 0 0 2 2h14c.55 0 1.052-.22 1.41-.59"/><path d="M21 15V5a2 2 0 0 0-2-2H9"/></svg>';

// ![[file]] and ![alt](url). `src` is already resolved by the decorator;
// `label` names the image in the broken-image placeholder. Double-click edits
// the source `editOffset` chars in; the corner button opens the viewer.
// `size` (image-sizes.js) lays the box out at its final size before the file
// loads and gives CM the line's real height while it isn't drawn.
export class ImageWidget extends WidgetType {
  constructor(src, label, editOffset, selected = false, size = null) {
    super();
    this.src = src;
    this.label = label;
    this.editOffset = editOffset;
    this.selected = selected;
    this.size = size;
  }

  eq(other) {
    return other.src === this.src && other.label === this.label && other.editOffset === this.editOffset &&
      other.selected === this.selected && sameSize(other.size, this.size);
  }

  // Selection and a learned size change the box only — rebuilding would reload the image.
  updateDOM(dom, _view, old) {
    if (old.src !== this.src || old.label !== this.label || old.editOffset !== this.editOffset) return false;
    dom.classList.toggle('cm-lp-image-selected', this.selected);
    if (!dom.classList.contains('cm-lp-image-broken')) sizeBox(dom, this.size);
    return true;
  }

  get estimatedHeight() {
    return this.size ? Math.ceil(imageBox(this.size, columnWidth()).height) : -1;
  }

  toDOM(view) {
    const wrap = document.createElement('span');
    wrap.className = 'cm-lp-wikiimage' + (this.selected ? ' cm-lp-image-selected' : '');
    sizeBox(wrap, this.size);
    const img = document.createElement('img');
    img.alt = this.label;
    img.draggable = false;
    img.addEventListener('load', () => rememberImageSize(this.src, img.naturalWidth, img.naturalHeight), { once: true });
    img.addEventListener('error', () => {
      sizeBox(wrap, null);
      wrap.classList.add('cm-lp-image-broken');
      wrap.innerHTML = BROKEN_ICON;
      const text = document.createElement('span');
      text.textContent = 'Image not found · ' + this.label;
      wrap.appendChild(text);
    }, { once: true });
    img.src = this.src;

    const zoom = document.createElement('button');
    zoom.type = 'button';
    zoom.className = 'cm-lp-image-zoom';
    zoom.title = 'View image';
    zoom.setAttribute('aria-label', 'View image');
    zoom.innerHTML = ZOOM_ICON;
    zoom.addEventListener('mousedown', (e) => {
      e.preventDefault();
      e.stopPropagation();
      openImageViewer(this.src, this.label);
    });
    wrap.append(img, zoom);
    // Atomic ranges keep mouse selections out of the source, so a double-click
    // is the way in. Caught on mousedown: WebKit never fires dblclick here.
    wrap.addEventListener('mousedown', (e) => {
      if (e.detail !== 2 || e.button !== 0 || !view.state.facet(EditorView.editable)) return;
      if (e.target instanceof Element && e.target.closest('.cm-lp-image-zoom')) return;
      e.preventDefault();
      const node = imageNodeAt(view.state, view.posAtDOM(wrap));
      if (!node) return;
      const at = Math.min(node.to, node.from + this.editOffset);
      view.dispatch({ selection: { anchor: at } });
      view.focus();
    });
    return wrap;
  }

  // CM would otherwise size the caret beside the widget to the whole image.
  // Report a text-height caret on the image's bottom edge, where text after it sits.
  coordsAt(dom, pos, side) {
    const rect = dom.getBoundingClientRect();
    const lineHeight = parseFloat(getComputedStyle(dom).lineHeight) || rect.height;
    const x = pos === 0 && side <= 0 ? rect.left : rect.right;
    return { left: x, right: x, top: rect.bottom - Math.min(lineHeight, rect.height), bottom: rect.bottom };
  }

  ignoreEvent() {
    return false;
  }
}

function sameSize(a, b) {
  return a === b || (!!a && !!b && a[0] === b[0] && a[1] === b[1]);
}

// The same box imageBox() predicts, in CSS so it follows the column's width.
function sizeBox(wrap, size) {
  wrap.classList.toggle('cm-lp-image-sized', !!size);
  wrap.style.width = size ? `min(${size[0]}px, 100%, ${(IMAGE_MAX_HEIGHT * size[0]) / size[1]}px)` : '';
  wrap.style.setProperty('--image-ratio', size ? `${size[0]} / ${size[1]}` : '');
}

// GFM "[ ]"/"[x]" → checkbox in the list's marker column. Position is read
// from the DOM at click time, not stored: an edit above would otherwise fail
// eq() and rebuild every checkbox below it.
export class TaskCheckboxWidget extends WidgetType {
  constructor(checked, columnEm) {
    super();
    this.checked = checked;
    this.columnEm = columnEm;
  }

  eq(other) {
    return other.checked === this.checked && other.columnEm === this.columnEm;
  }

  toDOM(view) {
    const box = document.createElement('span');
    box.className = 'cm-lp-list-marker';
    box.style.width = this.columnEm + 'em';
    const input = document.createElement('input');
    input.type = 'checkbox';
    input.className = 'cm-lp-task-checkbox';
    input.checked = this.checked;
    input.addEventListener('mousedown', (e) => {
      e.preventDefault();
      const pos = view.posAtDOM(box);
      const checked = /\[[xX]\]/.test(view.state.doc.sliceString(pos, pos + 3));
      view.dispatch({ changes: { from: pos, to: pos + 3, insert: checked ? '[ ]' : '[x]' } });
    });
    box.appendChild(input);
    return box;
  }

  ignoreEvent() {
    return false;
  }
}

// Bullet "•" or the ordered number, in a fixed-width marker column.
// Revealed line-start syntax ("## ", "> ") hung in the margin at zero width.
// A widget, not styled source text: an IME composition or a redraw can't
// land inside a sized box and shift the marker or misplace the caret.
export class HangMarkWidget extends WidgetType {
  constructor(text, className) {
    super();
    this.text = text;
    this.className = className;
  }

  eq(other) {
    return other.text === this.text && other.className === this.className;
  }

  toDOM(view) {
    const anchor = document.createElement('span');
    anchor.className = 'cm-lp-hang-anchor';
    const mark = document.createElement('span');
    mark.className = this.className + ' cm-lp-hang-mark';
    mark.textContent = this.text;
    anchor.appendChild(mark);
    // CM would map any click on a widget to one of its sides; keep the
    // source-text feel of landing between the clicked characters.
    mark.addEventListener('mousedown', (e) => {
      if (e.button !== 0 || e.shiftKey || !view.state.facet(EditorView.editable)) return;
      const hit = document.caretRangeFromPoint && document.caretRangeFromPoint(e.clientX, e.clientY);
      const offset = hit && hit.startContainer === mark.firstChild ? hit.startOffset : 0;
      e.preventDefault();
      view.dispatch({ selection: { anchor: view.posAtDOM(anchor) + offset }, userEvent: 'select.pointer' });
      view.focus();
    });
    return anchor;
  }

  ignoreEvent() {
    return true;
  }
}

export class ListMarkerWidget extends WidgetType {
  constructor(label, columnEm) {
    super();
    this.label = label;
    this.columnEm = columnEm;
  }

  eq(other) {
    return other.label === this.label && other.columnEm === this.columnEm;
  }

  toDOM() {
    const span = document.createElement('span');
    span.className = this.label ? 'cm-lp-list-marker cm-lp-list-mark-ol' : 'cm-lp-list-marker cm-lp-bullet';
    span.style.width = this.columnEm + 'em';
    span.textContent = this.label || '•';
    return span;
  }

  ignoreEvent() {
    return true;
  }
}

const COPY_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>';
const CHECK_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>';

// Opening fence line of a code block at rest: language label + copy button.
export class CodeHeaderWidget extends WidgetType {
  constructor(lang, code) {
    super();
    this.lang = lang;
    this.code = code;
  }

  eq(other) {
    return other.lang === this.lang && other.code === this.code;
  }

  toDOM() {
    const wrap = document.createElement('span');
    wrap.className = 'cm-lp-code-header';
    if (this.lang) {
      const label = document.createElement('span');
      label.className = 'cm-lp-code-lang';
      label.textContent = this.lang;
      wrap.appendChild(label);
    }
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'cm-lp-code-copy';
    btn.title = 'Copy code';
    btn.setAttribute('aria-label', 'Copy code');
    btn.innerHTML = COPY_ICON;
    // Keep the caret where it is — clicking chrome shouldn't open the block for editing.
    btn.addEventListener('mousedown', (e) => e.preventDefault());
    btn.addEventListener('click', () => {
      if (!navigator.clipboard) return;
      navigator.clipboard.writeText(this.code).then(() => {
        btn.innerHTML = CHECK_ICON;
        btn.classList.add('is-done');
        setTimeout(() => {
          btn.innerHTML = COPY_ICON;
          btn.classList.remove('is-done');
        }, 1200);
      }).catch(() => {});
    });
    wrap.appendChild(btn);
    return wrap;
  }

  // The copy button is ours; a click anywhere else on the header lets CM place the caret.
  ignoreEvent(e) {
    return !!(e.target.closest && e.target.closest('.cm-lp-code-copy'));
  }
}

// Thematic break: a rule across the line that holds its "---".
export class HorizontalRuleWidget extends WidgetType {
  eq() {
    return true;
  }

  toDOM() {
    const rule = document.createElement('span');
    rule.className = 'cm-lp-hr-rule';
    return rule;
  }

  // A click on the rule lands the caret on its line, revealing the source.
  ignoreEvent() {
    return false;
  }
}

// Empty "| |" cell — no TableCell node; nbsp keeps box height.
export class TableEmptyCellWidget extends WidgetType {
  constructor(className, style) {
    super();
    this.className = className;
    this.style = style || null;
  }

  eq(other) {
    return other.className === this.className && other.style === this.style;
  }

  toDOM() {
    const span = document.createElement('span');
    span.className = this.className;
    if (this.style) span.setAttribute('style', this.style);
    span.innerHTML = '&nbsp;';
    return span;
  }

  ignoreEvent() {
    return true;
  }
}

// Delimiter row hairline — needs inline-block + line font-size shrink (theme.js).
export class TableDelimiterWidget extends WidgetType {
  eq() {
    return true;
  }

  toDOM() {
    const span = document.createElement('span');
    span.className = 'cm-lp-table-delim-widget';
    return span;
  }

  ignoreEvent() {
    return true;
  }
}
