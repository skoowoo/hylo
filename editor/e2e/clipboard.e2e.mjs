import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { webkit } from 'playwright-core';
import { startServer } from './server.mjs';

let browser;
const source = await readFile(new URL('../../internal/server/view/assets/clipboard.js', import.meta.url), 'utf8');
before(async () => { browser = await webkit.launch(); });
after(async () => { await browser?.close(); });

async function open(mode) {
  const page = await browser.newPage();
  await page.setContent('<textarea id="editor">original selection</textarea><button id="copy">Copy markdown</button><textarea id="paste"></textarea>');
  await page.addScriptTag({ content: source });
  await page.evaluate((mode) => {
    const clipboard = mode === 'missing' ? undefined : {
      writeText: async (text) => {
        if (mode === 'denied') throw new Error('Not allowed');
        window.written = text;
      },
    };
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: clipboard });
    const editor = document.getElementById('editor');
    editor.focus();
    editor.setSelectionRange(2, 9, 'backward');
    document.getElementById('copy').addEventListener('mousedown', (e) => e.preventDefault());
    document.getElementById('copy').onclick = () => {
      window.__hyloClipboard.writeText('# 中文 Markdown\n\n**bold**\n```js\n1 < 2\n```').then(() => {
        window.result = 'copied';
      }).catch((e) => { window.result = e.message; });
    };
  }, mode);
  return page;
}

for (const mode of ['missing', 'denied']) {
  test(`WebKit copies complete Markdown with Clipboard API ${mode} and preserves selection`, async () => {
    const page = await open(mode);
    try {
      await page.click('#copy');
      await page.waitForFunction(() => window.result);
      assert.deepEqual(await page.evaluate(() => ({
        result: window.result,
        focus: document.activeElement.id,
        start: document.activeElement.selectionStart,
        end: document.activeElement.selectionEnd,
        direction: document.activeElement.selectionDirection,
        fields: document.querySelectorAll('textarea').length,
      })), { result: 'copied', focus: 'editor', start: 2, end: 9, direction: 'backward', fields: 2 });
      await page.focus('#paste');
      await page.keyboard.press(process.platform === 'darwin' ? 'Meta+v' : 'Control+v');
      assert.equal(await page.inputValue('#paste'), '# 中文 Markdown\n\n**bold**\n```js\n1 < 2\n```');
    } finally { await page.close(); }
  });
}

test('uses Clipboard API when available', async () => {
  const page = await open('available');
  try {
    await page.evaluate(() => { document.execCommand = () => { throw new Error('Fallback should not run'); }; });
    await page.click('#copy');
    await page.waitForFunction(() => window.result);
    assert.equal(await page.evaluate(() => window.result), 'copied');
    assert.equal(await page.evaluate(() => window.written), '# 中文 Markdown\n\n**bold**\n```js\n1 < 2\n```');
  } finally { await page.close(); }
});

for (const throws of [false, true]) {
  test(`failed copy rejects and restores focus when command ${throws ? 'throws' : 'returns false'}`, async () => {
    const page = await open('missing');
    try {
      await page.evaluate((throws) => {
        document.execCommand = () => {
          if (throws) throw new Error('Copy unavailable');
          return false;
        };
      }, throws);
      await page.click('#copy');
      await page.waitForFunction(() => window.result);
      assert.equal(await page.evaluate(() => window.result), throws ? 'Copy unavailable' : 'Could not copy to clipboard.');
      assert.equal(await page.evaluate(() => document.activeElement.id), 'editor');
      assert.equal(await page.locator('textarea').count(), 2);
    } finally { await page.close(); }
  });
}

test('editor code block copies without Clipboard API and keeps its caret', async () => {
  const server = await startServer();
  const page = await browser.newPage();
  try {
    await page.goto(server.url + '/?f=mixed.md');
    await page.waitForFunction(() => window.ready);
    await page.evaluate(() => {
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined });
      const doc = '正文\n\n```js\nconst text = "中文";\n```\n';
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: doc }, selection: { anchor: 1 } });
      cm.forceParsing(view, doc.length, 200);
      view.focus();
    });
    await page.click('.cm-lp-code-copy');
    await page.waitForSelector('.cm-lp-code-copy.is-done');
    assert.equal(await page.evaluate(() => view.state.selection.main.head), 1);
    assert.equal(await page.evaluate(() => view.hasFocus), true);
    await page.evaluate(() => {
      const paste = document.createElement('textarea');
      paste.id = 'paste';
      document.body.append(paste);
      paste.focus();
    });
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+v' : 'Control+v');
    assert.equal((await page.inputValue('#paste')).trimEnd(), 'const text = "中文";');
  } finally {
    await page.close();
    await server.close();
  }
});
