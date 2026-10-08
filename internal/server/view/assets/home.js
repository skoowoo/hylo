
// ── Search result selection ────────────────────────────────────────────────
// While Knowledge's graph view is showing, focus the matching node instead of
// opening the content pane; otherwise open the note in the content pane as usual.
window.handleSearchResultSelection = function (el) {
  if (!el || !el.dataset) return false;
  var path = el.dataset.previewPath || '';
  if (!path) return false;

  var hd = window._homeData;
  if (hd && hd.cy && hd.curType() === 'knowledge') {
    var node = hd.cy.getElementById(path);
    if (node && !node.empty()) {
      if (hd._focusedPath === path) hd._clearFocus();
      else hd._applyFocus(node);
      return true;
    }
    // Not in the currently-loaded graph — fall through to open in the content pane.
  }

  var nameEl = el.querySelector('.sr-name');
  var title = nameEl ? nameEl.textContent.trim() : (path.split('/').pop().replace(/\.md$/, '') || 'Note');
  if (window.__hyloContentPane) {
    void window.__hyloContentPane.openNoteInContentPane(path, title,
      el.dataset.noteIsKnowledge === 'true', false, el.dataset.noteIsIndex === 'true',
      el.dataset.noteCanCompile === 'true');
    return true;
  }
  return false;
};

// ── Agent bot avatar color ──────────────────────────────────────────────────
// Global (not a homeCtrl method) because the settings modal's agent-bots list
// renders in its own Alpine scope (settingsCtrl(), shared_settings_modal.go)
// which doesn't inherit homeCtrl's methods — both call this directly instead.
function _agentBotHash(name) {
  var h = 0;
  name = name || '';
  for (var i = 0; i < name.length; i++) h = (Math.imul(31, h) + name.charCodeAt(i)) | 0;
  return Math.abs(h);
}
// Hashing each name straight into a bucket collides often at small n (birthday
// paradox: 8 buckets, a handful of bots) even with a good hash. Fed the full
// current list, resolve that by giving every bot its hashed first choice, then
// bumping collisions to the next free bucket — deterministic and collision-free
// as long as the list is no longer than the 8-color palette.
window.agentBotColorFor = function(name, bots) {
  var palette = ['var(--bot0)', 'var(--bot1)', 'var(--bot2)', 'var(--bot3)', 'var(--bot4)', 'var(--bot5)', 'var(--bot6)', 'var(--bot7)'];
  var n = palette.length;
  if (!bots || !bots.length) return palette[_agentBotHash(name) % n];
  var used = new Array(n).fill(false);
  var mine = -1;
  for (var i = 0; i < bots.length; i++) {
    var bn = bots[i].name || '';
    var idx = _agentBotHash(bn) % n;
    var tries = 0;
    while (used[idx] && tries < n) { idx = (idx + 1) % n; tries++; }
    used[idx] = true;
    if (mine === -1 && bn === name) mine = idx;
  }
  return palette[mine === -1 ? _agentBotHash(name) % n : mine];
};

// ── Per-section list/grid layout preference ────────────────────────────────
// Knowledge/Memory/Pinned/Folders/Tags each get their own list-vs-grid choice
// (all folders share the single 'folder' bucket, all of Knowledge's
// All/category items share the single 'knowledge' bucket, and every tag's
// drilldown shares the single 'tags' bucket — rather than one per selection).
// Knowledge's value can also be 'graph' — see setListView()/listViewKey().
function loadListViewModes() {
  var defaults = { knowledge: 'list', memory: 'list', pinned: 'list', folder: 'list', tags: 'list' };
  try {
    return Object.assign(defaults, JSON.parse(localStorage.getItem('hylo-list-view') || '{}'));
  } catch (_) {
    return defaults;
  }
}

