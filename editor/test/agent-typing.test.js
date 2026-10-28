import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { ensureSyntaxTree } from '@codemirror/language';
import { history, undo } from '@codemirror/commands';
import { wikiMarkdownLanguage } from '../src/cm-live/index.js';
import { autocompletion, completionStatus } from '@codemirror/autocomplete';
import { planAgentInsert, planAgentRewrite, readAgentDoc, docBlocks, agentType, agentWritingExtension, agentWriting, closeFences } from '../src/cm-live/agent-typing.js';
import { revealOff } from '../src/cm-live/selection.js';
import { fenceAutoClose } from '../src/cm-live/fence-close.js';

const lang = wikiMarkdownLanguage();

function viewFor(doc, anchor = doc.length, extra = []) {
  const view = {
    state: EditorState.create({ doc, selection: { anchor }, extensions: [lang, history(), agentWritingExtension, extra] }),
    dispatch(...specs) { view.state = view.state.update(...specs).state; },
  };
  ensureSyntaxTree(view.state, view.state.doc.length, 5000);
  return view;
}

const plan = (doc, opts, body, anchor) => planAgentInsert(viewFor(doc, anchor).state, opts, body);
const bid = (state, n) => docBlocks(state)[n - 1].id;
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

test('type: lands in the document, leaves the caret after it and hands the editor back', async () => {
  const view = viewFor('# T\n');
  const p = planAgentInsert(view.state, {}, 'hello world');
  const res = await run(view, p);
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), '# T\n\nhello world\n');
  assert.equal(view.state.selection.main.head, view.state.doc.length);
  assert.equal(agentWriting(view.state), false);
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

test('type: the whole text is in the note from the first write; typing only reveals it', async () => {
  const view = viewFor('# T\n');
  const docs = new Set();
  const heads = [];
  const orig = view.dispatch;
  view.dispatch = (...a) => { orig(...a); docs.add(view.state.doc.toString()); heads.push(view.state.selection.main.head); };
  const text = '| a | b |\n| --- | --- |\n| one | two |\n';
  await agentType(view, { pos: 4, text }, { speed: 'slow', sleep: async () => {} });
  assert.deepEqual([...docs], ['# T\n', '# T\n' + text]);
  assert.ok(heads.length > 10, 'revealed in steps');
  assert.ok(heads.every((h, i) => i === 0 || h >= heads[i - 1]), 'the caret only moves forward');
});

test('type: a stop takes back what was not revealed, and one undo restores the note', async () => {
  const view = viewFor('# T\n');
  const ctrl = new AbortController();
  let calls = 0;
  const res = await agentType(view, { pos: 4, text: 'abcdefghijklmnop' }, {
    speed: 'slow', signal: ctrl.signal, sleep: async () => { if (++calls === 4) ctrl.abort(); },
  });
  assert.equal(res.interrupted, true);
  assert.equal(view.state.doc.toString(), '# T\n' + 'abcdefghijklmnop'.slice(0, res.chars));
  undo({ state: view.state, dispatch: (tr) => view.dispatch(tr) });
  assert.equal(view.state.doc.toString(), '# T\n');
});

test('type: the agent caret reveals no syntax until the user moves it', async () => {
  const view = viewFor('');
  const seen = [];
  await agentType(view, { pos: 0, text: '**bold** text\n' }, { speed: 'slow', sleep: async () => seen.push(revealOff(view.state)) });
  assert.ok(seen.every(Boolean));
  assert.equal(revealOff(view.state), true, 'still the agent caret after it finished');
  view.dispatch({ selection: { anchor: 2 } });
  assert.equal(revealOff(view.state), false);
});

test('closeFences: closes only a fence the text leaves open', () => {
  assert.equal(closeFences('```py\nx = 1\n'), '```py\nx = 1\n```\n');
  assert.equal(closeFences('> ~~~~\n> a'), '> ~~~~\n> a\n> ~~~~\n');
  for (const done of ['```js\na\n```\n', 'x ```inline``` y', '~~~\n```\n~~~', 'plain']) assert.equal(closeFences(done), done);
});

test('type: aborts when signalled, keeping what was revealed', async () => {
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
  assert.equal(agentWriting(view.state), false);
});

