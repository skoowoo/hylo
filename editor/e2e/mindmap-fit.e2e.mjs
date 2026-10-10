// A mindmap typed line by line from outside the card (as the agent does) must end up
// fitted: its first render sees only the root, so the view has to follow the content.
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

const OUTLINE = [
  '## Notes', '- markdown files', '- wikilinks', '- live preview', '- mindmap cards',
  '## Agent', '- types in the editor', '- rewrites a selection', '- reads blocks',
  '## Knowledge', '- compile notes into units', '- index by domain', '- recap and review',
  '## Data', '- local first', '- plain files',
];

test('a mindmap that grows while the caret is outside stays fitted', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 900, height: 900 } });
  try {
    await page.goto(server.url + '/?f=mindmap-grow.md');
    await page.waitForFunction(() => window.ready);
    await page.waitForSelector('.cm-card-widget svg g');
    await page.waitForTimeout(300);
    for (const line of OUTLINE) {
      await page.evaluate((line) => {
        const v = window.view;
        const close = v.state.doc.toString().indexOf('\n```\n', v.state.doc.toString().indexOf('```mindmap') + 3);
        v.dispatch({ changes: { from: close, insert: '\n' + line } });
      }, line);
      await page.waitForTimeout(60);
    }
    await page.waitForTimeout(600);
    const box = await page.evaluate(() => {
      const svg = document.querySelector('.cm-card-widget svg');
      const s = svg.getBoundingClientRect();
      const g = svg.querySelector('g').getBoundingClientRect();
      return { s: [s.left, s.top, s.right, s.bottom], g: [g.left, g.top, g.right, g.bottom] };
    });
    const [sl, st, sr, sb] = box.s;
    const [gl, gt, gr, gb] = box.g;
    assert.ok(gl >= sl - 2 && gt >= st - 2 && gr <= sr + 2 && gb <= sb + 2, `map ${box.g} overflows card ${box.s}`);
  } finally {
    await page.close();
  }
});
