
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

// ── Agent bot avatar ────────────────────────────────────────────────────────
// Global (not a homeCtrl method) because the settings modal's agent-bots list
// renders in its own Alpine scope (settingsCtrl(), settings_modal.js)
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
// as long as the list is no longer than the 8-color palette. Only enabled bots
// take part, so settings (which lists disabled ones too) matches the sidebar.
function _agentBotSlot(name, bots) {
  var n = 8;
  if (!bots || !bots.length) return _agentBotHash(name) % n;
  var used = new Array(n).fill(false);
  var mine = -1;
  for (var i = 0; i < bots.length; i++) {
    if (bots[i].enabled === false) continue;
    var bn = bots[i].name || '';
    var idx = _agentBotHash(bn) % n;
    var tries = 0;
    while (used[idx] && tries < n) { idx = (idx + 1) % n; tries++; }
    used[idx] = true;
    if (mine === -1 && bn === name) mine = idx;
  }
  return mine === -1 ? _agentBotHash(name) % n : mine;
}
// Each color slot has its own cheerful face, so a bot keeps one color and one
// face. A face is just two eyes, each a solid circle or ellipse, set on one of
// five spots; anim names the face's one-shot move in base.css.
var _BOT_EYES = {
  dot: function(x) { return '<circle cx="' + x + '" cy="0" r="5.4"/>'; },
  dotL: function(x) { return '<circle cx="' + x + '" cy="0" r="6.6"/>'; },
  dotS: function(x) { return '<circle cx="' + x + '" cy="0.6" r="3.8"/>'; },
  oval: function(x) { return '<ellipse cx="' + x + '" cy="0" rx="4.3" ry="6.6"/>'; },
  ovalW: function(x) { return '<ellipse cx="' + x + '" cy="0" rx="6.2" ry="4.2"/>'; },
};
// The four corners share one offset so the spot reads even at 20px.
var _BOT_C = [0, 0], _BOT_TL = [-8, -6], _BOT_TR = [8, -6], _BOT_BL = [-8, 6], _BOT_BR = [8, 6];
var _BOT_FACES = [
  { eyes: ['dot', 'dot'], at: _BOT_C, anim: 'squint' },
  { eyes: ['dotL', 'dotL'], at: _BOT_TR, anim: 'hop' },
  { eyes: ['oval', 'oval'], at: _BOT_C, anim: 'look' },
  { eyes: ['oval', 'dot'], at: _BOT_BL, tilt: 6, anim: 'wink' },
  { eyes: ['oval', 'dotS'], gap: 11, at: _BOT_TL, tilt: -8, anim: 'curious' },
  { eyes: ['dotS', 'dotS'], gap: 10, at: _BOT_BR, anim: 'wobble' },
  { eyes: ['dotL', 'dotS'], gap: 13, at: _BOT_BL, tilt: 5, anim: 'giggle' },
  { eyes: ['ovalW', 'ovalW'], gap: 13, at: _BOT_TL, anim: 'wave' },
];

window.agentBotAvatarSVG = function(name, bots) {
  var slot = _agentBotSlot(name, bots), f = _BOT_FACES[slot], gap = f.gap || 12;
  var eye = function(k, x) { return '<g class="eye eye--' + k + '">' + _BOT_EYES[k](x) + '</g>'; };
  return '<svg class="bot-face" data-anim="' + f.anim + '" style="--blink-at:' + (slot * 5 % 8) * 0.8 + 's"' +
    ' viewBox="0 0 100 100" aria-hidden="true"><circle cx="50" cy="50" r="50" style="fill:var(--bot' + slot + ')"/>' +
    '<g transform="translate(50 50) scale(1.3) translate(' + f.at[0] + ' ' + f.at[1] + ') rotate(' + (f.tilt || 0) + ')">' +
    '<g class="face"><g class="eyes">' + eye(f.eyes[0], -gap) + eye(f.eyes[1], gap) + '</g></g></g></svg>';
};

// Plays a face's one-shot move; the class clears itself when the move ends. A face
// that wasn't rendered never fires animationend, so a stale class restarts.
window.agentBotAvatarPlay = function(face) {
  if (!face) return;
  if (face.classList.contains('is-playing')) {
    if (Date.now() - face._playAt < 1500) return;
    face.classList.remove('is-playing');
    void face.getBoundingClientRect();
  }
  face._playAt = Date.now();
  face.classList.add('is-playing');
};
document.addEventListener('animationend', function(e) {
  var t = e.target;
  if (t.classList && t.classList.contains('face')) {
    var svg = t.closest('.bot-face');
    if (svg) svg.classList.remove('is-playing');
  }
});
// Rows that carry a bot avatar play it when the pointer comes onto the row.
document.addEventListener('pointerover', function(e) {
  var row = e.target.closest && e.target.closest('[data-avatar-hover]');
  if (!row || (e.relatedTarget && row.contains(e.relatedTarget))) return;
  window.agentBotAvatarPlay(row.querySelector('.bot-face'));
});

