// A server-side agent types into the live editor through ordinary transactions and caret.
// Its text goes in whole and unseen and is then revealed: a block typed piece by piece
// reshaped (column widths, wrapping) and moved everything around it at every step.
import { EditorState, StateField, StateEffect, Transaction, Annotation, EditorSelection, Prec } from '@codemirror/state';
import { Decoration, EditorView, BlockWrapper, BlockType } from '@codemirror/view';
import { ensureSyntaxTree, syntaxTree } from '@codemirror/language';
import { isolateHistory } from '@codemirror/commands';
import { completionStatus, closeCompletion } from '@codemirror/autocomplete';
import { findFrontmatterNode } from './frontmatter-syntax.js';
import { createRhythm } from './agent-rhythm.js';
import { agentCaret } from './selection.js';
import { docSynced } from './doc-sync.js';
import { glideTo, scrollDestination, measureRange } from './scroll.js';

// While the agent writes it owns the editor: its caret is the selection, and nothing
// else may move or edit it. Taking over again means stopping the agent first.
const setAgentWriting = StateEffect.define();

const agentWritingField = StateField.define({
  create: () => false,
  update(on, tr) {
    for (const e of tr.effects) if (e.is(setAgentWriting)) return e.value;
    return on;
  },
  provide: (f) => [
    EditorView.editable.compute([f], (state) => !state.field(f)),
    EditorView.editorAttributes.compute([f], (state) => (state.field(f) ? { class: 'cm-agent-writing' } : {})),
  ],
});

const agentWrite = Annotation.define();

const agentLock = EditorState.transactionFilter.of((tr) => (
  tr.startState.field(agentWritingField, false) && (tr.docChanged || tr.selection) && !tr.annotation(agentWrite) ? [] : tr
));

// The caret stays the agent's, revealing nothing, until the user moves it or edits:
// the text it just wrote stays as it was watched.
const agentCaretField = StateField.define({
  create: () => false,
  update(on, tr) {
    for (const e of tr.effects) if (e.is(setAgentWriting) && e.value) return true;
    if (on && (tr.selection || tr.docChanged) && !tr.annotation(agentWrite) && !tr.annotation(docSynced)) return false;
    return on;
  },
  provide: (f) => agentCaret.from(f),
});

// The written text not yet revealed. Whole lines and block widgets go in a hidden
// wrapper; the rest of the line being typed is hidden by an innermost mark, which keeps
// table cells and other marked boxes intact around it.
const setPending = StateEffect.define();

const pendingField = StateField.define({
  create: () => null,
  update(range, tr) {
    for (const e of tr.effects) if (e.is(setPending)) return e.value;
    if (!range || !tr.docChanged) return range;
    const from = tr.changes.mapPos(range.from, 1);
    const to = tr.changes.mapPos(range.to, -1);
    return to > from ? { from, to } : null;
  },
  provide: (f) => [
    Prec.highest(EditorView.decorations.from(f, (r) => (
      r ? Decoration.set(Decoration.mark({ class: 'cm-agent-pending' }).range(r.from, r.to)) : Decoration.none
    ))),
    EditorView.blockWrappers.compute([f], (state) => {
      const r = state.field(f);
      if (!r) return BlockWrapper.set([]);
      const doc = state.doc;
      const first = doc.lineAt(r.from);
      const from = r.from === first.from ? first.from : first.to + 1;
      const last = doc.lineAt(r.to);
      const to = r.to === last.to ? last.to : last.from - 1;
      return to >= from ? BlockWrapper.set(pendingBlock.range(from, to)) : BlockWrapper.set([]);
    }),
  ],
});

const pendingBlock = BlockWrapper.create({ tagName: 'div', attributes: { class: 'cm-agent-pending-block' } });

const pendingTheme = EditorView.baseTheme({
  '.cm-agent-pending, .cm-agent-pending-block': { visibility: 'hidden' },
});

