// Frontmatter as a properties card, like fence cards (fence-card/card-field.js), except
// only the Edit button opens the YAML source: arrowing past it never lands inside.
// A StateField because CM6 refuses block decorations from a ViewPlugin.
import { StateField, StateEffect, EditorState, EditorSelection, ChangeSet, Transaction } from '@codemirror/state';
import { Decoration, EditorView, WidgetType } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';
import { headInBlock, revealToggled, selectionMatters } from './selection.js';
import { findFrontmatterNode } from './frontmatter-syntax.js';
import { parseFrontmatter } from './frontmatter-yaml.js';
import { wikiLinksRevalidated } from './live-preview.js';
import { splitWikiLinkInner } from './wiki-syntax.js';
import { WikiLinkWidget } from './widgets.js';
import { ensureStyle, iconButton } from './fence-card/shell.js';
import { heldBlocks, blockHold, blockSpacer } from './block-hold.js';

export const setFrontmatterCollapsed = StateEffect.define();

// Outside the live-preview compartment in the host, so the flag survives a mode toggle.
export const frontmatterCollapseField = StateField.define({
  create: () => false,
  update(value, tr) {
    for (const e of tr.effects) if (e.is(setFrontmatterCollapsed)) value = e.value;
    return value;
  },
});

const editFrontmatter = StateEffect.define();

// True from the Edit button until the caret leaves the source. Undo/redo of a source
// edit puts the caret back inside, so it reopens the source too, and so does coming
// back from source mode with the caret inside.
const frontmatterEditingField = StateField.define({
  create(state) {
    const node = findFrontmatterNode(state);
    return !!node && !state.readOnly && state.selection.main.head < node.to;
  },
  update(value, tr) {
    if (tr.effects.some((e) => e.is(editFrontmatter))) return true;
    const node = findFrontmatterNode(tr.state);
    if (!node || tr.state.readOnly) return false;
    const inside = headInBlock(tr.state, node.from, node.to);
    return inside && (value || tr.isUserEvent('undo') || tr.isUserEvent('redo'));
  },
});

function isEditing(state) {
  return state.field(frontmatterEditingField, false);
}

// A caret under the card would be invisible; move it to the body start instead,
// or the card's edge when nothing follows it.
const keepCaretOut = EditorState.transactionFilter.of((tr) => {
  if (!tr.selection || tr.docChanged || isEditing(tr.startState) || tr.effects.some((e) => e.is(editFrontmatter))) return tr;
  const node = findFrontmatterNode(tr.startState);
  if (!node) return tr;
  const rest = Math.min(node.to + 1, tr.newDoc.length);
  const sel = tr.newSelection;
  const hidden = (r) => r.empty && r.head < rest;
  if (!sel.ranges.some(hidden)) return tr;
  const ranges = sel.ranges.map((r) => (hidden(r) ? EditorSelection.cursor(rest) : r));
  return [tr, { selection: EditorSelection.create(ranges, sel.mainIndex), sequential: true }];
});

// With nothing after the frontmatter the caret rests at the card's edge, glued to the
// closing "---". Typing there starts a new line; deleting into the card does nothing.
const CARET_EDITS = ['input.type', 'input.paste', 'input.drop', 'delete'];

const keepEditsOut = EditorState.transactionFilter.of((tr) => {
  if (!tr.docChanged || isEditing(tr.startState) || !CARET_EDITS.some((e) => tr.isUserEvent(e))) return tr;
  if (!tr.startState.selection.ranges.every((r) => r.empty)) return tr;
  const node = findFrontmatterNode(tr.startState);
  if (!node) return tr;
  let touches = false;
  let appendOnly = true;
  const changes = [];
  tr.changes.iterChanges((fromA, toA, fromB, toB, text) => {
    let insert = text.toString();
    if (fromA <= node.to) {
      touches = true;
      if (fromA !== node.to || toA !== node.to) appendOnly = false;
      else if (!insert.startsWith('\n')) insert = '\n' + insert;
    }
    changes.push({ from: fromA, to: toA, insert });
  });
  if (!touches) return tr;
  if (!appendOnly) return [];
  const set = ChangeSet.of(changes, tr.startState.doc.length);
  return {
    changes: set,
    selection: tr.startState.selection.map(set, 1),
    effects: tr.effects,
    scrollIntoView: tr.scrollIntoView,
    annotations: Transaction.userEvent.of(tr.annotation(Transaction.userEvent)),
  };
});

