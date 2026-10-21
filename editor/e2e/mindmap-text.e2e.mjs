// Long node text wraps at maxWidth, even without break opportunities, and clamps to three
// lines; layout must reserve only the clamped height, and the full text goes to the tooltip.
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

const LONG = '2'.repeat(200);
const MD = ['# Root', '## A', '- short', '- ' + LONG].join('\n');

test('long node text wraps, clamps and keeps its full text in the tooltip', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 900, height: 700 } });
  try {
    await page.goto(server.url + '/?f=mindmap-grow.md');
    await page.waitForFunction(() => window.ready);
    await page.evaluate((md) => {
      const v = window.view;
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: 'x\n\n```mindmap\n' + md + '\n```\n' }, selection: { anchor: 0 } });
    }, MD);
    await page.waitForSelector('.cm-card-widget g.markmap-node');
    await page.waitForTimeout(400);

    const m = await page.evaluate((long) => {
      const text = [...document.querySelectorAll('.mm-text')].find((e) => e.textContent === long);
      const fo = text.closest('foreignObject');
      const lh = parseFloat(getComputedStyle(text).lineHeight);
      return { width: text.scrollWidth, foHeight: +fo.getAttribute('height'), lh, title: text.title, clamped: text.scrollHeight > text.clientHeight };
    }, LONG);
    assert.ok(m.width <= 260, `text ${m.width}px wider than maxWidth`);
    assert.ok(m.clamped, 'long text not clamped');
    assert.ok(m.foHeight < m.lh * 4, `layout reserved ${m.foHeight}px, more than three lines`);
    assert.equal(m.title, LONG);
  } finally {
    await page.close();
  }
});
