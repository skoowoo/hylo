// Chrome shared by every fence renderer: sizing, toolbar, fullscreen overlay, theme tokens.
import { icon } from './icons.js';

const STYLE_ID = 'hy-card-style';

const CSS = `
.hy-card{position:relative;margin:.6em 0;border-radius:var(--r-sm,6px);background:var(--th-bg,rgba(24,24,27,.04));overflow:hidden}
.hy-card .markmap{--markmap-text-color:var(--prose-body,#27272a);--markmap-code-bg:var(--btn-bg,rgba(24,24,27,.06));--markmap-code-color:var(--prose-body,#27272a);--markmap-a-color:var(--accent-text,#2563eb);--markmap-a-hover-color:var(--accent-hov,#2563eb);--markmap-circle-open-bg:var(--bg,#fff);--markmap-font:400 var(--text-base,.875rem)/1.4 var(--font-ui,system-ui,sans-serif)}
.hy-card .markmap-link{stroke:var(--muted-soft,#9b9eac)}
.hy-card .mm-toggle{cursor:pointer;opacity:0;transition:opacity var(--motion-fast,100ms)}
.hy-card .markmap-node:hover>.mm-toggle,.hy-card .markmap-fold>.mm-toggle{opacity:1}
.hy-card .mm-toggle rect{fill:var(--markmap-circle-open-bg);stroke:var(--muted-soft,#9b9eac);transition:stroke var(--motion-fast,100ms)}
.hy-card .mm-toggle:hover rect{stroke:var(--accent,#5e6ad2)}
.hy-card .mm-toggle path{stroke:var(--muted,#6d7080);stroke-width:1.5;stroke-linecap:round}
.hy-card .mm-toggle:hover path{stroke:var(--accent,#5e6ad2)}
.hy-card .mm-toggle text{font:var(--fw-semibold,600) 10px var(--font-ui,system-ui,sans-serif);font-variant-numeric:tabular-nums;text-anchor:middle;dominant-baseline:central;fill:var(--muted,#6d7080)}
.hy-card .markmap-fold>.mm-toggle rect{fill:var(--btn-bg-hov,rgba(24,24,27,.1));stroke:none}
.hy-card .markmap-fold>.mm-toggle:hover text{fill:var(--accent-text,#5e6ad2)}
.hy-card .markmap-fold>.mm-toggle path{display:none}
.hy-card .markmap-foreign>div>div{padding:1px 4px;border-radius:var(--r-sm,6px)}
.hy-card .markmap-foreign>div>div:empty{display:none}
.hy-card .mm-text{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:3;overflow:hidden;overflow-wrap:anywhere}
.hy-card .markmap-node[data-depth="1"] .markmap-foreign>div>div{padding:6px 14px;background:var(--fg,#18181b);color:var(--bg,#fff);font-weight:var(--fw-semibold,600);font-size:var(--text-md,.9375rem)}
.hy-card .markmap-node[data-depth="2"] .markmap-foreign>div>div{padding:3px 10px;background:var(--btn-bg-hov,rgba(24,24,27,.1));color:var(--fg,#18181b);font-weight:var(--fw-medium,500)}
.hy-card .markmap-node:not([data-depth="1"],[data-depth="2"],[data-depth="3"]) .markmap-foreign>div>div{color:var(--muted,#6d7080);font-size:var(--text-sm,.8125rem)}
.hy-card .markmap-foreign code{font:.85em var(--font-mono,ui-monospace,monospace);padding:.1em .35em;border-radius:4px}
.hy-card-body{width:100%}
.hy-card-body svg{width:100%;height:100%;display:block}
.hy-card-bar{position:absolute;top:8px;right:8px;display:flex;gap:4px;opacity:0;transition:opacity var(--motion-base,160ms);will-change:opacity;z-index:1}
.hy-card:hover .hy-card-bar,.hy-card:focus-within .hy-card-bar{opacity:1}
.hy-btn{all:unset;box-sizing:border-box;cursor:pointer;display:flex;align-items:center;justify-content:center;width:24px;height:24px;color:var(--muted,#6d7080);border-radius:var(--r-sm,6px);transition:background-color var(--motion-fast,100ms),color var(--motion-fast,100ms)}
.hy-btn svg{width:14px;height:14px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}
.hy-btn:hover{color:var(--fg,#18181b);background:var(--btn-bg-hov,rgba(24,24,27,.1))}
.hy-btn:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px}
.hy-card-bar .hy-btn:not(:hover){background:var(--btn-bg,rgba(24,24,27,.06))}
.hy-card-inline{background:none;overflow:visible;contain:inline-size}
.hy-card-error{padding:12px;font-size:var(--text-xs,.75rem);color:var(--s-err,#dc2626)}
.hy-card-full{position:fixed;inset:0;z-index:800;display:flex;align-items:center;justify-content:center;padding:2rem;background:var(--lightbox-overlay-bg,rgba(0,0,0,.4));backdrop-filter:var(--glass-scrim-filter,blur(3px));-webkit-backdrop-filter:var(--glass-scrim-filter,blur(3px));-webkit-app-region:no-drag;transition:opacity var(--motion-base,160ms) ease}
.hy-card-full.hy-card-full-hidden{opacity:0}
.hy-card-full>.hy-card{width:100%;height:100%;max-width:1400px;margin:0;border-radius:var(--r-xl,16px);background:var(--surface-soft,#f5f5f7);box-shadow:var(--shadow-lg,0 14px 34px) var(--shadow-color,rgba(15,15,25,.1))}
.hy-card-full .hy-card-body{height:100%!important}
.hy-card-full .hy-card-bar{opacity:1;top:12px;right:12px}
.hy-card-full .markmap{--markmap-circle-open-bg:var(--surface-soft,#f5f5f7)}
.hy-pip{position:absolute;z-index:20;display:flex;flex-direction:column;min-width:240px;min-height:160px;background:var(--surface-soft,#f5f5f7);border:1px solid var(--line-edge,rgba(24,24,27,.16));border-radius:var(--r-lg,12px);box-shadow:var(--shadow-md,0 5px 14px) var(--shadow-color,rgba(15,15,25,.1));overflow:hidden}
.hy-pip-head{flex:none;display:flex;align-items:center;justify-content:space-between;gap:var(--space-xs,8px);height:32px;padding:0 4px 0 var(--space-sm,12px);font:var(--fw-medium,500) var(--text-2xs,.6875rem)/1 var(--font-ui,system-ui,sans-serif);letter-spacing:.04em;text-transform:uppercase;color:var(--muted,#6d7080);cursor:grab;user-select:none;touch-action:none}
.hy-pip-head:active{cursor:grabbing}
.hy-pip-sep{flex:none;height:1px;background:var(--line-hair,rgba(24,24,27,.09))}
.hy-pip-grip{position:absolute;right:0;bottom:0;width:16px;height:16px;z-index:2;cursor:nwse-resize;touch-action:none;color:var(--muted-soft,#9b9eac);background:linear-gradient(135deg,transparent 55%,currentColor 55%,currentColor 62%,transparent 62%,transparent 75%,currentColor 75%,currentColor 82%,transparent 82%);opacity:.7}
.hy-pip-grip:hover{opacity:1}
.hy-pip-body{flex:1;min-height:0;position:relative;overflow:hidden}
.hy-pip .hy-card-slot{height:100%;min-height:0!important}
.hy-pip .hy-card{height:100%;margin:0;border-radius:0!important;background:transparent}
.hy-pip .hy-card-body{height:100%!important}
.hy-pip .markmap{--markmap-circle-open-bg:var(--surface-soft,#f5f5f7)}
.hy-pip-folded{height:auto!important;min-height:0}
.hy-pip-folded .hy-pip-body,.hy-pip-folded .hy-pip-sep,.hy-pip-folded .hy-pip-grip{display:none}
`;

