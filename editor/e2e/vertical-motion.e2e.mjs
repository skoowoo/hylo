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

async function walk(t, fixture, direction) {
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
      await page.keyboard.press(up ? 'ArrowUp' : 'ArrowDown');
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

// Backspace right after a rendered link/image shows its source; the next press edits it.
test('Backspace after a widget reveals the source before deleting', async (t) => {
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