// The drawn caret shows unfocused too: the user usually watches from the chat composer.
const agentCaretTheme = EditorView.baseTheme({
  '&.cm-agent-writing > .cm-scroller > .cm-cursorLayer': { animation: 'steps(1) cm-blink 1.2s infinite' },
  '&.cm-agent-writing > .cm-scroller > .cm-cursorLayer .cm-cursor-primary': {
    display: 'block', borderLeft: '2px solid var(--accent, #5e6ad2)', marginLeft: '-1px',
  },
  '&.cm-agent-writing .cm-cursor-primary::after': {
    content: '"AI"', position: 'absolute', left: '-2px', bottom: '100%',
    padding: '0 4px', borderRadius: 'var(--r-xs, 4px)', font: "10px/14px var(--font-ui, system-ui, sans-serif)", whiteSpace: 'nowrap',
    color: 'var(--accent-fg, #fff)', background: 'var(--accent, #5e6ad2)',
  },
});

// The region about to be rewritten, shown briefly before it is replaced.
export const setAgentTarget = StateEffect.define();

const agentTargetField = StateField.define({
  create: () => null,
  update(range, tr) {
    for (const e of tr.effects) if (e.is(setAgentTarget)) return e.value;
    if (!range) return null;
    const from = tr.changes.mapPos(range.from, 1);
    const to = tr.changes.mapPos(range.to, -1);
    return to > from ? { from, to } : null;
  },
  provide: (f) => EditorView.decorations.from(f, (r) => (
    r ? Decoration.set([Decoration.mark({ class: 'cm-agent-target' }).range(r.from, r.to)]) : Decoration.none
  )),
});

const agentTargetTheme = EditorView.baseTheme({
  '.cm-agent-target': {
    background: 'color-mix(in srgb, var(--accent, #5e6ad2) 22%, transparent)',
    borderRadius: '2px',
  },
});

export const agentWritingExtension = [
  agentWritingField, agentLock, agentCaretField, agentCaretTheme, agentTargetField, agentTargetTheme, pendingField, pendingTheme,
];
export const agentWriting = (state) => state.field(agentWritingField, false) ?? false;

