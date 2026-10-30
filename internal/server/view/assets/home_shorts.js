// ── Image files from a paste or drop ──────────────────────────────────────
// Like editor_session.js's __hyloEditorFindImageFile, but collects every
// image file instead of just the first: a short or a chat message takes
// several at once (e.g. copying multiple files in Finder).
function __hyloImageFilesFrom(dt) {
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

// ── Links in a short (card or viewer): intercept internal ones ────────────
// A #tag filters the feed; wikilinks (/notes?…) → open in the content pane;
// external → new tab.
document.addEventListener('click', function (e) {
  var hashtag = e.target.closest ? e.target.closest('.shorts-entry-prose .hashtag') : null;
  if (hashtag && window._homeData) {
    e.preventDefault();
    e.stopPropagation();
    if (hashtag.closest('.lb-overlay')) window._homeData.closeLightbox();
    window._homeData.filterShorts(hashtag.dataset.tag, true);
    return;
  }
  var a = e.target.closest ? e.target.closest('a') : null;
  if (!a || !a.closest('.shorts-entry-prose')) return;
  var href = a.getAttribute('href');
  if (!href) return;
  e.preventDefault();
  e.stopPropagation();
  if (a.closest('.lb-overlay') && window._homeData) window._homeData.closeLightbox();
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

// An image opens the viewer at that image; a grid card (its text cut, its
// size fixed) opens it from anywhere outside its links and buttons.
document.addEventListener('click', function (e) {
  var t = e.target;
  var card = t.closest ? t.closest('.shorts-stream-groups .shorts-entry') : null;
  if (!card || card.classList.contains('is-editing')) return;
  var cell = t.closest('.shorts-img-cell');
  if (!cell && !card.closest('.is-grid')) return;
  if (t.closest('a, button, .shorts-entry-actions') || String(window.getSelection())) return;
  window.openShortViewer(card, cell ? Array.prototype.indexOf.call(card.querySelectorAll('.shorts-img-cell'), cell) : 0);
});

// A delete still waiting out its undo window is sent before the page goes.
window.addEventListener('pagehide', function () {
  if (window._homeData && window._homeData.commitShortDelete) window._homeData.commitShortDelete();
});

// Kept outside Alpine's reactive state: its proxy would break the identity checks.
var __hyloShortPendingDelete = null;

var __hyloShortImageRe = /!\[\[([^\]\[]+?)\]\]/g;

function __hyloShortDraft(extra) {
  return Object.assign({ text: '', images: [], saving: false, dropping: false }, extra || {});
}

// Images are stored as trailing wikilinks wherever they were attached;
// rendering (ExtractShortImages) always shows them below the text.
function __hyloShortDraftContent(d) {
  var content = d.text.trim();
  var links = d.images.filter((i) => i.src && !i.failed).map((i) => '![[' + i.name + ']]').join('\n');
  return links ? (content ? content + '\n\n' + links : links) : content;
}

function __hyloShortFitInput(root) {
  var el = root && root.querySelector('.shorts-compose-input');
  if (el) __hyloAutoResize(el);
  return el;
}

// The rendered card for one entry, wrapped in its day group.
async function __hyloShortFetchDay(id) {
  var resp = await fetch('/shorts/entry?id=' + encodeURIComponent(id), { headers: { 'HX-Request': 'true' } });
  if (!resp.ok) return null;
  var tpl = document.createElement('template');
  tpl.innerHTML = (await resp.text()).trim();
  return tpl.content.querySelector('.shorts-day');
}

async function __hyloShortGet(id) {
  var resp = await fetch('/api/vault/shorts/get', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: id }),
  });
  if (!resp.ok) throw new Error((await resp.text()) || 'Not found');
  return resp.json();
}

// The shown feed only: a parked shorts section (another tag or month) keeps its cards in <body>.
function __hyloShortCard(id) {
  return document.querySelector('#home-list-pane .shorts-stream-groups .shorts-entry[data-id="' + CSS.escape(id) + '"]');
}

