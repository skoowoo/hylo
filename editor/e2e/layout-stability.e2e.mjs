// What shows raw follows the caret, and showing it must never move the text
// around it: walking the caret over every kind of block leaves every other
// line where it was. Scrolling must not re-estimate lines already measured.
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

// The paragraph under the first heading in stability.md.
const BODY = 11;

const skip = (t) => t.skip('WebKit unavailable — run `npx playwright-core install webkit` (' + launchError.message.split('\n')[0] + ')');

async function open(fixture, height = 800) {
  const page = await browser.newPage({ viewport: { width: 900, height } });
  const warnings = [];
  page.on('console', (m) => { if (/failed to stabilize|loop restarted/i.test(m.text())) warnings.push(m.text()); });
  await page.goto(server.url + '/?f=' + fixture);
  await page.waitForFunction(() => window.ready);
  await page.evaluate(() => {
    window.settle = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 30))));
    window.tops = () => {
      const v = window.view;
      const out = [];
      for (let n = 1; n <= v.state.doc.lines; n++) out.push(v.lineBlockAt(v.state.doc.line(n).from).top);
      return out;
    };
  });
  return { page, warnings };
}

test('the caret revealing any line leaves the lines below in place', async (t) => {
  if (launchError) return skip(t);
  const { page, warnings } = await open('stability.md');
  try {
    const moved = await page.evaluate(async () => {
      const v = window.view;
      const doc = () => v.state.doc;
      const park = doc().line(doc().lines).to;
      const out = [];
      for (let k = 1; k < doc().lines; k++) {
        const line = doc().line(k);
        v.dispatch({ selection: { anchor: park } });
        v.dispatch({ effects: window.cm.EditorView.scrollIntoView(line.from, { y: 'center' }) });
        await window.settle();
        const base = window.tops();
        const block = v.lineBlockAt(line.from);
        const last = doc().lineAt(block.to).number;
        for (const pos of [line.from, line.to]) {
          v.dispatch({ selection: { anchor: pos } });
          await window.settle();
          // Hairline rows and the frontmatter hand the caret on; that's not a reveal.
          if (doc().lineAt(v.state.selection.main.head).number !== k) continue;
          const now = window.tops();
          for (let n = last + 1; n <= doc().lines; n++) {
            const d = now[n - 1] - base[n - 1];
            if (Math.abs(d) > 0.5) { out.push(`L${k} ${JSON.stringify(line.text)} moved L${n} by ${d.toFixed(1)}px`); break; }
          }
        }
      }
      return out;
    });
    assert.deepEqual(moved, []);
    assert.deepEqual(warnings, []);
  } finally {
    await page.close();
  }
});

// CM estimates unrendered lines from a text sample — the first drawn line of
// at most 20 printable ASCII chars in plain text nodes — and re-estimates all
// of them whenever that sample's metrics change. Short lines of different
// widths took turns as the sample, moving everything off screen on scroll, so
// no drawn line may qualify, including the caret's line once scrolled away.
test('no drawn line is ever taken as the text sample', async (t) => {
  if (launchError) return skip(t);
  const { page, warnings } = await open('samples.md', 500);
  try {
    const found = await page.evaluate(async () => {
      const v = window.view;
      const out = new Set();
      const scan = () => {
        for (const el of v.contentDOM.querySelectorAll('.cm-line')) {
          const kids = [...el.childNodes];
          const text = kids.map((k) => k.nodeValue || '').join('');
          if (kids.length && text.length <= 20 && kids.every((k) => k.nodeType === 3 && !/[^ -~]/.test(k.nodeValue))) out.add(text);
        }
      };
      v.dispatch({ selection: { anchor: v.state.doc.line(5).from } });
      for (let top = 0; top < v.scrollDOM.scrollHeight; top += 200) {
        v.scrollDOM.scrollTop = top;
        await window.settle();
        scan();
      }
      return [...out];
    });
    assert.deepEqual(found, []);
    assert.deepEqual(warnings, []);
  } finally {
    await page.close();
  }
});

