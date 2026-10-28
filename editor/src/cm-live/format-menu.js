import { EditorView, ViewPlugin } from '@codemirror/view';
import {
  toggleBold, toggleItalic, toggleStrikethrough, toggleInlineCode, insertLink,
  inlineFormatActiveAt, inlineCodeActiveAt,
} from './inline-format.js';
import { toggleBulletList, toggleOrderedList, toggleTaskList, listFormatActiveAt } from './list-format.js';
import { toggleBlockquote, quoteFormatActiveAt } from './quote-format.js';
import { rangeTouchesCode, rangeTouchesCodeBlock, rangeCrossesBlock, trimWhitespace, lineInsideCodeBlock } from './format-guards.js';
import { FORMAT_KEYS } from './format-keymap.js';
import { charPosAtCoords, wordRangeAt } from './cjk-words.js';
import { touchedLines } from './selection-lines.js';
import { lineIcon } from '../line-icons.js';

// Right-click formatting menu. There's no native context menu to coexist
// with here — the desktop shell never wires up `webContents.on('context-
// menu', ...)`, so `contextmenu`'s preventDefault() below is all that's
// needed to fully own the gesture; no main-process changes required.
//
// The popup is appended to document.body (not view.dom) and positioned
// `fixed`, so it escapes .cm-scroller's overflow clipping and stays
// viewport-relative no matter what transform an app-level panel container
// might apply for its own animations. That placement is also why this
// doesn't use EditorView.theme() for styling: theme() rules are always
// compiled as descendants of the .cm-editor root (see @codemirror/view's
// buildTheme), so they'd never match an element living outside it. A plain
// injected <style> is used instead.
//
// Visually this is a deliberate copy of content_pane.css's
// .content-pane-more-menu/.content-pane-more-item/.content-pane-more-divider
// (same padding/gap/font-size/icon-size/hover, same --line-hair inset
// divider) — cm-live/ can't reference those classes directly (app-agnostic
// library, no Hylo DOM/class dependencies), so the rules are duplicated
// here instead, using the same var-with-fallback convention as theme.js.
const STYLE_ID = 'cm-format-menu-styles';
function ensureStyles() {
  if (document.getElementById(STYLE_ID)) return;
  const style = document.createElement('style');
  style.id = STYLE_ID;
  style.textContent = `
.cm-format-menu {
  position: fixed;
  z-index: 1000;
  min-width: 190px;
  max-width: min(320px, 90vw);
  padding: 0.25rem;
  background: var(--surface-soft, #ffffff);
  border: var(--bd-w, 1px) solid var(--line-edge, rgba(24,24,27,0.16));
  border-radius: var(--r-lg, 12px);
  box-shadow: var(--shadow-sm, 0 4px 16px) var(--shadow-color, rgba(0,0,0,0.16));
  font-family: var(--font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
}
.cm-format-menu-row {
  display: flex; align-items: center; gap: 0.45rem;
  width: 100%; padding: 0.4rem 0.55rem; margin: 0;
  border: none; background: transparent; text-align: left;
  border-radius: var(--r-sm, 6px);
  color: var(--muted, #6d7080);
  font: inherit;
  font-size: var(--text-sm, 0.8125rem);
  white-space: nowrap;
  cursor: pointer;
}
.cm-format-menu-row svg { width: 13px; height: 13px; flex-shrink: 0; }
.cm-format-menu-kbd { margin-left: auto; padding-left: 1.5rem; color: var(--muted-soft, #9b9eac); font-size: var(--text-xs, 0.75rem); }
.cm-format-menu-row:hover:not(:disabled) { color: var(--fg, #1a1a1a); background: var(--icon-hov, rgba(0,0,0,0.06)); }
.cm-format-menu-row:disabled { opacity: 0.4; cursor: default; }
.cm-format-menu-row.is-active,
.cm-format-menu-row.is-active:hover { color: var(--accent-text, #5e6ad2); background: var(--tint-soft, rgba(94,106,210,0.1)); }
/* Inset to the rows' own text padding, not the menu's edge, so the line
   reads as a quiet seam under the content rather than a bar spanning wider
   than it — same rule and same --line-hair token as
   .content-pane-more-divider (that div sits inside a panel that already has
   its own border, so even --line-edge is too much weight here; same logic
   applies to this popup). */
.cm-format-menu-sep { margin: 0.25rem 0.55rem; border-top: var(--bd-w, 1px) solid var(--line-hair, rgba(24,24,27,0.09)); }
`;
  document.head.appendChild(style);
}

const ICONS = {
  bold: lineIcon('bold'),
  italic: lineIcon('italic'),
  strike: lineIcon('strikethrough'),
  bullet: lineIcon('bullet-list'),
  ordered: lineIcon('ordered-list'),
  task: lineIcon('task-list'),
  cut: lineIcon('cut'),
  copy: lineIcon('copy'),
  paste: lineIcon('paste'),
  code: lineIcon('code'),
  link: lineIcon('link'),
  quote: lineIcon('quote'),
};

