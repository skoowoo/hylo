// Run with: node --test internal/server/view/assets/
//
// Geometry rules for the line icon sources (../icons/*.svg), which a glance
// at the specimen misses: every corner is rounded, a stroke that ends on
// another one ends on its centreline (or its round cap shows past the other
// stroke's edge, like an arrow shaft beyond a rounded tip), box corners are
// circular, and a zoom never pushes ink out of the 24px box.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const DIR = path.join(__dirname, '..', 'icons');
const ARGS = { M: 2, L: 2, H: 1, V: 1, C: 6, S: 4, Q: 4, T: 2, A: 7, Z: 0 };

// Corners that read as round anyway: the half-moon's inner corners sit on the
// outer ring, and the book's spine points are covered by the spine's caps.
const CUSPS_OK = new Set(['appearance', 'book-open', 'knowledge']);

function parse(d) {
  const toks = d.match(/[MmLlHhVvCcSsQqTtAaZz]|[-+]?(?:\d+\.\d*|\.\d+|\d+)(?:[eE][-+]?\d+)?/g) || [];
  const segs = [];
  let i = 0, cmd = null, cx = 0, cy = 0, sx = 0, sy = 0;
  while (i < toks.length) {
    if (/[A-Za-z]/.test(toks[i])) {
      cmd = toks[i++];
      if (cmd === 'Z' || cmd === 'z') { segs.push(['Z']); cx = sx; cy = sy; continue; }
    }
    const up = cmd.toUpperCase(), rel = cmd !== up, n = ARGS[up];
    const a = toks.slice(i, i + n).map(Number); i += n;
    if (up === 'M') {
      const x = rel ? cx + a[0] : a[0], y = rel ? cy + a[1] : a[1];
      segs.push(['M', x, y]); cx = sx = x; cy = sy = y; cmd = rel ? 'l' : 'L';
    } else if (up === 'H') { cx = rel ? cx + a[0] : a[0]; segs.push(['L', cx, cy]); }
    else if (up === 'V') { cy = rel ? cy + a[0] : a[0]; segs.push(['L', cx, cy]); }
    else if (up === 'A') {
      const x = rel ? cx + a[5] : a[5], y = rel ? cy + a[6] : a[6];
      segs.push(['A', a[0], a[1], a[2], a[3], a[4], x, y]); cx = x; cy = y;
    } else {
      const p = a.slice();
      if (rel) for (let k = 0; k < n; k += 2) { p[k] += cx; p[k + 1] += cy; }
      segs.push([up, ...p]); cx = p[n - 2]; cy = p[n - 1];
    }
  }
  return segs;
}

// Subpaths as items {k, start, end, seg}; a closing Z becomes a line.
function subpaths(segs) {
  const out = [];
  let cur = null;
  for (const s of segs) {
    if (s[0] === 'M') { cur = { start: [s[1], s[2]], items: [], closed: false }; out.push(cur); continue; }
    const prev = cur.items.length ? cur.items[cur.items.length - 1].end : cur.start;
    if (s[0] === 'Z') {
      cur.closed = true;
      if (Math.hypot(prev[0] - cur.start[0], prev[1] - cur.start[1]) > 1e-6) cur.items.push({ k: 'L', start: prev, end: cur.start });
      cur = { start: cur.start, items: [], closed: false }; out.push(cur);
      continue;
    }
    cur.items.push({ k: s[0], start: prev, end: [s[s.length - 2], s[s.length - 1]], seg: s });
  }
  return out.filter((p) => p.items.length);
}

const unit = (v) => { const l = Math.hypot(v[0], v[1]); return l > 1e-9 ? [v[0] / l, v[1] / l] : null; };
const dist = (a, b) => Math.hypot(a[0] - b[0], a[1] - b[1]);