// Latest known hash per short id ('' once deleted): a render that comes back
// older than that is dropped, so out-of-order fetches can't undo a newer edit.
var __hyloShortLatest = new Map();

async function __hyloShortRender(id) {
  var day = await __hyloShortFetchDay(id);
  return day && day.firstElementChild.dataset.hash === __hyloShortLatest.get(id) ? day : null;
}

// Puts a freshly rendered card (in its day group) in the feed: over its old
// self, or newest-first at the top; out of a feed filtered to a tag it no
// longer carries. A card being edited waits for its editor.
function __hyloShortPut(day) {
  var fresh = day.firstElementChild;
  var old = __hyloShortCard(fresh.dataset.id);
  var tag = window._homeData ? window._homeData.curParam('tag') : '';
  if (tag && !fresh.dataset.tags.split(' ').includes(tag)) {
    if (old && !old.classList.contains('is-editing')) __hyloShortRemoveCard(old);
    return;
  }
  if (old) {
    if (!old.classList.contains('is-editing') && old.dataset.hash !== fresh.dataset.hash) old.replaceWith(fresh);
    return;
  }
  var groups = document.getElementById('shorts-stream-groups');
  if (!groups) return;
  var first = groups.querySelector('.shorts-day');
  if (first && first.dataset.day === day.dataset.day) first.prepend(fresh);
  else groups.prepend(day);
  document.querySelector('#home-list-pane .shorts-empty')?.remove();
}

// Re-renders the shown feed's tag row once a burst of short changes settles.
var __hyloShortTagsTimer = 0;
function __hyloShortTagsRefresh() {
  clearTimeout(__hyloShortTagsTimer);
  __hyloShortTagsTimer = setTimeout(async function () {
    var tag = window._homeData ? window._homeData.curParam('tag') : '';
    var resp = await fetch('/shorts/tags?tag=' + encodeURIComponent(tag), { headers: { 'HX-Request': 'true' } });
    var row = document.querySelector('#home-list-pane .shorts-tags');
    if (!resp.ok || !row || (window._homeData && window._homeData.curParam('tag') !== tag)) return;
    var tpl = document.createElement('template');
    tpl.innerHTML = (await resp.text()).trim();
    row.replaceWith(tpl.content.firstElementChild);
  }, 300);
}

function __hyloShortRemoveCard(card) {
  var day = card.parentElement;
  card.remove();
  if (day && day.classList.contains('shorts-day') && !day.children.length) day.remove();
}

