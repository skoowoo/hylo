// ── Images gallery embedded in the list pane (lightbox + select mode) ─────
var _lbOpen = null; // set by homeCtrl.init; receives the lightbox data object

window._imgSelectMode = false;
window._imgUpdateSelected = null;

window.openImageLightbox = function (el) {
  if (window._imgSelectMode) {
    el.classList.toggle('is-selected');
    if (window._imgUpdateSelected) window._imgUpdateSelected();
    return;
  }
  var d = el.dataset;
  var notes = [];
  try {
    var parsed = JSON.parse(d.imgNotes || '[]');
    if (Array.isArray(parsed)) notes = parsed;
  } catch (_) { /* ignore */ }
  if (_lbOpen) _lbOpen({
    src: d.imgSrc, name: d.imgName, dir: d.imgDir,
    size: d.imgSize, time: d.imgTime, ext: d.imgExt,
    notes: notes,
  });
};

function extToType(ext) {
  var m = {
    '.jpg': 'JPEG Image', '.jpeg': 'JPEG Image', '.png': 'PNG Image',
    '.gif': 'GIF Image', '.webp': 'WebP Image', '.avif': 'AVIF Image', '.svg': 'SVG Vector',
  };
  return m[(ext || '').toLowerCase()] || ((ext || '').toUpperCase().replace('.', '') + ' Image');
}

var __hyloTrashIcon = '<span class="confirm-title-icon"><svg width="13" height="13" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10M5 4V3a1 1 0 011-1h2a1 1 0 011 1v1M12 4l-1 8H3L2 4"/><path d="M6 7v3M8 7v3"/></svg></span>';

function __hyloConfirmDelete(title, message) {
  return window.showConfirm({ titleHTML: __hyloTrashIcon + title, message: message, confirmLabel: 'Delete', danger: true });
}

function __hyloDeleteImage(dir, name) {
  return fetch('/api/images/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ dir: dir, name: name }),
  });
}

function __hyloImgGridEmptyCheck() {
  var grid = document.getElementById('img-grid');
  if (!grid || grid.querySelector('.img-card, .img-sentinel, .img-empty')) return;
  var empty = document.createElement('div');
  empty.className = 'img-empty';
  empty.textContent = 'No images found';
  grid.appendChild(empty);
}

function homeImagesMixin() {
  return {
    lightbox: null,
    selectMode: false,
    selectedCount: 0,
    initImages() {
      _lbOpen = (data) => { this.lightbox = data; };
      // Route both overlays through the shared ESC stack (shared_keys.go)
      // instead of @keydown.escape.window: that binding fires on window's
      // bubble phase, but the stack's document-capture listener runs first
      // and swallows Escape whenever the editor's content pane (or any other
      // stack entry) is open, so window never sees the key.
      this.$watch('lightbox', (val) => {
        if (val) { if (window.__hyloEscPush) window.__hyloEscPush('lightbox', () => { this.lightbox = null; }); }
        else if (window.__hyloEscPop) window.__hyloEscPop('lightbox');
      });
      window._imgUpdateSelected = () => {
        this.selectedCount = document.querySelectorAll('.img-card.is-selected').length;
      };
    },
    // ── Images: select mode + bulk delete ──────────────────────────────────
    enterSelectMode() {
      this.lightbox = null;
      this.selectMode = true;
      window._imgSelectMode = true;
    },
    exitSelectMode() {
      this.selectMode = false;
      window._imgSelectMode = false;
      document.querySelectorAll('.img-card.is-selected').forEach((c) => c.classList.remove('is-selected'));
      this.selectedCount = 0;
    },
    async deleteSelected() {
      var cards = Array.from(document.querySelectorAll('.img-card.is-selected'));
      if (!cards.length) return;
      var n = cards.length;
      var noun = n + ' image' + (n > 1 ? 's' : '');
      var ok = await __hyloConfirmDelete('Delete ' + noun, 'Permanently delete ' + noun + '? This cannot be undone.');
      if (!ok) return;
      var results = await Promise.allSettled(cards.map((card) =>
        __hyloDeleteImage(card.dataset.imgDir, card.dataset.imgName).then((r) => ({ ok: r.ok, card }))
      ));
      results.forEach((r) => {
        if (r.status === 'fulfilled' && r.value.ok) r.value.card.remove();
      });
      this.exitSelectMode();
      __hyloImgGridEmptyCheck();
    },
    closeLightbox() { this.lightbox = null; },
    async deleteLightboxImage() {
      var lb = this.lightbox;
      if (!lb || !lb.name || lb.dir == null) return;
      var linkHint = (lb.notes && lb.notes.length)
        ? (' It is still referenced from ' + lb.notes.length + ' note(s); those embeds will break.')
        : '';
      var ok = await __hyloConfirmDelete('Delete image', 'Permanently delete "' + lb.name + '" from the vault?' + linkHint);
      if (!ok) return;
      try {
        var resp = await __hyloDeleteImage(lb.dir, lb.name);
        if (!resp.ok) {
          window.showError((await resp.text()).trim() || 'Delete failed.', 'Delete failed');
          return;
        }
        var dir = lb.dir, name = lb.name;
        this.closeLightbox();
        document.querySelectorAll('#img-grid .img-card').forEach(function (c) {
          if (c.dataset.imgDir === dir && c.dataset.imgName === name) c.remove();
        });
        __hyloImgGridEmptyCheck();
      } catch (e) {
        window.showError((e && e.message) ? e.message : 'Delete failed.', 'Delete failed');
      }
    },
    // Close lightbox first, then open the note in the content pane after the
    // leave animation (160 ms) finishes so the two panels don't collide.
    async openLinkedNote(noteName) {
      this.lightbox = null;
      try {
        var resp = await fetch('/api/notes/resolve', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: noteName + '.md' }),
        });
        if (!resp.ok) return;
        var data = await resp.json();
        var matches = data.matches;
        if (!Array.isArray(matches) || matches.length === 0) return;
        var n = matches[0];
        var notePath = n.dir === '/' ? '/' + n.name : n.dir + '/' + n.name;
        await new Promise(function (r) { setTimeout(r, 180); });
        if (window.__hyloContentPane) {
          void window.__hyloContentPane.openNoteInContentPane(notePath, noteName, false, !!n.pinned);
        }
      } catch (e) { /* ignore */ }
    }
  };
}