// ── Per-section list/grid layout preference ────────────────────────────────
// Knowledge/Memory/Pinned/Folders/Tags each get their own list-vs-grid choice
// (all folders share the single 'folder' bucket, all of Knowledge's
// All/category items share the single 'knowledge' bucket, and every tag's
// drilldown shares the single 'tags' bucket — rather than one per selection).
// Knowledge's value can also be 'graph' — see setListView()/listViewKey().
function loadListViewModes() {
  var defaults = { knowledge: 'list', memory: 'list', pinned: 'list', recent: 'list', folder: 'list', tags: 'list' };
  try {
    return Object.assign(defaults, JSON.parse(localStorage.getItem('hylo-list-view') || '{}'));
  } catch (_) {
    return defaults;
  }
}

// ── Sidebar reflow motion ────────────────────────────────────────────────
// When rows are added or removed (a tab opens or closes, a refresh lands),
// everything that stays slides from where it was instead of jumping, and
// what is new fades in. Capture before the change, play after it renders.
var __hyloSideFlipGen = 0;
function __hyloSideFlipTargets(nav) {
  return Array.prototype.filter.call(
    nav.querySelectorAll(':scope > :not(script), .home-side-child'),
    function (el) { return el.getClientRects().length > 0; });
}
// Content-relative, so a scroll in between doesn't read as movement.
function __hyloSideFlipTops(nav, els) {
  var base = nav.getBoundingClientRect().top - nav.scrollTop;
  var tops = new Map();
  els.forEach(function (el) { tops.set(el, el.getBoundingClientRect().top - base); });
  return tops;
}
function __hyloSideFlipFrame(el) {
  return el.classList.contains('home-side-child') ? el.closest('.home-side-nav > *') : null;
}
function __hyloSideFlipCapture() {
  var nav = document.querySelector('.home-side-nav');
  if (!nav) return null;
  var els = __hyloSideFlipTargets(nav);
  return { nav: nav, gen: ++__hyloSideFlipGen, before: __hyloSideFlipTops(nav, els) };
}
function __hyloSideFlipPlay(snap) {
  // A newer capture owns the motion; this one's "before" is stale.
  if (!snap || snap.gen !== __hyloSideFlipGen) return;
  var nav = snap.nav;
  var els = __hyloSideFlipTargets(nav);
  els.forEach(function (el) { if (el._flip) { el._flip.cancel(); el._flip = null; } });
  var after = __hyloSideFlipTops(nav, els);
  var cs = getComputedStyle(document.documentElement);
  var ms = parseFloat(cs.getPropertyValue('--motion-slow')) || 0;
  var ease = cs.getPropertyValue('--ease-out').trim() || 'ease-out';
  els.forEach(function (el) {
    var frame = __hyloSideFlipFrame(el);
    var b = snap.before.get(el);
    var fb = frame ? snap.before.get(frame) : 0;
    if (b === undefined || fb === undefined) {
      el._flip = el.animate([{ opacity: 0 }, { opacity: 1 }], { duration: ms, easing: ease });
      return;
    }
    var dy = (b - fb) - (after.get(el) - (frame ? after.get(frame) : 0));
    if (Math.abs(dy) < 0.5) return;
    el._flip = el.animate([{ transform: 'translateY(' + dy + 'px)' }, { transform: 'none' }], { duration: ms, easing: ease });
  });
}

// ── x-press ───────────────────────────────────────────────────────────────
// x-press="expr" runs on a left mouse press instead of waiting ~80ms for the
// release, like native sidebars and tab strips. Keyboard activation (a click
// with detail 0) and touch/pen taps still go through click, so a scroll
// gesture never activates. A child that must not activate its parent stops
// pointerdown (and click) itself. The press doesn't move focus either: the
// action already ran, and the mousedown that follows would undo any focus it set.
document.addEventListener('alpine:init', function () {
  Alpine.directive('press', function (el, { expression }, { evaluateLater, cleanup }) {
    var run = evaluateLater(expression);
    var pressed = false;
    function onPointerDown(e) {
      // Ctrl+click is the macOS secondary click.
      pressed = e.pointerType === 'mouse' && e.button === 0 && !e.ctrlKey;
      if (pressed) run(function () {}, { scope: { $event: e } });
    }
    function onMouseDown(e) {
      if (pressed) e.preventDefault();
    }
    function onClick(e) {
      if (pressed && e.detail > 0) { pressed = false; return; }
      run(function () {}, { scope: { $event: e } });
    }
    el.addEventListener('pointerdown', onPointerDown);
    el.addEventListener('mousedown', onMouseDown);
    el.addEventListener('click', onClick);
    cleanup(function () {
      el.removeEventListener('pointerdown', onPointerDown);
      el.removeEventListener('mousedown', onMouseDown);
      el.removeEventListener('click', onClick);
    });
  });
});