// Holds while the card is the taller of the two; a longer source pushes on,
// as typing more lines would.
test('opening and leaving the frontmatter source keeps the text below in place', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('stability.md');
  try {
    const bodyTop = () => page.evaluate(async (n) => {
      await window.settle();
      const v = window.view;
      return v.lineBlockAt(v.state.doc.line(n).from).top;
    }, BODY);
    // The harness starts with the caret at 0, which opens the source.
    await page.evaluate((n) => window.view.dispatch({ selection: { anchor: window.view.state.doc.line(n).from } }), BODY);
    const rest = await bodyTop();
    await page.hover('.cm-fm-card');
    await page.click('.cm-fm-card .hy-btn');
    assert.equal(await page.evaluate(() => !!document.querySelector('.cm-fm-card')), false);
    const opened = await bodyTop();
    assert.ok(Math.abs(opened - rest) < 0.5, 'opening the source moved the body ' + rest + ' → ' + opened);
    await page.evaluate((n) => window.view.dispatch({ selection: { anchor: window.view.state.doc.line(n).from } }), BODY);
    assert.equal(await page.evaluate(() => !!document.querySelector('.cm-fm-card')), true);
    assert.ok(Math.abs((await bodyTop()) - rest) < 0.5, 'closing the source moved the body');
  } finally {
    await page.close();
  }
});

// A card scrolled out of the viewport loses its widget; it must come back as it
// was, not remount blank and redraw.
test('a card scrolled away and back returns drawn, not remounted', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('handbook.md', 700);
  try {
    const result = await page.evaluate(async () => {
      const v = window.view;
      const at = v.state.doc.toString().indexOf('```mindmap');
      v.dispatch({ effects: window.cm.EditorView.scrollIntoView(at, { y: 'center' }) });
      await new Promise((r) => setTimeout(r, 1500));
      const card = document.querySelector('.cm-card-widget');
      v.scrollDOM.scrollTop = 0;
      await window.settle();
      const gone = !card.isConnected;
      v.dispatch({ effects: window.cm.EditorView.scrollIntoView(at, { y: 'center' }) });
      // The first painted frame after the scroll back.
      await new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
      const back = document.querySelector('.cm-card-widget');
      return { gone, same: back === card, drawn: !!(back && back.querySelector('svg g')) };
    });
    assert.deepEqual(result, { gone: true, same: true, drawn: true });
  } finally {
    await page.close();
  }
});

// CM estimates the lines it hasn't drawn, and the scrollbar is proportional to that
// guess, so it lurched as each new stretch was measured. A note is measured whole
// behind the view once open (cm-live/scroll.js): the text in view and the cards
// on screen stay put meanwhile, and afterwards scrolling never changes the height.
test('an open note is measured whole without moving what is on screen', async (t) => {
  if (launchError) return skip(t);
  const { page, warnings } = await open('handbook.md');
  try {
    const result = await page.evaluate(async () => {
      const v = window.view, sd = v.scrollDOM;
      const at = v.state.doc.toString().lastIndexOf('```mindmap');
      v.dispatch({ effects: window.cm.EditorView.scrollIntoView(at, { y: 'center' }) });
      await new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
      const card = document.querySelector('.cm-card-widget');
      const box = sd.getBoundingClientRect();
      const pos = v.posAtCoords({ x: box.left + box.width / 2, y: box.top + box.height / 2 }, false);
      const y0 = v.coordsAtPos(pos).top;
      let drift = 0;
      for (let i = 0; i < 40; i++) {
        await new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
        drift = Math.max(drift, Math.abs(v.coordsAtPos(pos).top - y0));
      }
      const height = sd.scrollHeight;
      const heights = new Set();
      for (let top = 0; top < height; top += 200) {
        sd.scrollTop = top;
        await window.settle();
        heights.add(sd.scrollHeight - height);
      }
      return { drift: drift < 1, sameCard: document.querySelector('.cm-card-widget') === card || !card.isConnected, heights: [...heights] };
    });
    assert.deepEqual(result, { drift: true, sameCard: true, heights: [0] });
    assert.deepEqual(warnings, []);
  } finally {
    await page.close();
  }
});

