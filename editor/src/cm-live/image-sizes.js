// Natural sizes of note images, kept across sessions. With the size known a
// picture's box is laid out at its final size before the file loads, and CM's
// height estimate for an image line it hasn't drawn is the real one, so
// scrolling onto it never moves the text or the scrollbar.
const STORE_KEY = 'hylo.imageSizes';
const STORE_MAX = 2000;
const CONCURRENCY = 4;

// Matches theme.js's .cm-lp-wikiimage cap.
export const IMAGE_MAX_HEIGHT = 360;

let sizes = null; // src → [width, height]
let saveTimer = 0;

function store() {
  if (sizes) return sizes;
  sizes = new Map();
  try {
    for (const [src, size] of JSON.parse(localStorage.getItem(STORE_KEY) || '[]')) sizes.set(src, size);
  } catch {}
  return sizes;
}

function save() {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    try {
      localStorage.setItem(STORE_KEY, JSON.stringify([...store()].slice(-STORE_MAX)));
    } catch {}
  }, 500);
}

const listeners = new Set();

// fn(src) after a size is learned or corrected; returns the unsubscribe.
export function onImageSize(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function imageSize(src) {
  return store().get(src) || null;
}

export function rememberImageSize(src, width, height) {
  if (!width || !height) return;
  const prev = store().get(src);
  if (prev && prev[0] === width && prev[1] === height) return;
  store().delete(src);
  store().set(src, [width, height]);
  save();
  for (const fn of listeners) fn(src);
}

// The box an image takes in a column `column` px wide: its own width up to the
// column, scaled down further to stay within IMAGE_MAX_HEIGHT.
export function imageBox([width, height], column) {
  const w = Math.min(width, column, (IMAGE_MAX_HEIGHT * width) / height);
  return { width: w, height: (w * height) / width };
}

// The editor's text column, for height estimates made without a view at hand.
let column = 720;
export const columnWidth = () => column;
export function setColumnWidth(px) {
  if (px > 0) column = px;
}

const queued = [];
const waiting = new Map(); // src → resolvers of probes in flight
// Failed this session: the widget's own load shows it broken; don't refetch.
const failed = new Set();
let active = 0;

function pump() {
  while (active < CONCURRENCY && queued.length) {
    const src = queued.shift();
    active++;
    const img = new Image();
    const done = () => {
      active--;
      if (img.naturalWidth) rememberImageSize(src, img.naturalWidth, img.naturalHeight);
      else failed.add(src);
      for (const resolve of waiting.get(src)) resolve();
      waiting.delete(src);
      pump();
    };
    img.onload = done;
    img.onerror = done;
    img.src = src;
  }
}

// Learns the sizes of `srcs` not known yet, in order: pass the nearest first.
// Resolves when all of them are known or have failed.
export function probeImageSizes(srcs) {
  if (typeof Image === 'undefined') return Promise.resolve();
  const pending = [];
  for (const src of srcs) {
    if (store().has(src) || failed.has(src)) continue;
    if (!waiting.has(src)) {
      waiting.set(src, []);
      queued.push(src);
    }
    pending.push(new Promise((resolve) => waiting.get(src).push(resolve)));
  }
  pump();
  return Promise.all(pending);
}