// SVG arc endpoint form to centre form (SVG 1.1 F.6.5), elliptical arcs too.
function arc(s, seg) {
  let [, rx, ry, rotDeg, large, sweep, x, y] = seg;
  const phi = rotDeg * Math.PI / 180, cos = Math.cos(phi), sin = Math.sin(phi);
  const dx = (s[0] - x) / 2, dy = (s[1] - y) / 2;
  const x1 = cos * dx + sin * dy, y1 = -sin * dx + cos * dy;
  rx = Math.abs(rx); ry = Math.abs(ry);
  const lam = (x1 * x1) / (rx * rx) + (y1 * y1) / (ry * ry);
  if (lam > 1) { rx *= Math.sqrt(lam); ry *= Math.sqrt(lam); }
  const num = rx * rx * ry * ry - rx * rx * y1 * y1 - ry * ry * x1 * x1;
  const k = (large === sweep ? -1 : 1) * Math.sqrt(Math.max(num, 0) / (rx * rx * y1 * y1 + ry * ry * x1 * x1));
  const cx1 = k * rx * y1 / ry, cy1 = -k * ry * x1 / rx;
  const cx = cos * cx1 - sin * cy1 + (s[0] + x) / 2, cy = sin * cx1 + cos * cy1 + (s[1] + y) / 2;
  const ang = (ux, uy, vx, vy) => Math.atan2(ux * vy - uy * vx, ux * vx + uy * vy);
  const t1 = ang(1, 0, (x1 - cx1) / rx, (y1 - cy1) / ry);
  let dt = ang((x1 - cx1) / rx, (y1 - cy1) / ry, (-x1 - cx1) / rx, (-y1 - cy1) / ry);
  if (!sweep && dt > 0) dt -= 2 * Math.PI;
  if (sweep && dt < 0) dt += 2 * Math.PI;
  const at = (t) => [cx + rx * Math.cos(t) * cos - ry * Math.sin(t) * sin, cy + rx * Math.cos(t) * sin + ry * Math.sin(t) * cos];
  const dir = (t) => unit([(-rx * Math.sin(t) * cos - ry * Math.cos(t) * sin) * Math.sign(dt), (-rx * Math.sin(t) * sin + ry * Math.cos(t) * cos) * Math.sign(dt)]);
  return { t1, dt, at, dir };
}

// Direction of travel at an item's start and end.
function tangents(it, prevCtrl) {
  const s = it.start, e = it.end, g = it.seg;
  if (it.k === 'L') { const d = unit([e[0] - s[0], e[1] - s[1]]); return [d, d]; }
  if (it.k === 'A') {
    const a = arc(s, g);
    return [a.dir(a.t1), a.dir(a.t1 + a.dt)];
  }
  let c1, c2;
  if (it.k === 'C') { c1 = [g[1], g[2]]; c2 = [g[3], g[4]]; }
  else if (it.k === 'S') { c1 = prevCtrl ? [2 * s[0] - prevCtrl[0], 2 * s[1] - prevCtrl[1]] : s; c2 = [g[1], g[2]]; }
  else { c1 = c2 = [g[1], g[2]]; }
  return [unit([c1[0] - s[0], c1[1] - s[1]]) || unit([c2[0] - s[0], c2[1] - s[1]]),
          unit([e[0] - c2[0], e[1] - c2[1]]) || unit([e[0] - c1[0], e[1] - c1[1]])];
}

function samplePath(d) {
  const pts = [];
  for (const sp of subpaths(parse(d))) {
    let prevC = null;
    for (const it of sp.items) {
      const s = it.start, e = it.end, g = it.seg;
      if (it.k === 'L') {
        const n = Math.max(2, Math.ceil(dist(s, e) / 0.05));
        for (let i = 0; i <= n; i++) pts.push([s[0] + (e[0] - s[0]) * i / n, s[1] + (e[1] - s[1]) * i / n]);
        prevC = null;
      } else if (it.k === 'A') {
        const a = arc(s, g);
        for (let i = 0; i <= 400; i++) pts.push(a.at(a.t1 + a.dt * i / 400));
        prevC = null;
      } else {
        let c1, c2;
        if (it.k === 'C') { c1 = [g[1], g[2]]; c2 = [g[3], g[4]]; }
        else if (it.k === 'S') { c1 = prevC ? [2 * s[0] - prevC[0], 2 * s[1] - prevC[1]] : s; c2 = [g[1], g[2]]; }
        else { const q = [g[1], g[2]]; c1 = [s[0] + 2 / 3 * (q[0] - s[0]), s[1] + 2 / 3 * (q[1] - s[1])]; c2 = [e[0] + 2 / 3 * (q[0] - e[0]), e[1] + 2 / 3 * (q[1] - e[1])]; }
        for (let i = 0; i <= 200; i++) {
          const t = i / 200, u = 1 - t;
          pts.push([0, 1].map((j) => u ** 3 * s[j] + 3 * u * u * t * c1[j] + 3 * u * t * t * c2[j] + t ** 3 * e[j]));
        }
        prevC = c2;
      }
    }
  }
  return pts;
}

const attr = (tag, n) => { const m = tag.match(new RegExp(`\\s${n}="([^"]*)"`)); return m ? m[1] : null; };

