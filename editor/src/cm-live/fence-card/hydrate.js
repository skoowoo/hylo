// For HTML produced by marked: swap <pre><code class="language-X"> for the live renderer.
// Hosts own the lifecycle: release before re-rendering or removing, or the renderer's
// theme listener keeps the detached map alive.
import { cardEntry } from './registry.js';
import { mountCard } from './host.js';

const live = new Map();

export function releaseFenceCards(root) {
  for (const [host, handles] of live) {
    if (host !== root && !root.contains(host)) continue;
    handles.forEach((h) => h.destroy());
    live.delete(host);
  }
}

export function hydrateFenceCards(root) {
  releaseFenceCards(root);
  const handles = [];
  for (const code of root.querySelectorAll('pre > code[class*="language-"]')) {
    const lang = (/language-(\S+)/.exec(code.className) || [])[1];
    if (!lang || !cardEntry(lang)) continue;
    const handle = mountCard(lang, code.textContent);
    code.parentElement.replaceWith(handle.el);
    handles.push(handle);
  }
  if (handles.length) live.set(root, handles);
}