// ── Home Alpine controller ────────────────────────────────────────────────
function homeCtrl() {
  var ctrl = Object.assign(contentPaneCtrl(), homeImagesMixin(), homeGraphMixin(), homeInboxMixin(), homeShortsMixin(), {
    curKey: __hyloSectionKey(__hyloInitialSectionURL()),
    // Each of Knowledge/Memory/Pinned/Folders keeps its own list-vs-grid
    // choice (all folders share the single 'folder' bucket, and all of
    // Knowledge's All/category items share the single 'knowledge' bucket,
    // rather than one per selection) — see listViewKey()/setListView().
    listViewModes: loadListViewModes(),
    foldersOpen: true,
    knowledgeOpen: false,
    chatsOpen: true,
    // Chat transcript lives on this._chat (chat_kernel.js). These fields are
    // what the sidebar and the run-completion toast still bind to.
    agentBots: [],
    selectedAgentBotId: '',
    isMac: /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent),
    toastText: '',
    toastKind: 'ok',
    toastVisible: false,
    _toastTimer: null,
    init() {
      this.initContentPane();
      this.initImages();
      this.initGraph();
      this.initInbox();
      window._homeData = this;
      window.__hyloHotkeys.register('refresh', 'r', doHomeRefresh);
      // Bot list lives in the sidebar, so it loads even when Chats is closed.
      // The kernel keeps a run alive if the pane is swapped away.
      var self = this;
      this._chat = window.__hyloChatCreate({
        onMates: function (mates) { self.agentBots = mates; },
        onMate: function (id) {
          self.selectedAgentBotId = id;
        },
        onToast: function (text, kind) { self.showToast(text, kind); },
      });
      this._chat.start();
      // The first render isn't an htmx swap, but arrives like one.
      __hyloSectionArrived(__hyloInitialSectionURL(), null);
    },
    // One list/grid bucket per section type, so every folder (and every tag) shares one.
    listViewKey() { return this.curType(); },
    curParams() { return __hyloSectionParams(this.curKey); },
    curType() { return this.curParams().get('type') || ''; },
    curParam(name) { return this.curParams().get(name) || ''; },
    currentListView() { return this.listViewModes[this.listViewKey()] || 'list'; },
    setListView(mode) {
      var key = this.listViewKey();
      this.listViewModes[key] = mode;
      try { localStorage.setItem('hylo-list-view', JSON.stringify(this.listViewModes)); } catch (_) { /* ignore */ }
      // Unlike list<->grid (same rows, CSS-only), graph is a different
      // markup shape entirely and has to come from the server.
      if (key === 'knowledge') this.go(this._knowledgeURL(this.curParam('index')));
    },
    _knowledgeURL(index) {
      var url = '/home/section?type=knowledge&view=' + (this.listViewModes.knowledge || 'list');
      if (index) url += '&index=' + encodeURIComponent(index);
      return url;
    },

    selectKnowledgeIndex(path) { this.go(this._knowledgeURL(path)); },

    // Switching tag from the filtered list's picker skips the tag wall.
    selectTag(tag) { this.go('/home/section?type=tags&tag=' + encodeURIComponent(tag)); },

    go(url) {
      this.curKey = __hyloSectionKey(url);
      // Lightbox and image select-mode point at the section being left.
      // The pane itself is parked, so coming back reattaches the same nodes.
      // The inbox detail stays open — it renders from inboxSelected, not the
      // list DOM.
      this.lightbox = null;
      if (this.selectMode) this.exitSelectMode();
      if (!__hyloSectionTryRestore(url)) __hyloSectionFetch(url);
    },

    reloadActiveSection() {
      // Explicit refresh: drop every parked section, then refetch what's on screen.
      __hyloSectionCacheInvalidate();
      __hyloSectionFetch(this.curKey);
    },

    selectChatAgentBot(agentBotId) {
      if (!this._chat) return;
      this._chat.tryOpen(agentBotId);
      this._showChat();
    },

    _showChat() {
      var url = '/home/section?type=chat';
      if (document.getElementById('chat-root')) this.curKey = __hyloSectionKey(url);
      else this.go(url);
    },

    initChatSection() {
      if (!this._chat) return;
      var root = document.getElementById('chat-root');
      if (root) this._chat.attach(root);
      void this._chat.bootstrap();
    },

    agentBotColor(name) {
      return window.agentBotColorFor(name, this.agentBots);
    },

    agentBotInitials(name) {
      if (!name) return '?';
      return name.trim().slice(0, 1).toUpperCase();
    },

    referenceNote(e) {
      var stem = (e && e.detail && e.detail.stem) || '';
      if (this._chat) this._chat.insertWikilink(stem);
    },

    // Editor toolbar's "send to agent bot" — unlike selectChatAgentBot
    // (resumes that bot's most recent conversation), this always lands on a
    // fresh one: handing a note to a bot from outside the chat panel reads
    // as starting something new, not continuing whatever that bot's chat
    // was last doing.
    async sendNoteToAgentBot(e) {
      if (!this._chat) return;
      var mateId = (e && e.detail && e.detail.mateId) || '';
      var stem = (e && e.detail && e.detail.stem) || '';
      if (!mateId) return;
      await this._chat.openFreshChat(mateId);
      this._showChat();
      this._chat.insertWikilink(stem);
    },

    // Any caller (e.g. the drag-to-move flow) can use the .run-toast element, not just agent runs.
    showToast(text, kind) {
      this.toastText = text;
      this.toastKind = kind === 'err' ? 'err' : 'ok';
      this.toastVisible = true;
      if (this._toastTimer) clearTimeout(this._toastTimer);
      this._toastTimer = setTimeout(() => { this.toastVisible = false; }, 6000);
    },

  });

  return ctrl;
}