const IS_MAC = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);

function keyHint(key) {
  if (!key) return '';
  const parts = key.split('-');
  const last = parts.pop().toUpperCase();
  if (IS_MAC) {
    const sym = { Mod: '⌘', Shift: '⇧', Alt: '⌥', Ctrl: '⌃' };
    return parts.map((p) => sym[p]).sort((a, b) => '⌃⌥⇧⌘'.indexOf(a) - '⌃⌥⇧⌘'.indexOf(b)).join('') + last;
  }
  return parts.map((p) => (p === 'Mod' ? 'Ctrl' : p)).concat(last).join('+');
}

// execCommand routes through CM's own copy/cut handlers (line-wise copy, undo
// event); writeText is the fallback where the command is refused.
function clipboardCommand(cmd) {
  return (view) => {
    const sel = view.state.selection.main;
    if (document.execCommand(cmd)) return true;
    if (!navigator.clipboard) return false;
    void navigator.clipboard.writeText(view.state.sliceDoc(sel.from, sel.to));
    if (cmd === 'cut') view.dispatch({ changes: { from: sel.from, to: sel.to }, userEvent: 'delete.cut' });
    return true;
  };
}

// WebKit may show its own "Paste" confirmation before readText resolves.
function pasteFromClipboard(view) {
  navigator.clipboard.readText().then((text) => {
    if (!text) return;
    view.dispatch(view.state.replaceSelection(text), { userEvent: 'input.paste', scrollIntoView: true });
  }).catch(() => {});
  return true;
}

const canPaste = () => !!(navigator.clipboard && navigator.clipboard.readText);

// Each action owns its own isActive check instead of branching on `group` —
// group only decides menu layout (which section it's in, and whether it
// needs real selected text to be enabled), not which state it reflects.
const ACTIONS = [
  { id: 'cut', label: 'Cut', group: 'clipboard', key: 'Mod-x', run: clipboardCommand('cut'), isActive: () => false },
  { id: 'copy', label: 'Copy', group: 'clipboard', key: 'Mod-c', run: clipboardCommand('copy'), isActive: () => false },
  { id: 'paste', label: 'Paste', group: 'clipboard', key: 'Mod-v', run: pasteFromClipboard, isActive: () => false },
  { id: 'bold', label: 'Bold', group: 'inline', key: FORMAT_KEYS.bold, run: toggleBold, isActive: (state, sel) => inlineFormatActiveAt(state, sel).bold },
  { id: 'italic', label: 'Italic', group: 'inline', key: FORMAT_KEYS.italic, run: toggleItalic, isActive: (state, sel) => inlineFormatActiveAt(state, sel).italic },
  { id: 'strike', label: 'Strikethrough', group: 'inline', key: FORMAT_KEYS.strike, run: toggleStrikethrough, isActive: (state, sel) => inlineFormatActiveAt(state, sel).strike },
  { id: 'code', label: 'Code', group: 'inline', key: FORMAT_KEYS.code, run: toggleInlineCode, isActive: inlineCodeActiveAt },
  { id: 'link', label: 'Link', group: 'inline', key: FORMAT_KEYS.link, run: insertLink, isActive: () => false },
  { id: 'bullet', label: 'Bullet List', group: 'block', run: toggleBulletList, isActive: (state, sel) => listFormatActiveAt(state, sel) === 'bullet' },
  { id: 'ordered', label: 'Numbered List', group: 'block', run: toggleOrderedList, isActive: (state, sel) => listFormatActiveAt(state, sel) === 'ordered' },
  { id: 'task', label: 'Checklist', group: 'block', run: toggleTaskList, isActive: (state, sel) => listFormatActiveAt(state, sel) === 'task' },
  { id: 'quote', label: 'Quote', group: 'block', run: toggleBlockquote, isActive: quoteFormatActiveAt },
];

class FormatMenuPlugin {
  constructor(view) {
    this.view = view;
    this.dom = null;
    this._isOpen = false;
    this._onWindowMouseDown = this._onWindowMouseDown.bind(this);
    this._onWindowKeyDown = this._onWindowKeyDown.bind(this);
    this._onWindowScroll = this._onWindowScroll.bind(this);
  }

  update(update) {
    // Any of these mean the menu's premise (this selection, this text) is
    // stale — close rather than leave it pointing at content that moved.
    if (this._isOpen && (update.selectionSet || update.docChanged || update.focusChanged)) this.close();
  }

  destroy() {
    this.close();
    if (this.dom) this.dom.remove();
  }

  _dom() {
    if (this.dom) return this.dom;
    ensureStyles();
    const dom = document.createElement('div');
    dom.className = 'cm-format-menu';
    dom.style.display = 'none';
    document.body.appendChild(dom);
    this.dom = dom;
    return dom;
  }

