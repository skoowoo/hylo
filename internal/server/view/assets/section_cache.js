// ── Home partial refresh via HTMX ────────────────────────────────────────
// Refreshes the sidebar's counts + folder list (hx-partial, morphed so the
// groups keep their Alpine open/closed state), then re-fetches whichever
// section is currently shown in the list pane — only the browser knows that,
// so it can't be folded into the response itself.
function doHomeRefresh() {
  __hyloRefreshSidebar();
  if (window._homeData) {
    if (typeof window._homeData.reloadActiveSection === 'function') window._homeData.reloadActiveSection();
    if (typeof window._homeData.refreshUnreadCount === 'function') window._homeData.refreshUnreadCount();
  }
}

// ── Section DOM cache ──────────────────────────────────────────────────────
// Sidebar switches used to innerHTML-swap #home-list-pane, which discarded
// scroll position and pages already loaded. Cacheable sections are moved
// into a hidden holder that stays inside <body> — Alpine's root — via
// mutateDom, so the move is invisible to Alpine and its bindings survive.
// Ids are stripped while parked: getElementById would otherwise hit the
// hidden copy. Chat is left out; its kernel already detaches and reattaches.
// A note change from the live feed drops parked sections of the affected
// kinds (__hyloNotesChanged). Manual refresh drops everything.
var __hyloSectionCache = new Map(); // insertion order is the LRU order
var __hyloSectionCacheMax = 20;
// A backstop in case the live feed misses something: a section loaded longer
// ago than this is refetched instead of restored.
var __hyloSectionCacheTTL = 30 * 60 * 1000;
var __hyloSectionShownURL = '';
var __hyloSectionShownAt = Date.now();
var __hyloSectionShownStale = false;
var __hyloSectionCacheableTypes = {
  pinned: true, recent: true, folder: true, memory: true, knowledge: true,
  tags: true, images: true, shorts: true, inbox: true,
};

function __hyloSectionKey(raw) {
  if (!raw) return '';
  try {
    var u = new URL(raw, window.location.origin);
    if (u.pathname !== '/home/section') return '';
    // Callers differ in param order and encoding (URLSearchParams vs encodeURIComponent).
    u.searchParams.sort();
    return u.pathname + '?' + u.searchParams.toString();
  } catch (e) {
    return '';
  }
}

function __hyloSectionParams(key) {
  try { return new URL(key, window.location.origin).searchParams; }
  catch (e) { return new URLSearchParams(); }
}

function __hyloSectionType(key) {
  return __hyloSectionParams(key).get('type') || '';
}

function __hyloInitialSectionURL() {
  var u = new URL(window.location.href);
  var type = u.searchParams.get('type') || 'inbox';
  var q = new URLSearchParams();
  q.set('type', type);
  var path = u.searchParams.get('path');
  if (path) q.set('path', path);
  var tag = u.searchParams.get('tag');
  if (tag) q.set('tag', tag);
  if (type === 'knowledge') {
    q.set('view', u.searchParams.get('view') || 'list');
    var index = u.searchParams.get('index');
    if (index) q.set('index', index);
  }
  var from = u.searchParams.get('from');
  if (from) q.set('from', from);
  return '/home/section?' + q.toString();
}

__hyloSectionShownURL = __hyloSectionKey(__hyloInitialSectionURL());

function __hyloSectionStashIds(root) {
  var nodes = root.querySelectorAll('[id]');
  for (var i = 0; i < nodes.length; i++) {
    nodes[i].setAttribute('data-hylo-stashed-id', nodes[i].id);
    nodes[i].removeAttribute('id');
  }
}

function __hyloSectionUnstashIds(root) {
  var nodes = root.querySelectorAll('[data-hylo-stashed-id]');
  for (var i = 0; i < nodes.length; i++) {
    nodes[i].id = nodes[i].getAttribute('data-hylo-stashed-id');
    nodes[i].removeAttribute('data-hylo-stashed-id');
  }
}

