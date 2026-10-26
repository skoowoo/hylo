  function searchOverlay() {
    return {
      show: false,
      hasQuery: false,
      activeIdx: -1,
      _previewCtrl: null,
      isMac: /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent),
      modes: [
        { key: 'all',       label: 'All',       desc: 'Search everywhere',              field: '',        kind: '',          placeholder: 'Search notes…' },
        { key: 'name',      label: 'Name',      desc: 'Search by filename',             field: 'name',    kind: '',          placeholder: 'Search by filename…' },
        { key: 'content',   label: 'Content',   desc: 'Search in file content',         field: 'content', kind: '',          placeholder: 'Search in content…' },
        { key: 'tag',       label: 'Tag',       desc: 'Search by tag',                  field: 'tag',     kind: '',          placeholder: 'Search by tag…' },
        { key: 'knowledge', label: 'Knowledge', desc: 'Knowledge notes only',           field: '',        kind: 'knowledge', placeholder: 'Search knowledge notes…' },
        { key: 'short',     label: 'Short',     desc: 'Short notes only',               field: '',        kind: 'short',     placeholder: 'Search short notes…' },
        { key: 'raw',       label: 'Raw',       desc: 'Raw notes only',                 field: '',        kind: 'raw',       placeholder: 'Search raw notes…' },
        { key: 'index',     label: 'Index',     desc: 'Index notes only',               field: '',        kind: 'index',     placeholder: 'Search index notes…' },
      ],
      mode: { key: 'all', label: 'All', desc: 'Search everywhere', field: '', kind: '', placeholder: 'Search notes…' },
      showModes: false,
      modeIdx: 0,
      init() {
        this.mode = this.modes[0];
        window.__hyloHotkeys.register('toggle-search', 'k', () => {
          this.show ? this.close() : this.open();
        });
      },
      _items() { return Array.from(document.querySelectorAll('#search-results a')); },
      _highlight(items) {
        items.forEach((el, i) => el.classList.toggle('is-active', i === this.activeIdx));
        if (this.activeIdx >= 0) {
          items[this.activeIdx].scrollIntoView({ block: 'nearest' });
          this._loadPreview(items[this.activeIdx]);
        }
      },
      _loadPreview(el) {
        const path = el && el.dataset.previewPath;
        if (!path) return;
        if (this._previewCtrl) this._previewCtrl.abort();
        this._previewCtrl = new AbortController();
        const panel = document.getElementById('search-preview');
        if (!panel) return;
        panel.innerHTML = '<div class="srch-prev-spin">…</div>';
        fetch('/notes/fragment?path=' + encodeURIComponent(path), { signal: this._previewCtrl.signal, headers: { 'HX-Request': 'true' } })
          .then(r => r.ok ? r.text() : Promise.reject())
          .then(html => { panel.innerHTML = '<div class="srch-preview-pane">' + html + '</div>'; })
          .catch(() => {});
      },
      _clearPreview() {
        if (this._previewCtrl) { this._previewCtrl.abort(); this._previewCtrl = null; }
        const p = document.getElementById('search-preview');
        if (p) p.innerHTML = '<div class="srch-prev-hint">Navigate results to preview</div>';
      },
      _clearQuery() {
        if (this.$refs.input) this.$refs.input.value = '';
        this.hasQuery = false;
        this.activeIdx = -1;
        this._clearPreview();
        const sr = document.getElementById('search-results');
        if (sr) sr.innerHTML = '';
        if (this.$refs.input) this.$refs.input.focus();
      },
      open() {
        this.show = true;
        window.__hyloSearchOpen = true;
        this.activeIdx = -1;
        if (window.__hyloEscPush) window.__hyloEscPush('search', () => {
          if (this.showModes) { this.closeModeMenu(); }
          else if (this.hasQuery) { this._clearQuery(); }
          else { this.close(); }
        }, this.$el);
        this.$nextTick(() => this.$refs.input && this.$refs.input.focus());
      },
      openWithMode(detail) {
        this.open();
        if (detail && detail.mode) {
          const m = this.modes.find(x => x.key === detail.mode);
          if (m) this.$nextTick(() => this.selectMode(m));
        }
      },
      close() {
        if (window.__hyloEscPop) window.__hyloEscPop('search');
        this.show = false;
        window.__hyloSearchOpen = false;
        this.activeIdx = -1;
        if (this._previewCtrl) { this._previewCtrl.abort(); this._previewCtrl = null; }
        setTimeout(() => {
          if (this.show) return;
          this.hasQuery = false;
          this.showModes = false;
          this.mode = this.modes[0];
          if (this.$refs.input) {
            this.$refs.input.value = '';
            const sr = document.getElementById('search-results');
            if (sr) sr.innerHTML = '';
          }
          const p = document.getElementById('search-preview');
          if (p) p.innerHTML = '<div class="srch-prev-hint">Navigate results to preview</div>';
        }, 150);
      },
      openModeMenu() {
        this.showModes = true;
        this.modeIdx = this.modes.findIndex(m => m.key === this.mode.key);
        if (this.modeIdx < 0) this.modeIdx = 0;
      },
      closeModeMenu() {
        this.showModes = false;
      },
      selectMode(m) {
        this.mode = m;
        this.showModes = false;
        this.$nextTick(() => {
          if (this.$refs.input) this.$refs.input.focus();
          if (this.hasQuery) htmx.trigger(this.$refs.input, 'search');
        });
      },
      onInput(e) {
        const val = e.target.value;
        if (val === '/' && !this.hasQuery) {
          e.target.value = '';
          this.openModeMenu();
          return;
        }
        this.hasQuery = val.trim().length > 0;
      },
      onInputKeydown(e) {
        if (e.isComposing || e.keyCode === 229) return;
        if (this.showModes) {
          if (e.key === 'ArrowDown') {
            e.preventDefault(); e.stopPropagation();
            this.modeIdx = Math.min(this.modeIdx + 1, this.modes.length - 1);
          } else if (e.key === 'ArrowUp') {
            e.preventDefault(); e.stopPropagation();
            this.modeIdx = Math.max(this.modeIdx - 1, 0);
          } else if (e.key === 'Enter') {
            e.preventDefault(); e.stopPropagation();
            this.selectMode(this.modes[this.modeIdx]);
          } else if (e.key === 'Escape') {
            e.preventDefault(); e.stopPropagation();
            this.closeModeMenu();
          } else if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
            e.preventDefault();
            const idx = this.modes.findIndex(m =>
              m.label.toLowerCase().startsWith(e.key.toLowerCase())
            );
            if (idx >= 0) this.modeIdx = idx;
          }
          return;
        }
        if (e.key === 'ArrowDown') { e.preventDefault(); this.moveDown(); }
        else if (e.key === 'ArrowUp') { e.preventDefault(); this.moveUp(); }
        else if (e.key === 'Enter') { e.preventDefault(); this.handleInputEnter(e); }
      },
      moveDown() {
        const items = this._items();
        if (!items.length) return;
        this.activeIdx = Math.min(this.activeIdx + 1, items.length - 1);
        this._highlight(items);
      },
      moveUp() {
        const items = this._items();
        if (!items.length) return;
        this.activeIdx = Math.max(this.activeIdx - 1, 0);
        this._highlight(items);
      },
      confirm() {
        if (this.activeIdx < 0) return;
        const el = this._items()[this.activeIdx];
        if (el) this.selectResult(el);
      },
      // Same underlying action as chat's "@" mention (path_ac.js) — both
      // just reference a note as a [[stem]] wikilink, this is only a
      // different way to land on the note (global search vs typing inline).
      confirmReference() {
        if (this.activeIdx < 0) return;
        const el = this._items()[this.activeIdx];
        const path = el && el.dataset.previewPath;
        if (!path) return;
        const slash = path.lastIndexOf('/');
        const stem = (slash === -1 ? path : path.slice(slash + 1)).replace(/\.(md|markdown)$/i, '');
        try { navigator.clipboard.writeText('[[' + stem + ']]'); } catch(_) {}
        window.dispatchEvent(new CustomEvent('hylo:reference-note', { detail: { stem } }));
        this.close();
      },
      handleInputEnter(e) {
        if (e.metaKey || e.ctrlKey) { this.confirmReference(); } else { this.confirm(); }
      },
      selectResult(el) {
        if (!el) return;
        if (typeof window.handleSearchResultSelection === 'function' &&
            window.handleSearchResultSelection(el)) {
          this.close();
          return;
        }
        window.location.href = el.href;
      },
      onModeHover(e) {
        const el = e.target && e.target.closest ? e.target.closest('.srch-mode-item') : null;
        if (!el) return;
        const items = Array.from(document.querySelectorAll('.srch-mode-item'));
        const idx = items.indexOf(el);
        if (idx >= 0 && idx !== this.modeIdx) this.modeIdx = idx;
      },
      onResultHover(e) {
        const el = e.target && e.target.closest ? e.target.closest('#search-results a') : null;
        if (!el) return;
        const items = this._items();
        const idx = items.indexOf(el);
        if (idx >= 0 && idx !== this.activeIdx) {
          this.activeIdx = idx;
          this._highlight(items);
        }
      },
      onResultClick(e) {
        const el = e.target && e.target.closest ? e.target.closest('#search-results a') : null;
        if (!el) return;
        if (typeof window.handleSearchResultSelection === 'function' &&
            window.handleSearchResultSelection(el)) {
          e.preventDefault();
          this.close();
        }
      },
      onResultsSwap(e) {
        if (e.detail && e.detail.ctx && e.detail.ctx.target && e.detail.ctx.target.id === 'search-results') {
          this.activeIdx = -1;
          this._clearPreview();
          this.$nextTick(() => {
            const items = this._items();
            if (items.length) {
              this.activeIdx = 0;
              this._highlight(items);
            }
          });
        }
      },
      onKey(e) {
        if (!this.show) return;
        // When focus leaves the input (e.g. user scrolled the preview panel),
        // still handle nav keys so the results list stays navigable.
        if (document.activeElement === this.$refs.input) return;
        const mod = this.isMac ? e.metaKey : e.ctrlKey;
        if (e.key === 'ArrowDown')  { e.preventDefault(); this.moveDown(); }
        else if (e.key === 'ArrowUp')   { e.preventDefault(); this.moveUp(); }
        else if (e.key === 'Enter' && mod) { e.preventDefault(); this.confirmReference(); }
        else if (e.key === 'Enter')     { e.preventDefault(); this.confirm(); }
      }
    }
  }
