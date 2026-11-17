import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { webkit } from 'playwright-core';
import { startServer } from './server.mjs';

let browser, server;
before(async () => { browser = await webkit.launch(); server = await startServer(); });
after(async () => { await browser?.close(); await server?.close(); });

async function open(native = false) {
  const page = await browser.newPage();
  await page.goto(server.url + '/?f=widgets.md');
  await page.waitForFunction(() => window.ready);
  await page.evaluate(() => {
    window.showError = (message) => { window.copyError = message; };
    document.getElementById('area').style.height = '70%';
  });
  for (const name of ['clipboard.js', 'shared_icons_line.js', 'image_clipboard.js']) {
    await page.addScriptTag({ content: await readFile(new URL('../../internal/server/view/assets/' + name, import.meta.url), 'utf8') });
  }
  if (native) {
    await page.evaluate(() => {
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined });
      window.__TAURI_INTERNALS__ = { invoke: async (command, args) => {
        const data = new Uint8Array(args.image);
        const header = new DataView(data.buffer);
        window.copiedImage = { command, signature: [...data.slice(0, 8)], width: header.getUint32(16), height: header.getUint32(20) };
      } };
    });
    await page.addScriptTag({ content: await readFile(new URL('../../desktop-tauri/src-tauri/src/shim.js', import.meta.url), 'utf8') });
  }
  return page;
}

for (const [surface, markup] of [
  ['editor', null],
  ['shorts', '<div class="shorts-img-cell"><img src="/image"><button data-copy-image="/image" aria-label="Copy image">Copy image</button></div>'],
  ['lightbox', '<div class="lb-panel"><div class="lb-viewer"><img class="lb-viewer-img" src="/image"></div><button data-copy-image="/image" aria-label="Copy image">Copy image</button></div>'],
]) {
  test(`${surface} copy button writes pixels without Clipboard API or opening the viewer`, async () => {
    const page = await open(true);
    try {
      if (markup) await page.evaluate(markup => {
        const surface = document.createElement('div');
        surface.id = 'surface';
        surface.style.cssText = 'position:fixed;inset:0;background:white;z-index:10';
        surface.innerHTML = markup;
        surface.querySelector('img').style.width = '120px';
        surface.addEventListener('click', () => { window.openedViewer = true; });
        document.body.append(surface);
      }, markup);
      if (!markup) await page.locator('.cm-lp-wikiimage').hover();
      const button = page.locator(markup ? '#surface [data-copy-image]' : '.cm-lp-image-copy');
      if (!markup) {
        const copy = await button.boundingBox();
        const zoom = await page.locator('.cm-lp-image-zoom').boundingBox();
        assert.ok(copy.x + copy.width <= zoom.x && Math.abs(copy.y - zoom.y) < 1, 'image actions overlap or are not on the same row');
      }
      await button.click();
      await page.waitForFunction(() => window.copiedImage || window.copyError);
      const result = await page.evaluate(() => ({ image: window.copiedImage, error: window.copyError }));
      assert.equal(result.error, undefined);
      assert.equal(result.image.command, 'plugin:clipboard-manager|write_image');
      assert.equal(result.image.width, 1600);
      assert.equal(result.image.height, 1000);
      assert.deepEqual(result.image.signature, [137, 80, 78, 71, 13, 10, 26, 10]);
      assert.equal(await page.evaluate(() => window.openedViewer), undefined);
      assert.equal(await button.getAttribute('title'), 'Copied');
    } finally { await page.close(); }
  });
}

test('browser copy writes a PNG that pastes as an image file', async () => {
  const page = await open();
  try {
    await page.evaluate(() => {
      const button = document.createElement('button');
      button.id = 'copy-image'; button.textContent = 'Copy';
      button.style.cssText = 'position:fixed;bottom:0';
      button.onclick = () => __hyloClipboard.writeImage('/image').then(() => { window.done = true; }).catch(e => { window.copyError = e.message; });
      document.body.append(button);
      const input = document.createElement('textarea');
      input.id = 'paste-image'; input.style.cssText = 'position:fixed;bottom:40px';
      input.addEventListener('paste', e => {
        window.pastedTypes = [...e.clipboardData.files].map(file => file.type);
        e.preventDefault();
      });
      document.body.append(input);
    });
    await page.click('#copy-image');
    await page.waitForFunction(() => window.done || window.copyError);
    assert.equal(await page.evaluate(() => window.copyError), undefined);
    await page.focus('#paste-image');
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+v' : 'Control+v');
    await page.waitForFunction(() => window.pastedTypes);
    assert.deepEqual(await page.evaluate(() => window.pastedTypes), ['image/png']);
  } finally { await page.close(); }
});

