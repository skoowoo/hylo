  // Rows render from `results` keyed by path, so a keystroke only moves, adds
  // or drops rows and the selection is plain state, never a DOM rescan.
  var SEARCH_DELAY = 80;
  // Long enough for held arrow keys to skip the rows they pass over.
  var PREVIEW_DELAY = 70;

  var SEARCH_MODES = [
    { key: 'all',       label: 'All',       desc: 'Search everywhere',              field: '',        kind: '',          placeholder: 'Search notes…' },
    { key: 'name',      label: 'Name',      desc: 'Search by filename',             field: 'name',    kind: '',          placeholder: 'Search by filename…' },
    { key: 'content',   label: 'Content',   desc: 'Search in file content',         field: 'content', kind: '',          placeholder: 'Search in content…' },
    { key: 'tag',       label: 'Tag',       desc: 'Search by tag',                  field: 'tag',     kind: '',          placeholder: 'Search by tag…' },
    { key: 'knowledge', label: 'Knowledge', desc: 'Knowledge notes only',           field: '',        kind: 'knowledge', placeholder: 'Search knowledge notes…' },
    { key: 'short',     label: 'Short',     desc: 'Short notes only',               field: '',        kind: 'short',     placeholder: 'Search short notes…' },
    { key: 'raw',       label: 'Raw',       desc: 'Raw notes only',                 field: '',        kind: 'raw',       placeholder: 'Search raw notes…' },
    { key: 'index',     label: 'Index',     desc: 'Index notes only',               field: '',        kind: 'index',     placeholder: 'Search index notes…' },
  ];

  function searchOverlay() {
    return {
      show: false,
      query: '',
      results: [],
      // A response has arrived for the current query; until then the body
      // stays closed instead of opening onto an empty list.
      ready: false,
      active: 0,
      showPreview: false,
      previewPath: '',
      previewHtml: '',
      _searchKey: '',
      _searchCtrl: null,
      _searchTimer: 0,
      _previewTimer: 0,
      _fragments: new Map(),
      _pointer: null,
      modes: SEARCH_MODES,
      mode: SEARCH_MODES[0],
      showModes: false,
      modeIdx: 0,
      get hasQuery() { return this.query !== ''; },
      init() {
        window.__hyloHotkeys.register('toggle-search', 'k', () => {
          this.show ? this.close() : this.open();
        });
      },

      open() {
        this.show = true;
        window.__hyloSearchOpen = true;
        this._pointer = null;
        if (window.__hyloEscPush) window.__hyloEscPush('search', () => {
          if (this.showModes) this.closeModeMenu();
          else if (this.hasQuery) this._clearQuery();
          else this.close();
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
        this._stopSearch();
        clearTimeout(this._previewTimer);
        // After the leave transition, so the panel doesn't empty as it fades.
        setTimeout(() => {
          if (this.show) return;
          if (this.$refs.input) this.$refs.input.value = '';
          this.query = '';
          this._dropResults();
          this.showModes = false;
          this.showPreview = false;
          this.mode = this.modes[0];
          this._fragments = new Map();
        }, 150);
      },

      // ── Query ──────────────────────────────────────────
      onInput(e) {
        // Pinyin keystrokes aren't the query; compositionend delivers it.
        if (e.isComposing) return;
        this.onQuery(e.target);
      },
      onQuery(input) {
        if (input.value === '/' && !this.hasQuery) {
          input.value = '';
          this.openModeMenu();
          return;
        }
        this.query = input.value.trim();
        if (!this.hasQuery) { this._dropResults(); return; }
        clearTimeout(this._searchTimer);
        this._searchTimer = setTimeout(() => this._search(), SEARCH_DELAY);
      },
      _search() {
        const params = new URLSearchParams({ q: this.query, field: this.mode.field, kind: this.mode.kind });
        const key = params.toString();
        if (!this.hasQuery || key === this._searchKey) return;
        this._searchKey = key;
        if (this._searchCtrl) this._searchCtrl.abort();
        const ctrl = this._searchCtrl = new AbortController();
        fetch('/notes/search?' + key, { signal: ctrl.signal })
          .then(r => r.ok ? r.json() : Promise.reject())
          .then(items => {
            if (ctrl !== this._searchCtrl) return;
            this._searchCtrl = null;
            this.results = items;
            this.active = 0;
            this.ready = true;
            if (this.$refs.list) this.$refs.list.scrollTop = 0;
            this._previewActive();
          })
          .catch(() => {
            // A failed query (e.g. index still loading) must retry on the next keystroke.
            if (ctrl === this._searchCtrl) { this._searchCtrl = null; this._searchKey = ''; }
          });
      },
      _stopSearch() {
        clearTimeout(this._searchTimer);
        if (this._searchCtrl) { this._searchCtrl.abort(); this._searchCtrl = null; this._searchKey = ''; }
      },
      _dropResults() {
        this._stopSearch();
        this._searchKey = '';
        this.results = [];
        this.ready = false;
        this.active = 0;
        this._dropPreview();
      },
      _clearQuery() {
        if (this.$refs.input) { this.$refs.input.value = ''; this.$refs.input.focus(); }
        this.query = '';
        this._dropResults();
      },

      // ── Modes ──────────────────────────────────────────
      openModeMenu() {
        this.showModes = true;
        this.modeIdx = Math.max(0, this.modes.findIndex(m => m.key === this.mode.key));
      },
      closeModeMenu() {
        this.showModes = false;
      },
      selectMode(m) {
        this.mode = m;
        this.showModes = false;
        if (this.$refs.input) this.$refs.input.focus();
        this._search();
      },

      // ── Navigation ─────────────────────────────────────
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
            const idx = this.modes.findIndex(m => m.label.toLowerCase().startsWith(e.key.toLowerCase()));
            if (idx >= 0) this.modeIdx = idx;
          }
          return;
        }
        this._navKey(e);
      },
      // Keeps the list driveable when focus has left the input (e.g. a click
      // into the preview).
      onKey(e) {
        if (!this.show || document.activeElement === this.$refs.input) return;
        if (this._navKey(e) && this.$refs.input) this.$refs.input.focus();
      },
      _navKey(e) {
        if (e.key === 'ArrowDown') this.move(1);
        else if (e.key === 'ArrowUp') this.move(-1);
        else if (e.key === 'Enter') this.choose(this.active);
        else if (e.key === 'Tab') { if (this.hasQuery) this.togglePreview(); }
        else return false;
        e.preventDefault();
        return true;
      },
      move(d) {
        if (!this.results.length) return;
        this.active = Math.max(0, Math.min(this.active + d, this.results.length - 1));
        const row = this.$refs.list && this.$refs.list.querySelectorAll('.sr-row')[this.active];
        if (row) row.scrollIntoView({ block: 'nearest' });
        this._previewActive();
      },
      choose(i) {
        const r = this.results[i];
        if (!r) return;
        if (typeof window.handleSearchResultSelection === 'function' && window.handleSearchResultSelection(r)) {
          this.close();
          return;
        }
        window.location.href = r.url;
      },
      // Scrolling the list under a resting pointer fires mousemove at the same
      // coordinates; only a real move may take the selection from the keyboard.
      _pointerMoved(e) {
        const moved = !this._pointer || this._pointer.x !== e.clientX || this._pointer.y !== e.clientY;
        this._pointer = { x: e.clientX, y: e.clientY };
        return moved;
      },
      onModeHover(e) {
        if (!this._pointerMoved(e)) return;
        const el = e.target.closest('.srch-mode-item');
        if (!el) return;
        const idx = Array.prototype.indexOf.call(el.parentNode.querySelectorAll('.srch-mode-item'), el);
        if (idx >= 0) this.modeIdx = idx;
      },
      onResultHover(e) {
        if (!this._pointerMoved(e)) return;
        const row = e.target.closest('.sr-row');
        if (!row) return;
        const i = +row.dataset.i;
        if (i !== this.active) { this.active = i; this._previewActive(); }
      },

      // ── Preview ────────────────────────────────────────
      // Opt-in so a plain search never fetches or renders a note.
      togglePreview() {
        this.showPreview = !this.showPreview;
        if (this.showPreview) this._previewActive();
        else this._dropPreview();
      },
      _dropPreview() {
        clearTimeout(this._previewTimer);
        this.previewPath = '';
        this.previewHtml = '';
      },
      // The shown note stays until the next one is ready, so moving never
      // flashes an empty or loading card.
      _previewActive() {
        if (!this.showPreview) return;
        clearTimeout(this._previewTimer);
        const r = this.results[this.active];
        if (!r) { this._dropPreview(); return; }
        if (r.path === this.previewPath) return;
        const path = r.path;
        const load = () => this._fragment(path).then(html => {
          const cur = this.results[this.active];
          if (!this.showPreview || !cur || cur.path !== path) return;
          this.previewPath = path;
          this.previewHtml = html;
          this.$nextTick(() => { if (this.$refs.preview) this.$refs.preview.scrollTop = 0; });
        }, () => {});
        if (this._fragments.has(path)) load();
        else this._previewTimer = setTimeout(load, PREVIEW_DELAY);
      },
      // Cached per open; a note edited meanwhile shows fresh on the next open.
      _fragment(path) {
        let p = this._fragments.get(path);
        if (!p) {
          p = fetch('/notes/fragment?path=' + encodeURIComponent(path), { headers: { 'HX-Request': 'true' } })
            .then(r => r.ok ? r.text() : Promise.reject());
          p.catch(() => this._fragments.delete(path));
          this._fragments.set(path, p);
        }
        return p;
      },
    }
  }
