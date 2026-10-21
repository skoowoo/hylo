// Standalone entry for non-CodeMirror hosts (chat) — avoids loading the full editor bundle.
export { hydrateFenceCards, releaseFenceCards } from './cm-live/fence-card/hydrate.js';
export { preloadInlineCards } from './cm-live/fence-card/registry.js';