// ── Home Alpine controller ────────────────────────────────────────────────
function homeCtrl() {
  var boot = JSON.parse(document.getElementById('home-boot').textContent);
  var ctrl = Object.assign(contentPaneCtrl(), homeImagesMixin(), homeGraphMixin(), homeInboxMixin(boot), homeShortsMixin(), {
    curKey: __hyloSectionKey(__hyloInitialSectionURL()),
    // Each of Knowledge/Memory/Pinned/Folders keeps its own list-vs-grid
    // choice (all folders share the single 'folder' bucket, and all of
    // Knowledge's All/category items share the single 'knowledge' bucket,
    // rather than one per selection) — see listViewKey()/setListView().
    listViewModes: loadListViewModes(),
    foldersOpen: false,
    knowledgeOpen: false,
    openTabsOpen: true,
    chatsOpen: true,
    // Chat transcript lives on this._chat (chat_kernel.js). These fields are
    // what the sidebar and the run-completion toast still bind to.
    // Seeded from the server so the rows render with the page; /api/mates refreshes them.
    agentBots: boot.bots || [],
    agentBotsLoaded: !!boot.bots,
    // A group with nothing in it has no chevron and opens its section instead.
    sideEmpty: { folders: false, knowledge: false },
    // Shown beside a collapsed group's label.
    sideCount: { folders: 0, knowledge: 0 },
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
      this.syncSideGroups();
      window.__hyloHotkeys.register('refresh', 'r', doHomeRefresh);
      // Bot list lives in the sidebar, so it loads even when Chats is closed.
      // The kernel keeps a run alive if the pane is swapped away.
      var self = this;
      this._chat = window.__hyloChatCreate({
        onMates: function (mates) { self.agentBots = mates; self.agentBotsLoaded = true; },
        onMate: function (id) {
          self.selectedAgentBotId = id;
        },
        onToast: function (text, kind) { self.showToast(text, kind); },
      });
      this._chat.start();
      // The first render isn't an htmx swap, but arrives like one.
      __hyloSectionArrived(__hyloInitialSectionURL(), null);
    },
    sideFade(nav) {
      var below = nav.scrollHeight - nav.clientHeight - nav.scrollTop;
      nav.style.setProperty('--fade-top', Math.min(nav.scrollTop, 24) + 'px');
      nav.style.setProperty('--fade-bottom', Math.min(Math.max(below, 0), 24) + 'px');
    },
    // Content height changes without a scroll (groups opening, tabs), so watch it too.
    initSideFade(nav) {
      var self = this;
      self.sideFade(nav);
      if (typeof ResizeObserver === 'undefined') return;
      var ro = new ResizeObserver(function () { self.sideFade(nav); });
      ro.observe(nav);
      Array.prototype.forEach.call(nav.children, function (c) { if (c.tagName !== 'SCRIPT') ro.observe(c); });
    },
    botsEmpty() { return this.agentBotsLoaded && this.agentBots.length === 0; },

    // The groups' bodies are server-rendered and morphed by /home/refresh;
    // each carries a .home-side-meta with its count and emptiness.
    syncSideGroups() {
      for (var k of ['folders', 'knowledge']) {
        var meta = document.querySelector('#home-side-' + k + '-body .home-side-meta');
        this.sideEmpty[k] = !!meta && meta.hasAttribute('data-empty');
        this.sideCount[k] = meta ? Number(meta.dataset.count) || 0 : 0;
      }
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
      // A maximized editor would cover the section the user just asked for.
      this.revealListPane();
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
      // The chat's DOM is the kernel's, so it refetches the conversation instead.
      if (this._chat && document.getElementById('chat-root')) {
        this._chat.reload();
        return;
      }
      __hyloSectionFetch(this.curKey);
    },

    selectChatAgentBot(agentBotId) {
      if (!this._chat) return;
      this._chat.tryOpen(agentBotId);
      this._showChat();
    },

    _showChat() {
      var url = '/home/section?type=chat';
      if (!document.getElementById('chat-root')) return this.go(url);
      this.revealListPane();
      this.curKey = __hyloSectionKey(url);
    },

    initChatSection() {
      if (!this._chat) return;
      var root = document.getElementById('chat-root');
      if (root) this._chat.attach(root);
      void this._chat.bootstrap();
    },

    agentBotAvatar(name) {
      return window.agentBotAvatarSVG(name, this.agentBots);
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

