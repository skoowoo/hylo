// Folding and unfolding a mindmap card must leave the caret where it was: a press that lets the
// DOM selection land inside the card gets read by CM's next flush as the caret entering the fence.
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

const MD = ['# Root', '## A', '- a1', '- a2', '## B', '- b1'].join('\n');

test('fold toggles and node clicks keep the card rendered', async (t) => {
  if (launchError) return t.skip('WebKit unavailable');
  const page = await browser.newPage({ viewport: { width: 900, height: 700 } });
  try {
    await page.goto(server.url + '/?f=mindmap-grow.md');
    await page.waitForFunction(() => window.ready);
    await page.evaluate((md) => {
      const v = window.view;
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: 'x\n\n```mindmap\n' + md + '\n```\n\nend\n' }, selection: { anchor: 0 } });
    }, MD);
    await page.waitForSelector('.cm-card-widget g.markmap-node');
    await page.waitForTimeout(400);

    const node = (txt) => page.locator('g.markmap-node[data-depth="2"]', { hasText: txt }).first();
    const center = async (loc) => {
      const b = await loc.boundingBox();
      return [b.x + b.width / 2, b.y + b.height / 2];
    };
    const expectCard = async (step) => {
      const s = await page.evaluate(() => ({ head: window.view.state.selection.main.head, card: !!document.querySelector('.cm-card-widget svg') }));
      assert.deepEqual(s, { head: 0, card: true }, `after ${step}`);
    };

    await node('A').hover();
    await page.mouse.click(...(await center(node('A').locator('.mm-toggle rect'))));
    await page.waitForTimeout(400);
    await expectCard('folding');

    await page.mouse.click(...(await center(node('A').locator('.mm-toggle rect'))));
    await page.waitForTimeout(400);
    await expectCard('unfolding by the count');

    const leaf = await page.locator('g.markmap-node', { hasText: 'b1' }).locator('foreignObject').boundingBox();
    await page.mouse.click(leaf.x + 6, leaf.y + leaf.height / 2);
    await node('B').hover();
    await page.mouse.click(...(await center(node('B').locator('.mm-toggle rect'))));
    await page.waitForTimeout(400);
    await expectCard('a node click then folding');
  } finally {
    await page.close();
  }
});
