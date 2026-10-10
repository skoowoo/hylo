import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState } from '@codemirror/state';
import { ensureSyntaxTree } from '@codemirror/language';
import { history, undo } from '@codemirror/commands';
import { wikiMarkdownLanguage } from '../src/cm-live/index.js';
import { autocompletion, completionStatus } from '@codemirror/autocomplete';
import { planAgentInsert, planAgentRewrite, readAgentDoc, docBlocks, agentType, agentCursorExtension, agentCursorPos, foreignEditCount } from '../src/cm-live/agent-typing.js';
import { fenceAutoClose } from '../src/cm-live/fence-close.js';

const lang = wikiMarkdownLanguage();

function viewFor(doc, anchor = doc.length, extra = []) {
  const view = {
    state: EditorState.create({ doc, selection: { anchor }, extensions: [lang, history(), agentCursorExtension, extra] }),
    dispatch(...specs) { view.state = view.state.update(...specs).state; },
  };
  ensureSyntaxTree(view.state, view.state.doc.length, 5000);
  return view;
}

const plan = (doc, opts, body, anchor) => planAgentInsert(viewFor(doc, anchor).state, opts, body);
const run = async (view, p, opts = {}) => agentType(view, p, { speed: 'instant', sleep: async () => {}, ...opts });

test('plan: end of an empty note types as-is', () => {
  assert.deepEqual(plan('', {}, 'hello\n'), { pos: 0, text: 'hello\n' });
});

test('plan: end of a note starts a new block', () => {
  const p = plan('# T\n\nbody\n', {}, 'next');
  assert.equal(p.pos, 10);
  assert.equal(p.text, '\nnext\n');
  assert.equal(plan('# T\n\nbody', {}, 'next').text, '\n\nnext\n');
  assert.equal(plan('# T\n\nbody\n\n', {}, 'next').text, 'next\n');
});

test('plan: skips frontmatter', () => {
  const doc = '---\ntitle: a\n---\n';
  const p = plan(doc, {}, 'hi');
  assert.equal(p.pos, doc.length);
  assert.equal(p.text, 'hi\n');
  assert.equal(plan(doc + 'x', { at: 'cursor' }, 'hi', 0).pos, doc.length);
});

test('plan: heading targets the end of that section only', () => {
  const doc = '# A\n\none\n\n## Sub\n\ntwo\n\n# B\n\nthree\n';
  const p = plan(doc, { heading: 'a' }, 'added');
  assert.equal(p.pos, doc.indexOf('two') + 3);
  assert.equal(p.text, '\n\nadded');
});

test('plan: heading inside a fence is ignored; missing heading errors', () => {
  const doc = '```\n# fake\n```\n\n# Real\n\nx\n';
  assert.ok(plan(doc, { heading: 'fake' }, 'y').error);
  assert.equal(plan(doc, { heading: 'real' }, 'y').pos, doc.length - 1);
});

test('plan: empty content errors', () => {
  assert.ok(plan('a', {}, '  \n').error);
});

test('type: lands in the document and clears the caret', async () => {
  const view = viewFor('# T\n');
  const p = planAgentInsert(view.state, {}, 'hello world');
  const res = await run(view, p);
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), '# T\n\nhello world\n');
  assert.equal(agentCursorPos(view.state), null);
});

test('type: pacing splits into many transactions', async () => {
  const view = viewFor('');
  let n = 0;
  const orig = view.dispatch;
  view.dispatch = (...a) => { n++; return orig(...a); };
  await agentType(view, { pos: 0, text: 'abcdefghij\n' }, { speed: 'slow', sleep: async () => {} });
  assert.equal(view.state.doc.toString(), 'abcdefghij\n');
  assert.ok(n > 3, `dispatches: ${n}`);
});