export function ensureStyle() {
  if (document.getElementById(STYLE_ID)) return;
  const el = document.createElement('style');
  el.id = STYLE_ID;
  el.textContent = CSS;
  document.head.appendChild(el);
}

export function iconButton(name, title, fn) {
  const b = document.createElement('button');
  b.type = 'button';
  b.className = 'hy-btn';
  b.innerHTML = icon(name);
  b.title = title;
  b.setAttribute('aria-label', title);
  // Keep the editor's selection (and the pip open) while clicking chrome.
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', fn);
  return b;
}

const ZOOM_STEP = 1.25;

function zoomButtons(getInst) {
  const by = (f) => () => { const i = getInst(); if (i && i.zoom) i.zoom(f); };
  return [iconButton('plus', 'Zoom in', by(ZOOM_STEP)), iconButton('minus', 'Zoom out', by(1 / ZOOM_STEP))];
}

const FADE_MS = 160;

// Lightbox-style overlay: scrim click, Esc, focus held on its buttons and restored on close.
function openOverlay({ lang, source, renderer, onClose }) {
  const prevFocus = document.activeElement;
  const overlay = document.createElement('div');
  overlay.className = 'hy-card-full hy-card-full-hidden';
  const panel = document.createElement('div');
  panel.className = `hy-card hy-card-${lang}`;
  panel.setAttribute('role', 'dialog');
  panel.setAttribute('aria-modal', 'true');
  panel.setAttribute('aria-label', lang);
  const body = document.createElement('div');
  body.className = 'hy-card-body';
  const bar = document.createElement('div');
  bar.className = 'hy-card-bar';
  let inst = null;
  let closed = false;

  const close = () => {
    if (closed) return;
    closed = true;
    document.removeEventListener('keydown', onKey, true);
    overlay.classList.add('hy-card-full-hidden');
    setTimeout(() => {
      if (inst) inst.destroy();
      overlay.remove();
    }, FADE_MS);
    if (prevFocus && prevFocus.isConnected) prevFocus.focus({ preventScroll: true });
    onClose();
  };
  const fit = iconButton('focus', 'Fit to view', () => inst && inst.fit && inst.fit());
  const shut = iconButton('x', 'Close (Esc)', close);
  bar.append(...zoomButtons(() => inst), fit, shut);
  panel.append(body, bar);
  overlay.appendChild(panel);

  function onKey(e) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      e.preventDefault();
      close();
    } else if (e.key === 'Tab') {
      e.preventDefault();
      (document.activeElement === fit ? shut : fit).focus();
    }
  }
  document.addEventListener('keydown', onKey, true);
  overlay.addEventListener('click', (e) => { if (e.target === overlay) close(); });
  document.body.appendChild(overlay);
  shut.focus({ preventScroll: true });
  // Opacity only: markmap fits against getBoundingClientRect, which a scale-in would skew.
  requestAnimationFrame(() => {
    if (closed) return;
    overlay.classList.remove('hy-card-full-hidden');
    inst = renderer.mount(body, source, { mode: 'full' });
  });
  return close;
}

