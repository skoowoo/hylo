// Arrow keys must walk a long live-preview note line by line in WebKit (the
// desktop app's engine). Regression guard for the height-map skew in
// decoration-helpers.js's excludeFromTextSample: a heading/code line taken as
// CM's text-size sample made one ArrowUp jump ~95 lines and logged
// "Viewport failed to stabilize".
import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { webkit } from 'playwright-core';
import { EditorState } from '@codemirror/state';
import { ensureSyntaxTree } from '@codemirror/language';
import { wikiMarkdownLanguage } from '../src/cm-live/index.js';
import { cardFence } from '../src/cm-live/fence-card/registry.js';
import { startServer } from './server.mjs';

const FIXTURES = path.join(path.dirname(fileURLToPath(import.meta.url)), 'fixtures');
// A few presses landing on the same line is just soft wrapping; many means stuck.
const MAX_STALL = 20;

let browser = null;
let server = null;
let launchError = null;

before(async () => {
  try {
    browser = await webkit.launch();
  } catch (e) {
    launchError = e;
    return;
  }
  server = await startServer();
});

after(async () => {
  if (browser) await browser.close();
  if (server) await server.close();
});

// Lines a single keypress may legitimately pass over: block widgets the caret
// can't stop inside (rules, rendered cards) and hairline rows (table delimiter,
// setext underline).
function layoutOf(doc) {
  const state = EditorState.create({ doc, extensions: [wikiMarkdownLanguage()] });
  const tree = ensureSyntaxTree(state, doc.length, 10000);
  const skippable = new Set();
  let bodyStart = 0;
  const lineNo = (pos) => state.doc.lineAt(pos).number;
  tree.iterate({
    enter(n) {
      if (n.name === 'Frontmatter') bodyStart = Math.min(n.to + 1, doc.length);
      if (n.name === 'HorizontalRule') skippable.add(lineNo(n.from));
      if (n.name === 'TableDelimiter' && n.node.parent && n.node.parent.name === 'Table') skippable.add(lineNo(n.from));
      if (n.name === 'SetextHeading1' || n.name === 'SetextHeading2') skippable.add(lineNo(n.to));
      if (n.name === 'FencedCode') {
        const card = cardFence(state.doc, n.node);
        if (card) for (let l = lineNo(card.from); l <= lineNo(card.to); l++) skippable.add(l);
      }
    },
  });
  return { skippable, firstLine: lineNo(bodyStart), lastLine: state.doc.lines };
}

async function walk(t, fixture, direction, shift = false) {
  if (launchError) return t.skip('WebKit unavailable — run `npx playwright-core install webkit` (' + launchError.message.split('\n')[0] + ')');
  const doc = fs.readFileSync(path.join(FIXTURES, fixture), 'utf8');
  const { skippable, firstLine, lastLine } = layoutOf(doc);
  const up = direction === 'up';

  const page = await browser.newPage({ viewport: { width: 900, height: 800 } });
  const warnings = [];
  page.on('console', (m) => { if (/failed to stabilize/i.test(m.text())) warnings.push(m.text()); });
  page.on('pageerror', (e) => warnings.push('pageerror: ' + e.message));
  try {
    await page.goto(server.url + '/?f=' + encodeURIComponent(fixture));
    await page.waitForFunction(() => window.ready);
    const startLine = up ? lastLine : firstLine;
    await page.evaluate((n) => {
      const pos = window.view.state.doc.line(n).from;
      window.view.dispatch({ selection: { anchor: pos }, effects: window.cm.EditorView.scrollIntoView(pos, { y: 'center' }) });
    }, startLine);
    // Let images settle so the walk starts from a measured layout.
    await page.waitForTimeout(500);

    const lineAfterFrame = () => page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => {
      const v = window.view;
      resolve(v.state.doc.lineAt(v.state.selection.main.head).number);
    })));
    const text = (n) => JSON.stringify(doc.split('\n')[n - 1]);

    let line = await lineAfterFrame();
    let stall = 0;
    while (up ? line > firstLine : line < lastLine) {
      await page.keyboard.press((shift ? 'Shift+' : '') + (up ? 'ArrowUp' : 'ArrowDown'));
      const next = await lineAfterFrame();
      const moved = up ? line - next : next - line;
      assert.ok(moved >= 0, `${fixture} ${direction}: moved the wrong way, L${line} → L${next}`);
      if (moved === 0) {
        assert.ok(++stall < MAX_STALL, `${fixture} ${direction}: stuck on L${line} ${text(line)}`);
        continue;
      }
      stall = 0;
      for (let l = Math.min(line, next) + 1; l < Math.max(line, next); l++) {
        assert.ok(skippable.has(l), `${fixture} ${direction}: L${line} → L${next} skipped L${l} ${text(l)}`);
      }
      line = next;
    }
    assert.deepEqual(warnings, [], `${fixture} ${direction}: ${warnings.join('; ')}`);
  } finally {
    await page.close();
  }
}

