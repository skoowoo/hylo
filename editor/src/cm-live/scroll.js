// The editor owns its scroll position. WKWebView scrolls an overflow:auto box in
// its UI process, so a scrollTop that CM writes mid-gesture (keeping the text
// still as lines above the viewport are measured) lands a frame late: the text
// jolted, and holding those writes back made the scrollbar jump at rest instead.
// With the scroller overflow:hidden only this page ever moves it: the wheel is
// applied here, CM's corrections land in the same frame, and the bar is drawn
// from the same numbers in CM's measure cycle. Touch devices keep native
// scrolling, which a script can't match for feel.
//
// CM only measures what it draws and estimates the rest, so the scrollbar was
// proportional to a guess that changed under it as unseen lines came into view.
// Once a note is open the whole of it is measured behind the view, a slice per
// frame, so the height map is exact before anyone scrolls there.
//
// A notched wheel comes in big steps (macOS scales each notch by how fast the
// wheel turns: one flick is 6–8 events of 40–400px), so those glide to where
// they point, as native scrolling animates them. A trackpad's deltas already
// arrive one per frame and are followed directly.
import { ViewPlugin, EditorView, keymap } from '@codemirror/view';
import { forceParsing } from '@codemirror/language';
import { keepingParkedCards } from './fence-card/card-field.js';

const SHOWN = 900; // ms the bar stays after the last scroll
const MIN_THUMB = 32;
const EDGE = 40; // px from an edge where a drag-and-drop over the note scrolls it
const SLICE = 8; // ms of measuring per frame
const BUSY = 200; // ms after a wheel, a drag or the view moving before measuring resumes
const GLIDE = 60; // ms time constant of a glide: most of the way in three

const owned = typeof matchMedia !== 'function' || !matchMedia('(any-pointer: coarse)').matches;

function wheelPixels(e, view, delta) {
  if (e.deltaMode === 1) return delta * view.defaultLineHeight;
  if (e.deltaMode === 2) return delta * view.scrollDOM.clientHeight;
  return delta;
}

// In WebKit an accelerated notch is the only pixel delta with a fraction; a
// trackpad's are whole pixels. Line and page deltas are notches too.
const stepped = (e) => e.deltaMode !== 0 || !Number.isInteger(e.deltaY);

// A box inside the note that can still scroll that way takes the wheel, as natively.
function nestedTakes(target, root, dy) {
  for (let el = target; el && el !== root; el = el.parentElement) {
    if (el.scrollHeight <= el.clientHeight + 1) continue;
    const oy = getComputedStyle(el).overflowY;
    if (oy !== 'auto' && oy !== 'scroll') continue;
    if (dy < 0 ? el.scrollTop > 0 : el.scrollTop + el.clientHeight < el.scrollHeight - 1) return true;
  }
  return false;
}

// WebKit's page step: most of the visible height, keeping some overlap.
const pageStep = (h) => Math.max(h * 0.875, h - 40, 1);

// A read-only note (reading view) pages with Space, as a document does; focus
// sits in the contenteditable there, where Space would otherwise do nothing.
const page = (dir) => (view) => {
  if (!view.state.readOnly) return false;
  const dy = dir * pageStep(view.scrollDOM.clientHeight);
  const owner = view.plugin(ownedScroll);
  if (owner) owner.glideBy(dy);
  else view.scrollDOM.scrollTop += dy;
  return true;
};
const readingKeys = keymap.of([{ key: 'Space', run: page(1), shift: page(-1) }]);

class OwnedScroll {
  constructor(view) {
    this.view = view;
    this.sd = view.scrollDOM;
    // Scroll positions are whole pixels; wheel steps and glides carry fractions.
    this.top = 0;
    this.wrote = -1;
    this.geo = null;
    this.drag = null;
    this.timer = 0;
    this.lastInput = 0;
    // Where the walk measuring the note goes next, and what its heights were measured against.
    this.walk = -1;
    this.walkKey = null;
    this.walkFrame = 0;
    this.walkSnap = null;
    this.walking = false;
    // The walk returns to where it started; that scroll event isn't a scroll.
    this.restTop = null;
    // A glide's destination, where it is now (fractional), and its frame.
    this.target = null;
    this.glideAt = 0;
    this.glideFrame = 0;
    this.glideTime = 0;

    this.bar = document.createElement('div');
    this.bar.className = 'cm-scrollbar';
    this.bar.setAttribute('aria-hidden', 'true');
    this.thumb = this.bar.appendChild(document.createElement('div'));
    this.thumb.className = 'cm-scrollbar-thumb';
    view.dom.appendChild(this.bar);

    this.onWheel = (e) => this.wheel(e);
    this.onScroll = () => {
      if (this.sd.scrollTop !== this.restTop) this.show();
      this.restTop = null;
      this.redraw();
    };
    this.onDragOver = (e) => this.dragOver(e);
    // A press on the bar must not move focus or start a text selection.
    this.onMouseDown = (e) => e.preventDefault();
    this.onPointerDown = (e) => this.press(e);
    this.onPointerMove = (e) => this.move(e);
    this.onPointerUp = () => this.release();
    view.dom.addEventListener('wheel', this.onWheel, { passive: false });
    this.sd.addEventListener('scroll', this.onScroll, { passive: true });
    this.sd.addEventListener('dragover', this.onDragOver);
    this.bar.addEventListener('mousedown', this.onMouseDown);
    this.bar.addEventListener('pointerdown', this.onPointerDown);
    this.bar.addEventListener('pointermove', this.onPointerMove);
    this.bar.addEventListener('pointerup', this.onPointerUp);
    this.bar.addEventListener('pointercancel', this.onPointerUp);
    this.redraw();
    this.measureAll();
  }

