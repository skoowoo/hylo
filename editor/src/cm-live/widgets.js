import { WidgetType } from '@codemirror/view';
import { allowFrontmatterCaret } from './frontmatter-readonly.js';
import { openImageViewer } from './image-viewer.js';

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

// Rendered height per src: a re-created <img> holds its box while it decodes
// instead of collapsing to zero, and the height map gets a real estimate.
const imageHeightCache = new Map();

const ZOOM_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h6v6"/><path d="M9 21H3v-6"/><path d="m21 3-7 7"/><path d="m3 21 7-7"/></svg>';
const BROKEN_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m2 2 20 20"/><path d="M10.41 10.41a2 2 0 1 1-2.83-2.83"/><path d="M13.5 13.5 6 21"/><path d="M18 12l3 3"/><path d="M3.59 3.59A1.99 1.99 0 0 0 3 5v14a2 2 0 0 0 2 2h14c.55 0 1.052-.22 1.41-.59"/><path d="M21 15V5a2 2 0 0 0-2-2H9"/></svg>';

// ![[file]] and ![alt](url). `src` is already resolved by the decorator;
// `label` names the image in the broken-image placeholder. A click edits the
// source (`editOffset` chars in); the corner button opens the viewer.
export class ImageWidget extends WidgetType {
  constructor(src, label, editOffset) {
    super();
    this.src = src;
    this.label = label;
    this.editOffset = editOffset;
  }

  eq(other) {
    return other.src === this.src && other.label === this.label;
  }

  get estimatedHeight() {
    return imageHeightCache.get(this.src) ?? -1;
  }