function __hyloSectionCacheDrop(entry) {
  if (entry.cy) {
    try { entry.cy.destroy(); } catch (e) { /* ignore */ }
    entry.cy = null;
  }
  var holder = entry.holder;
  // Parked via mutateDom, so Alpine never observed the nodes leaving the
  // live tree. Removing the holder alone doesn't list those children in the
  // mutation record — destroy them or their effects keep running detached.
  if (window.Alpine && Alpine.destroyTree) {
    var child = holder.firstElementChild;
    while (child) {
      var next = child.nextElementSibling;
      Alpine.destroyTree(child);
      child = next;
    }
  }
  if (holder.parentNode) holder.remove();
}

function __hyloSectionCacheDelete(key) {
  var entry = __hyloSectionCache.get(key);
  if (!entry) return;
  __hyloSectionCache.delete(key);
  __hyloSectionCacheDrop(entry);
}

function __hyloSectionCacheInvalidate() {
  __hyloSectionCache.forEach(__hyloSectionCacheDrop);
  __hyloSectionCache.clear();
}

function __hyloIsType(type) {
  return function (key) { return __hyloSectionType(key) === type; };
}

function __hyloDropParked(pred) {
  var keys = [];
  __hyloSectionCache.forEach(function (_, key) { if (pred(key)) keys.push(key); });
  keys.forEach(__hyloSectionCacheDelete);
}

// Drops matching parked sections; true when the shown one matches too.
function __hyloDropWhere(pred) {
  __hyloDropParked(pred);
  return !!__hyloSectionShownURL && pred(__hyloSectionShownURL);
}

// The pane is the source of every section load: hx-sync="this:replace" on it
// lets a newer load cancel an older one, apart from /home/refresh on <body>.
function __hyloSectionFetch(url) {
  htmx.ajax('GET', url, { source: '#home-list-pane', target: '#home-list-pane', swap: 'innerHTML' });
}

function __hyloRefreshSidebar() {
  Promise.resolve(htmx.ajax('GET', '/home/refresh', { target: document.body, swap: 'none' })).then(function () {
    if (window._homeData) window._homeData.syncSideEmpty();
  }, function () {});
}

function __hyloRefetchShown() {
  if (!__hyloSectionShownURL) return;
  // The chat's DOM is the kernel's: remounting it would drop its built rows and refocus the composer.
  if (__hyloSectionType(__hyloSectionShownURL) === 'chat') {
    if (window._homeData && window._homeData._chat) void window._homeData._chat.reload();
    return;
  }
  // Leaving before the refetch lands must not park the outdated copy.
  __hyloSectionShownStale = true;
  __hyloSectionFetch(__hyloSectionShownURL);
}

// An animation running on the shown list holds its reload until it's done;
// the reload runs once the last hold is released.
var __hyloRefetchHolds = 0;
var __hyloRefetchHeld = false;
function __hyloHoldRefetch() {
  __hyloRefetchHolds++;
  var released = false;
  return function () {
    if (released) return;
    released = true;
    if (--__hyloRefetchHolds > 0 || !__hyloRefetchHeld) return;
    __hyloRefetchHeld = false;
    __hyloRefetchShown();
  };
}

// Note changes come from the live feed (live.js), from any source.
function __hyloNotesChanged(changes) {
  var editing = window.__hyloEditor ? __hyloEditor.currentPath : '';
  var a = __hyloNoteChanges.affect(changes, { editing: editing });
  var refetch = false;
  if (a.all) {
    __hyloDropParked(function () { return true; });
    refetch = true;
  } else {
    Object.keys(a.park).forEach(function (t) { __hyloDropParked(__hyloIsType(t)); });
    Object.keys(a.refetch).forEach(function (t) { if (__hyloDropWhere(__hyloIsType(t))) refetch = true; });
  }
  if (a.sidebar) __hyloRefreshSidebar();
  if (!refetch) return;
  if (__hyloRefetchHolds > 0) __hyloRefetchHeld = true;
  else __hyloRefetchShown();
}
__hyloLive.on('notes', __hyloNotesChanged);

function __hyloSectionCacheRemember(key, entry) {
  __hyloSectionCacheDelete(key);
  var now = Date.now();
  __hyloDropParked(function (k) { return now - __hyloSectionCache.get(k).loadedAt > __hyloSectionCacheTTL; });
  __hyloSectionCache.set(key, entry);
  while (__hyloSectionCache.size > __hyloSectionCacheMax) {
    __hyloSectionCacheDelete(__hyloSectionCache.keys().next().value);
  }
}