test('type: fence is opened with its closer and never left unclosed', async () => {
  const view = viewFor('');
  const seen = [];
  const orig = view.dispatch;
  view.dispatch = (...a) => { orig(...a); seen.push(view.state.doc.toString()); };
  const text = 'Intro\n\n```mermaid\ngraph TD\n  A-->B\n```\n\nOutro\n';
  const res = await agentType(view, { pos: 0, text }, { speed: 'slow', sleep: async () => {} });
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), text);
  for (const doc of seen.filter((d) => d.includes('```'))) {
    assert.equal((doc.match(/^```/gm) || []).length % 2, 0, `unclosed fence in: ${JSON.stringify(doc)}`);
  }
});

test('type: ~~~ fence and quoted fence close correctly', async () => {
  const view = viewFor('');
  const text = '> ```js\n> a()\n> ```\n\n~~~\nb\n~~~\n';
  await run(view, { pos: 0, text }, { speed: 'slow' });
  assert.equal(view.state.doc.toString(), text);
});

test('type: unterminated fence in the source still ends closed', async () => {
  const view = viewFor('');
  await run(view, { pos: 0, text: '```py\nx = 1\n' }, { speed: 'slow' });
  assert.equal((view.state.doc.toString().match(/^```/gm) || []).length, 2);
});

test('type: inserts mid-document and keeps the rest', async () => {
  const doc = '# A\n\none\n\n# B\n\nthree\n';
  const view = viewFor(doc);
  const p = planAgentInsert(view.state, { heading: 'A' }, 'added');
  await run(view, p);
  assert.equal(view.state.doc.toString(), '# A\n\none\n\nadded\n\n# B\n\nthree\n');
});

test('type: aborts when signalled, keeping what was typed', async () => {
  const view = viewFor('');
  const ctrl = new AbortController();
  let calls = 0;
  const res = await agentType(view, { pos: 0, text: 'abcdefghijklmnop' }, {
    speed: 'slow', signal: ctrl.signal,
    sleep: async () => { if (++calls === 2) ctrl.abort(); },
  });
  assert.equal(res.interrupted, true);
  assert.ok(res.chars > 0 && res.chars < 16);
  assert.equal(view.state.doc.toString().length, res.chars);
  assert.equal(agentCursorPos(view.state), null);
});

test('type: aborts when the note changes underneath', async () => {
  const view = viewFor('');
  let ok = true;
  const res = await agentType(view, { pos: 0, text: 'abcdefghij' }, {
    speed: 'slow', isCurrent: () => ok,
    sleep: async () => { ok = false; },
  });
  assert.equal(res.interrupted, true);
});

test('type: user edits above the caret shift where typing continues', async () => {
  const view = viewFor('tail');
  let n = 0;
  const res = await agentType(view, { pos: 4, text: 'abcdefghij' }, {
    speed: 'slow',
    sleep: async () => { if (++n === 1) view.dispatch({ changes: { from: 0, insert: 'XX' } }); },
  });
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), 'XXtailabcdefghij');
});

test('type: one undo reverts the whole agent write', async () => {
  const view = viewFor('# T\n');
  const p = planAgentInsert(view.state, {}, 'a fairly long paragraph typed by the agent');
  await agentType(view, p, { speed: 'slow', sleep: async () => {} });
  assert.notEqual(view.state.doc.toString(), '# T\n');
  undo({ state: view.state, dispatch: (tr) => view.dispatch(tr) });
  assert.equal(view.state.doc.toString(), '# T\n');
});

test('type: CJK and emoji are not split', async () => {
  const view = viewFor('');
  const text = '你好，世界 👨‍👩‍👧 end';
  await run(view, { pos: 0, text }, { speed: 'slow' });
  assert.equal(view.state.doc.toString(), text);
});

const DOC = '---\ntitle: t\n---\n# Title\n\nFirst paragraph here.\n\n- a\n- b\n\n```js\nx()\n```\n\nLast one.\n';

test('read: blocks are numbered, frontmatter skipped', () => {
  const { blocks, selection } = readAgentDoc(viewFor(DOC).state);
  assert.deepEqual(blocks.map((b) => [b.id, b.kind]), [
    ['b1', 'heading'], ['b2', 'paragraph'], ['b3', 'list'], ['b4', 'code'], ['b5', 'paragraph'],
  ]);
  assert.equal(blocks[1].text, 'First paragraph here.');
  assert.equal(selection, null);
});

test('read: reports the user selection and the blocks it touches', () => {
  const from = DOC.indexOf('paragraph');
  const view = viewFor(DOC);
  view.dispatch({ selection: { anchor: from, head: from + 9 } });
  const { selection } = readAgentDoc(view.state);
  assert.equal(selection.text, 'paragraph');
  assert.deepEqual(selection.blocks, ['b2']);
});

test('rewrite plan: block target needs a matching expect', () => {
  const state = viewFor(DOC).state;
  assert.ok(planAgentRewrite(state, { target: 'b2' }, 'x').error.includes('expect'));
  assert.ok(planAgentRewrite(state, { target: 'b2', expect: 'Something else' }, 'x').error.includes('changed'));
  const p = planAgentRewrite(state, { target: 'b2', expect: 'First paragraph' }, 'New text.\n');
  assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'First paragraph here.');
  assert.equal(p.text, 'New text.');
});

test('rewrite plan: block ranges and bad ids', () => {
  const state = viewFor(DOC).state;
  const p = planAgentRewrite(state, { target: 'b2-b3', expect: 'First' }, 'x');
  assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'First paragraph here.\n\n- a\n- b');
  assert.ok(planAgentRewrite(state, { target: 'b9', expect: 'x' }, 'x').error);
  assert.ok(planAgentRewrite(state, { target: 'nope', expect: 'x' }, 'x').error);
});

test('rewrite plan: selection', () => {
  const view = viewFor(DOC);
  assert.ok(planAgentRewrite(view.state, { target: 'selection' }, 'x').error);
  const from = DOC.indexOf('First');
  view.dispatch({ selection: { anchor: from, head: from + 5 } });
  const p = planAgentRewrite(view.state, { target: 'selection' }, 'Second');
  assert.deepEqual([p.pos, p.replaceTo, p.text], [from, from + 5, 'Second']);
});

test('rewrite plan: quoted spans must be unique and outside frontmatter', () => {
  const state = viewFor(DOC).state;
  assert.ok(planAgentRewrite(state, { fromText: 'nowhere' }, 'x').error);
  assert.ok(planAgentRewrite(state, { fromText: 'a' }, 'x').error.includes('more than once'));
  const p = planAgentRewrite(state, { fromText: 'First', toText: 'here.' }, 'x');
  assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'First paragraph here.');
  assert.ok(planAgentRewrite(state, { fromText: 'title: t' }, 'x').error.includes('frontmatter'));
});

test('rewrite plan: deleting a block takes its separating blank lines', () => {
  const state = viewFor(DOC).state;
  const p = planAgentRewrite(state, { target: 'b2', expect: 'First' }, '');
  assert.equal(p.text, '');
  assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'First paragraph here.\n\n');
  const last = planAgentRewrite(state, { target: 'b5', expect: 'Last' }, '');
  assert.equal(state.sliceDoc(last.pos, last.replaceTo), 'Last one.\n');
});

test('rewrite: replaces the region and one undo restores it', async () => {
  const view = viewFor(DOC);
  const p = planAgentRewrite(view.state, { target: 'b2', expect: 'First' }, 'A much better paragraph, typed slowly by the agent.');
  const res = await agentType(view, p, { speed: 'slow', sleep: async () => {} });
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), DOC.replace('First paragraph here.', 'A much better paragraph, typed slowly by the agent.'));
  undo({ state: view.state, dispatch: (tr) => view.dispatch(tr) });
  assert.equal(view.state.doc.toString(), DOC);
});

test('rewrite: delete only', async () => {
  const view = viewFor(DOC);
  const p = planAgentRewrite(view.state, { target: 'b2', expect: 'First' }, '');
  await run(view, p);
  assert.equal(view.state.doc.toString(), DOC.replace('First paragraph here.\n\n', ''));
});

test('rewrite: interrupted before the first write leaves the note untouched', async () => {
  const view = viewFor(DOC);
  const ctrl = new AbortController();
  const p = planAgentRewrite(view.state, { target: 'b2', expect: 'First' }, 'x');
  const res = await agentType(view, p, { speed: 'slow', signal: ctrl.signal, sleep: async () => ctrl.abort() });
  assert.equal(res.interrupted, true);
  assert.equal(view.state.doc.toString(), DOC);
});

test('rewrite: a fenced block in the new text is written closed', async () => {
  const view = viewFor(DOC);
  const text = 'Intro.\n\n```py\nprint(1)\n```';
  const seen = [];
  const orig = view.dispatch;
  view.dispatch = (...a) => { orig(...a); seen.push(view.state.doc.toString()); };
  const p = planAgentRewrite(view.state, { target: 'b2', expect: 'First' }, text);
  await agentType(view, p, { speed: 'slow', sleep: async () => {} });
  assert.equal(view.state.doc.toString(), DOC.replace('First paragraph here.', text));
  for (const d of seen) assert.equal((d.match(/^```/gm) || []).length % 2, 0, JSON.stringify(d));
});

test('docBlocks ids follow document order', () => {
  assert.equal(docBlocks(viewFor('a\n\nb\n\nc').state).length, 3);
});

test('plan: frontmatter with no trailing newline keeps the text off its closer', async () => {
  const view = viewFor('---\ntitle: a\n---');
  const p = planAgentInsert(view.state, {}, 'hi');
  await run(view, p);
  assert.equal(view.state.doc.toString(), '---\ntitle: a\n---\nhi\n');
});

test('rewrite plan: to_text inside from_text never shrinks the region', () => {
  const state = viewFor('abc def ghi\n').state;
  const p = planAgentRewrite(state, { fromText: 'abc def', toText: 'abc' }, 'X');
  assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'abc def');
});

test('type: completions opened by the typing are closed', async () => {
  const source = () => ({ from: 0, options: [{ label: 'note' }] });
  const view = viewFor('', 0, autocompletion({ override: [source] }));
  view.dispatch({ changes: { from: 0, insert: 'x' }, userEvent: 'input.type' });
  assert.ok(completionStatus(view.state), 'typing should open completions');
  await run(view, { pos: 1, text: '[[note' }, { speed: 'slow' });
  assert.equal(completionStatus(view.state), null);
});

test('rewrite: an edit while the region is highlighted moves what gets replaced', async () => {
  const view = viewFor(DOC);
  const p = planAgentRewrite(view.state, { target: 'b2', expect: 'First' }, 'New.');
  let n = 0;
  await agentType(view, p, {
    speed: 'slow',
    sleep: async () => { if (++n === 1) view.dispatch({ changes: { from: p.pos + 5, insert: ' long' } }); },
  });
  assert.equal(view.state.doc.toString(), DOC.replace('First paragraph here.', 'New.'));
});

test('type: agent writes are not counted as foreign edits', async () => {
  const view = viewFor('');
  await run(view, { pos: 0, text: 'abc' }, { speed: 'slow' });
  assert.equal(foreignEditCount(view.state), 0);
  view.dispatch({ changes: { from: 0, insert: 'x' } });
  assert.equal(foreignEditCount(view.state), 1);
});

test('type: chars counts characters, not UTF-16 units', async () => {
  const view = viewFor('');
  const res = await run(view, { pos: 0, text: '😀😀' });
  assert.equal(res.chars, 2);
});

test('never writes frontmatter: content may not make the note start with ---', () => {
  assert.ok(plan('', {}, '---\ntitle: x\n---\nbody').error.includes('frontmatter'));
  assert.ok(plan('', {}, '--- divider').error);
  assert.ok(plan('body\n', { at: 'cursor' }, '---', 0).error);
  assert.equal(plan('', {}, 'body\n\n---\n').text, 'body\n\n---\n');
  const state = viewFor('Intro\n\n---\n\nRest\n').state;
  assert.ok(planAgentRewrite(state, { target: 'b1', expect: 'Intro' }, '---').error);
  assert.ok(planAgentRewrite(state, { target: 'b1', expect: 'Intro' }, '').error, 'deleting b1 would expose the rule');
  assert.ok(!planAgentRewrite(state, { target: 'b1', expect: 'Intro' }, 'Hello').error);
});