  // CM corrects scrollTop for re-measured lines at the very end of its measure
  // loop, after measure requests have run, so the bar is drawn once the loop
  // is done. A transaction's update comes before its measure, so that waits.
  updated(update) {
    if (update.docChanged) {
      // Its doc positions are the old doc's.
      this.walkSnap = null;
      if (this.changedUnseen(update)) this.measureAll();
    }
    // Moving the caret or scrolling to something is a new destination; the agent's
    // caret is followed by gliding (agent-typing.js), so its steps don't stop that.
    if (this.target != null && update.transactions.some((tr) => (tr.selection && !tr.isUserEvent('select.agent')) || tr.scrollIntoView)) this.stopGlide();
    // The pane sliding open or the view scrolling: measuring waits until it's still.
    if (!this.walking && (update.geometryChanged || update.viewportChanged)) this.lastInput = performance.now();
    if (!(update.geometryChanged || update.heightChanged || update.viewportChanged)) return;
    if (update.transactions.length) this.redraw();
    else this.draw(this.read());
  }

  // Lines changed outside the drawn viewport (a sync from disk) are estimates again.
  changedUnseen(update) {
    const { from, to } = update.view.viewport;
    let unseen = false;
    update.changes.iterChangedRanges((fromA, toA, fromB, toB) => {
      if (fromB < from || toB > to) unseen = true;
    });
    return unseen;
  }

  measureAll() {
    this.walk = 0;
    this.walkKey = null;
    this.scheduleWalk();
  }

  scheduleWalk() {
    if (this.walkFrame) return;
    this.walkFrame = requestAnimationFrame(() => {
      this.walkFrame = setTimeout(() => {
        this.walkFrame = 0;
        this.walkSlice();
      });
    });
  }

  // Each slice scrolls through the next stretch of the note, measuring as CM
  // would on the way, and returns before anything paints. A step is one
  // viewport: CM measures only the visible lines of what it draws.
  walkSlice() {
    const { view, sd } = this;
    if (this.walk < 0 || !view.dom.isConnected) return;
    // Out of sight (a pane still closed) CM draws and measures nothing; resumes on the next geometry change.
    if (!view.inView || !sd.clientHeight) return;
    // All of it is on screen, so all of it is measured.
    if (sd.scrollHeight <= sd.clientHeight) return void (this.walk = -1);
    if (view.composing || this.drag || this.target != null || performance.now() - this.lastInput < BUSY) return this.scheduleWalk();
    // A line measured before its part of the tree is parsed is drawn raw, so parse first.
    if (!forceParsing(view, view.state.doc.length, SLICE)) return this.scheduleWalk();
    // Lands a scrollIntoView still pending, so the slice returns to where it put the view.
    view.measure();
    // The first slice's snapshot until the view moves: a fresh one per slice
    // would round the position to whole pixels again each time.
    const snap = this.walkSnap && sd.scrollTop === this.walkSnap.top ? this.walkSnap.effect : view.scrollSnapshot();
    const end = performance.now() + SLICE;
    this.walking = true;
    try {
      keepingParkedCards(() => {
        do {
          sd.scrollTop = this.walk;
          view.measure();
          this.walk += sd.clientHeight;
        } while (this.walk < sd.scrollHeight && performance.now() < end);
        this.walkKey = this.measureKey();
        view.dispatch({ effects: snap });
        view.measure();
      });
    } finally {
      this.walking = false;
    }
    this.restTop = sd.scrollTop;
    this.walkSnap = { effect: snap, top: sd.scrollTop };
    this.wrote = -1;
    if (this.walk >= sd.scrollHeight) this.walk = -1;
    else this.scheduleWalk();
  }

