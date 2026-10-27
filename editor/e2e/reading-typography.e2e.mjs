// Reading view turns on CJK typography (autospace, pretty wrapping) that
// editing leaves off; drawn selection must still cover the text it selects.
import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { webkit } from 'playwright-core';
import { startServer } from './server.mjs';

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

async function open(t, doc) {
  if (launchError) {
    t.skip('WebKit unavailable — run `npx playwright-core install webkit` (' + launchError.message.split('\n')[0] + ')');
    return null;
  }
  const page = await browser.newPage({ viewport: { width: 700, height: 400 } });
  await page.goto(server.url + '/?f=mixed.md');
  await page.waitForFunction(() => window.ready);
  await page.evaluate((doc) => {
    const v = window.view;
    v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: doc }, selection: { anchor: doc.length } });
  }, doc);
  return page;
}

const setReading = (page, on) =>
  page.evaluate((on) => new Promise((resolve) => {
    const v = window.view;
    v.dispatch({ effects: window.reading.reconfigure(on ? window.cm.readingExtensions() : []) });
    requestAnimationFrame(() => requestAnimationFrame(resolve));
  }), on);

const lineWidth = (page, needle) =>
  page.evaluate((needle) => {
    const v = window.view;
    const at = v.state.doc.toString().indexOf(needle);
    return v.coordsAtPos(at + needle.length, -1).right - v.coordsAtPos(at, 1).left;
  }, needle);

test('autospace separates CJK and Latin in reading view, not in code', async (t) => {
  const page = await open(t, '中文Latin中文\n\n`代码code代码`\n');
  if (!page) return;
  try {
    await setReading(page, false);
    const prose = await lineWidth(page, '中文Latin中文');
    const code = await lineWidth(page, '代码code代码');
    await setReading(page, true);
    assert.ok((await lineWidth(page, '中文Latin中文')) - prose > 2, 'no autospace in reading view');
    assert.ok(Math.abs((await lineWidth(page, '代码code代码')) - code) < 0.5, 'inline code got autospace');
  } finally {
    await page.close();
  }
});

// Characters sharing the visual line of the paragraph's last character.
const lastLineChars = (page) =>
  page.evaluate(() => {
    const v = window.view;
    const end = v.state.doc.line(1).to;
    const top = v.coordsAtPos(end, -1).top;
    let n = 0;
    while (n < end && Math.abs(v.coordsAtPos(end - n - 1, 1).top - top) < 1) n++;
    return n;
  });

test('reading view keeps a lone character off the last line', async (t) => {
  const page = await open(t, 'x');
  if (!page) return;
  try {
    const perLine = await page.evaluate(() => {
      const v = window.view;
      const probe = document.createElement('span');
      probe.textContent = '字';
      v.contentDOM.querySelector('.cm-line').appendChild(probe);
      const w = probe.getBoundingClientRect().width;
      probe.remove();
      return Math.floor(v.contentDOM.clientWidth / w);
    });
    await page.evaluate((doc) => {
      const v = window.view;
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: doc } });
    }, '字'.repeat(perLine * 2 + 1));

    await setReading(page, false);
    assert.equal(await lastLineChars(page), 1, 'fixture should orphan one character while editing');
    await setReading(page, true);
    assert.ok((await lastLineChars(page)) > 1, 'last line still holds a single character');
  } finally {
    await page.close();
  }
});

test('drawn selection covers autospaced text in reading view', async (t) => {
  const page = await open(t, '前面中文Latin文字和123数字后面\n');
  if (!page) return;
  try {
    await setReading(page, true);
    const r = await page.evaluate(() => new Promise((resolve) => {
      const v = window.view;
      const text = v.state.doc.toString();
      const from = text.indexOf('中');
      const to = text.indexOf('后');
      v.dispatch({ selection: { anchor: from, head: to } });
      requestAnimationFrame(() => requestAnimationFrame(() => {
        const bg = [...v.dom.querySelectorAll('.cm-lp-selection-layer .cm-selectionBackground')].map((e) => e.getBoundingClientRect());
        resolve({
          drawn: bg.length ? { left: Math.min(...bg.map((b) => b.left)), right: Math.max(...bg.map((b) => b.right)) } : null,
          text: { left: v.coordsAtPos(from, 1).left, right: v.coordsAtPos(to, -1).right },
        });
      }));
    }));
    assert.ok(r.drawn, 'no selection drawn');
    assert.ok(Math.abs(r.drawn.left - r.text.left) < 1, `selection starts at ${r.drawn.left}, text at ${r.text.left}`);
    assert.ok(Math.abs(r.drawn.right - r.text.right) < 1, `selection ends at ${r.drawn.right}, text at ${r.text.right}`);
  } finally {
    await page.close();
  }
});
