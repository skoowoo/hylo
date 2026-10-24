// Lets a server-side agent type into the live editor as if it were a person:
// changes go through ordinary transactions and move the editor's own caret, so rendering,
// undo and autosave behave as for human input.
import { EditorState, StateField, StateEffect, Transaction, Annotation, EditorSelection } from '@codemirror/state';
import { Decoration, EditorView } from '@codemirror/view';
import { ensureSyntaxTree, syntaxTree } from '@codemirror/language';
import { isolateHistory } from '@codemirror/commands';
import { completionStatus, closeCompletion } from '@codemirror/autocomplete';
import { findFrontmatterNode } from './frontmatter-syntax.js';
import { resolveAncestor, nameIs } from './tree-utils.js';
import { createRhythm } from './agent-rhythm.js';

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

// The drawn caret shows unfocused too: the user usually watches from the chat composer.
const agentCaretTheme = EditorView.baseTheme({
  '&.cm-agent-writing > .cm-scroller > .cm-cursorLayer': { animation: 'steps(1) cm-blink 1.2s infinite' },
  '&.cm-agent-writing > .cm-scroller > .cm-cursorLayer .cm-cursor-primary': {
    display: 'block', borderLeft: '2px solid var(--accent, #5e6ad2)', marginLeft: '-1px',
  },
  '&.cm-agent-writing .cm-cursor-primary::after': {
    content: '"AI"', position: 'absolute', left: '-2px', bottom: '100%',
    padding: '0 4px', borderRadius: '3px', font: "10px/14px var(--font-ui, system-ui, sans-serif)", whiteSpace: 'nowrap',
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

export const agentWritingExtension = [agentWritingField, agentLock, agentCaretTheme, agentTargetField, agentTargetTheme];
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

// Top-level markdown blocks, numbered b1..bn. Ids are positional, so callers must
// pair them with an `expect` check before acting on a possibly stale read.
export function docBlocks(state) {
  ensureSyntaxTree(state, state.doc.length, 300);
  const out = [];
  for (let n = syntaxTree(state).topNode.firstChild; n; n = n.nextSibling) {
    if (n.name === 'Frontmatter') continue;
    out.push({ id: `b${out.length + 1}`, kind: blockKind(n.name), from: n.from, to: n.to });
  }
  return out;
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

const norm = (s) => s.replace(/\s+/g, ' ').trim();

// Region to replace and the text to type there. `target` is "selection", or a block id / range
// ("b3", "b3-b5") that must come with `expect` (the block's opening words); otherwise the span
// from `fromText` through `toText` is used, and fromText must occur exactly once.
export function planAgentRewrite(state, { target, expect, fromText, toText } = {}, body = '') {
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
    const m = /^b(\d+)(?:-b?(\d+))?$/.exec(target.trim());
    if (!m) return { error: `invalid target: ${target}` };
    const blocks = docBlocks(state);
    const i = Number(m[1]) - 1;
    const j = (m[2] ? Number(m[2]) : Number(m[1])) - 1;
    if (i < 0 || j < i || j >= blocks.length) return { error: `no such block: ${target} (the note has ${blocks.length})` };
    if (!expect) return { error: 'expect is required with a block target' };
    from = blocks[i].from;
    to = blocks[j].to;
    if (!norm(doc.sliceString(from, to)).startsWith(norm(expect))) {
      return { error: 'the note changed since it was read; call editor_read again' };
    }
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

// Keeps the agent's position in a comfortable band of the viewport. Scrolling happens in
// one hop (to `target` of the height) only once it leaves the band, so the view moves
// occasionally instead of creeping every tick. Wins over the user's own scrolling by design.
function follow(view, pos, { top = 0.08, bottom = 0.68, target = 0.4 } = {}) {
  const scroller = view.scrollDOM;
  const h = scroller && scroller.clientHeight;
  if (!h) return;
  const c = view.coordsAtPos(pos);
  if (c) {
    const rel = (c.top - scroller.getBoundingClientRect().top) / h;
    if (rel >= top && rel <= bottom) return;
    // At the end of the note there is nothing below to scroll into view.
    if (rel > bottom && scroller.scrollTop + h >= scroller.scrollHeight - 1) return;
  }
  view.dispatch({ effects: EditorView.scrollIntoView(pos, { y: 'start', yMargin: Math.round(h * target) }) });
}

const SPEEDS = { slow: 25, normal: 70, fast: 200, instant: Infinity };
const FOLLOW_MS = 120;

const segmenter = typeof Intl !== 'undefined' && Intl.Segmenter ? new Intl.Segmenter(undefined, { granularity: 'grapheme' }) : null;
function graphemes(s) {
  if (!segmenter) return Array.from(s, (g, i) => ({ segment: g, index: i }));
  return Array.from(segmenter.segment(s), (x) => ({ segment: x.segment, index: x.index }));
}

// Sticky, matched in place via execAt, so long content is never re-sliced per line.
const OPEN_RE = /([ \t]*(?:>[ \t]?)*[ \t]*)(`{3,}|~{3,})([^`\n]*)(?=\n|$)/y;
const closerRe = (ch, len) => new RegExp(`\\n[ \\t]*(?:>[ \\t]?)*[ \\t]*${ch === '`' ? '`' : '~'}{${len},}[ \\t]*(?=\\n|$)`, 'y');
const execAt = (re, s, i) => { re.lastIndex = i; return re.exec(s); };
const codePoints = (s) => { let n = 0; for (const _ of s) n++; return n; };

const inCode = (state, pos) => !!resolveAncestor(state, pos, nameIs(['FencedCode', 'CodeBlock']), -1);

const defaultSleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Types plan.text at plan.pos. Resolves { ok, interrupted, chars, error? } and never throws.
// A fence opener is written together with its closer, like the editor's own auto-close,
// so the document never passes through an unclosed fence that would swallow what follows.
export async function agentType(view, plan, opts = {}) {
  const { signal, isCurrent = () => true, sleep = defaultSleep, now = Date.now } = opts;
  const cps = SPEEDS[opts.speed] ?? SPEEDS.normal;
  const perChar = cps === Infinity ? 0 : 1000 / cps;
  const rhythm = perChar ? createRhythm(perChar, opts.random) : null;
  const src = plan.text;
  const segs = graphemes(src);
  let typed = 0;
  let first = true;
  let open = null;

  const abort = (reason) => ({ ok: false, interrupted: true, chars: typed, error: reason });
  const pos = () => view.state.selection.main.head;

  // The region being rewritten goes away in the first write, so deleting and the
  // first typed chunk are one transaction and undo reverts the whole rewrite.
  let deleting = plan.replaceTo != null && plan.replaceTo > plan.pos;
  const deleteEnd = (at) => (deleting ? Math.max(at, plan.replaceTo) : at);

  // The rhythm's pauses can exceed the history's grouping window; a private clock that
  // advances 1ms per write keeps the whole write one undo step whatever the pauses were.
  let clock = Date.now();

  // caret: where the selection lands, as an offset into `insert`.
  const push = (from, insert, caret = insert.length) => {
    const annotations = [Transaction.time.of(clock++), agentWrite.of(true)];
    if (first) annotations.push(isolateHistory.of('before'));
    first = false;
    const effects = [];
    let to = from;
    if (deleting) {
      to = deleteEnd(from);
      effects.push(setAgentTarget.of(null));
      deleting = false;
    }
    view.dispatch({
      changes: { from, to, insert }, selection: EditorSelection.cursor(from + caret),
      effects, annotations, userEvent: 'input.type.agent',
    });
    // input.type keeps the write one undo group but also opens completions, e.g. after "[[".
    if (completionStatus(view.state)) closeCompletion(view);
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
    selection: EditorSelection.cursor(plan.pos),
    effects: [setAgentWriting.of(true), setAgentTarget.of(deleting ? { from: plan.pos, to: plan.replaceTo } : null)],
    annotations: agentWrite.of(true),
  });
  follow(view, plan.pos, { top: 0.05, bottom: 0.5, target: 0.25 });
  try {
    if (deleting && rhythm) await sleep(700);
    if (rhythm) await sleep(rhythm.beat());
    if (!src) {
      if (signal?.aborted) return abort('interrupted');
      if (!isCurrent()) return abort('editor switched to another note');
      if (!agentWriting(view.state)) return abort('editor state replaced');
      push(pos(), '');
      return { ok: true, interrupted: false, chars: 0 };
    }
    for (let j = 0; j < segs.length;) {
      if (signal?.aborted) return abort('interrupted');
      if (!isCurrent()) return abort('editor switched to another note');
      if (!agentWriting(view.state)) return abort('editor state replaced');
      const at = pos();
      const idx = segs[j].index;
      const atLineStart = idx === 0 || src[idx - 1] === '\n';

      if (open && src[idx] === '\n') {
        const m = execAt(open.closer, src, idx);
        if (m) {
          // Closer already exists below the caret: hop onto it instead of typing it again.
          const next = view.state.doc.lineAt(at).number + 1;
          const to = next <= view.state.doc.lines ? view.state.doc.line(next).to : at;
          view.dispatch({ selection: EditorSelection.cursor(to), annotations: agentWrite.of(true) });
          followSoon(to);
          typed += codePoints(m[0]);
          while (j < segs.length && segs[j].index < idx + m[0].length) j++;
          open = null;
          continue;
        }
      }

      if (!open && atLineStart) {
        const m = execAt(OPEN_RE, src, idx);
        const line = view.state.doc.lineAt(at);
        const delEnd = deleteEnd(at);
        const restEmpty = view.state.sliceDoc(delEnd, view.state.doc.lineAt(delEnd).to) === '';
        if (m && restEmpty) ensureSyntaxTree(view.state, view.state.doc.length, 50);
        if (m && restEmpty && !inCode(view.state, line.from)) {
          const fence = m[2];
          push(at, m[0] + '\n' + m[1] + fence, m[0].length);
          followSoon(at + m[0].length);
          typed += codePoints(m[0]);
          open = { closer: closerRe(fence[0], fence.length) };
          while (j < segs.length && segs[j].index < idx + m[0].length) j++;
          if (rhythm) await sleep(rhythm.gap(6));
          continue;
        }
      }

      const burst = rhythm ? rhythm.burstSize() : Infinity;
      let count = 0;
      let chunk = '';
      let pause = 0;
      while (j < segs.length && count < burst) {
        const s = segs[j];
        if (chunk && open && s.segment === '\n') break;
        if (chunk && s.index > idx && src[s.index - 1] === '\n' && execAt(OPEN_RE, src, s.index)) break;
        chunk += s.segment;
        j++;
        count++;
        pause = rhythm ? rhythm.pause(src, s.index + s.segment.length) : 0;
        if (pause) break;
      }
      push(at, chunk);
      followSoon(at + chunk.length);
      typed += codePoints(chunk);
      if (rhythm) await sleep(pause + rhythm.gap(count) + rhythm.hesitation());
    }
    if (agentWriting(view.state)) follow(view, pos());
    return { ok: true, interrupted: false, chars: typed };
  } catch (e) {
    return { ok: false, interrupted: false, chars: typed, error: (e && e.message) || String(e) };
  } finally {
    try { view.dispatch({ effects: [setAgentWriting.of(false), setAgentTarget.of(null)], annotations: [isolateHistory.of('after'), agentWrite.of(true)] }); } catch (_) { /* view destroyed */ }
  }
}