const HEADING_RE = /^ {0,3}(#{1,6})[ \t]+(.*?)[ \t#]*$/;
const FENCE_RE = /^ {0,3}(`{3,}|~{3,})/;

// End of the last non-blank line of the section under `heading`, or -1.
function sectionEnd(doc, heading, minPos) {
  const want = heading.replace(/^#+\s*/, '').trim().toLowerCase();
  let level = 0;
  let end = -1;
  let fence = null;
  for (let n = 1; n <= doc.lines; n++) {
    const line = doc.line(n);
    if (line.to < minPos) continue;
    const f = FENCE_RE.exec(line.text);
    if (f) {
      if (!fence) fence = f[1];
      else if (f[1][0] === fence[0] && f[1].length >= fence.length) fence = null;
    }
    const h = !fence && !f ? HEADING_RE.exec(line.text) : null;
    if (!level) {
      if (h && h[2].toLowerCase() === want) { level = h[1].length; end = line.to; }
    } else if (h && h[1].length <= level) {
      break;
    } else if (line.text.trim()) {
      end = line.to;
    }
  }
  return end;
}

// A note whose first line starts with "---" parses as frontmatter, which the agent never writes.
const FRONTMATTER_ERROR = 'the note would start with "---", which is frontmatter; editor tools never write frontmatter';
const opensFrontmatter = (doc, pos, text, to = pos) => pos === 0 && /^---/.test(text + doc.sliceString(to, to + 3));

const trailing = (s, ch, max) => { let n = 0; while (n < max && s[s.length - 1 - n] === ch) n++; return n; };
const leading = (s, ch, max) => { let n = 0; while (n < max && s[n] === ch) n++; return n; };

// Where `body` goes and the text to type, with blank lines added so it lands as its own block.
export function planAgentInsert(state, { at, heading } = {}, body = '') {
  const doc = state.doc;
  const fm = findFrontmatterNode(state);
  const minPos = fm ? Math.min(fm.to + 1, doc.length) : 0;

  let pos;
  if (heading) {
    pos = sectionEnd(doc, heading, minPos);
    if (pos < 0) return { error: `heading not found: ${heading}` };
  } else if (at === 'cursor') {
    pos = Math.max(state.selection.main.head, minPos);
  } else {
    pos = doc.length;
  }
  pos = Math.max(pos, minPos);

  const line = doc.lineAt(pos);
  const midLine = pos < line.to && at === 'cursor' && !heading;
  const text = body.replace(/\s+$/, '');
  if (!text) return { error: 'empty content' };
  if (opensFrontmatter(doc, pos, text)) return { error: FRONTMATTER_ERROR };
  if (midLine) return { pos, text };

  const before = doc.sliceString(0, pos);
  const after = doc.sliceString(pos);
  const hasBefore = pos > minPos && before.slice(minPos).trim() !== '';
  // A frontmatter closer with no newline after it must not get text on its own line.
  const lead = hasBefore
    ? '\n'.repeat(Math.max(0, 2 - trailing(before, '\n', 2)))
    : (pos > 0 && before[pos - 1] !== '\n' ? '\n' : '');
  const tail = pos >= doc.length
    ? '\n'
    : '\n'.repeat(Math.max(0, 2 - leading(after, '\n', 2)));
  return { pos, text: lead + text + tail };
}

const blockKind = (name) => {
  if (/^(ATX|Setext)Heading/.test(name)) return 'heading';
  if (name === 'BulletList' || name === 'OrderedList') return 'list';
  if (name === 'FencedCode' || name === 'CodeBlock') return 'code';
  if (name === 'Blockquote') return 'quote';
  if (name === 'HorizontalRule') return 'rule';
  return name.replace(/([a-z])([A-Z])/g, '$1_$2').toLowerCase();
};

const hash36 = (s) => {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) h = Math.imul(h ^ s.charCodeAt(i), 0x01000193);
  return (h >>> 0).toString(36).padStart(6, '0').slice(-6);
};

// Top-level markdown blocks. An id is derived from the block's text, so it outlives
// edits elsewhere and stops resolving once that block itself changes: a stale read
// can't point at the wrong text. Blocks of equal text are told apart by their order,
// which another copy shifts, so their ids also carry how many copies there are.
export function docBlocks(state) {
  ensureSyntaxTree(state, state.doc.length, 300);
  const nodes = [];
  const copies = new Map();
  for (let n = syntaxTree(state).topNode.firstChild; n; n = n.nextSibling) {
    if (n.name === 'Frontmatter') continue;
    const text = state.sliceDoc(n.from, n.to);
    nodes.push({ n, text });
    copies.set(text, (copies.get(text) || 0) + 1);
  }
  const seen = new Map();
  const taken = new Set();
  return nodes.map(({ n, text }) => {
    const nth = seen.get(text) || 0;
    seen.set(text, nth + 1);
    const key = copies.get(text) > 1 ? `${text}\0${nth}/${copies.get(text)}` : text;
    let id;
    for (let k = 0; taken.has(id = `b${hash36(k ? `${key}\0${k}` : key)}`); k++);
    taken.add(id);
    return { id, kind: blockKind(n.name), from: n.from, to: n.to };
  });
}

// What an agent needs to point at text: the blocks and whatever the user has selected.
export function readAgentDoc(state) {
  const blocks = docBlocks(state);
  const sel = state.selection.main;
  let selection = null;
  if (!sel.empty) {
    const ids = blocks.filter((b) => b.from < sel.to && b.to > sel.from).map((b) => b.id);
    selection = { text: state.sliceDoc(sel.from, sel.to), blocks: ids };
  }
  return { blocks: blocks.map((b) => ({ id: b.id, kind: b.kind, text: state.sliceDoc(b.from, b.to) })), selection };
}

// Region to replace and the text to type there. `target` is "selection", or a block id or an
// inclusive range of ids ("b4k2x7", "b4k2x7-b9x0m3"); otherwise the span from `fromText` through
// `toText` is used, and fromText must occur exactly once.
export function planAgentRewrite(state, { target, fromText, toText } = {}, body = '') {
  const doc = state.doc;
  const fm = findFrontmatterNode(state);
  const minPos = fm ? Math.min(fm.to + 1, doc.length) : 0;
  let from;
  let to;
  let whole = false;

  if (target === 'selection') {
    const r = state.selection.main;
    if (r.empty) return { error: 'nothing is selected in the editor' };
    ({ from, to } = r);
  } else if (target) {
    const m = /^(b[0-9a-z]+)(?:\s*(?:-|\.\.|–)\s*(b[0-9a-z]+))?$/i.exec(target.trim());
    if (!m) return { error: `invalid target: ${target}` };
    const blocks = docBlocks(state);
    const at = (id) => blocks.findIndex((b) => b.id === id.toLowerCase());
    const i = at(m[1]);
    const j = m[2] ? at(m[2]) : i;
    const missing = [m[1], m[2]].filter((id, k) => id && (k ? j : i) < 0);
    if (missing.length) {
      return { error: `block ${missing.join(' and ')} is not in the note: it was edited or removed since editor_read. Call editor_read again` };
    }
    if (j < i) return { error: `${m[2]} comes before ${m[1]}; give the range top to bottom` };
    from = blocks[i].from;
    to = blocks[j].to;
    whole = true;
  } else if (fromText) {
    const all = doc.toString();
    const first = all.indexOf(fromText);
    if (first < 0) return { error: 'from_text was not found in the note' };
    if (all.indexOf(fromText, first + 1) >= 0) return { error: 'from_text occurs more than once; quote more of it' };
    from = first;
    to = first + fromText.length;
    if (toText) {
      const end = all.indexOf(toText, from);
      if (end < 0) return { error: 'to_text was not found after from_text' };
      to = Math.max(to, end + toText.length);
    }
  } else {
    return { error: 'give a target, or from_text' };
  }
  if (from < minPos) return { error: 'frontmatter cannot be rewritten' };

  let text = whole ? body.trim() : body;
  if (!doc.sliceString(from, to).endsWith('\n')) text = text.replace(/\n+$/, '');
  if (text) return opensFrontmatter(doc, from, text, to) ? { error: FRONTMATTER_ERROR } : { pos: from, replaceTo: to, text };

  // Deleting a whole block also takes the blank lines that separated it.
  if (whole) {
    const after = leading(doc.sliceString(to, Math.min(to + 2, doc.length)), '\n', 2);
    if (after) {
      to += after;
    } else {
      const before = trailing(doc.sliceString(Math.max(minPos, from - 2), from), '\n', 2);
      from -= before;
    }
  }
  if (opensFrontmatter(doc, from, '', to)) return { error: FRONTMATTER_ERROR };
  return { pos: from, replaceTo: to, text: '' };
}

// Keeps the agent's position in a comfortable band of the viewport. The view glides
// (to `target` of the height) only once it leaves the band, so it moves occasionally
// instead of creeping every tick. Wins over the user's own scrolling by design.
function follow(view, pos, { top = 0.08, bottom = 0.68, target = 0.4 } = {}) {
  const scroller = view.scrollDOM;
  const h = scroller && scroller.clientHeight;
  if (!h) return;
  const c = view.coordsAtPos(pos);
  if (!c) {
    view.dispatch({ effects: EditorView.scrollIntoView(pos, { y: 'start', yMargin: Math.round(h * target) }) });
    return;
  }
  // Judged against where a glide already under way will leave the view.
  const y = c.top - scroller.getBoundingClientRect().top + scroller.scrollTop;
  const rel = (y - scrollDestination(view)) / h;
  if (rel >= top && rel <= bottom) return;
  glideTo(view, y - h * target);
}

const SPEEDS = { slow: 25, normal: 70, fast: 200, instant: Infinity };
const FOLLOW_MS = 120;

const segmenter = typeof Intl !== 'undefined' && Intl.Segmenter ? new Intl.Segmenter(undefined, { granularity: 'grapheme' }) : null;
function graphemes(s) {
  if (!segmenter) return Array.from(s, (g, i) => ({ segment: g, index: i }));
  return Array.from(segmenter.segment(s), (x) => ({ segment: x.segment, index: x.index }));
}

const codePoints = (s) => { let n = 0; for (const _ of s) n++; return n; };

const defaultSleep = (ms) => new Promise((r) => setTimeout(r, ms));

const FENCE_LINE_RE = /^([ \t]*(?:>[ \t]?)*[ \t]*)(`{3,}|~{3,})(.*)$/;

// An unclosed fence would swallow the rest of the note, so one the text leaves open is closed.
export function closeFences(text) {
  let open = null;
  for (const line of text.split('\n')) {
    const m = FENCE_LINE_RE.exec(line);
    if (!m) continue;
    if (!open) {
      if (m[2][0] !== '`' || !m[3].includes('`')) open = { prefix: m[1], fence: m[2] };
    } else if (m[2][0] === open.fence[0] && m[2].length >= open.fence.length && !m[3].trim()) {
      open = null;
    }
  }
  if (!open) return text;
  return (text.endsWith('\n') ? text : text + '\n') + open.prefix + open.fence + '\n';
}

// The plan's text with its own fences closed, unless it lands inside code, or
// replaces source that already left a fence open for the note's closer below.
function fencedText(state, plan) {
  const to = plan.replaceTo ?? plan.pos;
  ensureSyntaxTree(state, to, 50);
  for (let n = syntaxTree(state).resolveInner(plan.pos, -1); n; n = n.parent) {
    if (n.name === 'FencedCode' || n.name === 'CodeBlock') return plan.text;
  }
  const old = state.sliceDoc(plan.pos, to);
  return closeFences(old) === old ? closeFences(plan.text) : plan.text;
}

// Writes plan.text at plan.pos (replacing up to plan.replaceTo) and reveals it at a typing
// rhythm. Resolves { ok, interrupted, chars, error? } and never throws; `chars` counts what
// was revealed, and on a stop the unrevealed rest is taken out again.
export async function agentType(view, plan, opts = {}) {
  const { signal, isCurrent = () => true, sleep = defaultSleep, now = Date.now } = opts;
  const cps = SPEEDS[opts.speed] ?? SPEEDS.normal;
  const perChar = cps === Infinity ? 0 : 1000 / cps;
  const rhythm = perChar ? createRhythm(perChar, opts.random) : null;
  const src = fencedText(view.state, plan);
  const balanced = closeFences(src) === src;
  const segs = graphemes(src);
  const start = plan.pos;
  const end = start + src.length;
  const replacing = plan.replaceTo != null && plan.replaceTo > plan.pos;
  let typed = 0;
  let at = start;
  let inserted = false;

  // Long text types faster to fit the deadline: pace() scales the pauses, owed() is the
  // backlog against an even spread, so throttled timers can't push the bulk to the end.
  const { deadline } = opts;
  const total = codePoints(src);
  const begun = now();
  let natural = 0;
  const owed = () => (deadline > begun
    ? Math.ceil(total * Math.min(1, (now() - begun) / (deadline - begun))) - typed
    : 0);
  const pace = () => {
    if (!rhythm) return 0;
    if (deadline == null) return 1;
    const left = deadline - now();
    if (left <= 0) return 0;
    const rate = typed >= 40 ? natural / typed : perChar * 2.2;
    const need = (total - typed) * rate;
    return need <= left * 0.9 ? 1 : (left * 0.9) / need;
  };
  const rest = (p, ms) => { natural += ms; return sleep(ms * p); };

  // The rhythm's pauses can exceed the history's grouping window; a private clock that
  // advances 1ms per write keeps the whole write one undo step whatever the pauses were.
  let clock = Date.now();
  const write = (spec, annotations = []) => view.dispatch({
    ...spec, annotations: [Transaction.time.of(clock++), agentWrite.of(true), ...annotations], userEvent: 'input.type.agent',
  });
  const reveal = (pos) => {
    at = pos;
    view.dispatch({
      selection: EditorSelection.cursor(pos), effects: setPending.of(pos < end ? { from: pos, to: end } : null),
      // Not undo steps of their own: they'd split the write from a stop's take-back.
      annotations: [agentWrite.of(true), Transaction.addToHistory.of(false)], userEvent: 'select.agent',
    });
  };
  const stopped = () => {
    if (signal?.aborted) return 'interrupted';
    if (!isCurrent()) return 'editor switched to another note';
    if (!agentWriting(view.state)) return 'editor state replaced';
    return null;
  };
  const takeBackUnseen = () => {
    if (inserted && at < end && isCurrent() && agentWriting(view.state)) {
      // What was shown may stop inside a fence whose closing line was in the rest.
      const seen = src.slice(0, at - start);
      const close = balanced ? closeFences(seen).slice(seen.length) : '';
      write({ changes: { from: at, to: end, insert: close }, selection: EditorSelection.cursor(at), effects: setPending.of(null) });
    }
  };
  const abort = (reason) => {
    takeBackUnseen();
    return { ok: false, interrupted: true, chars: typed, error: reason };
  };

  // Measuring the caret forces layout, so follow it at most every FOLLOW_MS.
  let lastFollow = -Infinity;
  const followSoon = (p) => {
    const t = now();
    if (t - lastFollow < FOLLOW_MS) return;
    lastFollow = t;
    follow(view, p);
  };

  view.dispatch({
    selection: EditorSelection.cursor(start),
    effects: [setAgentWriting.of(true), setAgentTarget.of(replacing ? { from: start, to: plan.replaceTo } : null)],
    annotations: agentWrite.of(true), userEvent: 'select.agent',
  });
  follow(view, start, { top: 0.05, bottom: 0.5, target: 0.25 });
  try {
    const p0 = pace();
    if (replacing && p0) await sleep(700 * p0);
    if (p0) await sleep(rhythm.beat() * p0);
    let why = stopped();
    if (why) return abort(why);

    // The view keeps its place: scrolled to the bottom, CM would otherwise stay pinned
    // there and run ahead of the caret. Measuring first lands a pending follow.
    const effects = [setAgentTarget.of(null), setPending.of(src ? { from: start, to: end } : null)];
    if (view.scrollSnapshot) {
      view.measure();
      effects.push(view.scrollSnapshot());
    }
    write({
      changes: { from: start, to: replacing ? plan.replaceTo : start, insert: src },
      selection: EditorSelection.cursor(start), effects,
    }, [isolateHistory.of('before')]);
    inserted = true;
    // input.type keeps the write one undo group but also opens completions, e.g. after "[[".
    if (completionStatus(view.state)) closeCompletion(view);
    if (view.plugin) measureRange(view, start, end);

    for (let j = 0; j < segs.length;) {
      why = stopped();
      if (why) return abort(why);
      const p = pace();
      const r = p > 0 ? rhythm : null;
      // A block drawn as one widget (a card) can't be half shown: it appears whole.
      const block = view.lineBlockAt ? view.lineBlockAt(at) : null;
      const whole = block && block.type === BlockType.WidgetRange && block.to > at ? Math.min(block.to, end) : -1;
      const owe = r ? owed() : 0;
      const burst = r ? Math.max(r.burstSize(), owe) : Infinity;
      let count = 0;
      let chunk = '';
      let pause = 0;
      while (j < segs.length && (count < burst || start + segs[j].index < whole)) {
        const s = segs[j];
        chunk += s.segment;
        j++;
        count++;
        if (start + s.index < whole) continue;
        pause = r ? r.pause(src, s.index + s.segment.length) : 0;
        if (pause && count >= owe) break;
      }
      typed += codePoints(chunk);
      reveal(j < segs.length ? start + segs[j].index : end);
      followSoon(at);
      if (r) await rest(p, pause + r.gap(Math.min(count, burst)) + r.hesitation());
    }
    if (agentWriting(view.state)) follow(view, at);
    return { ok: true, interrupted: false, chars: typed };
  } catch (e) {
    try { takeBackUnseen(); } catch (_) { /* view destroyed */ }
    return { ok: false, interrupted: false, chars: typed, error: (e && e.message) || String(e) };
  } finally {
    try {
      view.dispatch({
        effects: [setAgentWriting.of(false), setAgentTarget.of(null), setPending.of(null)],
        annotations: [isolateHistory.of('after'), agentWrite.of(true)],
      });
    } catch (_) { /* view destroyed */ }
  }
}
