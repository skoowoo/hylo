import { cardEntry } from './registry.js';
import { mountShell, ensureStyle } from './shell.js';

// Synchronous handle; the renderer chunk lands later and fills the placeholder.
export function mountCard(info, source, { onEdit, mode } = {}) {
  const entry = cardEntry(info);
  ensureStyle();
  const el = document.createElement('div');
  el.className = 'hy-card-slot';
  el.style.minHeight = entry.minHeight + 'px';
  let shell = null;
  let latest = source;
  let destroyed = false;

  entry.load().then((renderer) => {
    if (destroyed) return;
    shell = mountShell({ lang: entry.lang, source: latest, renderer, minHeight: entry.minHeight, onEdit, mode });
    el.style.minHeight = '';
    el.replaceChildren(shell.el);
  }).catch(() => {
    if (destroyed) return;
    const msg = document.createElement('div');
    msg.className = 'hy-card-error';
    msg.textContent = `${entry.lang}: failed to load renderer`;
    el.replaceChildren(msg);
  });

  return {
    el,
    update(next) {
      latest = next;
      if (shell) shell.update(next);
    },
    destroy() {
      destroyed = true;
      if (shell) shell.destroy();
    },
  };
}
