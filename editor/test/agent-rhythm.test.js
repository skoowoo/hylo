import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EditorState } from '@codemirror/state';
import { history, undo } from '@codemirror/commands';
import { createRhythm } from '../src/cm-live/agent-rhythm.js';
import { agentType, agentWritingExtension } from '../src/cm-live/agent-typing.js';
import { wikiMarkdownLanguage } from '../src/cm-live/index.js';

const PER_CHAR = 1000 / 70;
const mid = () => 0.5;
const rhythm = (random = mid) => createRhythm(PER_CHAR, random);
const pauseAt = (src, marker, r = rhythm()) => r.pause(src, src.indexOf(marker) + marker.length);

test('pauses are layered: paragraph > heading > line > sentence > clause', () => {
  const src = '# Head\nline one\nline two. Next, word\n\nPara';
  const para = pauseAt(src, 'word\n\n');
  const heading = pauseAt(src, '# Head\n');
  const line = pauseAt(src, 'line one\n');
  const sentence = pauseAt(src, 'two.');
  const clause = pauseAt(src, 'Next,');
  assert.ok(para > heading && heading > line && line > sentence && sentence > clause && clause > 0,
    JSON.stringify({ para, heading, line, sentence, clause }));
});

test('no pause inside words, decimals or between the two newlines of a paragraph break', () => {
  const src = 'word 3.14 and more\n\nx';
  assert.equal(pauseAt(src, 'wo'), 0);
  assert.equal(pauseAt(src, '3.'), 0);
  assert.equal(pauseAt(src, 'more\n'), 0);
});

test('a sentence end followed by a newline leaves the pause to the newline', () => {
  const src = 'Done.\nNext';
  assert.equal(pauseAt(src, 'Done.'), 0);
  assert.ok(pauseAt(src, 'Done.\n') > 0);
});

test('CJK sentence and clause marks pause without trailing spaces', () => {
  const src = '你好，世界。再见';
  assert.ok(pauseAt(src, '，') > 0);
  assert.ok(pauseAt(src, '。') > pauseAt(src, '，'));
});

test('pauses stretch for slow typing and shrink for fast typing', () => {
  const src = 'a. b';
  const slow = createRhythm(1000 / 25, mid).pause(src, 2);
  const normal = createRhythm(PER_CHAR, mid).pause(src, 2);
  const fast = createRhythm(1000 / 200, mid).pause(src, 2);
  assert.ok(slow > normal && normal > fast, JSON.stringify({ slow, normal, fast }));
});

test('randomness varies the timing and keeps it bounded', () => {
  let seed = 7;
  const rand = () => (seed = (seed * 16807) % 2147483647) / 2147483647;
  const r = createRhythm(PER_CHAR, rand);
  const gaps = Array.from({ length: 200 }, () => r.gap(3));
  assert.ok(new Set(gaps.map((g) => Math.round(g))).size > 20);
  assert.ok(gaps.every((g) => g > 0 && g < 3 * PER_CHAR * 1.6));
  const sizes = new Set(Array.from({ length: 200 }, () => r.burstSize()));
  assert.ok(sizes.size > 1 && Math.min(...sizes) >= 1);
  const hes = Array.from({ length: 2000 }, () => r.hesitation()).filter(Boolean);
  assert.ok(hes.length > 0 && hes.length < 100);
});

test('long pauses do not split the undo step', async () => {
  const lang = wikiMarkdownLanguage();
  const view = {
    state: EditorState.create({ doc: '# T\n', extensions: [lang, history(), agentWritingExtension] }),
    dispatch(...specs) { view.state = view.state.update(...specs).state; },
  };
  const realNow = Date.now;
  let now = realNow();
  Date.now = () => now;
  try {
    const text = 'First paragraph, with a pause.\n\nSecond paragraph after a long beat.';
    const res = await agentType(view, { pos: 4, text }, { speed: 'slow', sleep: async (ms) => { now += ms + 2000; } });
    assert.equal(res.ok, true);
    assert.equal(view.state.doc.toString(), '# T\n' + text);
    undo({ state: view.state, dispatch: (tr) => view.dispatch(tr) });
    assert.equal(view.state.doc.toString(), '# T\n');
  } finally {
    Date.now = realNow;
  }
});