  toDOM(view) {
    const wrap = document.createElement('span');
    wrap.className = 'cm-lp-wikiimage';
    const img = document.createElement('img');
    img.alt = this.label;
    img.draggable = false;
    const known = imageHeightCache.get(this.src);
    if (known) wrap.style.minHeight = known + 'px';
    img.addEventListener('load', () => {
      wrap.style.minHeight = '';
      if (wrap.offsetHeight) imageHeightCache.set(this.src, wrap.offsetHeight);
    }, { once: true });
    img.addEventListener('error', () => {
      imageHeightCache.delete(this.src);
      wrap.style.minHeight = '';
      wrap.className = 'cm-lp-wikiimage cm-lp-image-broken';
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
    editOnMouseDown(wrap, view, this.editOffset);
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

// Thematic break as <hr>. Single-line block only — multi-line block replace
// corrupts CM6 height-map (old frontmatter card bug).
export class HorizontalRuleWidget extends WidgetType {
  // Blank after hr: drop bottom padding so it doesn't stack on blank-line height.
  constructor(followedByBlank) {
    super();
    this.followedByBlank = !!followedByBlank;
  }

  eq(other) {
    return other.followedByBlank === this.followedByBlank;
  }

  toDOM() {
    const hr = document.createElement('hr');
    hr.className = 'cm-lp-hr' + (this.followedByBlank ? ' cm-lp-hr-tight-bottom' : '');
    return hr;
  }

  ignoreEvent() {
    return true;
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

// "Metadata" header row above the frontmatter block — click toggles the
// whole-block collapse (state lives in frontmatter-collapse.js, not here);
// the pencil button (only rendered when onEdit is wired up — an app-level
// concern, e.g. content_pane.js opening its edit dialog) is a separate hit target
// so it doesn't also trigger the collapse toggle.
//
// showEdit: the button stays in the DOM either way (so the row's layout
// doesn't jump when it appears) but is hidden — via CSS class, not
// display:none, so it fades rather than pops — until the caret is actually
// inside the frontmatter block (frontmatter-collapse.js computes this).
export class FrontmatterHeaderWidget extends WidgetType {
  constructor(collapsed, onToggle, onEdit, showEdit) {
    super();
    this.collapsed = collapsed;
    this.onToggle = onToggle;
    this.onEdit = onEdit || null;
    this.showEdit = !!showEdit;
  }

  eq(other) {
    return (
      other.collapsed === this.collapsed &&
      !!other.onEdit === !!this.onEdit &&
      other.showEdit === this.showEdit
    );
  }

  toDOM(view) {
    const row = document.createElement('div');
    row.className = 'cm-lp-fm-header' + (this.collapsed ? ' cm-lp-fm-header-collapsed' : '');
    row.setAttribute('role', 'button');
    row.tabIndex = 0;
    row.title = this.collapsed ? 'Expand metadata' : 'Collapse metadata';
    const chevron = document.createElement('span');
    chevron.className = 'cm-lp-fm-header-chevron';
    chevron.innerHTML =
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>';
    const label = document.createElement('span');
    label.className = 'cm-lp-fm-header-label';
    label.textContent = 'Metadata';
    row.appendChild(chevron);
    row.appendChild(label);
    row.addEventListener('mousedown', (e) => {
      e.preventDefault();
      this.onToggle(view);
    });

    if (this.onEdit) {
      const editBtn = document.createElement('button');
      editBtn.type = 'button';
      editBtn.className = 'cm-lp-fm-header-edit' + (this.showEdit ? '' : ' cm-lp-fm-header-edit-hidden');
      editBtn.tabIndex = this.showEdit ? 0 : -1;
      editBtn.title = 'Edit metadata';
      editBtn.innerHTML =
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/></svg>';
      editBtn.addEventListener('mousedown', (e) => {
        e.preventDefault();
        e.stopPropagation();
        this.onEdit(view);
      });
      row.appendChild(editBtn);
    }

    return row;
  }

  ignoreEvent() {
    return false;
  }
}

// Frontmatter collapsed line — same hairline trick as TableDelimiterWidget.
export class FrontmatterCollapsedLineWidget extends WidgetType {
  eq() {
    return true;
  }

  toDOM() {
    const span = document.createElement('span');
    span.className = 'cm-lp-fm-collapsed-widget';
    return span;
  }

  ignoreEvent() {
    return true;
  }
}

// Per-field icon — inserted before the key label; purely decorative, matches
// Obsidian's Properties panel at a glance. tag/list/number/text are the
// value-type fallback (decorators.js's fmFieldType); heading/layers/user/
// link/clock are fixed icons for system-defined keys (decorators.js's
// FM_KEY_ICON_OVERRIDES) that win over the value-type guess.
const FM_ICON_PATHS = {
  tag:
    '<path d="M12.586 2.586A2 2 0 0 0 11.172 2H4a2 2 0 0 0-2 2v7.172a2 2 0 0 0 .586 1.414l8.704 8.704a2.426 2.426 0 0 0 3.42 0l6.58-6.58a2.426 2.426 0 0 0 0-3.42Z"/><circle cx="7.5" cy="7.5" r=".5" fill="currentColor"/>',
  list: '<path d="M3 12h.01"/><path d="M3 18h.01"/><path d="M3 6h.01"/><path d="M8 12h13"/><path d="M8 18h13"/><path d="M8 6h13"/>',
  number: '<path d="M4 9h16"/><path d="M4 15h16"/><path d="M10 3 8 21"/><path d="M16 3 14 21"/>',
  text: '<path d="M17 6.1H3"/><path d="M21 12.1H3"/><path d="M15.1 18H3"/>',
  // title
  heading: '<path d="M6 12h12"/><path d="M6 20V4"/><path d="M18 20V4"/>',
  // kind
  layers:
    '<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>',
  // author
  user: '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
  // source / source_notes
  link:
    '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
  // clipped / clipped_at / created_at / last_compiled_at
  clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
};

export class FrontmatterKeyIconWidget extends WidgetType {
  constructor(type) {
    super();
    this.type = FM_ICON_PATHS[type] ? type : 'text';
  }

  eq(other) {
    return other.type === this.type;
  }

  toDOM() {
    const span = document.createElement('span');
    span.className = 'cm-lp-fm-icon';
    span.innerHTML =
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">' +
      FM_ICON_PATHS[this.type] +
      '</svg>';
    return span;
  }

  ignoreEvent() {
    return true;
  }
}

// Invisible icon+label-shaped spacer standing in for a block-list item's
// raw indent/"- " marker — same classes as the real key icon + label, just
// hidden, so it measures pixel-identical to them regardless of the label's
// smaller font-size (a ch-based padding-left on the line itself can't
// reproduce that, since ch resolves against the *line's* font there).
export class FrontmatterListIndentWidget extends WidgetType {
  constructor(labelWidthCh) {
    super();
    this.labelWidthCh = labelWidthCh;
  }

  eq(other) {
    return other.labelWidthCh === this.labelWidthCh;
  }

  toDOM() {
    const wrap = document.createElement('span');
    wrap.className = 'cm-lp-fm-list-indent';
    const icon = document.createElement('span');
    icon.className = 'cm-lp-fm-icon';
    const label = document.createElement('span');
    label.className = 'cm-lp-fm-label';
    label.style.width = this.labelWidthCh + 'ch';
    wrap.appendChild(icon);
    wrap.appendChild(label);
    wrap.appendChild(document.createTextNode(' '));
    return wrap;
  }

  ignoreEvent() {
    return true;
  }
}

// "+N more" — selection-only into first hidden line; decorateFrontmatter expands.
export class FrontmatterMoreWidget extends WidgetType {
  // ownLine: true when this is the sole content of its row (block-list
  // overflow, left-aligned under the value column) — the default inline
  // spacing (margin-left, for sitting right after a tag/item on the same
  // line) would just push it off that alignment.
  constructor(count, revealPos, ownLine) {
    super();
    this.count = count;
    this.revealPos = revealPos;
    this.ownLine = !!ownLine;
  }

  eq(other) {
    return other.count === this.count && other.revealPos === this.revealPos && other.ownLine === this.ownLine;
  }

  toDOM(view) {
    const span = document.createElement('span');
    span.className = 'cm-lp-fm-more' + (this.ownLine ? ' cm-lp-fm-more-own-line' : '');
    span.textContent = '+' + this.count + ' more';
    span.addEventListener('mousedown', (e) => {
      e.preventDefault();
      view.dispatch({ selection: { anchor: this.revealPos }, annotations: allowFrontmatterCaret.of(true) });
    });
    return span;
  }

  ignoreEvent() {
    return false;
  }
}