  // Measures from..to right away, the way the walk does: text just written outside the
  // view would otherwise keep estimated heights, and the bar would shift once it's seen.
  measureRange(from, to) {
    const { view, sd } = this;
    if (!view.inView || !sd.clientHeight || view.composing) return;
    forceParsing(view, to, SLICE);
    view.measure();
    const snap = view.scrollSnapshot();
    const { viewport } = view;
    if (from >= viewport.from && to <= viewport.to) return;
    this.walking = true;
    try {
      keepingParkedCards(() => {
        let y = Math.max(0, view.lineBlockAt(from).top);
        do {
          sd.scrollTop = y;
          view.measure();
          y += sd.clientHeight;
        } while (y < view.lineBlockAt(to).bottom && y < sd.scrollHeight);
        view.dispatch({ effects: snap });
        view.measure();
      });
    } finally {
      this.walking = false;
    }
    this.restTop = sd.scrollTop;
    this.wrote = -1;
  }

  scrollTo(top) {
    this.stopGlide();
    this.write(top);
  }

  write(top) {
    const sd = this.sd;
    const to = Math.max(0, Math.min(top, sd.scrollHeight - sd.clientHeight));
    sd.scrollTop = to;
    this.top = to;
    this.wrote = sd.scrollTop;
  }

  // From the fractional position this page last wrote, unless something else
  // (CM's corrections, scrollIntoView) has moved the scroller since.
  current() {
    const now = this.sd.scrollTop;
    return Math.abs(now - this.wrote) < 1 ? this.top : now;
  }

  wheel(e) {
    if (e.defaultPrevented || e.ctrlKey) return;
    const view = this.view;
    const dy = wheelPixels(e, view, e.deltaY);
    if (!dy || Math.abs(wheelPixels(e, view, e.deltaX)) > Math.abs(dy)) return;
    if (nestedTakes(e.target, view.dom, dy)) return;
    e.preventDefault();
    this.lastInput = performance.now();
    if (stepped(e)) this.glideBy(dy);
    else this.scrollTo(this.current() + dy);
  }

  // Moves the destination; the view follows frame by frame, so steps arriving
  // mid-glide extend one motion instead of restarting it.
  glideBy(dy) {
    const sd = this.sd;
    if (this.target == null) {
      this.glideAt = this.current();
      this.target = this.glideAt;
      this.top = this.glideAt;
      this.wrote = sd.scrollTop;
      this.glideTime = 0;
    }
    this.target = Math.max(0, Math.min(this.target + dy, sd.scrollHeight - sd.clientHeight));
    if (!this.glideFrame) this.glideFrame = requestAnimationFrame((t) => this.glide(t));
  }

  glide(now) {
    this.glideFrame = 0;
    if (this.target == null) return;
    const sd = this.sd;
    // CM moved the view since the last frame (keeping text still as heights
    // changed above it): the glide carries that along rather than undoing it.
    const moved = sd.scrollTop - this.wrote;
    if (this.wrote >= 0 && Math.abs(moved) >= 1) {
      this.glideAt += moved;
      this.target += moved;
    }
    // A glide's first frame moves a frame's worth, whenever in the frame it began.
    const dt = this.glideTime ? Math.min(Math.max(now - this.glideTime, 0), 50) : 16.7;
    this.glideTime = now;
    this.glideAt = this.target - (this.target - this.glideAt) * Math.exp(-dt / GLIDE);
    if (Math.abs(this.target - this.glideAt) < 0.5) this.glideAt = this.target;
    this.lastInput = performance.now();
    this.write(this.glideAt);
    if (this.glideAt === this.target || Math.abs(sd.scrollTop - this.glideAt) >= 1) this.target = null;
    else this.glideFrame = requestAnimationFrame((t) => this.glide(t));
  }

  stopGlide() {
    this.target = null;
    if (this.glideFrame) cancelAnimationFrame(this.glideFrame);
    this.glideFrame = 0;
  }

  dragOver(e) {
    const box = this.sd.getBoundingClientRect();
    const near = e.clientY < box.top + EDGE ? e.clientY - box.top - EDGE : e.clientY > box.bottom - EDGE ? e.clientY - box.bottom + EDGE : 0;
    if (near) this.scrollTo(this.current() + near / 2);
  }

  // What CM's height estimates rest on: when it changes (wrap width, a font
  // arriving) CM estimates every line again, measured or not.
  measureKey() {
    const view = this.view;
    return `${view.contentDOM.clientWidth} ${view.defaultLineHeight} ${view.defaultCharacterWidth}`;
  }

  read() {
    const sd = this.sd;
    return { top: sd.scrollTop, height: sd.scrollHeight, client: sd.clientHeight, offset: sd.offsetTop, key: this.measureKey() };
  }

  redraw() {
    this.view.requestMeasure({ key: this, read: () => this.read(), write: (geo) => this.draw(geo) });
  }