const LIST_VISIBLE_ITEMS = 3;
const WIKILINK_RE = /\[\[([^\]]+)\]\]/g;
const HAS_WIKILINK_RE = /\[\[[^\]]+\]\]/;
const URL_RE = /^https?:\/\/\S+$/i;
const NUMBER_RE = /^-?\d+(\.\d+)?$/;
const BOOL_RE = /^(true|false)$/i;
const DATE_RE = /^(\d{4}-\d{2}-\d{2})(?:[T ](\d{2}:\d{2})(?::\d{2}(?:\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$/;

// Lucide glyphs. System keys get a fixed icon; everything else is picked by value shape.
const ICONS = {
  tag: '<path d="M12.586 2.586A2 2 0 0 0 11.172 2H4a2 2 0 0 0-2 2v7.172a2 2 0 0 0 .586 1.414l8.704 8.704a2.426 2.426 0 0 0 3.42 0l6.58-6.58a2.426 2.426 0 0 0 0-3.42Z"/><circle cx="7.5" cy="7.5" r=".5" fill="currentColor"/>',
  list: '<path d="M3 12h.01"/><path d="M3 18h.01"/><path d="M3 6h.01"/><path d="M8 12h13"/><path d="M8 18h13"/><path d="M8 6h13"/>',
  number: '<path d="M4 9h16"/><path d="M4 15h16"/><path d="M10 3 8 21"/><path d="M16 3 14 21"/>',
  text: '<path d="M17 6.1H3"/><path d="M21 12.1H3"/><path d="M15.1 18H3"/>',
  heading: '<path d="M6 12h12"/><path d="M6 20V4"/><path d="M18 20V4"/>',
  layers: '<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>',
  user: '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
  clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
  calendar: '<path d="M8 2v4"/><path d="M16 2v4"/><rect width="18" height="18" x="3" y="4" rx="2"/><path d="M3 10h18"/>',
  check: '<rect width="18" height="18" x="3" y="3" rx="2"/><path d="m9 12 2 2 4-4"/>',
  braces: '<path d="M8 3H7a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5c0 1.1.9 2 2 2h1"/><path d="M16 21h1a2 2 0 0 0 2-2v-5c0-1.1.9-2 2-2a2 2 0 0 1-2-2V5a2 2 0 0 0-2-2h-1"/>',
  chevron: '<path d="m6 9 6 6 6-6"/>',
};

const KEY_ICONS = {
  title: 'heading',
  kind: 'layers',
  tags: 'tag',
  author: 'user',
  source: 'link',
  source_notes: 'link',
  clipped: 'clock',
  clipped_at: 'clock',
  created_at: 'clock',
  last_compiled_at: 'clock',
};

function svg(name) {
  return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name]}</svg>`;
}

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text != null) node.textContent = text;
  return node;
}

function scalarKind(value) {
  if (value.type !== 'scalar' || value.quoted) return value.type === 'scalar' && URL_RE.test(value.text) ? 'url' : 'text';
  if (BOOL_RE.test(value.text)) return 'bool';
  if (NUMBER_RE.test(value.text)) return 'number';
  if (DATE_RE.test(value.text)) return 'date';
  return URL_RE.test(value.text) ? 'url' : 'text';
}

function iconFor(field) {
  const fixed = KEY_ICONS[field.key.trim().toLowerCase()];
  if (fixed) return fixed;
  const { value } = field;
  if (value.type === 'list') return 'list';
  if (value.type === 'map') return 'braces';
  return { bool: 'check', number: 'number', date: 'calendar', url: 'link' }[scalarKind(value)] || 'text';
}

// Local time without seconds; the raw value stays in the tooltip.
function formatDate(text) {
  const m = DATE_RE.exec(text);
  if (!m[2]) return m[1];
  const d = new Date(m[3] ? text : text.replace(' ', 'T'));
  if (isNaN(d)) return text;
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function wikiTargets(fields) {
  const out = [];
  const scan = (s) => { for (const m of s.matchAll(WIKILINK_RE)) out.push(splitWikiLinkInner(m[1]).target); };
  for (const { value } of fields) {
    if (value.type === 'scalar' || value.type === 'text') scan(value.text);
    else if (value.type === 'list') value.items.forEach(scan);
    else if (value.type === 'map') value.entries.forEach((e) => scan(e.text));
  }
  return out;
}

const estimateHeight = (collapsed, fieldCount) => (collapsed ? 32 : 32 + fieldCount * 26);

class FrontmatterCardWidget extends WidgetType {
  constructor(source, fields, collapsed, canEdit, broken, options) {
    super();
    this.source = source;
    this.fields = fields;
    this.collapsed = collapsed;
    this.canEdit = canEdit;
    this.broken = broken;
    this.options = options;
  }

  eq(other) {
    return (
      other.source === this.source &&
      other.collapsed === this.collapsed &&
      other.canEdit === this.canEdit &&
      other.broken.join('\n') === this.broken.join('\n')
    );
  }

  get estimatedHeight() {
    return estimateHeight(this.collapsed, this.fields.length);
  }

  // Text with [[links]] rendered as the same chips the body uses.
  inline(view, parent, text) {
    let last = 0;
    for (const m of text.matchAll(WIKILINK_RE)) {
      if (m.index > last) parent.appendChild(document.createTextNode(text.slice(last, m.index)));
      const { target, heading, alias } = splitWikiLinkInner(m[1]);
      const onClick = this.options.onWikiLinkClick || null;
      const broken = !!(target && this.options.isWikiLinkBroken && this.options.isWikiLinkBroken(target));
      parent.appendChild(new WikiLinkWidget(target, heading, alias, onClick, broken).toDOM(view));
      last = m.index + m[0].length;
    }
    if (last < text.length) parent.appendChild(document.createTextNode(text.slice(last)));
    return parent;
  }

  list(view, cell, key, items) {
    const isTags = key.trim().toLowerCase() === 'tags';
    const chip = (item) => {
      const c = el('span', 'cm-fm-chip');
      if (isTags && !HAS_WIKILINK_RE.test(item)) c.textContent = '#' + item.replace(/^#/, '');
      else this.inline(view, c, item);
      return c;
    };
    const visible = items.length > LIST_VISIBLE_ITEMS + 1 ? LIST_VISIBLE_ITEMS : items.length;
    items.slice(0, visible).forEach((item) => cell.appendChild(chip(item)));
    if (visible < items.length) {
      const more = el('button', 'cm-fm-more', '+' + (items.length - visible));
      more.type = 'button';
      more.title = 'Show all';
      more.addEventListener('click', () => more.replaceWith(...items.slice(visible).map(chip)));
      cell.appendChild(more);
    }
  }

  value(view, field) {
    const cell = el('div', 'cm-fm-val');
    const { value } = field;
    if (value.type === 'empty' || (value.type === 'list' && !value.items.length)) {
      cell.classList.add('cm-fm-val-empty');
      cell.textContent = 'Empty';
    } else if (value.type === 'list') {
      cell.classList.add('cm-fm-val-list');
      this.list(view, cell, field.key, value.items);
    } else if (value.type === 'map') {
      for (const e of value.entries) {
        const row = el('div', 'cm-fm-sub');
        row.appendChild(el('span', 'cm-fm-sub-key', e.key));
        this.inline(view, row, e.text);
        cell.appendChild(row);
      }
    } else if (value.type === 'text') {
      cell.classList.add('cm-fm-val-pre');
      this.inline(view, cell, value.text);
    } else {
      const kind = scalarKind(value);
      if (kind === 'url') {
        const a = el('a', 'cm-fm-link', value.text.replace(/^https?:\/\/(www\.)?/i, ''));
        a.href = value.text;
        a.title = value.text;
        a.addEventListener('click', (e) => {
          e.preventDefault();
          window.open(value.text, '_blank', 'noopener,noreferrer');
        });
        cell.appendChild(a);
      } else if (kind === 'bool') {
        const on = value.text.toLowerCase() === 'true';
        cell.appendChild(el('span', 'cm-fm-bool' + (on ? ' cm-fm-bool-on' : ''), on ? 'Yes' : 'No'));
      } else if (kind === 'date') {
        cell.textContent = formatDate(value.text);
        cell.title = value.text;
      } else {
        if (kind === 'number') cell.classList.add('cm-fm-val-num');
        this.inline(view, cell, value.text);
      }
    }
    return cell;
  }

  toDOM(view) {
    ensureStyle();
    const wrap = el('div', 'hy-card hy-card-inline cm-fm-card' + (this.collapsed ? ' cm-fm-card-collapsed' : ''));
    // Keep a press from dropping the DOM selection inside, which CM would read
    // as the caret entering the block. Only Edit opens the source.
    wrap.addEventListener('mousedown', (e) => e.preventDefault(), true);

    const head = el('div', 'cm-fm-head');
    head.setAttribute('role', 'button');
    head.tabIndex = -1;
    head.title = this.collapsed ? 'Expand metadata' : 'Collapse metadata';
    head.innerHTML = `<span class="cm-fm-chevron">${svg('chevron')}</span>`;
    head.appendChild(el('span', 'cm-fm-title', 'Metadata'));
    if (this.collapsed) head.appendChild(el('span', 'cm-fm-count', String(this.fields.length)));
    head.addEventListener('click', () => view.dispatch({ effects: setFrontmatterCollapsed.of(!this.collapsed) }));
    wrap.appendChild(head);

    if (!this.collapsed) {
      const grid = el('div', 'cm-fm-grid');
      for (const field of this.fields) {
        const key = el('div', 'cm-fm-key');
        key.innerHTML = `<span class="cm-fm-icon">${svg(iconFor(field))}</span>`;
        key.appendChild(el('span', 'cm-fm-name', field.key));
        key.title = field.key;
        grid.append(key, this.value(view, field));
      }
      wrap.appendChild(grid);
    }

    if (this.canEdit) {
      const bar = el('div', 'hy-card-bar');
      bar.appendChild(
        iconButton('code-xml', 'Edit source', () => {
          view.dispatch({ selection: { anchor: editPos(view.state) }, effects: editFrontmatter.of(true), scrollIntoView: true });
          view.focus();
        })
      );
      wrap.appendChild(bar);
    }
    return wrap;
  }

  ignoreEvent() {
    return true;
  }
}

// End of the first YAML line; the opening "---" when the block is empty.
function editPos(state) {
  const node = findFrontmatterNode(state);
  const doc = state.doc;
  const open = doc.lineAt(node ? node.from : 0);
  const next = doc.line(Math.min(open.number + 1, doc.lines));
  return next.to < (node ? node.to : 0) - 3 ? next.to : open.to;
}

function interior(state, node) {
  const text = state.doc.sliceString(node.from, node.to);
  return text.slice(4, Math.max(4, text.length - 4));
}

function build(state, options) {
  const node = findFrontmatterNode(state);
  if (!node) return Decoration.none;
  if (isEditing(state)) return Decoration.set([blockSpacer(node.to)]);
  const source = interior(state, node);
  const fields = parseFrontmatter(source);
  const broken = options.isWikiLinkBroken ? wikiTargets(fields).filter((t) => t && options.isWikiLinkBroken(t)) : [];
  const widget = new FrontmatterCardWidget(
    source,
    fields,
    state.field(frontmatterCollapseField, false),
    !state.readOnly,
    broken,
    options
  );
  return Decoration.set([Decoration.replace({ widget, block: true }).range(node.from, node.to)]);
}

export function frontmatterCardField(options = {}) {
  const cardField = StateField.define({
    create(state) {
      return { decorations: build(state, options), tree: syntaxTree(state) };
    },
    update(value, tr) {
      const tree = syntaxTree(tr.state);
      const rebuild =
        tr.docChanged ||
        selectionMatters(tr) ||
        tree !== value.tree ||
        revealToggled(tr.startState, tr.state) ||
        tr.effects.some((e) => e.is(setFrontmatterCollapsed) || e.is(wikiLinksRevalidated));
      return rebuild ? { decorations: build(tr.state, options), tree } : value;
    },
    provide: (f) => EditorView.decorations.from(f, (v) => v.decorations),
  });
  // While the source is open, block-hold.js keeps the card's height.
  const held = heldBlocks.of((state) => {
    const node = findFrontmatterNode(state);
    if (!node) return [];
    const collapsed = state.field(frontmatterCollapseField, false);
    const source = interior(state, node);
    const editing = isEditing(state);
    // The estimate only matters for a card never measured before editing.
    const estimate = editing ? estimateHeight(collapsed, parseFrontmatter(source).length) : 0;
    return [{ from: node.from, to: node.to, key: 'frontmatter\n' + collapsed + '\n' + source, editing, estimate }];
  });
  return [frontmatterEditingField, keepCaretOut, keepEditsOut, cardField, held, blockHold];
}

// livePreviewPlugin hook: the source shown while editing looks like a code block.
// Returns false so the walk doesn't descend into the YAML tree.
export function decorateFrontmatterSource(node, view, decos) {
  const { state } = view;
  if (!isEditing(state)) return false;
  const first = state.doc.lineAt(node.from).number;
  const last = state.doc.lineAt(node.to).number;
  for (let n = first; n <= last; n++) {
    const classes = ['cm-lp-codeblock'];
    if (n === first) classes.push('cm-lp-codeblock-first', 'cm-lp-codeblock-fence');
    if (n === last) classes.push('cm-lp-codeblock-last', 'cm-lp-codeblock-fence');
    decos.push(Decoration.line({ class: classes.join(' ') }).range(state.doc.line(n).from));
  }
  return false;
}