  open(event) {
    const view = this.view;
    const pos = charPosAtCoords(view, event.clientX, event.clientY);
    // Right-click on a bare cursor (no selection) over a word auto-selects
    // it first, same as most editors — makes the inline actions usable
    // immediately instead of a dead click. CJK goes by segmented words.
    if (view.state.selection.main.empty && pos != null) {
      const word = wordRangeAt(view.state, pos);
      if (word) view.dispatch({ selection: word });
    }
    const sel = view.state.selection.main;
    // Mirrors the guards inline-format.js's toggleWrap itself runs, so a
    // disabled row and a no-op click never disagree: needs real, non-
    // whitespace, single-block, non-code text to do anything sensible.
    const trimmedSel = sel.empty ? sel : trimWhitespace(view.state, sel.from, sel.to);
    const inlineDisabled = sel.empty
      || trimmedSel.from >= trimmedSel.to
      || rangeTouchesCode(view.state, sel.from, sel.to)
      || rangeCrossesBlock(view.state, sel.from, sel.to);
    // Mirrors list-format.js/quote-format.js's own code-block guard.
    const blockDisabled = touchedLines(view.state).some(line => lineInsideCodeBlock(view.state, line));
    const disabled = {
      cut: sel.empty,
      copy: sel.empty,
      paste: !canPaste(),
      // Inline code toggles off an existing span, so only block code rules it out.
      code: sel.empty || trimmedSel.from >= trimmedSel.to
        || rangeTouchesCodeBlock(view.state, sel.from, sel.to)
        || rangeCrossesBlock(view.state, sel.from, sel.to),
    };

    const dom = this._dom();
    dom.textContent = '';
    let lastGroup = null;
    for (const action of ACTIONS) {
      if (lastGroup && action.group !== lastGroup) {
        const sep = document.createElement('div');
        sep.className = 'cm-format-menu-sep';
        dom.appendChild(sep);
      }
      lastGroup = action.group;

      const row = document.createElement('button');
      row.type = 'button';
      row.className = 'cm-format-menu-row';
      // ICONS/label are fixed internal constants, never user data.
      row.innerHTML = ICONS[action.id] + '<span>' + action.label + '</span>'
        + '<span class="cm-format-menu-kbd">' + keyHint(action.key) + '</span>';
      // Inline actions need real, formattable selected text; block actions
      // (list/quote) are line-level and work from a bare cursor
      // (Notion/Obsidian-style "make this line a list/quote"), so only the
      // code-block check applies to them.
      const off = action.id in disabled ? disabled[action.id] : action.group === 'inline' ? inlineDisabled : blockDisabled;
      if (off) row.disabled = true;
      if (action.isActive(view.state, sel)) row.classList.add('is-active');
      // preventDefault keeps the editor focused/selected through the click —
      // without it, the default mousedown blur fires update() with
      // focusChanged (closing the menu) before the click event ever lands.
      row.addEventListener('mousedown', e => e.preventDefault());
      row.addEventListener('click', () => {
        action.run(view);
        this.close();
        view.focus();
      });
      dom.appendChild(row);
    }

    dom.style.display = 'block';
    dom.style.left = event.clientX + 'px';
    dom.style.top = event.clientY + 'px';
    const rect = dom.getBoundingClientRect();
    const overflowX = rect.right - window.innerWidth;
    const overflowY = rect.bottom - window.innerHeight;
    if (overflowX > 0) dom.style.left = Math.max(0, event.clientX - overflowX) + 'px';
    if (overflowY > 0) dom.style.top = Math.max(0, event.clientY - overflowY) + 'px';

    this._isOpen = true;
    window.addEventListener('mousedown', this._onWindowMouseDown, true);
    window.addEventListener('keydown', this._onWindowKeyDown, true);
    window.addEventListener('scroll', this._onWindowScroll, true);
  }

  close() {
    if (!this._isOpen) return;
    this._isOpen = false;
    if (this.dom) this.dom.style.display = 'none';
    window.removeEventListener('mousedown', this._onWindowMouseDown, true);
    window.removeEventListener('keydown', this._onWindowKeyDown, true);
    window.removeEventListener('scroll', this._onWindowScroll, true);
  }

  _onWindowMouseDown(e) {
    if (this.dom && this.dom.contains(e.target)) return;
    this.close();
  }

  _onWindowKeyDown(e) {
    if (e.key === 'Escape') this.close();
  }

  _onWindowScroll() {
    this.close();
  }
}

const formatMenuPlugin = ViewPlugin.fromClass(FormatMenuPlugin);

export function selectionFormatMenu() {
  return [
    formatMenuPlugin,
    EditorView.domEventHandlers({
      contextmenu(event, view) {
        // readOnly doesn't block dispatch, so the menu would still rewrite the doc.
        if (view.state.readOnly) return false;
        event.preventDefault();
        view.plugin(formatMenuPlugin).open(event);
        return true;
      },
    }),
  ];
}
