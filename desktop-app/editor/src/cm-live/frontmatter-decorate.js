// ── Frontmatter — in-place line decorations (no card widget). Regex only
// for flat key:value / flow arrays / indented continuations.
import { Decoration } from '@codemirror/view';
import { selectionTouchesRange } from './selection.js';
import { frontmatterCollapseField } from './frontmatter-collapse.js';
import {
  FrontmatterCollapsedLineWidget,
  FrontmatterMoreWidget,
  FrontmatterKeyIconWidget,
  FrontmatterListIndentWidget,
} from './widgets.js';
import { hideRange, styleRange } from './decoration-helpers.js';

const FM_KEY_LINE_RE = /^([A-Za-z0-9_.-]+)(:)(\s*)([\s\S]*)$/;
const FM_URL_RE = /^https?:\/\//i;
const FM_CONTINUATION_RE = /^[ \t]/;
const FM_NUMBER_RE = /^-?\d+(\.\d+)?$/;
// Arrays/lists over this many items collapse behind "+N more" (whole
// frontmatter block always renders in full now — only long array/list
// values get truncated, so a note with lots of tags stays scannable).
const FM_ARRAY_VISIBLE_ITEMS = 3;

// One group per top-level field (continuations fold in) so array/list groups
// can be told apart from plain scalar fields for icon choice + truncation.
function fmFieldGroups(doc, fromLineNo, toLineNo) {
  const groups = [];
  let current = null;
  for (let n = fromLineNo; n <= toLineNo; n++) {
    const text = doc.line(n).text;
    // A blank line (some YAML list authors leave one between "- item"
    // entries) folds into whatever group is open instead of starting a new
    // one-line "field" — otherwise it renders as a phantom full-height row
    // with a stray key icon and nothing next to it.
    const isBlank = text.trim().length === 0;
    if (current && (isBlank || FM_CONTINUATION_RE.test(text))) {
      current.toNo = n;
    } else {
      current = { fromNo: n, toNo: n };
      groups.push(current);
    }
  }
  return groups;
}

function decorateFmArrayValue(rawValue, valueFrom, valueTo, decos, state) {
  const m = /^\[([\s\S]*)\]\s*$/.exec(rawValue);
  if (!m) return false;
  const closeIdx = rawValue.lastIndexOf(']');
  const inner = rawValue.slice(1, closeIdx);
  hideRange(valueFrom, valueFrom + 1, decos);
  const parts = inner.split(',');
  // First pass: item ranges only, so overflow/reveal can be decided before
  // any decoration is pushed.
  const items = [];
  let pos = valueFrom + 1;
  for (let i = 0; i < parts.length; i++) {
    const raw = parts[i];
    const trimmed = raw.trim();
    const leadingWs = raw.length - raw.trimStart().length;
    const itemFrom = pos + leadingWs;
    const itemTo = itemFrom + trimmed.length;
    const segEnd = pos + raw.length;
    items.push({ itemFrom, itemTo });
    pos = segEnd + (i < parts.length - 1 ? 1 : 0); // +1 for the comma
  }
  const overflow = items.length > FM_ARRAY_VISIBLE_ITEMS;
  const revealed = overflow && selectionTouchesRange(state, items[FM_ARRAY_VISIBLE_ITEMS].itemFrom, valueTo);
  const visibleCount = overflow && !revealed ? FM_ARRAY_VISIBLE_ITEMS : items.length;

  pos = valueFrom + 1;
  for (let i = 0; i < visibleCount; i++) {
    const it = items[i];
    hideRange(pos, it.itemFrom, decos);
    if (it.itemTo > it.itemFrom) styleRange(it.itemFrom, it.itemTo, 'cm-lp-fm-tag', decos);
    pos = it.itemTo;
  }
  if (overflow && !revealed) {
    decos.push(
      Decoration.widget({
        widget: new FrontmatterMoreWidget(items.length - visibleCount, items[visibleCount].itemFrom),
        side: 1,
      }).range(pos)
    );
  }
  hideRange(pos, valueTo, decos);
  return true;
}

// ×1.15: letter-spacing + uppercase widen beyond plain ch count.
function frontmatterLabelWidthCh(doc, fromLineNo, toLineNo) {
  let maxLen = 0;
  for (let n = fromLineNo; n <= toLineNo; n++) {
    const m = FM_KEY_LINE_RE.exec(doc.line(n).text);
    if (m) maxLen = Math.max(maxLen, m[1].length + m[2].length);
  }
  return Math.ceil(maxLen * 1.15) + 1;
}

