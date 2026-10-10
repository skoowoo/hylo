// Full-window view of an inline image. Lives on document.body (outside the
// editor) and never takes focus, so closing it can't scroll the editor the
// way a WebKit focus() return would.
const STYLE_ID = 'cm-image-viewer-styles';
const FADE_MS = 160;

function ensureStyles() {
  if (document.getElementById(STYLE_ID)) return;
  const style = document.createElement('style');
  style.id = STYLE_ID;
  style.textContent = `
.cm-image-viewer {
  position: fixed; inset: 0; z-index: 800;
  display: flex; align-items: center; justify-content: center; padding: 2rem;
  background: var(--lightbox-overlay-bg, rgba(0,0,0,0.6));
  backdrop-filter: var(--glass-scrim-filter, blur(3px));
  -webkit-backdrop-filter: var(--glass-scrim-filter, blur(3px));
  -webkit-app-region: no-drag;
  cursor: zoom-out;
  opacity: 1; transition: opacity ${FADE_MS}ms ease;
}
.cm-image-viewer.cm-image-viewer-hidden { opacity: 0; }
.cm-image-viewer img {
  max-width: 100%; max-height: 100%; object-fit: contain;
  border-radius: var(--r-sm, 6px);
  box-shadow: var(--shadow-lg, 0 14px 34px) var(--shadow-color, rgba(15,15,25,0.25));
}`;
  document.head.appendChild(style);
}

export function openImageViewer(src, alt) {
  ensureStyles();
  const overlay = document.createElement('div');
  overlay.className = 'cm-image-viewer cm-image-viewer-hidden';
  overlay.setAttribute('role', 'dialog');
  overlay.setAttribute('aria-label', alt || 'Image');
  const img = document.createElement('img');
  img.src = src;
  img.alt = alt || '';
  overlay.appendChild(img);

  let closed = false;
  const close = () => {
    if (closed) return;
    closed = true;
    document.removeEventListener('keydown', onKey, true);
    overlay.classList.add('cm-image-viewer-hidden');
    setTimeout(() => overlay.remove(), FADE_MS);
  };
  function onKey(e) {
    if (e.key !== 'Escape') return;
    e.preventDefault();
    e.stopPropagation();
    close();
  }
  // mousedown, not click: the editor below must not see the press either.
  overlay.addEventListener('mousedown', (e) => {
    e.preventDefault();
    close();
  });
  document.addEventListener('keydown', onKey, true);
  document.body.appendChild(overlay);
  requestAnimationFrame(() => overlay.classList.remove('cm-image-viewer-hidden'));
}