function __hyloSectionMutate(fn) {
  if (window.Alpine && Alpine.mutateDom) Alpine.mutateDom(fn);
  else fn();
}

// display:none on the parked holder clears scrollTop. Read it while the
// section is still on screen, and write it back once the nodes are visible.
// Chat keeps its own scroll: it sticks to the newest message.
var __hyloSectionScrollSel = '.sec-body:not(.chat-scroll)';

function __hyloSectionCaptureScroll(root) {
  var nodes = root.querySelectorAll(__hyloSectionScrollSel);
  var saved = [];
  for (var i = 0; i < nodes.length; i++) saved.push({ el: nodes[i], top: nodes[i].scrollTop });
  return saved;
}

function __hyloSectionApplyScroll(saved) {
  if (!saved.length) return;
  function apply() {
    for (var i = 0; i < saved.length; i++) saved[i].el.scrollTop = saved[i].top;
  }
  apply();
  requestAnimationFrame(function () {
    apply();
    requestAnimationFrame(apply);
  });
}

// Unhooks whatever the shown section attached outside its own nodes.
function __hyloSectionTeardown(pane) {
  var hd = window._homeData;
  if (hd && hd._chat && pane.querySelector('#chat-root')) hd._chat.detach();
  if (hd && hd.cy) { hd.cy.destroy(); hd.cy = null; }
  if (__hyloTagCloudObserver) { __hyloTagCloudObserver.disconnect(); __hyloTagCloudObserver = null; }
  clearTimeout(__hyloTagCloudResizeTimer);
}

// Move the live pane's nodes into the cache, or throw them away when the
// section isn't cacheable (chat) or is known to be outdated.
function __hyloSectionPark() {
  var key = __hyloSectionShownURL;
  var pane = document.getElementById('home-list-pane');
  if (!pane || !pane.firstElementChild) return;
  if (!__hyloSectionCacheableTypes[__hyloSectionType(key)] || __hyloSectionShownStale) {
    __hyloSectionTeardown(pane);
    while (pane.firstChild) pane.removeChild(pane.firstChild);
    return;
  }
  var entry = {
    holder: document.createElement('div'), loadedAt: __hyloSectionShownAt,
    cy: null, zoom: 1, pan: { x: 0, y: 0 }, scroll: __hyloSectionCaptureScroll(pane),
  };
  entry.holder.hidden = true;
  entry.holder.setAttribute('data-hylo-section-cache', '');
  var hd = window._homeData;
  if (hd && hd.cy && pane.querySelector('#graph-canvas')) {
    entry.cy = hd.cy;
    entry.zoom = hd.cy.zoom();
    entry.pan = { x: hd.cy.pan().x, y: hd.cy.pan().y };
    hd.cy = null;
  }
  __hyloSectionTeardown(pane);
  __hyloSectionMutate(function () {
    document.body.appendChild(entry.holder);
    while (pane.firstChild) entry.holder.appendChild(pane.firstChild);
    __hyloSectionStashIds(entry.holder);
  });
  __hyloSectionCacheRemember(key, entry);
}

function __hyloSectionRestoreGraph(cy, zoom, pan) {
  var tries = 0;
  function apply() {
    if (cy.destroyed()) return;
    cy.resize();
    if (cy.width() < 2 && tries++ < 8) { requestAnimationFrame(apply); return; }
    cy.zoom(zoom);
    cy.pan(pan);
  }
  requestAnimationFrame(apply);
}

// Puts a fresh parked copy of url back in the pane. Called before any fetch,
// so a cached section never waits behind another pane request.
function __hyloSectionTryRestore(url) {
  var key = __hyloSectionKey(url);
  var entry = key && __hyloSectionCache.get(key);
  if (!entry) return false;
  if (Date.now() - entry.loadedAt > __hyloSectionCacheTTL) {
    __hyloSectionCacheDelete(key);
    return false;
  }
  var pane = document.getElementById('home-list-pane');
  if (!pane) return false;
  // A load for another section may still be in flight; its response is stale now.
  htmx.trigger(pane, 'htmx:abort');
  __hyloSectionPark();
  __hyloSectionCache.delete(key);
  __hyloSectionMutate(function () {
    __hyloSectionUnstashIds(entry.holder);
    while (entry.holder.firstChild) pane.appendChild(entry.holder.firstChild);
    entry.holder.remove();
  });
  __hyloSectionArrived(key, entry);
  return true;
}