test('docBlocks: another copy of a repeated block retires the old ids', () => {
  const before = docBlocks(viewFor('a\n\nx\n\nb\n\nx\n').state).map((b) => b.id);
  const after = docBlocks(viewFor('x\n\na\n\nx\n\nb\n\nx\n').state).map((b) => b.id);
  assert.equal(after.includes(before[0]) && after.includes(before[2]), true);
  assert.equal(after.includes(before[1]) || after.includes(before[3]), false);
});

test('type: fences are closed only where the note does not close them', async () => {
  const inside = viewFor('```md\na\n\n```\n', 8);
  await run(inside, { pos: 8, text: '```py\n' });
  assert.equal(inside.state.doc.toString(), '```md\na\n```py\n\n```\n');
  const head = viewFor('```js\nconst a = 1;\n```\n');
  await run(head, { pos: 0, replaceTo: 6, text: '```ts\n' });
  assert.equal(head.state.doc.toString(), '```ts\nconst a = 1;\n```\n');
  const fresh = viewFor('x\n');
  await run(fresh, { pos: 2, text: '```py\ny\n' });
  assert.equal(fresh.state.doc.toString(), 'x\n```py\ny\n```\n');
});

test('type: a stop inside a code fence leaves the fence closed', async () => {
  const view = viewFor('');
  const ctrl = new AbortController();
  let calls = 0;
  const res = await agentType(view, { pos: 0, text: '```js\nconst a = 1;\n```\nafter\n' }, {
    speed: 'slow', signal: ctrl.signal,
    sleep: async () => { if (++calls === 10) ctrl.abort(); },
  });
  assert.equal(res.interrupted, true);
  const doc = view.state.doc.toString();
  assert.ok(res.chars > 6 && res.chars < 19, String(res.chars));
  assert.match(doc, /^```js\nconst[^`]*\n```\n$/);
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

test('type: while the agent writes, nothing else edits the note or moves the caret', async () => {
  const view = viewFor('tail');
  let n = 0;
  const res = await agentType(view, { pos: 4, text: 'abcdefghij' }, {
    speed: 'slow',
    sleep: async () => {
      if (++n !== 2) return;
      view.dispatch({ changes: { from: 0, insert: 'XX' }, userEvent: 'input.type' });
      view.dispatch({ selection: { anchor: 0 }, userEvent: 'select.pointer' });
      view.dispatch({ selection: { anchor: 1 } });
      undo({ state: view.state, dispatch: (tr) => view.dispatch(tr) });
    },
  });
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), 'tailabcdefghij');
  assert.equal(view.state.selection.main.head, view.state.doc.length);
  view.dispatch({ selection: { anchor: 0 } });
  assert.equal(view.state.selection.main.head, 0, 'the editor is the user\'s again');
});

test('type: the editor is not editable while the agent writes', async () => {
  const view = viewFor('');
  const seen = [];
  await agentType(view, { pos: 0, text: 'abc' }, { speed: 'slow', sleep: async () => seen.push(view.state.facet(EditorView.editable)) });
  assert.ok(seen.length && seen.every((v) => v === false), JSON.stringify(seen));
  assert.equal(view.state.facet(EditorView.editable), true);
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

test('read: blocks carry ids, frontmatter skipped', () => {
  const { blocks, selection } = readAgentDoc(viewFor(DOC).state);
  assert.deepEqual(blocks.map((b) => b.kind), ['heading', 'paragraph', 'list', 'code', 'paragraph']);
  assert.ok(blocks.every((b) => /^b[0-9a-z]{6}$/.test(b.id)));
  assert.equal(new Set(blocks.map((b) => b.id)).size, 5);
  assert.equal(blocks[1].text, 'First paragraph here.');
  assert.equal(selection, null);
});

test('read: ids follow the text, not the position', () => {
  const before = docBlocks(viewFor(DOC).state).map((b) => b.id);
  const edited = DOC.replace('# Title\n\n', '# Title\n\nA new paragraph.\n\n').replace('Last one.', 'Last two.');
  const after = docBlocks(viewFor(edited).state).map((b) => b.id);
  assert.deepEqual(after.filter((id) => before.includes(id)), before.slice(0, 4));
  assert.ok(!after.includes(before[4]), 'an edited block gets a new id');
});

test('read: identical blocks get distinct ids', () => {
  const ids = docBlocks(viewFor('## Same\n\n## Same\n').state).map((b) => b.id);
  assert.equal(new Set(ids).size, 2);
});

test('read: reports the user selection and the blocks it touches', () => {
  const from = DOC.indexOf('paragraph');
  const view = viewFor(DOC);
  view.dispatch({ selection: { anchor: from, head: from + 9 } });
  const { selection } = readAgentDoc(view.state);
  assert.equal(selection.text, 'paragraph');
  assert.deepEqual(selection.blocks, [bid(view.state, 2)]);
});

test('rewrite plan: block target', () => {
  const state = viewFor(DOC).state;
  const p = planAgentRewrite(state, { target: bid(state, 2) }, 'New text.\n');
  assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'First paragraph here.');
  assert.equal(p.text, 'New text.');
});

test('rewrite plan: an id from before the block was edited no longer resolves', () => {
  const stale = bid(viewFor(DOC).state, 2);
  const state = viewFor(DOC.replace('First paragraph', 'Changed paragraph')).state;
  assert.match(planAgentRewrite(state, { target: stale }, 'x').error, /edited or removed/);
});

test('rewrite plan: block ranges and bad ids', () => {
  const state = viewFor(DOC).state;
  const [b2, b3] = [bid(state, 2), bid(state, 3)];
  for (const target of [`${b2}-${b3}`, `${b2}..${b3}`, `${b2} - ${b3}`]) {
    const p = planAgentRewrite(state, { target }, 'x');
    assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'First paragraph here.\n\n- a\n- b');
  }
  assert.match(planAgentRewrite(state, { target: `${b3}-${b2}` }, 'x').error, /top to bottom/);
  assert.ok(planAgentRewrite(state, { target: 'b9zzz' }, 'x').error);
  assert.ok(planAgentRewrite(state, { target: 'nope' }, 'x').error);
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
  const p = planAgentRewrite(state, { target: bid(state, 2) }, '');
  assert.equal(p.text, '');
  assert.equal(state.sliceDoc(p.pos, p.replaceTo), 'First paragraph here.\n\n');
  const last = planAgentRewrite(state, { target: bid(state, 5) }, '');
  assert.equal(state.sliceDoc(last.pos, last.replaceTo), 'Last one.\n');
});

test('rewrite: replaces the region and one undo restores it', async () => {
  const view = viewFor(DOC);
  const p = planAgentRewrite(view.state, { target: bid(view.state, 2) }, 'A much better paragraph, typed slowly by the agent.');
  const res = await agentType(view, p, { speed: 'slow', sleep: async () => {} });
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), DOC.replace('First paragraph here.', 'A much better paragraph, typed slowly by the agent.'));
  undo({ state: view.state, dispatch: (tr) => view.dispatch(tr) });
  assert.equal(view.state.doc.toString(), DOC);
});

test('rewrite: the replaced selection collapses into the agent caret', async () => {
  const view = viewFor(DOC);
  const from = DOC.indexOf('First');
  view.dispatch({ selection: { anchor: from, head: from + 'First paragraph here.'.length } });
  const p = planAgentRewrite(view.state, { target: 'selection' }, 'A much better paragraph.');
  const seen = [];
  await agentType(view, p, { speed: 'slow', sleep: async () => seen.push(view.state.selection.main.empty) });
  assert.ok(seen.every(Boolean), 'no selection spans the typed text while typing');
  assert.equal(view.state.selection.main.head, from + 'A much better paragraph.'.length);
});

test('rewrite: delete only', async () => {
  const view = viewFor(DOC);
  const p = planAgentRewrite(view.state, { target: bid(view.state, 2) }, '');
  await run(view, p);
  assert.equal(view.state.doc.toString(), DOC.replace('First paragraph here.\n\n', ''));
});

test('rewrite: interrupted before the first write leaves the note untouched', async () => {
  const view = viewFor(DOC);
  const ctrl = new AbortController();
  const p = planAgentRewrite(view.state, { target: bid(view.state, 2) }, 'x');
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
  const p = planAgentRewrite(view.state, { target: bid(view.state, 2) }, text);
  await agentType(view, p, { speed: 'slow', sleep: async () => {} });
  assert.equal(view.state.doc.toString(), DOC.replace('First paragraph here.', text));
  for (const d of seen) assert.equal((d.match(/^```/gm) || []).length % 2, 0, JSON.stringify(d));
});

test('docBlocks: one per top-level block', () => {
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
  assert.ok(planAgentRewrite(state, { target: bid(state, 1) }, '---').error);
  assert.ok(planAgentRewrite(state, { target: bid(state, 1) }, '').error, 'deleting b1 would expose the rule');
  assert.ok(!planAgentRewrite(state, { target: bid(state, 1) }, 'Hello').error);
});