test('typing sleeps follow the rhythm: a beat first, longer for paragraph breaks', async () => {
  const lang = wikiMarkdownLanguage();
  const view = {
    state: EditorState.create({ doc: '', extensions: [lang, agentWritingExtension] }),
    dispatch(...specs) { view.state = view.state.update(...specs).state; },
  };
  const sleeps = [];
  await agentType(view, { pos: 0, text: 'One.\n\nTwo.' }, { speed: 'normal', random: mid, sleep: async (ms) => { sleeps.push(ms); } });
  assert.ok(sleeps[0] >= 250 && sleeps[0] <= 450 * 1.01 + 1, `first beat ${sleeps[0]}`);
  assert.ok(Math.max(...sleeps) >= 350, `longest ${Math.max(...sleeps)}`);
  assert.equal(view.state.doc.toString(), 'One.\n\nTwo.');
});

test('instant speed never sleeps', async () => {
  const lang = wikiMarkdownLanguage();
  const view = {
    state: EditorState.create({ doc: '', extensions: [lang, agentWritingExtension] }),
    dispatch(...specs) { view.state = view.state.update(...specs).state; },
  };
  let slept = 0;
  await agentType(view, { pos: 0, text: 'A. B.\n\nC' }, { speed: 'instant', sleep: async () => { slept++; } });
  assert.equal(slept, 0);
});

const paragraphs = (n) => Array.from({ length: n }, (_, i) => `第${i}段：这一段有好几个句子。每句都会停一下，逗号也会，最后还有段落之间的停顿。`).join('\n\n');

// A fake clock driven by the sleeps, plus a cost per step, like a page's main thread.
// `largest` is the biggest jump of the caret, which is what appears at once.
function clocked(text, { budget, sleepFloor = 0 }) {
  const lang = wikiMarkdownLanguage();
  let t = 0;
  let writes = 0;
  let largest = 0;
  const view = {
    state: EditorState.create({ doc: '', extensions: [lang, agentWritingExtension] }),
    dispatch(...specs) {
      const before = view.state.selection.main.head;
      view.state = view.state.update(...specs).state;
      largest = Math.max(largest, view.state.selection.main.head - before);
      writes++;
      t += 1;
    },
  };
  const done = agentType(view, { pos: 0, text }, {
    speed: 'normal', random: mid, now: () => t, deadline: budget,
    sleep: async (ms) => { t += Math.max(ms, sleepFloor); },
  });
  return done.then((res) => ({ res, view, elapsed: t, writes, largest }));
}

test('long text types faster to finish within its deadline', async () => {
  const text = paragraphs(40);
  const free = await clocked(text, { budget: undefined });
  assert.ok(free.elapsed > 60000, `unpaced takes ${free.elapsed}ms`);
  const { res, view, elapsed, writes, largest } = await clocked(text, { budget: 30000 });
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), text);
  assert.ok(elapsed <= 30500, `took ${elapsed}ms`);
  assert.ok(writes > text.length / 20, `still typed, in ${writes} writes`);
  assert.ok(largest < 60, `no dump at the end: largest write ${largest} chars`);
});

test('short text keeps its natural rhythm under a deadline', async () => {
  const text = paragraphs(2);
  const free = await clocked(text, { budget: undefined });
  const paced = await clocked(text, { budget: 30000 });
  assert.equal(paced.elapsed, free.elapsed);
});

// Background windows clamp timers to ~1s; bursts grow so typing still ends in time.
test('throttled timers still finish by the deadline', async () => {
  const text = paragraphs(40);
  const { res, view, elapsed, largest } = await clocked(text, { budget: 30000, sleepFloor: 1000 });
  assert.equal(res.ok, true);
  assert.equal(view.state.doc.toString(), text);
  assert.ok(elapsed <= 31000, `took ${elapsed}ms`);
  assert.ok(largest < text.length / 4, `largest write ${largest} of ${text.length}`);
});
