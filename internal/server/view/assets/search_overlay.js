  // Rows render from `results` keyed by path, so a keystroke only moves, adds
  // or drops rows and the selection is plain state, never a DOM rescan.
  // No debounce: a search answers in a few ms and each keystroke aborts the last.
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

  var KIND_LABEL = { knowledge: 'Knowledge', short: 'Short', index: 'Index' };

  // A kind mode's filter, applied to a tab the way the server classifies notes.
  function tabKind(t) {
    if (t.isKnowledge) return 'knowledge';
    if (t.isIndex) return 'index';
    return /^\/_shorts\//.test(t.path) ? 'short' : 'raw';
  }

  // Same row shape as GET /notes/search, built from the editor's open tabs.
  // Untitled tabs have no file to open, so they're left out, as are tabs a
  // kind mode excludes. Order matches the sidebar; the active tab starts selected.
  function openTabItems(kind) {
    var pane = window.__hyloContentPane;
    var tabs = pane && Array.isArray(pane.tabs) ? pane.tabs : [];
    var activeIdx = pane ? pane.activeTab : -1;
    var items = [];
    var active = 0;
    for (var i = 0; i < tabs.length; i++) {
      var t = tabs[i];
      if (!t || !t.path || (kind && tabKind(t) !== kind)) continue;
      if (i === activeIdx) active = items.length;
      var file = String(t.path).split('/').pop();
      var slash = String(t.path).lastIndexOf('/');
      var dir = slash <= 0 ? '/' : t.path.slice(0, slash);
      var dirLabel = '/';
      if (dir !== '/') {
        dir = dir.replace(/^\/+/, '');
        var cut = dir.indexOf('/');
        dirLabel = cut === -1 ? dir : dir.slice(0, cut);
      }
      items.push({
        section: 'Open tabs',
        name: t.title || file.replace(/\.md$/i, '') || 'Note',
        dirLabel: dirLabel,
        url: '/notes?name=' + encodeURIComponent(file),
        path: t.path,
        kind: tabKind(t),
        updated: '',
        isKnowledge: !!t.isKnowledge,
        isIndex: !!t.isIndex,
        canCompile: !!t.canCompile,
      });
    }
    return { items: items, active: active };
  }

  // The sidebar's bots as rows: all of them for an empty query, at most three
  // name matches for a typed one.
  function botItems(query) {
    var home = window._homeData;
    var bots = home && Array.isArray(home.agentBots) ? home.agentBots : [];
    var q = query.toLowerCase();
    var out = [];
    for (var i = 0; i < bots.length && (!q || out.length < 3); i++) {
      var b = bots[i];
      if (!b || !b.id || (q && String(b.name || '').toLowerCase().indexOf(q) === -1)) continue;
      out.push({ type: 'bot', section: 'Bots', id: b.id, name: b.name, path: 'bot:' + b.id, unread: b.unread || 0, desc: b.description || '' });
    }
    return out;
  }

  function searchOverlay() {
    return {
      show: false,
      query: '',
      results: [],
      // 'tabs' is the open-note list shown with an empty query; 'search' is a
      // response for the current query. A typed query keeps the last listing
      // until its own response replaces it.
      listing: '',
      active: 0,
      showPreview: false,
      previewPath: '',
      previewHtml: '',
      _searchKey: '',
      _searchCtrl: null,
      _previewTimer: 0,
      _fragments: new Map(),
      _pointer: null,
      modes: SEARCH_MODES,
      mode: SEARCH_MODES[0],
      showModes: false,
      modeIdx: 0,
      get hasQuery() { return this.query !== ''; },
      // Bots belong to no note kind or field, so only the All scope lists them.
      get withBots() { return !this.mode.field && !this.mode.kind; },
      get activeBot() {
        const r = this.results[this.active];
        return r && r.type === 'bot' ? r : null;
      },
      // A label heads each run of rows from one source.
      sectionLabel(i) {
        const r = this.results[i], prev = this.results[i - 1];
        if (!r || (prev && prev.section === r.section)) return '';
        if (r.section !== 'Notes') return r.section;
        const n = this.results.filter(x => x.section === 'Notes').length;
        return (this.mode.kind ? this.mode.label : 'Notes') + ' · ' + n;
      },
      get scopeLabel() {
        if (this.hasQuery) return 'Search · ' + this.mode.label;
        if (this.mode.kind) return 'Open ' + this.mode.label.toLowerCase() + ' tabs';
        return this.withBots ? 'Open tabs and bots' : 'Open tabs';
      },
      get emptyTitle() {
        if (this.listing === 'tabs') return this.mode.kind ? 'No open ' + this.mode.label.toLowerCase() + ' tabs' : 'No open tabs';
        return 'No notes match “' + this.query + '”';
      },
      get emptyHint() {
        return this.listing === 'tabs' ? 'Type to search every note' : 'Try another filter';
      },
      // The sidebar's face for this bot, so it reads as the same one.
      botAvatar(r) {
        const home = window._homeData;
        return window.agentBotAvatarSVG ? window.agentBotAvatarSVG(r.name, home ? home.agentBots : []) : '';
      },
      // Type, then time: what tells two same-named notes apart.
      accessory(r) {
        if (r.type === 'bot') return r.unread > 0 ? r.unread + ' new' : '';
        return [KIND_LABEL[r.kind], r.updated].filter(Boolean).join(' · ');
      },
      init() {
        window.__hyloHotkeys.register('toggle-search', 'k', () => {
          this.show ? this.close() : this.open();
        });
      },

      open() {
        this.show = true;
        window.__hyloSearchOpen = true;
        this._pointer = null;
        this._showOpenTabs();
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
        if (!this.hasQuery) { this._showOpenTabs(); return; }
        this._search();
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
            items.forEach(n => { n.section = 'Notes'; });
            this.results = (this.withBots ? botItems(this.query) : []).concat(items);
            this.listing = 'search';
            this.active = 0;
            if (this.$refs.list) this.$refs.list.scrollTop = 0;
            this._previewActive();
          })
          .catch(() => {
            // A failed query (e.g. index still loading) must retry on the next keystroke.
            if (ctrl === this._searchCtrl) { this._searchCtrl = null; this._searchKey = ''; }
          });
      },
      _stopSearch() {
        if (this._searchCtrl) { this._searchCtrl.abort(); this._searchCtrl = null; this._searchKey = ''; }
      },
      _dropResults() {
        this._stopSearch();
        this._searchKey = '';
        this.results = [];
        this.listing = '';
        this.active = 0;
        this._dropPreview();
      },
      // Empty query is the open-tab list; none open is its empty state.
      _showOpenTabs() {
        this._stopSearch();
        this._searchKey = '';
        var built = openTabItems(this.mode.kind);
        this.results = built.items.concat(this.withBots ? botItems('') : []);
        this.listing = 'tabs';
        this.active = built.active;
        if (!this.results.length) this._dropPreview();
        var self = this;
        this.$nextTick(function () { self._scrollActive(true); });
        this._previewActive();
      },
      _clearQuery() {
        if (this.$refs.input) { this.$refs.input.value = ''; this.$refs.input.focus(); }
        this.query = '';
        this._showOpenTabs();
      },

      // ── Modes ──────────────────────────────────────────
      openModeMenu() {
        this.showModes = true;
        this.modeIdx = Math.max(0, this.modes.findIndex(m => m.key === this.mode.key));
      },
      closeModeMenu() {
        this.showModes = false;
      },
      toggleModeMenu() {
        if (this.showModes) this.closeModeMenu();
        else this.openModeMenu();
        if (this.$refs.input) this.$refs.input.focus();
      },
      selectMode(m) {
        this.mode = m;
        this.showModes = false;
        if (this.$refs.input) this.$refs.input.focus();
        if (this.hasQuery) this._search();
        else this._showOpenTabs();
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
        // A typed query whose response hasn't arrived must not open a tab still listed.
        else if (e.key === 'Enter') { if (this.listing === 'search' || !this.hasQuery) this.choose(this.active); }
        else if (e.key === 'Tab') { if (this.results.length) this.togglePreview(); }
        else return false;
        e.preventDefault();
        return true;
      },
      move(d) {
        if (!this.results.length) return;
        this.active = Math.max(0, Math.min(this.active + d, this.results.length - 1));
        this._scrollActive(false);
        this._previewActive();
      },
      _scrollActive(reset) {
        const list = this.$refs.list;
        if (!list) return;
        if (reset) list.scrollTop = 0;
        const row = list.querySelectorAll('.sr-row')[this.active];
        if (row) row.scrollIntoView({ block: 'nearest' });
      },
      choose(i) {
        const r = this.results[i];
        if (!r) return;
        if (r.type === 'bot') {
          this.close();
          if (window._homeData) window._homeData.selectChatAgentBot(r.id);
          return;
        }
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
        if (el) this.modeIdx = +el.dataset.i;
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
        // A bot's card is drawn from its row; there is no note to fetch.
        if (r.type === 'bot' || r.path === this.previewPath) return;
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