// Every way a section reaches the pane — first render, htmx swap, cache
// restore — ends here. A restored section keeps its initialized nodes, so
// only what lives outside them (graph, tag cloud layout) is redone.
function __hyloSectionArrived(url, entry) {
  __hyloSectionShownURL = __hyloSectionKey(url);
  __hyloSectionShownAt = entry ? entry.loadedAt : Date.now();
  __hyloSectionShownStale = false;
  __hyloUpdateDraggableCards();
  var hd = window._homeData;
  if (hd) {
    if (document.getElementById('graph-canvas')) {
      if (entry && entry.cy) {
        hd.cy = entry.cy;
        __hyloSectionRestoreGraph(entry.cy, entry.zoom, entry.pan);
      } else {
        hd.loadGraph();
      }
    }
    if (!entry) {
      if (document.getElementById('home-inbox-list')) hd.loadInbox();
      if (document.getElementById('chat-root')) hd.initChatSection();
    }
  }
  if (entry) __hyloSectionApplyScroll(entry.scroll);
  if (document.getElementById('tag-cloud')) __hyloRenderTagCloud();
}

// App-level htmx hooks; the name is whitelisted in page_home.html's htmx-config.
// Section loads all come from __hyloSectionFetch (source = target = the pane).
htmx.registerExtension('hylo', {
  htmx_before_swap: function (elt, detail) {
    // htmx re-executes <script> in every swapped fragment and 4.0 dropped the
    // allowScriptTags switch. Fragments carry rendered note HTML (raw HTML
    // allowed), so no response may bring its own scripts.
    detail.tasks.forEach(function (t) {
      if (t.fragment) t.fragment.querySelectorAll('script').forEach(function (s) { s.remove(); });
    });
    var ctx = detail.ctx;
    var pane = document.getElementById('home-list-pane');
    if (!ctx || !pane || ctx.target !== pane) return;
    // Parking waits for a response that will swap, so the old section stays
    // on screen while loading and survives a failed request.
    var swaps = detail.tasks.some(function (t) {
      return t.type === 'main' && t.swapSpec.style !== 'none';
    });
    if (!swaps) return;
    // A refetch of the shown section replaces it; anything else parks it.
    var key = __hyloSectionKey(ctx.request.action);
    if (key && key !== __hyloSectionShownURL) __hyloSectionPark();
    else __hyloSectionTeardown(pane);
  },

  htmx_after_swap: function (elt, detail) {
    var ctx = detail.ctx;
    var pane = document.getElementById('home-list-pane');
    if (!ctx || !pane || ctx.target !== pane || !ctx.response) return;
    try {
      var u = new URL(ctx.response.raw.url);
      __hyloSectionArrived(u.pathname + u.search, null);
    } catch (err) {
      console.error('[hylo] section activation failed', err);
    }
  },

  // Error statuses never swap (htmx-config noSwap), so these toasts are the
  // only feedback a failed fragment request gives.
  htmx_response_error: function (elt, detail) {
    if (detail.ctx && detail.ctx.response) __hyloRequestFailed(detail.ctx, detail.ctx.response.status);
  },

  htmx_error: function (elt, detail) {
    var err = detail.error;
    if (!detail.ctx || !err || err.name === 'AbortError') return;
    __hyloRequestFailed(detail.ctx, err.name + ': ' + err.message);
  },
});

function __hyloRequestFailed(ctx, why) {
  var path = '';
  try { path = new URL(ctx.request.action, window.location.origin).pathname; } catch (_) { /* ignore */ }
  console.warn('[htmx] request failed', path, why, ctx);
  if (window._homeData) window._homeData.showToast('Request failed: ' + path + ' (' + why + ')', 'err');
}
