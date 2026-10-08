// ── Shorts composer: clipboard image extraction ────────────────────────────
// Like editor_session.js's __hyloEditorFindImageFile, but collects every
// image file in the clipboard instead of just the first — a short's
// composer accepts pasting several images at once (e.g. copying multiple
// files in Finder), unlike the note editor's single inline-insert paste.
function __hyloImageFilesFromClipboard(dt) {
  if (!dt) return [];
  var items = Array.from(dt.items || []);
  var files = [];
  for (var i = 0; i < items.length; i++) {
    if (items[i].kind === 'file' && items[i].type.indexOf('image/') === 0) {
      var f = items[i].getAsFile();
      if (f) files.push(f);
    }
  }
  return files;
}

// ── Shorts entries embedded in the list pane: intercept internal links ────
// Wikilinks (/notes?…) → open in the content pane; external → new tab.
document.addEventListener('click', function (e) {
  var a = e.target.closest ? e.target.closest('a') : null;
  if (!a || !a.closest('.shorts-entry-prose')) return;
  var href = a.getAttribute('href');
  if (!href) return;
  e.preventDefault();
  e.stopPropagation();
  if (href.indexOf('/notes?') === 0) {
    try {
      var u = new URL(href, window.location.origin);
      var name = u.searchParams.get('name') || '';
      var path = u.searchParams.get('path') || '';
      if (path && window.__hyloContentPane) {
        var title = a.textContent.trim() || path.split('/').pop().replace(/\.md$/, '');
        void window.__hyloContentPane.openNoteInContentPane(path, title, false, false);
      } else if (name) {
        void __hyloContentPaneOpenWikiLink(name.replace(/\.md$/, ''));
      }
    } catch (_) { /* ignore */ }
  } else if (/^https?:\/\//.test(href)) {
    window.open(href, '_blank', 'noopener,noreferrer');
  }
}, true);

function homeShortsMixin() {
  return {
    // ── Shorts: inline composer (replaces the old short_dialog.js overlay) ──
    shortComposeText: '',
    shortComposeSaving: false,
    // Attachments, uploaded out-of-band of typed text (Weibo/Twitter-style —
    // order in shortComposeText never matters, see saveShortCompose). Each:
    // {id, name, previewUrl (local, for instant feedback), src (server path
    // once uploaded), uploading, failed}.
    shortComposeImages: [],
    // ── Shorts: inline composer ─────────────────────────────────────────
    // Only intercepts the paste when it actually carries image file(s) —
    // a plain text paste (the common case) falls through untouched.
    handleShortComposePaste(e) {
      var files = __hyloImageFilesFromClipboard(e.clipboardData);
      if (!files.length) return;
      e.preventDefault();
      files.forEach((f) => this.addShortComposeImage(f));
    },
    handleShortComposeFilePick(e) {
      var files = Array.from((e.target && e.target.files) || []);
      files.forEach((f) => { if (f.type.indexOf('image/') === 0) this.addShortComposeImage(f); });
      e.target.value = ''; // same file re-pickable next time
    },
    addShortComposeImage(file) {
      var item = {
        id: 'si-' + Date.now().toString(36) + Math.random().toString(36).slice(2, 7),
        name: file.name || 'image',
        previewUrl: URL.createObjectURL(file),
        src: '',
        uploading: true,
        failed: false,
      };
      this.shortComposeImages.push(item);
      // Mutate the item through this.shortComposeImages, not the closed-over
      // `item` var — that still points at the plain object from before the
      // push, and writing to it directly bypasses Alpine's reactive proxy
      // (silently — no error, the data really does change), so x-show on
      // img.uploading never re-renders and the spinner never stops.
      __hyloEditorUploadImage(file).then((src) => {
        var cur = this.shortComposeImages.find((i) => i.id === item.id);
        if (!cur) return; // removed while the upload was still in flight
        cur.src = src;
        cur.name = src.split('/').pop();
        cur.uploading = false;
      }).catch((err) => {
        var cur = this.shortComposeImages.find((i) => i.id === item.id);
        if (cur) { cur.uploading = false; cur.failed = true; }
        window.showError((err && err.message) || 'Image upload failed.', 'Upload error');
      });
    },
    removeShortComposeImage(id) {
      var idx = this.shortComposeImages.findIndex((i) => i.id === id);
      if (idx < 0) return;
      if (this.shortComposeImages[idx].previewUrl) URL.revokeObjectURL(this.shortComposeImages[idx].previewUrl);
      this.shortComposeImages.splice(idx, 1);
    },
    async saveShortCompose() {
      var text = this.shortComposeText.trim();
      var images = this.shortComposeImages.filter((i) => i.src && !i.failed);
      if ((!text && !images.length) || this.shortComposeSaving) return;
      if (this.shortComposeImages.some((i) => i.uploading)) {
        window.showError('Images are still uploading — hang on a moment.', 'Please wait');
        return;
      }
      this.shortComposeSaving = true;
      try {
        // Images are stored as trailing wikilinks regardless of where they
        // were attached — rendering (shorts.go's ExtractShortImages) always
        // shows text first and images in a grid below, Weibo/Twitter-style.
        var content = text;
        if (images.length) {
          var wikilinks = images.map((i) => '![[' + i.name + ']]').join('\n');
          content = content ? content + '\n\n' + wikilinks : wikilinks;
        }
        var resp = await fetch('/api/vault/shorts', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ content: content }),
        });
        if (!resp.ok) {
          var msg = await resp.text();
          throw new Error(msg || 'Save failed');
        }
        this.shortComposeText = '';
        this.shortComposeImages.forEach((i) => { if (i.previewUrl) URL.revokeObjectURL(i.previewUrl); });
        this.shortComposeImages = [];
        if (window.__hyloVaultChanged) window.__hyloVaultChanged({ op: 'short', hasImages: images.length > 0 });
      } catch (err) {
        window.showError('Failed to save: ' + (err && err.message ? err.message : String(err)), 'Save failed');
      } finally {
        this.shortComposeSaving = false;
      }
    }
  };
}