// System-defined keys get a fixed icon regardless of value shape — value-type
// icons (tag/list/number/text) below are the fallback for everything else.
const FM_KEY_ICON_OVERRIDES = {
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

// Type drives which icon renders next to the key — 'tags' is special-cased
// (Obsidian does the same), other arrays/lists get the generic list icon.
function fmFieldType(key, rawValue, isArrayGroup) {
  const override = FM_KEY_ICON_OVERRIDES[key.trim().toLowerCase()];
  if (override) return override;
  if (isArrayGroup) return key.trim().toLowerCase() === 'tags' ? 'tag' : 'list';
  return FM_NUMBER_RE.test(rawValue.trim()) ? 'number' : 'text';
}

function decorateFmFieldLine(text, lineFrom, decos, labelWidthCh, type, state) {
  const m = FM_KEY_LINE_RE.exec(text);
  if (!m) {
    decos.push(Decoration.widget({ widget: new FrontmatterKeyIconWidget(type || 'text'), side: -1 }).range(lineFrom));
    if (text.trim().length > 0) styleRange(lineFrom, lineFrom + text.length, 'cm-lp-fm-val', decos);
    return;
  }
  const [, key, colon, gap, rawValue] = m;
  const keyTo = lineFrom + key.length;
  const colonTo = keyTo + colon.length;
  decos.push(Decoration.widget({ widget: new FrontmatterKeyIconWidget(type || 'text'), side: -1 }).range(lineFrom));
  decos.push(
    Decoration.mark({ class: 'cm-lp-fm-label', attributes: { style: 'width:' + labelWidthCh + 'ch' } }).range(
      lineFrom,
      colonTo
    )
  );
  styleRange(lineFrom, keyTo, 'cm-lp-fm-key', decos);
  styleRange(keyTo, colonTo, 'cm-lp-fm-colon', decos);
  const valueFrom = colonTo + gap.length;
  const valueTo = lineFrom + text.length;
  if (valueFrom >= valueTo) return;
  if (decorateFmArrayValue(rawValue, valueFrom, valueTo, decos, state)) return;
  const cls = FM_URL_RE.test(rawValue.trim()) ? 'cm-lp-fm-val cm-lp-fm-link' : 'cm-lp-fm-val';
  styleRange(valueFrom, valueTo, cls, decos);
}

function decorateFmContinuationLine(text, lineFrom, decos, labelWidthCh) {
  if (text.trim().length === 0) return;
  const m = /^(\s*)(-\s?)?([\s\S]*)$/.exec(text);
  const indent = m[1];
  const bullet = m[2] || '';
  const pos = lineFrom + indent.length + bullet.length;
  // Replace the raw indent+"- " with an invisible icon+label-shaped spacer
  // instead of hiding it and padding-left-ing the line to compensate — the
  // real label renders at a smaller font-size than the line itself, so a
  // ch-based calc() on the line's padding (sized in the line's own font)
  // can't reproduce the label's actual (smaller-font) width. Reusing the
  // identical icon/label markup, just invisible, measures itself the same
  // way the real one does and lines up with it pixel-for-pixel.
  decos.push(Decoration.replace({ widget: new FrontmatterListIndentWidget(labelWidthCh) }).range(lineFrom, pos));
  const textEnd = lineFrom + text.length;
  if (pos < textEnd) styleRange(pos, textEnd, 'cm-lp-fm-val cm-lp-fm-list-item', decos);
}

export function decorateFrontmatter(node, view, decos) {
  const state = view.state;
  const doc = state.doc;
  const fmFrom = node.from;
  const fmTo = node.to;
  const firstLine = doc.lineAt(fmFrom);
  const lastLine = doc.lineAt(fmTo);

  if (state.field(frontmatterCollapseField, false)) {
    // Whole block collapsed via the "Metadata" header toggle — hairline
    // every line individually rather than one multi-line replace (that's
    // the known CM6 height-map crash this file works around everywhere
    // else too; see fmFieldGroups' blank-line handling above).
    for (let n = firstLine.number; n <= lastLine.number; n++) {
      const line = doc.line(n);
      decos.push(Decoration.line({ class: 'cm-lp-fm-line cm-lp-fm-collapsed' }).range(line.from));
      if (line.length > 0) {
        decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(line.from, line.to));
      } else {
        decos.push(Decoration.widget({ widget: new FrontmatterCollapsedLineWidget(), side: 1 }).range(line.from));
      }
    }
    return;
  }

  // Unclosed → no closing "---"; rest is interior.
  const hasClosingDelimiter = lastLine.number > firstLine.number && doc.line(lastLine.number).text.trim() === '---';
  const interiorFromNo = firstLine.number + 1;
  const interiorToNo = hasClosingDelimiter ? lastLine.number - 1 : lastLine.number;
  const hasInterior = interiorToNo >= interiorFromNo;
  const groups = hasInterior ? fmFieldGroups(doc, interiorFromNo, interiorToNo) : [];
  const labelWidthCh = hasInterior ? frontmatterLabelWidthCh(doc, interiorFromNo, interiorToNo) : 0;

  // Frontmatter is read-only in this view now (frontmatter-readonly.js) —
  // always the decorated/collapsed form, never raw syntax on caret entry;
  // editing happens through the header's edit button (an app-level dialog).
  decos.push(Decoration.line({ class: 'cm-lp-fm-line cm-lp-fm-first cm-lp-fm-collapsed' }).range(firstLine.from));
  decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(firstLine.from, firstLine.to));

  groups.forEach((group) => {
    const keyLineText = doc.line(group.fromNo).text;

    // A blank line with nothing open before it (rare — e.g. right after the
    // opening "---") ends up as its own one-line "group"; render it as a
    // hairline instead of a field row so it doesn't get a stray key icon.
    if (keyLineText.trim().length === 0) {
      const blankLine = doc.line(group.fromNo);
      const classes = ['cm-lp-fm-line', 'cm-lp-fm-collapsed'];
      if (!hasClosingDelimiter && group.fromNo === interiorToNo) classes.push('cm-lp-fm-last');
      decos.push(Decoration.line({ class: classes.join(' ') }).range(blankLine.from));
      // A truly empty line has nothing for CM6 to measure against, so its
      // height falls back to a full line instead of the shrunk hairline —
      // same widgetBuffer anchor the non-empty collapse branches use below.
      decos.push(Decoration.widget({ widget: new FrontmatterCollapsedLineWidget(), side: 1 }).range(blankLine.from));
      return;
    }

    const keyMatch = FM_KEY_LINE_RE.exec(keyLineText);
    const isMultilineList = group.toNo > group.fromNo;
    const isFlowArray = keyMatch ? /^\[[\s\S]*\]\s*$/.test(keyMatch[4].trim()) : false;
    const isArrayGroup = isMultilineList || isFlowArray;
    const type = keyMatch ? fmFieldType(keyMatch[1], keyMatch[4], isArrayGroup) : 'text';

    // Key line (always rendered — no more whole-block collapsing).
    const keyLine = doc.line(group.fromNo);
    const keyLineClasses = ['cm-lp-fm-line'];
    if (!hasClosingDelimiter && group.fromNo === interiorToNo) keyLineClasses.push('cm-lp-fm-last');
    decos.push(Decoration.line({ class: keyLineClasses.join(' ') }).range(keyLine.from));
    decorateFmFieldLine(keyLine.text, keyLine.from, decos, labelWidthCh, type, state);

    if (!isMultilineList) return;

    // Block-list items (indented "- item" continuation lines) — collapse
    // past FM_ARRAY_VISIBLE_ITEMS, same "+N more" affordance as flow arrays.
    // Blank lines a list author left between entries render as a hairline
    // (allLineNos) but don't count toward the visible-item cutoff (itemLineNos).
    const allLineNos = [];
    for (let n = group.fromNo + 1; n <= group.toNo; n++) allLineNos.push(n);
    const itemLineNos = allLineNos.filter((n) => doc.line(n).text.trim().length > 0);
    const overflow = itemLineNos.length > FM_ARRAY_VISIBLE_ITEMS;
    const revealed =
      overflow && selectionTouchesRange(state, doc.line(itemLineNos[FM_ARRAY_VISIBLE_ITEMS]).from, doc.line(group.toNo).to);
    const visibleCount = overflow && !revealed ? FM_ARRAY_VISIBLE_ITEMS : itemLineNos.length;

    allLineNos.forEach((n) => {
      const line = doc.line(n);
      const isLastRenderedLine = !hasClosingDelimiter && n === interiorToNo;
      const lineClasses = ['cm-lp-fm-line'];
      if (isLastRenderedLine) lineClasses.push('cm-lp-fm-last');
      const isBlank = line.text.trim().length === 0;
      const idx = isBlank ? -1 : itemLineNos.indexOf(n);

      // First hidden item's own line carries "+N more" as a normal,
      // left-aligned row (same invisible icon+label indent as a real item)
      // instead of collapsing to a hairline like the rest of the overflow.
      if (overflow && !revealed && idx === visibleCount) {
        decos.push(Decoration.line({ class: lineClasses.join(' ') }).range(line.from));
        const m = /^(\s*)(-\s?)?([\s\S]*)$/.exec(line.text);
        const prefixEnd = line.from + m[1].length + (m[2] || '').length;
        decos.push(
          Decoration.replace({ widget: new FrontmatterListIndentWidget(labelWidthCh) }).range(line.from, prefixEnd)
        );
        decos.push(
          Decoration.replace({
            widget: new FrontmatterMoreWidget(itemLineNos.length - visibleCount, line.from, true),
          }).range(prefixEnd, line.to)
        );
        return;
      }

      if (isBlank || idx >= visibleCount) {
        decos.push(Decoration.line({ class: lineClasses.concat('cm-lp-fm-collapsed').join(' ') }).range(line.from));
        if (line.length > 0) {
          decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(line.from, line.to));
        } else {
          // Zero-length (blank) line — same widgetBuffer anchor, as a point
          // widget instead of a replace since there's no range to consume.
          decos.push(Decoration.widget({ widget: new FrontmatterCollapsedLineWidget(), side: 1 }).range(line.from));
        }
        return;
      }

      decos.push(Decoration.line({ class: lineClasses.join(' ') }).range(line.from));
      decorateFmContinuationLine(line.text, line.from, decos, labelWidthCh);
    });
  });

  if (hasClosingDelimiter) {
    decos.push(Decoration.line({ class: 'cm-lp-fm-line cm-lp-fm-last cm-lp-fm-collapsed' }).range(lastLine.from));
    decos.push(Decoration.replace({ widget: new FrontmatterCollapsedLineWidget() }).range(lastLine.from, lastLine.to));
  }
}