  draw(geo) {
    this.geo = geo;
    if (this.walkKey != null && geo.key !== this.walkKey) this.measureAll();
    else if (this.walk >= 0) this.scheduleWalk();
    const range = geo.height - geo.client;
    this.bar.hidden = range < 1;
    if (range < 1) return;
    const len = Math.max(MIN_THUMB, (geo.client * geo.client) / geo.height);
    const at = (Math.min(Math.max(geo.top, 0), range) / range) * (geo.client - len);
    this.bar.style.top = geo.offset + 'px';
    this.bar.style.height = geo.client + 'px';
    this.thumb.style.height = len + 'px';
    this.thumb.style.transform = `translateY(${at}px)`;
  }

  show() {
    this.bar.classList.add('cm-scrollbar-shown');
    clearTimeout(this.timer);
    this.timer = setTimeout(() => {
      if (!this.drag) this.bar.classList.remove('cm-scrollbar-shown');
    }, SHOWN);
  }

  press(e) {
    if (e.button !== 0 || !this.geo) return;
    const thumb = this.thumb.getBoundingClientRect();
    if (e.target !== this.thumb) {
      // Pages toward the press, like the system's default track click.
      const step = pageStep(this.geo.client);
      this.glideBy(e.clientY < thumb.top ? -step : step);
      return;
    }
    this.bar.setPointerCapture(e.pointerId);
    this.bar.classList.add('cm-scrollbar-dragging');
    this.drag = { y: e.clientY, from: thumb.top - this.bar.getBoundingClientRect().top, len: thumb.height };
    this.show();
  }

  // The thumb follows the pointer as a fraction of the track, so heights CM
  // learns mid-drag move the text, not the thumb under the pointer.
  move(e) {
    const d = this.drag;
    if (!d) return;
    const room = this.geo.client - d.len;
    if (room <= 0) return;
    this.lastInput = performance.now();
    const frac = Math.min(Math.max((d.from + e.clientY - d.y) / room, 0), 1);
    const sd = this.sd;
    this.scrollTo(frac * (sd.scrollHeight - sd.clientHeight));
  }

  release() {
    if (!this.drag) return;
    this.drag = null;
    this.bar.classList.remove('cm-scrollbar-dragging');
    this.show();
  }

  destroy() {
    clearTimeout(this.timer);
    this.stopGlide();
    this.walk = -1;
    this.view.dom.removeEventListener('wheel', this.onWheel);
    this.sd.removeEventListener('scroll', this.onScroll);
    this.sd.removeEventListener('dragover', this.onDragOver);
    this.bar.remove();
  }
}

const ownedScroll = ViewPlugin.fromClass(OwnedScroll);

// Where the view is headed: a glide's destination, else where it is.
export function scrollDestination(view) {
  const owner = view.plugin(ownedScroll);
  return owner && owner.target != null ? owner.target : view.scrollDOM.scrollTop;
}

export function measureRange(view, from, to) {
  const owner = view.plugin(ownedScroll);
  if (owner) owner.measureRange(from, to);
}

export function glideTo(view, top) {
  const owner = view.plugin(ownedScroll);
  if (owner) owner.glideBy(top - (owner.target ?? owner.current()));
  else view.scrollDOM.scrollTop = top;
}

export const editorScroll = owned
  ? [
    readingKeys,
    ownedScroll,
    EditorView.updateListener.of((update) => {
      const plugin = update.view.plugin(ownedScroll);
      if (plugin) plugin.updated(update);
    }),
    EditorView.theme({
      '.cm-scroller': { overflow: 'hidden' },
      // The column whatever it holds: a line wider than the column used to widen
      // it, and on a new wrap width CM estimates every line again.
      '.cm-content': { minWidth: '0' },
      '.cm-scrollbar': {
        position: 'absolute',
        right: '0',
        width: '11px',
        zIndex: '2',
        opacity: '0',
        transition: 'opacity 0.3s',
        borderRadius: '0',
      },
      '.cm-scrollbar:hover, .cm-scrollbar-shown, .cm-scrollbar-dragging': { opacity: '1', transition: 'none' },
      '.cm-scrollbar-thumb': {
        position: 'absolute',
        top: '0',
        right: '2px',
        width: '7px',
        borderRadius: 'var(--r-full, 999px)',
        background: 'var(--scrollbar-thumb, rgba(0, 0, 0, 0.3))',
      },
      '.cm-scrollbar:hover .cm-scrollbar-thumb, .cm-scrollbar-dragging .cm-scrollbar-thumb': {
        background: 'var(--scrollbar-thumb-hov, rgba(0, 0, 0, 0.45))',
      },
    }),
  ]
  : [readingKeys];