for (const fixture of ['handbook.md', 'mixed.md']) {
  for (const direction of ['up', 'down']) {
    test(`arrow ${direction} walks ${fixture} line by line`, (t) => walk(t, fixture, direction));
  }
}

// Extending lands on wrap points, where a range's default side differs from a caret's.
for (const direction of ['up', 'down']) {
  test(`shift+arrow ${direction} extends through mixed.md line by line`, (t) => walk(t, 'mixed.md', direction, true));
}

// Entering a mindmap card swaps it for its (shorter) source; a spacer keeps
// the slot's height so the text below stays put. Leaving restores the card.
test('editing a mindmap card keeps the text below in place', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const doc = fs.readFileSync(path.join(FIXTURES, 'handbook.md'), 'utf8');
  const lines = doc.split('\n');
  // The second card: short source, so the spacer has work to do.
  const open = lines.findIndex((l, i) => l === '```mindmap' && i > lines.indexOf('```mindmap')) + 1;
  const close = lines.findIndex((l, i) => i >= open && l === '```') + 1;
  const page = await browser.newPage({ viewport: { width: 900, height: 900 } });
  try {
    await page.goto(server.url + '/?f=handbook.md');
    await page.waitForFunction(() => window.ready);
    const gap = () => page.evaluate(([above, below]) => {
      const v = window.view;
      const top = (n) => {
        const { node } = v.domAtPos(v.state.doc.line(n).from);
        return (node.nodeType === 1 ? node : node.parentElement).closest('.cm-line').getBoundingClientRect().top;
      };
      return top(below) - top(above);
    }, [open - 1, close + 1]);
    const caret = (n) => page.evaluate((n) => {
      const v = window.view, pos = v.state.doc.line(n).from;
      v.dispatch({ selection: { anchor: pos }, effects: window.cm.EditorView.scrollIntoView(pos, { y: 'center' }) });
    }, n);
    await caret(close + 1);
    await page.waitForTimeout(800);
    const rest = await gap();
    await caret(open + 2);
    await page.waitForTimeout(400);
    const editing = await gap();
    await caret(close + 1);
    await page.waitForTimeout(800);
    const back = await gap();
    assert.ok(Math.abs(editing - rest) <= 2, `card slot ${rest}px at rest, ${editing}px while editing`);
    assert.ok(Math.abs(back - rest) <= 2, `card slot ${rest}px at rest, ${back}px after leaving`);
  } finally {
    await page.close();
  }
});

// Backspace right after a rendered wikilink shows its source; the next press edits it.
// Images delete as a unit — covered by the test below.
test('Backspace after a wikilink reveals the source before deleting', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 900, height: 600 } });
  try {
    await page.goto(server.url + '/?f=widgets.md');
    await page.waitForFunction(() => window.ready);
    const state = () => page.evaluate(() => ({
      doc: window.view.state.doc.toString(),
      raw: !!document.querySelector('.cm-lp-wikilink-raw'),
    }));
    await page.evaluate(() => { window.view.focus(); window.view.dispatch({ selection: { anchor: 12 } }); });
    await page.keyboard.press('Backspace');
    assert.deepEqual(await state(), { doc: 'see [[note]]\n\n![a](/image?x)\n', raw: true });
    await page.keyboard.press('Backspace');
    assert.equal((await state()).doc, 'see [[note]\n\n![a](/image?x)\n');
  } finally {
    await page.close();
  }
});

// Only the card's Edit button opens the frontmatter source. Arrowing up or
// Backspace at the body start leaves the caret and the block alone.
test('frontmatter source opens only from the Edit button', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const doc = '---\ntitle: T\ntags: [a, b]\n---\nbody\n';
  const body = doc.indexOf('body');
  const page = await browser.newPage({ viewport: { width: 900, height: 600 } });
  try {
    await page.goto(server.url + '/?f=widgets.md');
    await page.waitForFunction(() => window.ready);
    const state = () => page.evaluate(() => ({
      doc: window.view.state.doc.toString(),
      head: window.view.state.selection.main.head,
      card: !!document.querySelector('.cm-fm-card'),
    }));
    await page.evaluate(([doc, body]) => {
      window.view.focus();
      window.view.dispatch({ changes: { from: 0, to: window.view.state.doc.length, insert: doc }, selection: { anchor: body } });
      window.cm.forceParsing(window.view, doc.length, 500);
    }, [doc, body]);
    const atRest = { doc, head: body, card: true };
    assert.deepEqual(await state(), atRest);

    await page.keyboard.press('ArrowUp');
    assert.deepEqual(await state(), atRest);
    await page.keyboard.press('Backspace');
    assert.deepEqual(await state(), atRest);

    await page.hover('.cm-fm-card');
    await page.click('.cm-fm-card .hy-btn');
    assert.deepEqual(await state(), { doc, head: 'title: T'.length + 4, card: false });
    await page.keyboard.press('ArrowDown');
    assert.equal((await state()).card, false);

    for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowDown');
    assert.equal((await state()).card, true);
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('ArrowUp');
    assert.deepEqual(await state(), atRest);
  } finally {
    await page.close();
  }
});