// WKWebView applies a scrollTop written mid-gesture a frame late (playwright can't
// show that lag), so the editor scrolls itself and the text and the drawn thumb
// only ever move with the wheel.
test('a wheel scroll up a measured note never moves the text or the bar back', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('scroll.md');
  try {
    await page.waitForTimeout(600);
    await page.evaluate(async () => {
      window.view.scrollDOM.scrollTop = 1e7;
      await window.settle();
    });
    await page.mouse.move(500, 400);
    // Every painted frame for a few frames after the step: a stale bar lasts one.
    const frames = (pos) => page.evaluate((pos) => new Promise((r) => {
      const out = [];
      const next = () => requestAnimationFrame(() => setTimeout(() => {
        const v = window.view;
        const box = v.scrollDOM.getBoundingClientRect();
        let c = pos == null ? null : v.coordsAtPos(pos);
        if (!c || c.top < box.top + 60 || c.top > box.bottom - 60) {
          pos = v.posAtCoords({ x: box.left + box.width / 2, y: box.top + box.height / 2 }, false);
          c = v.coordsAtPos(pos);
        }
        out.push({ pos, y: c.top, thumb: document.querySelector('.cm-scrollbar-thumb').getBoundingClientRect().top });
        if (out.length < 4) next();
        else r(out);
      }));
      next();
    }), pos);
    let last = (await frames(null)).pop();
    const back = [];
    for (let i = 0; i < 60; i++) {
      const top = await page.evaluate(() => window.view.scrollDOM.scrollTop);
      await page.mouse.wheel(0, -120);
      for (const now of await frames(last.pos)) {
        if (now.pos === last.pos && now.y < last.y - 1) back.push(`text ${(now.y - last.y).toFixed(1)}px at step ${i}`);
        if (now.thumb > last.thumb + 1) back.push(`thumb ${(now.thumb - last.thumb).toFixed(1)}px at step ${i}`);
        last = now;
      }
      const moved = top - (await page.evaluate(() => window.view.scrollDOM.scrollTop));
      if (top > 120 && Math.abs(moved - 120) > 1) back.push(`step ${i} moved ${moved}px, corrected`);
    }
    assert.deepEqual(back, []);
  } finally {
    await page.close();
  }
});

// macOS scales each notch of a mouse wheel by its speed, so one flick is a few
// events of hundreds of pixels; WebKit gives only those fractional deltas. They
// glide over a few frames; a trackpad's whole-pixel deltas are followed at once.
test('a wheel notch glides to where it points, a trackpad delta lands at once', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('scroll.md');
  try {
    await page.waitForTimeout(600);
    const result = await page.evaluate(async () => {
      const v = window.view, sd = v.scrollDOM;
      const frame = () => new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
      const wheel = (deltaY) => v.contentDOM.dispatchEvent(new WheelEvent('wheel', { deltaY, bubbles: true, cancelable: true }));
      sd.scrollTop = 2000;
      await window.settle();
      const start = sd.scrollTop;
      wheel(240.5);
      wheel(260.25);
      const tops = [];
      for (let i = 0; i < 30; i++) {
        await frame();
        tops.push(sd.scrollTop);
      }
      const steady = sd.scrollTop;
      wheel(40);
      const direct = sd.scrollTop - steady;
      return {
        glided: tops[0] - start > 0 && tops[0] - start < 400,
        forward: tops.every((x, i) => i === 0 || x >= tops[i - 1]),
        landed: Math.abs(steady - start - 500.75) <= 1,
        direct,
      };
    });
    assert.deepEqual(result, { glided: true, forward: true, landed: true, direct: 40 });
  } finally {
    await page.close();
  }
});

test('a glide ends where the caret is moved to', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('scroll.md');
  try {
    await page.waitForTimeout(600);
    const top = await page.evaluate(async () => {
      const v = window.view, sd = v.scrollDOM;
      sd.scrollTop = 2000;
      await window.settle();
      v.contentDOM.dispatchEvent(new WheelEvent('wheel', { deltaY: 600.5, bubbles: true, cancelable: true }));
      await new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
      v.dispatch({ selection: { anchor: 0 }, scrollIntoView: true });
      for (let i = 0; i < 20; i++) await new Promise((r) => requestAnimationFrame(() => setTimeout(r)));
      return sd.scrollTop;
    });
    assert.ok(top < 100, `the glide went on to ${top}`);
  } finally {
    await page.close();
  }
});

test('a new text size measures the note whole again', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('handbook.md');
  try {
    await page.waitForTimeout(600);
    await page.addStyleTag({ content: '.cm-content { font-size: 19px !important; }' });
    await page.evaluate(() => window.view.requestMeasure());
    await page.waitForTimeout(1000);
    const heights = await page.evaluate(async () => {
      const sd = window.view.scrollDOM;
      const height = sd.scrollHeight;
      const out = new Set();
      for (let top = 0; top < height; top += 200) {
        sd.scrollTop = top;
        await window.settle();
        out.add(sd.scrollHeight - height);
      }
      return [...out];
    });
    assert.deepEqual(heights, [0]);
  } finally {
    await page.close();
  }
});

