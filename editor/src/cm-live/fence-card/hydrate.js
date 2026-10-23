// For HTML produced by marked: swap <pre><code class="language-X"> for the live renderer.
// Hosts own the lifecycle: release before removing, or the renderer's theme listener
// keeps the detached map alive. Re-hydrating a host after re-rendering its HTML keeps
// each card that is still in the same place and updates its source, so a streaming
// reply grows its cards in place instead of swapping a code block for one at the end.
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

// openTail: the last fence in root is still being streamed, so its card gets a partial source.
export function hydrateFenceCards(root, { openTail = false } = {}) {
  const prev = live.get(root) || [];
  live.delete(root);
  releaseFenceCards(root);
  const handles = [];
  const codes = root.querySelectorAll('pre > code[class*="language-"]');
  const tail = openTail ? codes[codes.length - 1] : null;
  for (const code of codes) {
    const lang = (/language-(\S+)/.exec(code.className) || [])[1];
    if (!lang || !cardEntry(lang)) continue;
    const source = code.textContent;
    const partial = code === tail;
    let h = prev[handles.length];
    if (h && h.lang === cardEntry(lang).lang && !root.contains(h.el)) {
      prev[handles.length] = null;
      // Back in the page first, so the card measures its new layout before paint.
      code.parentElement.replaceWith(h.el);
      if (h.source !== source || h.partial !== partial) h.update(source, { partial });
    } else {
      h = mountCard(lang, source, { replace: code.parentElement, partial });
      h.lang = cardEntry(lang).lang;
    }
    h.source = source;
    h.partial = partial;
    handles.push(h);
  }
  prev.forEach((h) => { if (h) h.destroy(); });
  if (handles.length) live.set(root, handles);
}