// onEdit is supplied by the CM widget only; other hosts have no source to jump to.
// mode tells the renderer which view state it may share: 'card' | 'pip' | 'full'.
export function mountShell({ into, lang, source, renderer, minHeight, onEdit, mode = 'card' }) {
  ensureStyle();
  const root = document.createElement('div');
  root.className = `hy-card hy-card-${lang}`;
  const body = document.createElement('div');
  body.className = 'hy-card-body';
  body.style.height = minHeight + 'px';
  root.appendChild(body);

  let inst = null;
  let current = source;
  let destroyed = false;
  let closeOverlay = null;
  const fail = (e) => {
    if (destroyed) return;
    body.replaceChildren();
    const msg = document.createElement('div');
    msg.className = 'hy-card-error';
    msg.textContent = `${lang}: ${e && e.message ? e.message : e}`;
    body.appendChild(msg);
  };

  const bar = document.createElement('div');
  bar.className = 'hy-card-bar';
  bar.append(
    ...zoomButtons(() => inst),
    iconButton('focus', 'Fit to view', () => inst && inst.fit && inst.fit()),
    iconButton('expand', 'Fullscreen', () => {
      if (!closeOverlay) closeOverlay = openOverlay({ lang, source: current, renderer, onClose: () => { closeOverlay = null; } });
    }),
  );
  if (onEdit) bar.append(iconButton('code-xml', 'Edit source', () => onEdit()));
  root.appendChild(bar);
  into.replaceChildren(root);

  // Deferred a frame: the body's height is fixed, so the card paints before the renderer's first layout.
  requestAnimationFrame(() => {
    if (destroyed) return;
    try {
      inst = renderer.mount(body, current, { mode, onError: fail });
    } catch (e) {
      fail(e);
    }
  });

  return {
    update(next) {
      current = next;
      if (inst) inst.update(next);
    },
    destroy() {
      destroyed = true;
      if (closeOverlay) closeOverlay();
      if (inst) inst.destroy();
    },
  };
}

// Chrome for inline cards: none, the body just grows with the content.
// Attached before the renderer mounts, so it lays out at its final size right away.
export function mountInline({ into, lang, source, renderer }) {
  ensureStyle();
  const root = document.createElement('div');
  root.className = `hy-card hy-card-inline hy-card-${lang}`;
  const body = document.createElement('div');
  body.className = 'hy-card-body';
  root.appendChild(body);
  into.replaceChildren(root);
  let inst = null;
  try {
    inst = renderer.mount(body, source, { mode: 'card' });
  } catch (e) {
    const msg = document.createElement('div');
    msg.className = 'hy-card-error';
    msg.textContent = `${lang}: ${e && e.message ? e.message : e}`;
    body.replaceChildren(msg);
  }
  return {
    update(next) {
      if (inst) inst.update(next);
    },
    destroy() {
      if (inst) inst.destroy();
    },
  };
}