test('selecting an editor image copies its markdown, and a failed download reports an error', async () => {
  const page = await open(true);
  try {
    await page.evaluate(() => {
      const text = view.state.doc.toString();
      const from = text.indexOf('![a]');
      const markdown = '![a](/image?x)';
      view.dispatch({ selection: { anchor: from, head: from + markdown.length } });
      view.focus();
      const input = document.createElement('textarea');
      input.id = 'paste';
      input.addEventListener('paste', e => { window.pasted = e.clipboardData.getData('text/plain'); });
      document.body.append(input);
    });
    const key = process.platform === 'darwin' ? 'Meta' : 'Control';
    await page.keyboard.press(key + '+c');
    await page.focus('#paste');
    await page.keyboard.press(key + '+v');
    await page.waitForFunction(() => window.pasted);
    assert.equal(await page.evaluate(() => pasted), '![a](/image?x)');
    assert.equal(await page.evaluate(() => window.copiedImage), undefined);
    await page.evaluate(async () => {
      try { await __hyloClipboard.writeImage('/not-found'); }
      catch (e) { window.copyError = e.message; }
    });
    assert.match(await page.evaluate(() => copyError), /404/);
  } finally { await page.close(); }
});

test('JPEG images are converted to PNG and native clipboard failures remain visible', async () => {
  const page = await open(true);
  try {
    await page.evaluate(async () => {
      const canvas = document.createElement('canvas');
      canvas.width = 3; canvas.height = 2;
      canvas.getContext('2d').fillRect(0, 0, 3, 2);
      await __hyloClipboard.writeImage(canvas.toDataURL('image/jpeg'));
    });
    assert.deepEqual(await page.evaluate(() => [copiedImage.width, copiedImage.height, copiedImage.signature]), [3, 2, [137, 80, 78, 71, 13, 10, 26, 10]]);
    await page.evaluate(() => {
      window.__TAURI_INTERNALS__.invoke = async () => { throw new Error('Clipboard unavailable'); };
    });
    await page.locator('.cm-lp-wikiimage').hover();
    await page.getByRole('button', { name: 'Copy image', exact: true }).click();
    await page.waitForFunction(() => window.copyError);
    assert.equal(await page.evaluate(() => copyError), 'Clipboard unavailable');
  } finally { await page.close(); }
});

test('image right-clicks no longer open an application copy menu', async () => {
  const page = await open(true);
  try {
    await page.locator('.cm-lp-image-zoom').click();
    await page.locator('.cm-image-viewer img').click({ button: 'right' });
    assert.equal(await page.getByRole('menuitem', { name: 'Copy image' }).count(), 0);
    assert.equal(await page.locator('.cm-image-viewer:not(.cm-image-viewer-hidden)').count(), 1);
  } finally { await page.close(); }
});

test('keyboard copy does not write the open viewer', async () => {
  const page = await open(true);
  try {
    await page.locator('.cm-lp-image-zoom').click();
    await page.waitForSelector('.cm-image-viewer:not(.cm-image-viewer-hidden)');
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+c' : 'Control+c');
    await page.waitForTimeout(800);
    assert.equal(await page.evaluate(() => window.copiedImage), undefined);
  } finally { await page.close(); }
});

test('the gallery lightbox copy button follows the currently displayed image', async () => {
  const page = await open(true);
  try {
    const template = await readFile(new URL('../../internal/server/view/assets/home_images_lightbox.html', import.meta.url), 'utf8');
    const mixin = await readFile(new URL('../../internal/server/view/assets/home_images.js', import.meta.url), 'utf8');
    await page.addScriptTag({ content: mixin });
    await page.evaluate(template => {
      const root = document.createElement('div');
      root.id = 'surface';
      root.setAttribute('x-data', 'homeImagesMixin()');
      root.style.cssText = 'position:fixed;inset:0;background:white;z-index:10';
      root.innerHTML = template.replace(/\{\{[^}]*\}\}/g, '');
      document.body.append(root);
    }, template);
    await page.addScriptTag({ url: server.url + '/vendor/alpine.min.js' });
    await page.waitForFunction(() => window.Alpine);
    await page.evaluate(() => {
      const state = Alpine.$data(document.getElementById('surface'));
      state.lbData = { src: '/image', name: 'First', ext: '.svg', dir: null };
      state.lightbox = state.lbData;
    });
    await page.locator('#surface .lb-viewer-img').evaluate(img => { img.style.width = '120px'; });
    const button = page.locator('#surface [data-copy-image]');
    await button.click();
    await page.waitForFunction(() => window.copiedImage);
    assert.equal(await page.evaluate(() => copiedImage.width), 1600);
    await page.evaluate(() => {
      const canvas = document.createElement('canvas');
      canvas.width = 4; canvas.height = 3;
      const state = Alpine.$data(document.getElementById('surface'));
      state.lbData = { src: canvas.toDataURL(), name: 'Second', ext: '.png', dir: null };
      window.copiedImage = null;
    });
    await button.click();
    await page.waitForFunction(() => window.copiedImage);
    assert.equal(await page.evaluate(() => copiedImage.width), 4);
    assert.equal(await page.locator('.img-card [data-copy-image]').count(), 0);
  } finally { await page.close(); }
});