async function imagePage(doc, count) {
  const page = await browser.newPage({ viewport: { width: 900, height: 1400 } });
  await page.goto(server.url + '/?f=widgets.md');
  await page.waitForFunction(() => window.ready);
  await page.evaluate((doc) => {
    window.view.dispatch({ changes: { from: 0, to: window.view.state.doc.length, insert: doc } });
    window.cm.forceParsing(window.view, doc.length, 500);
  }, doc);
  await page.waitForFunction((count) => {
    const imgs = document.querySelectorAll('.cm-lp-wikiimage img');
    return imgs.length === count && [...imgs].every((i) => i.getBoundingClientRect().height > 0);
  }, count);
  return page;
}

const imageBox = (page, i) => page.evaluate((i) => {
  const r = document.querySelectorAll('.cm-lp-wikiimage')[i].getBoundingClientRect();
  return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, x: r.left + r.width / 2, y: r.top + r.height / 2 };
}, i);

const editorState = (page) => page.evaluate(() => {
  const doc = window.view.state.doc.toString();
  const sel = window.view.state.selection.main;
  return {
    doc,
    text: doc.slice(sel.from, sel.to),
    head: sel.head,
    raw: document.querySelector('.cm-lp-wikiimage-raw')?.textContent ?? null,
    outlined: document.querySelectorAll('.cm-lp-image-selected').length,
  };
});

test('clicking an image puts the caret beside it and one Backspace deletes it', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const doc = 'before\n\n![a](/image?x)\n\nafter';
  const page = await imagePage(doc, 1);
  try {
    const box = await imageBox(page, 0);
    await page.mouse.click(box.right - 10, box.y);
    const caret = await editorState(page);
    assert.equal(caret.head, doc.indexOf('\n\nafter'));
    assert.equal(caret.raw, null);
    await page.keyboard.press('Backspace');
    assert.equal((await editorState(page)).doc, 'before\n\n\n\nafter');
  } finally {
    await page.close();
  }
});

test('drag and shift-click select images whole; Backspace deletes them together', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const doc = 'before\n\n![a](/image?x)\n\n![b](/image?y)\n\nafter';
  const page = await imagePage(doc, 2);
  try {
    const a = await imageBox(page, 0);
    const b = await imageBox(page, 1);
    await page.mouse.move(a.left + 10, a.y);
    await page.mouse.down();
    await page.mouse.move(b.right - 10, b.y, { steps: 10 });
    await page.mouse.up();
    const dragged = await editorState(page);
    assert.equal(dragged.text, '![a](/image?x)\n\n![b](/image?y)');
    assert.equal(dragged.outlined, 2);
    assert.equal(dragged.raw, null);
    await page.mouse.click(a.left + 10, a.y);
    await page.keyboard.down('Shift');
    await page.mouse.click(b.right - 10, b.y);
    await page.keyboard.up('Shift');
    assert.equal((await editorState(page)).text, '![a](/image?x)\n\n![b](/image?y)');
    await page.keyboard.press('Backspace');
    assert.equal((await editorState(page)).doc, 'before\n\n\n\nafter');
  } finally {
    await page.close();
  }
});

test('double-clicking an image shows its source', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await imagePage('before\n\n![a](/image?x)\n\nafter', 1);
  try {
    const box = await imageBox(page, 0);
    await page.mouse.dblclick(box.x, box.y);
    assert.equal((await editorState(page)).raw, '![a](/image?x)');
  } finally {
    await page.close();
  }
});

test('a linked image selects and deletes as one, keeping its url hidden', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const link = '[![Image](https://img.example/a.png)](https://mp.weixin.qq.com/s?x=1 "title")';
  const page = await imagePage('before\n\n' + link + '\n\nafter', 1);
  try {
    const box = await imageBox(page, 0);
    await page.mouse.click(box.left + 2, box.y);
    await page.keyboard.down('Shift');
    await page.mouse.click(box.right - 2, box.y);
    await page.keyboard.up('Shift');
    const selected = await editorState(page);
    assert.equal(selected.text, link);
    assert.equal(selected.outlined, 1);
    assert.equal(await page.evaluate(() => document.querySelector('.cm-content').innerText.includes('mp.weixin')), false);
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Backspace');
    assert.equal((await editorState(page)).doc, 'before\n\n\n\nafter');
    await page.keyboard.press('Meta+z');
    await page.mouse.dblclick(box.x, box.y);
    assert.equal((await editorState(page)).raw, '![Image](https://img.example/a.png)');
  } finally {
    await page.close();
  }
});
