// The agent's text goes in whole and is revealed as it types, so nothing on the page
// moves while it does: no line changes place or height, and the note's height stays put.
// Typing it piece by piece used to reshape tables, lists and headings at every step.
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

const skip = (t) => t.skip('WebKit unavailable — run `npx playwright-core install webkit` (' + launchError.message.split('\n')[0] + ')');

const TABLE = [
  '| 方案 | 适合谁 | 主要成本 | 风险 |',
  '| --- | --- | --- | --- |',
  '| 自建机房 | 有稳定大规模负载、并且有专职运维团队的公司 | 一次性硬件投入和长期人力 | 容量规划失误会直接变成闲置或者事故 |',
  '| 公有云托管 | 早期团队，需求变化快 | 按量计费，规模大了账单会很难看 | 厂商锁定，出站流量费 |',
  '| Serverless | 流量波峰波谷明显的业务 | 冷启动与调用费 | 调试困难，状态外置 |',
].join('\n');

const MIXED = [
  '## 运维的价差', '',
  '这一段有 **加粗的判断** 和一个 [链接](https://example.com) ，还有 `code` 与 [[Agent Infra]]，句子写得比较长，好让它折行。', '',
  '> 写软件越来越容易，互联网规模的运维越来越难。', '',
  '- 监控、告警、备份', '- 回滚、权限、账单', '  - 嵌套的一条', '',
  '1. 有序第一', '2. 有序第二', '',
  '```mindmap', '# 运维', '## 脏活', '- 巡检', '## 判断', '- 边界', '```', '',
  '```bash', 'kubectl rollout undo deployment/api', '```', '',
  '---', '',
  '收尾，*斜体* 和 ~~删除线~~。',
].join('\n');

// Types `text` (into the middle of the note when `mid`) and records every visible line's
// place and height per frame once the text is in; returns what moved.
// `heights`: the note's height per change, then at the end.
async function typeAndWatch(text, { mid = false, width = 560 } = {}) {
  const page = await browser.newPage({ viewport: { width, height: 800 } });
  await page.goto(server.url + '/?f=agent-table.md');
  await page.waitForFunction(() => window.ready);
  try {
    return await page.evaluate(async ([text, mid]) => {
      const { cm, view: v } = window;
      v.contentDOM.blur();
      const settle = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 30))));
      // The note measures itself after opening (scroll.js); that settles first.
      for (let h = -1; h !== v.scrollDOM.scrollHeight;) {
        h = v.scrollDOM.scrollHeight;
        await new Promise((r) => setTimeout(r, 400));
      }
      if (mid) {
        v.dispatch({ effects: cm.EditorView.scrollIntoView(v.state.doc.line(20).from, { y: 'center' }) });
        await settle();
      }
      const plan = mid ? { pos: v.state.doc.line(21).to, text: '\n\n' + text } : cm.planAgentInsert(v.state, {}, text);
      const expected = v.state.doc.sliceString(0, plan.pos) + plan.text + v.state.doc.sliceString(plan.pos);
      const len0 = v.state.doc.length;
      const moved = [];
      const heights = [];
      const seen = new Map();
      let running = true;
      const frame = () => requestAnimationFrame(() => setTimeout(() => {
        if (!running) return;
        if (v.state.doc.length !== len0) {
          if (heights[heights.length - 1] !== v.scrollDOM.scrollHeight) heights.push(v.scrollDOM.scrollHeight);
          for (const b of v.viewportLineBlocks) {
            const was = seen.get(b.from);
            const now = `${Math.round(b.top)}/${Math.round(b.height)}`;
            if (was && was !== now) moved.push(`line at ${b.from}: ${was} -> ${now}`);
            seen.set(b.from, now);
          }
        }
        frame();
      }, 0));
      frame();
      const res = await cm.agentType(v, plan, { speed: 'normal', deadline: Date.now() + 4000 });
      await settle();
      running = false;
      const caret = v.coordsAtPos(v.state.selection.main.head);
      const box = v.scrollDOM.getBoundingClientRect();
      return {
        ok: res.ok, moved, heights: [...heights, v.scrollDOM.scrollHeight], same: v.state.doc.toString() === expected,
        caretOnScreen: !!caret && caret.top >= box.top && caret.bottom <= box.bottom,
      };
    }, [text, mid]);
  } finally {
    await page.close();
  }
}

// The note's height is final once the text is in. A line the caret reaches before the
// view does (a busy page's follow lags) is drawn on its own, which re-estimates it for a
// frame; that may blip off screen, within a line, never as a drift.
function steadyHeight(heights) {
  assert.equal(heights[heights.length - 1], heights[0], `note height drifted: ${heights}`);
  for (const h of heights) assert.ok(Math.abs(h - heights[0]) < 8, `note height jumped: ${heights}`);
}

test('a table typed into the middle of a note moves nothing around it', async (t) => {
  if (launchError) return skip(t);
  const r = await typeAndWatch(TABLE, { mid: true });
  assert.ok(r.ok && r.same);
  assert.deepEqual(r.moved, []);
  steadyHeight(r.heights);
});

test('screens of mixed blocks typed at the end of a note move nothing, and the caret stays in view', async (t) => {
  if (launchError) return skip(t);
  const r = await typeAndWatch([MIXED, TABLE, MIXED.replace(/mindmap/, 'js'), TABLE].join('\n\n'));
  assert.ok(r.ok && r.same);
  assert.deepEqual(r.moved, []);
  steadyHeight(r.heights);
  assert.ok(r.caretOnScreen);
});
