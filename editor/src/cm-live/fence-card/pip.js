// Floating preview shown while the caret is inside a fence card's source. It lives outside the
// document flow, so editing never pushes the text below it around. Draggable and resizable;
// geometry is shared across blocks and remembered.
import { ViewPlugin } from '@codemirror/view';
import { selectionTouchesRange } from '../selection.js';
import { mountCard } from './host.js';
import { ensureStyle, iconButton } from './shell.js';
import { icon } from './icons.js';

const STORE_KEY = 'hy-card-pip';
const DEFAULT = { w: 380, h: 300 };
const MARGIN = 16;
const MIN_W = 240;
const MIN_H = 160;
const UPDATE_DELAY = 200;

function loadGeom() {
  try {
    return { ...DEFAULT, ...JSON.parse(localStorage.getItem(STORE_KEY) || '{}') };
  } catch {
    return { ...DEFAULT };
  }
}

function saveGeom(g) {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify(g));
  } catch {}
}

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

class Pip {
  constructor(view, info, body) {
    ensureStyle();
    this.view = view;
    this.info = info;
    this.body = body;
    this.folded = false;
    this.geom = loadGeom();
    this.el = document.createElement('div');
    this.el.className = 'hy-pip';

    const head = document.createElement('div');
    head.className = 'hy-pip-head';
    this.title = document.createElement('span');
    this.title.textContent = info;
    const fold = iconButton('minus', 'Collapse', () => {
      this.toggleFold();
      const label = this.folded ? 'Expand' : 'Collapse';
      fold.innerHTML = icon(this.folded ? 'plus' : 'minus');
      fold.title = label;
      fold.setAttribute('aria-label', label);
    });
    head.append(this.title, fold);

    this.content = document.createElement('div');
    this.content.className = 'hy-pip-body';
    const sep = document.createElement('div');
    sep.className = 'hy-pip-sep';
    const grip = document.createElement('div');
    grip.className = 'hy-pip-grip';
    this.el.append(head, sep, this.content, grip);

    if (getComputedStyle(view.dom).position === 'static') view.dom.style.position = 'relative';
    view.dom.appendChild(this.el);
    this.apply();

    this.card = mountCard(info, body, { mode: 'pip' });
    this.content.appendChild(this.card.el);

    head.addEventListener('pointerdown', (e) => this.startDrag(e));
    grip.addEventListener('pointerdown', (e) => this.startResize(e));
    // The editor box shrinks with the window; keep the panel inside it.
    this.ro = new ResizeObserver(() => this.apply());
    this.ro.observe(view.dom);
  }

  // Clamps to the editor box. Only a user gesture commits the clamped size; a smaller
  // window just squeezes the panel and it grows back with the window.
  apply(commit = false) {
    const box = this.view.dom.getBoundingClientRect();
    const g = this.geom;
    const w = clamp(g.w, MIN_W, Math.max(MIN_W, box.width));
    const h = clamp(g.h, MIN_H, Math.max(MIN_H, box.height));
    if (commit) Object.assign(g, { w, h });
    const shownH = this.folded ? this.el.offsetHeight : h;
    // First open lands bottom-right; afterwards left/top are whatever the user dragged to.
    if (g.left == null) g.left = box.width - w - MARGIN;
    if (g.top == null) g.top = box.height - h - MARGIN;
    g.left = clamp(g.left, 0, Math.max(0, box.width - w));
    g.top = clamp(g.top, 0, Math.max(0, box.height - shownH));
    Object.assign(this.el.style, { left: g.left + 'px', top: g.top + 'px', width: w + 'px', height: this.folded ? '' : h + 'px' });
  }

  // Folding keeps the bottom edge put, so the bar sinks to the floor and unfolds upward.
  toggleFold() {
    const g = this.geom;
    const bottom = g.top + (this.folded ? this.el.offsetHeight : g.h);
    this.folded = !this.folded;
    this.el.classList.toggle('hy-pip-folded', this.folded);
    this.el.style.height = this.folded ? '' : g.h + 'px';
    g.top = bottom - (this.folded ? this.el.offsetHeight : g.h);
    this.apply(true);
    if (!this.folded) saveGeom(g);
  }

  // Pointer capture keeps moves coming outside the element; cancel (e.g. system gesture) ends it like up.
  track(e, onMove, onEnd) {
    e.preventDefault();
    const target = e.currentTarget;
    target.setPointerCapture(e.pointerId);
    let lastX = e.clientX;
    let lastY = e.clientY;
    const move = (ev) => {
      onMove(ev.clientX - lastX, ev.clientY - lastY);
      lastX = ev.clientX;
      lastY = ev.clientY;
      this.apply(true);
    };
    const end = () => {
      target.removeEventListener('pointermove', move);
      target.removeEventListener('pointerup', end);
      target.removeEventListener('pointercancel', end);
      onEnd();
    };
    target.addEventListener('pointermove', move);
    target.addEventListener('pointerup', end);
    target.addEventListener('pointercancel', end);
  }

  startDrag(e) {
    if (e.button !== 0 || e.target.closest('button')) return;
    this.track(e, (dx, dy) => {
      this.geom.left += dx;
      this.geom.top += dy;
    }, () => { if (!this.folded) saveGeom(this.geom); });
  }

  startResize(e) {
    if (e.button !== 0) return;
    this.track(e, (dx, dy) => {
      this.geom.w += dx;
      this.geom.h += dy;
    }, () => saveGeom(this.geom));
  }

  retarget(body) {
    if (body === this.body) return;
    this.body = body;
    this.card.update(body);
  }

  destroy() {
    this.ro.disconnect();
    this.card.destroy();
    this.el.remove();
  }
}

export function cardPip(cardState) {
  return ViewPlugin.fromClass(class {
    constructor(view) {
      this.pip = null;
      this.timer = null;
      this.sync(view);
    }

    update(u) {
      if (u.selectionSet || u.state.field(cardState) !== u.startState.field(cardState)) this.sync(u.view);
    }

    active(view) {
      const { found } = view.state.field(cardState);
      return found.find((f) => selectionTouchesRange(view.state, f.from, f.to));
    }

    sync(view) {
      const hit = this.active(view);
      if (!hit) return this.close();
      if (this.pip && this.pip.info === hit.info) {
        clearTimeout(this.timer);
        this.timer = setTimeout(() => this.pip && this.pip.retarget(hit.body), UPDATE_DELAY);
        return;
      }
      this.close();
      this.pip = new Pip(view, hit.info, hit.body);
    }

    close() {
      clearTimeout(this.timer);
      if (this.pip) this.pip.destroy();
      this.pip = null;
    }

    destroy() {
      this.close();
    }
  });
}