function homeShortsMixin() {
  return {
    // ── Shorts composer and card actions ────────────────────────────────
    // One draft shape serves the composer (shortCompose) and the card being
    // edited (shortEdit, which carries the entry's id and hash). Cards follow
    // saves card by card (syncShort), so the section never reloads under the user.
    shortCompose: __hyloShortDraft(),
    shortEdit: null,

    shortDraftReady(d) {
      return !!d && !d.saving && !d.images.some((i) => i.uploading) &&
        (!!d.text.trim() || d.images.some((i) => i.src && !i.failed));
    },
    handleShortKeydown(e, d) {
      if (e.isComposing || e.keyCode === 229) return;
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        this.submitShortDraft(d);
      } else if (e.key === 'Escape') {
        e.preventDefault();
        if (d.id) this.cancelShortEdit();
        else e.target.blur();
      }
    },
    // Only intercepts the paste when it actually carries image file(s) —
    // a plain text paste (the common case) falls through untouched.
    handleShortPaste(e, d) {
      var files = __hyloImageFilesFrom(e.clipboardData);
      if (!files.length) return;
      e.preventDefault();
      files.forEach((f) => this.addShortImage(d, f));
    },
    handleShortDragOver(e, d) {
      if (!Array.from(e.dataTransfer.types || []).includes('Files')) return;
      e.preventDefault();
      d.dropping = true;
    },
    handleShortDragLeave(e, d) {
      if (!e.currentTarget.contains(e.relatedTarget)) d.dropping = false;
    },
    handleShortDrop(e, d) {
      d.dropping = false;
      if (!Array.from(e.dataTransfer.types || []).includes('Files')) return;
      e.preventDefault();
      __hyloImageFilesFrom(e.dataTransfer).forEach((f) => this.addShortImage(d, f));
      e.currentTarget.querySelector('.shorts-compose-input')?.focus();
    },
    handleShortFilePick(e, d) {
      Array.from(e.target.files || []).forEach((f) => { if (f.type.indexOf('image/') === 0) this.addShortImage(d, f); });
      e.target.value = ''; // same file re-pickable next time
    },
    addShortImage(d, file) {
      var id = 'si-' + Date.now().toString(36) + Math.random().toString(36).slice(2, 7);
      d.images.push({ id: id, name: file.name || 'image', previewUrl: URL.createObjectURL(file), src: '', uploading: true, failed: false });
      // Look the item up again through d: the pushed object is wrapped by
      // Alpine's reactive proxy, and writing to the original skips re-render.
      var find = () => d.images.find((i) => i.id === id);
      __hyloEditorUploadImage(file).then((src) => {
        var cur = find();
        if (!cur) return; // removed while the upload was still in flight
        cur.src = src;
        cur.name = src.split('/').pop();
        cur.uploading = false;
      }).catch((err) => {
        var cur = find();
        if (cur) { cur.uploading = false; cur.failed = true; }
        window.showError((err && err.message) || 'Image upload failed.', 'Upload error');
      });
    },
    removeShortImage(d, id) {
      var idx = d.images.findIndex((i) => i.id === id);
      if (idx < 0) return;
      if (d.images[idx].previewUrl) URL.revokeObjectURL(d.images[idx].previewUrl);
      d.images.splice(idx, 1);
    },
    submitShortDraft(d) {
      if (!this.shortDraftReady(d)) return;
      if (d.id) this.saveShortEdit();
      else this.saveShortCompose();
    },

    // Clears the field at once and keeps focus there; on failure the draft
    // comes back if nothing new was typed meanwhile.
    async saveShortCompose() {
      var draft = this.shortCompose;
      var content = __hyloShortDraftContent(draft);
      this.shortCompose = __hyloShortDraft({ saving: true });
      this.$nextTick(() => __hyloShortFitInput(document.querySelector('#home-list-pane .shorts-stream-inner')));
      try {
        var resp = await fetch('/api/vault/shorts', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ content: content }),
        });
        if (!resp.ok) throw new Error((await resp.text()) || 'Save failed');
        var entry = await resp.json();
        draft.images.forEach((i) => { if (i.previewUrl) URL.revokeObjectURL(i.previewUrl); });
        // Writing a new short clears the filters (a tag, a month) so it shows at the top.
        var unfiltered = __hyloSectionUnfiltered(this.curKey);
        if (unfiltered === this.curKey) await this.syncShort(entry);
        else this.go(unfiltered);
      } catch (err) {
        if (!this.shortCompose.text && !this.shortCompose.images.length) {
          draft.saving = false;
          this.shortCompose = draft;
          this.$nextTick(() => __hyloShortFitInput(document.querySelector('#home-list-pane .shorts-stream-inner')));
        }
        window.showError('Failed to save: ' + (err && err.message ? err.message : String(err)), 'Save failed');
      } finally {
        this.shortCompose.saving = false;
      }
    },
    // The tag row toggles its tag; a #tag in a short always filters to it.
    // A month picked meanwhile stays, as the tag does when picking a month.
    filterShorts(tag, keep) {
      var params = new URLSearchParams(this.curParams());
      if (keep || tag !== params.get('tag')) params.set('tag', tag);
      else params.delete('tag');
      this.go('/home/section?' + params.toString());
    },
    // A new short belongs at the top of the feed, which a jump to an earlier
    // month has scrolled out of the view.
    _shortsAtTop() {
      var now = new Date();
      var from = this.curParam('from');
      return !from || from === now.getFullYear() + '-' + String(now.getMonth() + 1).padStart(2, '0');
    },
    // Brings the shown feed's card for one short in line with a change
    // ({id, hash}, no hash once deleted), whoever made it.
    async syncShort(s) {
      __hyloShortLatest.set(s.id, s.hash || '');
      __hyloShortTagsRefresh();
      var card = __hyloShortCard(s.id);
      if (!s.hash) {
        if (card) __hyloShortRemoveCard(card);
        return;
      }
      if (card ? card.dataset.hash === s.hash : !this._shortsAtTop()) return;
      var day = await __hyloShortRender(s.id);
      if (day) __hyloShortPut(day);
    },

    // ── Edit in place: the card's content gives way to the composer ──────
    async editShort(card) {
      if (this.shortEdit && !(await this.cancelShortEdit())) return;
      var entry;
      try { entry = await __hyloShortGet(card.dataset.id); } catch (err) {
        window.showError('Could not open this short: ' + err.message, 'Edit failed');
        return;
      }
      var thumbs = {};
      card.querySelectorAll('.shorts-img-cell').forEach((c) => { thumbs[c.dataset.imgName] = c.dataset.imgSrc; });
      var images = Array.from(entry.content.matchAll(__hyloShortImageRe), (m, i) => {
        var name = m[1].trim();
        return { id: 'se-' + i, name: name, previewUrl: '', src: thumbs[name] || '/api/images/serve?name=' + encodeURIComponent(name), uploading: false, failed: false };
      });
      this.shortEdit = __hyloShortDraft({
        id: entry.id, hash: entry.hash, original: entry.content, card: card,
        text: entry.content.replace(__hyloShortImageRe, '').trim(), images: images,
      });
      card.append(document.getElementById('shorts-edit-tpl').content.firstElementChild.cloneNode(true));
      card.classList.add('is-editing');
      this.$nextTick(() => {
        var el = __hyloShortFitInput(card);
        if (!el) return;
        el.focus();
        el.setSelectionRange(el.value.length, el.value.length);
      });
    },
    // Resolves false when the user keeps editing.
    async cancelShortEdit() {
      var d = this.shortEdit;
      if (!d) return true;
      if (__hyloShortDraftContent(d) !== d.original.trim()) {
        var ok = await window.showConfirm({ title: 'Discard changes?', message: 'Your edits to this short will be lost.', confirmLabel: 'Discard', danger: true });
        if (!ok) return false;
      }
      this._closeShortEdit();
      return true;
    },
    // day: the card a save just rendered, put in place as the editor goes.
    _closeShortEdit(day) {
      var d = this.shortEdit;
      if (!d) return;
      d.card.querySelector(':scope > .shorts-compose-card')?.remove();
      d.card.classList.remove('is-editing');
      this.shortEdit = null;
      if (day) __hyloShortPut(day);
      // A change from elsewhere that arrived while editing was held for the editor.
      var latest = __hyloShortLatest.get(d.id);
      var card = __hyloShortCard(d.id);
      if (latest !== undefined && card && card.dataset.hash !== latest) void this.syncShort({ id: d.id, hash: latest });
    },
    async saveShortEdit() {
      var d = this.shortEdit;
      var content = __hyloShortDraftContent(d);
      if (content === d.original.trim()) { this._closeShortEdit(); return; }
      d.saving = true;
      try {
        var resp = await fetch('/api/vault/shorts/update', {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ id: d.id, hash: d.hash, content: content }),
        });
        if (resp.status === 409) {
          d.saving = false;
          var reload = await window.showConfirm({
            title: 'Changed elsewhere',
            message: 'This short was edited somewhere else since you opened it. Load that version? Your edits here will be replaced.',
            confirmLabel: 'Load latest', danger: false,
          });
          if (reload) {
            // The card is brought up to date first: the editor opens on its replacement.
            this._closeShortEdit();
            var latest = await __hyloShortGet(d.id).catch(() => null);
            await this.syncShort({ id: d.id, hash: latest ? latest.hash : '' });
            var card = __hyloShortCard(d.id);
            if (card) this.editShort(card);
          }
          return;
        }
        if (!resp.ok) throw new Error((await resp.text()) || 'Save failed');
        var entry = await resp.json();
        __hyloShortLatest.set(entry.id, entry.hash);
        this._closeShortEdit(await __hyloShortRender(entry.id));
      } catch (err) {
        d.saving = false;
        window.showError('Failed to save: ' + (err && err.message ? err.message : String(err)), 'Save failed');
      }
    },

    // ── Delete with undo: the card hides at once and the delete is sent
    // when the toast's undo window closes (or the page goes away first). ──
    deleteShort(card) {
      this.commitShortDelete();
      if (this.shortEdit && this.shortEdit.card === card) this._closeShortEdit();
      card.hidden = true;
      var pending = { card: card, id: card.dataset.id, hash: card.dataset.hash };
      pending.timer = setTimeout(() => this.commitShortDelete(), 6000);
      __hyloShortPendingDelete = pending;
      this.showToast('Short deleted', 'ok', { label: 'Undo', run: () => this.undoShortDelete(pending), owner: 'short:' + pending.id });
    },
    undoShortDelete(pending) {
      if (__hyloShortPendingDelete !== pending) return;
      clearTimeout(pending.timer);
      __hyloShortPendingDelete = null;
      pending.card.hidden = false;
    },
    commitShortDelete() {
      var p = __hyloShortPendingDelete;
      if (!p) return;
      clearTimeout(p.timer);
      __hyloShortPendingDelete = null;
      if (this.toastAction && this.toastAction.owner === 'short:' + p.id) this.toastVisible = false;
      fetch('/api/vault/shorts/delete', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, keepalive: true,
        body: JSON.stringify({ id: p.id, hash: p.hash }),
      }).then(async (resp) => {
        // Already gone (deleted elsewhere meanwhile) is what was asked for.
        if (!resp.ok && resp.status !== 404) throw new Error((await resp.text()) || 'Delete failed');
        __hyloShortRemoveCard(p.card);
      }).catch((err) => {
        p.card.hidden = false;
        window.showError('Failed to delete: ' + err.message, 'Delete failed');
      });
    },

    shortMenu(btn, e) {
      var card = btn.closest('.shorts-entry');
      __hyloCardMenu.toggle(btn, [
        { icon: 'copy', label: 'Copy text', run: () => this.copyShort(card) },
        { icon: 'external-link', label: 'Show in daily note', run: () => this.openShortDay(card, true) },
        'divider',
        { icon: 'trash', label: 'Delete', danger: true, run: () => this.deleteShort(card) },
      ], e);
    },
    async copyShort(card) {
      try {
        await navigator.clipboard.writeText((await __hyloShortGet(card.dataset.id)).content);
        this.showToast('Copied', 'ok');
      } catch (err) {
        window.showError('Could not copy: ' + err.message, 'Copy failed');
      }
    },
    // Opens the daily file; atEntry also scrolls to this entry's heading.
    async openShortDay(card, atEntry) {
      var path = card.dataset.path;
      var pane = window.__hyloContentPane;
      if (!path || !pane) return;
      __hyloEditor.pendingHeading = atEntry ? 'Short Note: ' + card.dataset.stamp : null;
      await pane.openNoteInContentPane(path, path.split('/').pop().replace(/\.md$/, ''), false, false);
      // Consumed by the open, or it bailed; either way it mustn't leak into the next one.
      __hyloEditor.pendingHeading = null;
    },
  };
}