test('the scrollbar thumb drags to either end and a track press pages', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('scroll.md');
  try {
    await page.waitForTimeout(600);
    const top = () => page.evaluate(() => window.view.scrollDOM.scrollTop);
    const max = await page.evaluate(() => window.view.scrollDOM.scrollHeight - window.view.scrollDOM.clientHeight);
    const thumb = await page.evaluate(() => {
      const r = document.querySelector('.cm-scrollbar-thumb').getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    });
    await page.mouse.move(thumb.x, thumb.y);
    await page.mouse.down();
    for (let y = thumb.y; y <= 800; y += 40) await page.mouse.move(thumb.x, y);
    await page.mouse.move(thumb.x, 900);
    await page.mouse.up();
    await page.evaluate(() => window.settle());
    const end = await page.evaluate(() => window.view.scrollDOM.scrollHeight - window.view.scrollDOM.clientHeight);
    assert.ok(Math.abs((await top()) - end) < 1, `the drag ended at ${await top()}, not ${end} (was ${max})`);
    assert.equal(await page.evaluate(() => window.view.hasFocus), true, 'pressing the bar took focus from the text');
    const before = await top();
    await page.mouse.click(thumb.x, 20);
    await page.waitForTimeout(400);
    assert.ok(before - (await top()) > 500, 'a press above the thumb did not page up');
  } finally {
    await page.close();
  }
});

// WKWebView scrolls the caret into view whenever the editor gains focus (playwright's
// WebKit doesn't). Closing a card's fullscreen view hands focus back only with the caret on
// screen, so the note stays put either way.
test('closing a card\'s fullscreen view keeps the note where it was', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('handbook.md');
  try {
    const top = () => page.evaluate(() => Math.round(window.view.scrollDOM.scrollTop));
    const roundTrip = async (close) => {
      await page.hover('.cm-card-widget');
      await page.click('.cm-card-widget [title="Fullscreen"]');
      await page.waitForSelector('.hy-card-full:not(.hy-card-full-hidden)');
      await close();
      await page.waitForTimeout(300);
    };
    const closers = [() => page.keyboard.press('Escape'), () => page.click('.hy-card-full [title="Close (Esc)"]')];
    for (const caret of ['away', 'below']) {
      await page.evaluate(async (caret) => {
        const v = window.view;
        const doc = v.state.doc.toString();
        const at = doc.indexOf('```mindmap');
        v.focus();
        v.dispatch({ selection: { anchor: caret === 'away' ? 0 : v.state.doc.lineAt(doc.indexOf('\n```', at + 3) + 1).to + 2 } });
        v.dispatch({ effects: window.cm.EditorView.scrollIntoView(at, { y: 'center' }) });
        await new Promise((r) => setTimeout(r, 1200));
      }, caret);
      const before = await top();
      for (const close of closers) {
        await roundTrip(close);
        assert.equal(await top(), before, caret);
        assert.equal(await page.evaluate(() => window.view.hasFocus), caret === 'below', caret);
      }
    }
  } finally {
    await page.close();
  }
});

// A line starting or ending with hidden syntax has several zero-width pieces at that
// edge, where CM's own selection layer hit-tested at random and filled the whole line
// now and then: the highlight flashed while dragging. Each step must draw one piece,
// growing with the drag. (In a table row CM's mouse hit-testing still picks either side
// of a cell gap, so the head itself can step back there; only the drawing is checked.)
test('drag-selecting within a heading, quote or table row draws one growing piece', async (t) => {
  if (launchError) return skip(t);
  const { page } = await open('stability.md');
  try {
    for (const text of ['## Heading two', '> Quote line', '| one   | two   |']) {
      const box = await page.evaluate((text) => {
        const v = window.view;
        const n = v.state.doc.toString().split('\n').indexOf(text) + 1;
        const line = v.state.doc.line(n);
        v.dispatch({ selection: { anchor: v.state.doc.line(n + 4).from }, effects: window.cm.EditorView.scrollIntoView(line.from, { y: 'center' }) });
        return new Promise((r) => requestAnimationFrame(() => setTimeout(() => {
          const s = v.coordsAtPos(line.from + text.search(/\w/) + 1), e = v.coordsAtPos(line.to - 1);
          r({ x0: s.left, x1: e.left, y: (s.top + s.bottom) / 2 });
        }, 50)));
      }, text);
      await page.mouse.move(box.x0, box.y);
      await page.mouse.down();
      let last = 0;
      for (let i = 1; i <= 16; i++) {
        await page.mouse.move(box.x0 + ((box.x1 - box.x0) * i) / 16, box.y);
        const pieces = await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => setTimeout(() => {
          r([...document.querySelectorAll('.cm-lp-selection-layer .cm-selectionBackground')].map((e) => e.getBoundingClientRect().width));
        }))));
        if (!pieces.length) continue;
        assert.equal(pieces.length, 1, `${text}: step ${i} drew ${pieces.length} pieces`);
        if (!text.startsWith('|')) assert.ok(pieces[0] >= last - 0.5, `${text}: step ${i} shrank from ${last} to ${pieces[0]}`);
        last = pieces[0];
      }
      await page.mouse.up();
    }
  } finally {
    await page.close();
  }
});
