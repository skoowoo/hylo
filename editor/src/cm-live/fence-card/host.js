import { cardEntry } from './registry.js';
import { mountShell, mountInline, ensureStyle, cardFrame } from './shell.js';
import { focusInPlace } from '../focus.js';

// Synchronous handle; the renderer chunk lands later and fills the placeholder.
// A block card's placeholder is its shell's empty frame, so an editor measuring
// the widget before the shell mounts gets the card's final height.
// With `replace`, the card takes that node's place first and, when its renderer is
// already loaded, mounts there at once, at its final size.
// `partial` marks a source still being typed or streamed; update can change it.
export function mountCard(info, source, { onEdit, returnFocus, mode, replace, partial = false } = {}) {
  const entry = cardEntry(info);
  ensureStyle();
  const el = document.createElement('div');
  el.className = 'hy-card-slot';
  if (entry.inline) el.style.minHeight = entry.minHeight + 'px';
  else el.appendChild(cardFrame(entry.lang, entry.minHeight).root);
  let shell = null;
  let latest = source;
  let destroyed = false;

  const mount = (renderer) => {
    if (destroyed) return;
    el.style.minHeight = '';
    shell = entry.inline
      ? mountInline({ into: el, lang: entry.lang, source: latest, renderer, onEdit, mode, partial })
      : mountShell({ into: el, lang: entry.lang, source: latest, renderer, minHeight: entry.minHeight, onEdit, returnFocus, mode });
  };
  if (replace) replace.replaceWith(el);
  if (replace && entry.renderer) mount(entry.renderer);
  else entry.load().then(mount).catch(() => {
    if (destroyed) return;
    const msg = document.createElement('div');
    msg.className = 'hy-card-error';
    msg.textContent = `${entry.lang}: failed to load renderer`;
    el.replaceChildren(msg);
  });

  return {
    el,
    get mounted() {
      return !!shell;
    },
    update(next, opts) {
      latest = next;
      if (opts) partial = !!opts.partial;
      if (shell) shell.update(next, { partial });
    },
    destroy() {
      destroyed = true;
      if (shell) shell.destroy();
    },
  };
}

// For an editor hosting cards: focus coming back into it stays put (focus.js).
export function returnEditorFocus(view, el) {
  if (!view.dom.contains(el)) return false;
  focusInPlace(view);
  return true;
}
