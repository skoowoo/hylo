// A revealed "## " / "> " hangs in the margin. It must not move while the
// caret steps around it or text is typed after it, and the caret must stay
// out of it at the text start (WebKit composes IME text into whatever box
// holds the caret, which used to nudge the "#"s mid-input).
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

async function openBlankLine(t) {
  if (launchError) {
    t.skip('WebKit unavailable — run `npx playwright-core install webkit` (' + launchError.message.split('\n')[0] + ')');
    return null;
  }
  const page = await browser.newPage({ viewport: { width: 700, height: 300 } });
  await page.goto(server.url + '/?f=mixed.md');
  await page.waitForFunction(() => window.ready);
  await page.evaluate(() => {
    const v = window.view;
    v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: '测试\n\n' }, selection: { anchor: 4 } });
    v.focus();
  });
  return page;
}

const probe = (page) =>
  page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => {
    const v = window.view;
    const head = v.state.selection.main.head;
    const marker = v.contentDOM.querySelector('.cm-lp-hang-mark, .cm-lp-hang');
    const focus = getSelection().focusNode;
    const focusEl = focus.nodeType === 3 ? focus.parentElement : focus;
    resolve({
      head,
      text: v.state.doc.lineAt(head).text,
      markerLeft: marker.getBoundingClientRect().left,
      markerRight: marker.getBoundingClientRect().right,
      caretX: v.coordsAtPos(head).left,
      textStartX: v.coordsAtPos(v.state.doc.lineAt(head).to).left,
      caretInMarker: !!focusEl.closest('.cm-lp-hang-anchor, .cm-lp-hang'),
    });
  })));

for (const [name, syntax] of [['heading', '## '], ['quote', '> ']]) {
  test(`${name} marker stays put while typing and stepping into it`, async (t) => {
    const page = await openBlankLine(t);
    if (!page) return;
    try {
      await page.keyboard.type(syntax);
      const empty = await probe(page);
      assert.equal(empty.caretInMarker, false, 'caret at the text start sits inside the marker box');
      assert.ok(empty.markerRight <= empty.caretX + 1, 'marker overlaps the text start');

      await page.keyboard.type('测试');
      const typed = await probe(page);
      assert.ok(Math.abs(typed.markerRight - empty.markerRight) < 1, 'marker moved once text was typed');

      await page.keyboard.press('ArrowLeft');
      await page.keyboard.press('ArrowLeft');
      const start = await probe(page);
      assert.ok(Math.abs(start.caretX - empty.caretX) < 1, 'text start moved');
      await page.keyboard.press('ArrowLeft');
      const inside = await probe(page);
      assert.equal(inside.head, start.head - 1, '← from the text start should step into the marker');
      assert.ok(Math.abs(inside.markerRight - empty.markerRight) < 1, 'marker moved when the caret stepped in');
    } finally {
      await page.close();
    }
  });
}

test('clicking a hung heading marker puts the caret in it', async (t) => {
  const page = await openBlankLine(t);
  if (!page) return;
  try {
    await page.keyboard.type('## 标题');
    const box = await page.locator('.cm-lp-hang-mark').boundingBox();
    // Just right of the first "#": between the two.
    const x = await page.evaluate(() => {
      const r = document.createRange();
      const t = document.querySelector('.cm-lp-hang-mark').firstChild;
      r.setStart(t, 0);
      r.setEnd(t, 1);
      return r.getBoundingClientRect().right + 1;
    });
    await page.mouse.click(x, box.y + box.height / 2);
    const after = await probe(page);
    assert.equal(after.head, 5, 'click between the "#"s');
    assert.equal(after.caretInMarker, true);
  } finally {
    await page.close();
  }
});