function sampleShape(tag) {
  const g = (n) => Number(attr(tag, n) || 0);
  if (tag.startsWith('<circle') || tag.startsWith('<ellipse')) {
    const cx = g('cx'), cy = g('cy'), rx = g('r') || g('rx'), ry = g('r') || g('ry');
    return Array.from({ length: 2001 }, (_, i) => [cx + rx * Math.cos(i / 2000 * 2 * Math.PI), cy + ry * Math.sin(i / 2000 * 2 * Math.PI)]);
  }
  if (tag.startsWith('<rect')) {
    const x = g('x'), y = g('y'), w = g('width'), h = g('height'), r = Math.min(g('rx'), w / 2, h / 2);
    return samplePath(`M${x + r} ${y}H${x + w - r}A${r} ${r} 0 0 1 ${x + w} ${y + r}V${y + h - r}A${r} ${r} 0 0 1 ${x + w - r} ${y + h}H${x + r}A${r} ${r} 0 0 1 ${x} ${y + h - r}V${y + r}A${r} ${r} 0 0 1 ${x + r} ${y}Z`);
  }
  return samplePath(attr(tag, 'd'));
}

const icons = fs.readdirSync(DIR).filter((f) => f.endsWith('.svg')).map((f) => {
  const text = fs.readFileSync(path.join(DIR, f), 'utf8');
  return { name: f.slice(0, -4), text, zoom: Number(attr(text, 'data-zoom') || 1), shapes: text.match(/<(?:path|rect|circle|ellipse)\b[^>]*\/?>/g) || [] };
});

test('there are icons to check', () => {
  assert.ok(icons.length > 50, `found only ${icons.length} icon sources`);
});

for (const icon of icons) {
  test(`${icon.name}: every corner is rounded`, () => {
    if (CUSPS_OK.has(icon.name)) return;
    for (const tag of icon.shapes.filter((t) => t.startsWith('<path'))) {
      for (const sp of subpaths(parse(attr(tag, 'd')))) {
        const tg = []; let prevC = null;
        for (const it of sp.items) {
          tg.push(tangents(it, prevC));
          prevC = it.k === 'C' ? [it.seg[3], it.seg[4]] : it.k === 'S' ? [it.seg[1], it.seg[2]] : null;
        }
        const n = sp.items.length;
        for (let i = 0; i < (sp.closed ? n : n - 1); i++) {
          const a = tg[i][1], b = tg[(i + 1) % n][0];
          if (!a || !b) continue;
          const turn = Math.acos(Math.max(-1, Math.min(1, a[0] * b[0] + a[1] * b[1]))) * 180 / Math.PI;
          assert.ok(turn <= 20, `sharp ${Math.round(turn)}° corner at ${sp.items[i].end.map((v) => v.toFixed(2))}`);
        }
      }
    }
  });

  test(`${icon.name}: strokes that meet end on each other's centreline`, () => {
    const pts = icon.shapes.map(sampleShape);
    icon.shapes.forEach((tag, k) => {
      if (!tag.startsWith('<path') || attr(tag, 'fill') === 'currentColor') return;
      for (const sp of subpaths(parse(attr(tag, 'd')))) {
        if (sp.closed) continue;
        for (const e of [sp.start, sp.items[sp.items.length - 1].end]) {
          pts.forEach((other, m) => {
            if (m === k || !other.length) return;
            const d = Math.min(...other.map((p) => dist(e, p)));
            assert.ok(d <= 0.12 || d >= 1, `end ${e.map((v) => v.toFixed(2))} is ${d.toFixed(2)} off another stroke, so its cap shows past it`);
          });
        }
      }
    });
  });

  test(`${icon.name}: box corners are circular`, () => {
    for (const tag of icon.shapes.filter((t) => t.startsWith('<rect'))) {
      const rx = attr(tag, 'rx'), ry = attr(tag, 'ry');
      assert.ok(ry === null || ry === rx, `elliptical corner: ${tag}`);
    }
  });

  test(`${icon.name}: the zoom keeps all ink inside the box`, () => {
    // The drawn stroke is 2 box units wide at every zoom, so ink reaches 1 past the centreline.
    for (const p of icon.shapes.flatMap(sampleShape)) {
      for (const v of p) {
        const z = 12 + (v - 12) * icon.zoom;
        assert.ok(z - 1 >= -0.01 && z + 1 <= 24.01, `ink at ${z.toFixed(2)} leaves the box (zoom ${icon.zoom})`);
      }
    }
  });
}
