
// ── Search result selection ────────────────────────────────────────────────
// While Knowledge's graph view is showing, focus the matching node instead of
// opening the content pane (mirrors graph.js's override for the standalone page);
// otherwise open the note in the content pane as usual.
window.handleSearchResultSelection = function (el) {
  if (!el || !el.dataset) return false;
  var path = el.dataset.previewPath || '';
  if (!path) return false;

  var hd = window._homeData;
  if (hd && hd.cy && hd.activeKey.indexOf('knowledge:') === 0) {
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

// ── Images gallery embedded in the list pane (lightbox + select mode) ─────
// (Mirrors images.js's globals/controller for the standalone /images page.)
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

function extToType(ext) {
  var m = {
    '.jpg': 'JPEG Image', '.jpeg': 'JPEG Image', '.png': 'PNG Image',
    '.gif': 'GIF Image', '.webp': 'WebP Image', '.avif': 'AVIF Image', '.svg': 'SVG Vector',
  };
  return m[(ext || '').toLowerCase()] || ((ext || '').toUpperCase().replace('.', '') + ' Image');
}

// ── Home partial refresh via HTMX ────────────────────────────────────────
// Refreshes the sidebar's counts + folder list (OOB), then re-fetches
// whichever section is currently shown in the list pane — only the browser
// knows that, so it can't be folded into the OOB response itself.
// htmx's default settle behavior copies "class"/"style" from an OOB swap's
// old element onto the new one (for CSS-transition continuity), then strips
// them again after the settle delay. That collides with Alpine's own
// reactive style="display:none" on x-show: htmx would immediately erase the
// inline style Alpine had just set on the freshly swapped-in
// #home-side-knowledge-body/#home-side-folders-body, leaving it visible
// regardless of knowledgeOpen/foldersOpen — i.e. the sidebar group reads as
// auto-expanded after every /home/refresh. Nothing here relies on htmx's
// class/style settle animation, so just disable it.
htmx.config.attributesToSettle = [];

function doHomeRefresh() {
  htmx.ajax('GET', '/home/refresh', { target: document.body, swap: 'none' });
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
// A vault change names what happened (__hyloVaultChanged). Parked sections
// that the change cannot affect stay. A plain note save patches rows in
// place, including parked copies (editor_session.js). Manual refresh still
// drops everything.
var __hyloSectionCache = new Map(); // insertion order is the LRU order
var __hyloSectionCacheMax = 20;
// Agents, clips and edits on disk never reach __hyloVaultChanged, so a
// section loaded longer ago than this is refetched instead of restored.
var __hyloSectionCacheTTL = 10 * 60 * 1000;
var __hyloSectionShownURL = '';
var __hyloSectionShownAt = Date.now();
var __hyloSectionShownStale = false;
// Key of the latest navigation; a response for any other key is stale.
var __hyloSectionWant = '';
var __hyloSectionCacheableTypes = {
  pinned: true, folder: true, memory: true, knowledge: true,
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
  var type = u.searchParams.get('type') || 'pinned';
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

function __hyloNoteDir(path) {
  if (!path) return '/';
  var slash = path.lastIndexOf('/');
  return slash <= 0 ? '/' : path.slice(0, slash);
}

function __hyloSectionListsDir(key, dir) {
  var params = __hyloSectionParams(key);
  var type = params.get('type');
  if (type === 'folder') return (params.get('path') || '/') === dir;
  if (type === 'memory') return dir === '/_memory';
  if (type === 'knowledge' && !params.get('index')) return dir === __hyloKnowledgeDir;
  return false;
}

function __hyloIsType(type) {
  return function (key) { return __hyloSectionType(key) === type; };
}

function __hyloIsGraph(key) {
  var params = __hyloSectionParams(key);
  return params.get('type') === 'knowledge' && params.get('view') === 'graph';
}

// Graph and per-index lists aren't a flat directory, so a row patch can't fix them.
function __hyloIsKnowledgeDerived(key) {
  return __hyloIsGraph(key) || (__hyloSectionType(key) === 'knowledge' && !!__hyloSectionParams(key).get('index'));
}

function __hyloPreviewSection(key) {
  var type = __hyloSectionType(key);
  return type === 'folder' || type === 'memory' || type === 'pinned' || type === 'tags' || type === 'knowledge';
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

function __hyloSectionFetch(url) {
  __hyloSectionWant = __hyloSectionKey(url);
  // Pane as source gives section loads their own htmx sync queue, apart
  // from /home/refresh on <body>.
  htmx.ajax('GET', url, { source: '#home-list-pane', target: '#home-list-pane', swap: 'innerHTML' });
}

function __hyloRefreshSidebar() {
  if (window.htmx) htmx.ajax('GET', '/home/refresh', { target: document.body, swap: 'none' });
}

function __hyloRefetchShown() {
  if (!__hyloSectionShownURL || !window.htmx) return;
  // Leaving before the refetch lands must not park the outdated copy.
  __hyloSectionShownStale = true;
  __hyloSectionFetch(__hyloSectionShownURL);
}

function __hyloAdjustCount(root, delta) {
  var el = root && root.querySelector('.home-list-count');
  if (!el) return;
  var n = parseInt(el.textContent, 10);
  if (isNaN(n)) return;
  el.textContent = String(Math.max(0, n + delta));
}

function __hyloNoteRows(path) {
  return document.querySelectorAll('.home-note-row[data-note-path="' + CSS.escape(path) + '"]');
}

function __hyloForSectionRoots(fn) {
  var pane = document.getElementById('home-list-pane');
  if (pane && __hyloSectionShownURL) fn(__hyloSectionShownURL, pane, true);
  __hyloSectionCache.forEach(function (entry, key) { fn(key, entry.holder, false); });
}

// Remove the note from lists that contain it, and fix the count even when
// it sits on a page that hasn't loaded. An emptied list is refetched (shown)
// or dropped (parked) so the empty state isn't a blank pane.
function __hyloForgetNoteInLists(path, pinned) {
  var dir = __hyloNoteDir(path);
  var refetchShown = false;
  var emptied = [];
  __hyloForSectionRoots(function (key, root, live) {
    if (!__hyloSectionListsDir(key, dir) && !(pinned && __hyloSectionType(key) === 'pinned')) return;
    var row = root.querySelector('.home-note-row[data-note-path="' + CSS.escape(path) + '"]');
    if (row) row.remove();
    __hyloAdjustCount(root, -1);
    if (root.querySelector('.home-note-row')) return;
    if (live) refetchShown = true;
    else emptied.push(key);
  });
  emptied.forEach(__hyloSectionCacheDelete);
  return refetchShown;
}

function __hyloRetargetNoteRows(oldPath, newPath) {
  if (!oldPath || !newPath || oldPath === newPath) return;
  var rows = __hyloNoteRows(oldPath);
  var stem = __hyloNoteStem(newPath);
  var oldStem = __hyloNoteStem(oldPath);
  var dir = __hyloNoteDir(newPath);
  for (var i = 0; i < rows.length; i++) {
    var row = rows[i];
    row.dataset.notePath = newPath;
    if (row.dataset.noteTitle === oldStem) {
      row.dataset.noteTitle = stem;
      var titleEl = row.querySelector('.home-note-row-title');
      if (titleEl && titleEl.textContent === oldStem) titleEl.textContent = stem;
    }
    var dirEl = row.querySelector('.home-note-row-dir');
    if (dirEl) dirEl.textContent = dir;
  }
}

var __hyloPinBadgeHTML = '<span class="home-note-badge home-note-badge--pin" title="Pinned"><svg fill="currentColor" viewBox="0 0 24 24"><path d="M17 3a2 2 0 0 1 2 2v15a1 1 0 0 1-1.496.868l-4.512-2.578a2 2 0 0 0-1.984 0l-4.512 2.578A1 1 0 0 1 5 20V5a2 2 0 0 1 2-2z"/></svg></span>';
var __hyloCompiledBadgeHTML = '<span class="home-note-badge home-note-badge--done" title="Compiled"><svg fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24"><path d="M20 6 9 17l-5-5"/></svg></span>';

function __hyloSetPinnedBadge(path, pinned) {
  var rows = __hyloNoteRows(path);
  for (var i = 0; i < rows.length; i++) {
    var row = rows[i];
    row.dataset.notePinned = pinned ? 'true' : 'false';
    var badge = row.querySelector('.home-note-badge--pin');
    if (pinned && !badge) {
      var host = row.querySelector('.home-note-row-badges');
      if (host) host.insertAdjacentHTML('afterbegin', __hyloPinBadgeHTML);
    } else if (!pinned && badge) badge.remove();
  }
}

function __hyloSetCompiledBadge(path) {
  var rows = __hyloNoteRows(path);
  for (var i = 0; i < rows.length; i++) {
    var row = rows[i];
    row.dataset.noteCanCompile = 'false';
    if (row.querySelector('.home-note-row-badges [title="Compiled"]')) continue;
    var host = row.querySelector('.home-note-row-badges');
    if (host) host.insertAdjacentHTML('beforeend', __hyloCompiledBadgeHTML);
  }
}

// Compared across saves so a typo doesn't throw away the tag cloud or graph.
var __hyloNoteSig = new Map();

function __hyloCleanTag(s) {
  return String(s || '').trim().replace(/^['"]|['"]$/g, '').trim();
}

function __hyloExtractTags(md) {
  var m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(md || '');
  if (!m) return [];
  var lines = m[1].split(/\r?\n/);
  var tags = [];
  var inList = false;
  for (var i = 0; i < lines.length; i++) {
    var line = lines[i];
    if (!inList) {
      var bracket = /^tags:\s*\[(.*)\]\s*$/.exec(line);
      if (bracket) return bracket[1].split(',').map(__hyloCleanTag).filter(Boolean).sort();
      if (/^tags:\s*$/.test(line)) { inList = true; continue; }
      var one = /^tags:\s*(\S.*)$/.exec(line);
      if (one) return [__hyloCleanTag(one[1])].filter(Boolean);
    } else {
      var item = /^\s*-\s*(\S.*)$/.exec(line);
      if (item) tags.push(__hyloCleanTag(item[1]));
      else if (/^\S/.test(line)) break;
    }
  }
  tags.sort();
  return tags;
}

function __hyloExtractLinks(md) {
  var out = [];
  var text = md || '';
  var re = /\[\[([^\]|#]+)/g;
  var found;
  while ((found = re.exec(text))) {
    if (found.index > 0 && text.charAt(found.index - 1) === '!') continue;
    var name = found[1].trim();
    if (name) out.push(name);
  }
  out.sort();
  return out;
}

function __hyloNoteSigFromContent(md) {
  return { tags: __hyloExtractTags(md).join('\n'), links: __hyloExtractLinks(md).join('\n') };
}

function __hyloNoteSigRemember(path, md) {
  if (path) __hyloNoteSig.set(path, __hyloNoteSigFromContent(md));
}

function __hyloNoteSigDiff(path, md) {
  var next = __hyloNoteSigFromContent(md);
  var prev = path ? __hyloNoteSig.get(path) : null;
  if (path) __hyloNoteSig.set(path, next);
  if (!prev) return { tagsChanged: true, linksChanged: true };
  return { tagsChanged: prev.tags !== next.tags, linksChanged: prev.links !== next.links };
}

function __hyloMoveSig(oldPath, newPath) {
  var sig = __hyloNoteSig.get(oldPath);
  if (!sig) return;
  __hyloNoteSig.delete(oldPath);
  __hyloNoteSig.set(newPath, sig);
}

// change.op: write, create, pin, delete, rename, rename-sweep, move, compile, short, images.
function __hyloVaultChanged(change) {
  if (!change || !change.op) return;
  var refetch = false;
  var sidebar = false;
  function drop(pred) { if (__hyloDropWhere(pred)) refetch = true; }
  function dropDerived() {
    // The graph colors nodes by their first tag, so a tag edit reaches it too.
    if (change.tagsChanged) { drop(__hyloIsType('tags')); drop(__hyloIsGraph); }
    if (change.linksChanged) drop(__hyloIsType('knowledge'));
  }
  switch (change.op) {
    case 'write':
      dropDerived();
      break;
    case 'create':
      drop(function (key) { return __hyloSectionListsDir(key, change.dir); });
      dropDerived();
      sidebar = true;
      break;
    case 'pin':
      __hyloSetPinnedBadge(change.path, !!change.pinned);
      drop(__hyloIsType('pinned'));
      break;
    case 'delete':
      if (__hyloForgetNoteInLists(change.path, !!change.pinned)) refetch = true;
      drop(__hyloIsType('tags'));
      drop(__hyloIsKnowledgeDerived);
      sidebar = true;
      break;
    case 'rename':
      __hyloRetargetNoteRows(change.path, change.newPath);
      __hyloMoveSig(change.path, change.newPath);
      drop(__hyloIsGraph);
      sidebar = true;
      break;
    case 'rename-sweep':
      // Wikilink rewrite can change other notes' excerpts and the graph.
      drop(__hyloPreviewSection);
      break;
    case 'move':
      // The dragged card flies out of the shown list on its own; only its count needs fixing.
      if (__hyloSectionListsDir(__hyloSectionShownURL, change.fromDir)) {
        var pane = document.getElementById('home-list-pane');
        __hyloAdjustCount(pane, -1);
        var countEl = pane && pane.querySelector('.home-list-count');
        if (countEl && parseInt(countEl.textContent, 10) === 0) refetch = true;
      }
      __hyloDropParked(function (key) {
        return __hyloSectionListsDir(key, change.toDir) || __hyloSectionListsDir(key, change.fromDir);
      });
      __hyloRetargetNoteRows(change.path, change.newPath);
      __hyloMoveSig(change.path, change.newPath);
      drop(__hyloIsGraph);
      sidebar = true;
      break;
    case 'compile':
      __hyloSetCompiledBadge(change.path);
      drop(__hyloIsType('knowledge'));
      drop(__hyloIsType('tags'));
      sidebar = true;
      break;
    case 'short':
      drop(__hyloIsType('shorts'));
      if (change.hasImages) drop(__hyloIsType('images'));
      break;
    case 'images':
      drop(__hyloIsType('images'));
      break;
    default:
      return;
  }
  if (sidebar) __hyloRefreshSidebar();
  if (refetch) __hyloRefetchShown();
}

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
var __hyloSectionScrollSel = '.home-list-body, .img-scroll, .shorts-stream-wrap, .home-inbox-list';

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
  __hyloSectionWant = key;
  __hyloSectionPark();
  __hyloSectionCache.delete(key);
  __hyloSectionMutate(function () {
    __hyloSectionUnstashIds(entry.holder);
    while (entry.holder.firstChild) pane.appendChild(entry.holder.firstChild);
    entry.holder.remove();
  });
  __hyloSectionShownURL = key;
  __hyloSectionShownAt = entry.loadedAt;
  __hyloSectionShownStale = false;
  var hd = window._homeData;
  if (hd) {
    hd._lastURL = key;
    if (entry.cy) {
      hd.cy = entry.cy;
      __hyloSectionRestoreGraph(entry.cy, entry.zoom, entry.pan);
    } else if (document.getElementById('graph-canvas')) {
      hd.knowledgeIndexPath = __hyloSectionParams(key).get('index') || '';
      hd.loadGraph();
    }
  }
  __hyloUpdateDraggableCards();
  __hyloSectionApplyScroll(entry.scroll);
  if (document.getElementById('tag-cloud')) __hyloRenderTagCloud();
  return true;
}

// Matches internal/inbox.defaultListLimit — the server-side page size used
// when a request omits ?limit=. Kept in sync manually since the client
// needs to know a full page was returned to decide whether more exist.
var INBOX_PAGE_SIZE = 50;

// ── Home Alpine controller ────────────────────────────────────────────────
function homeCtrl() {
  var ctrl = Object.assign(contentPaneCtrl(), {
    activeKey: 'pinned',
    // Each of Knowledge/Memory/Pinned/Folders keeps its own list-vs-grid
    // choice (all folders share the single 'folder' bucket, and all of
    // Knowledge's All/category items share the single 'knowledge' bucket,
    // rather than one per selection) — see listViewKey()/setListView().
    listViewModes: loadListViewModes(),
    foldersOpen: true,
    knowledgeOpen: false,
    chatsOpen: true,
    _lastURL: __hyloInitialSectionURL(),
    lightbox: null,
    selectMode: false,
    selectedCount: 0,
    // ── Graph (mirrors graph.js's graphCtrl state) ─────────────────────────
    loading: false,
    empty: false,
    knowledgeIndexPath: '',
    cy: null,
    _graphTooltip: null,
    _focusedPath: '',
    nodePanel: null,
    graphColorMode: (function () {
      try { return localStorage.getItem('hylo-graph-color') === 'type' ? 'type' : 'cluster'; } catch (_) { return 'cluster'; }
    })(),
    // ── Inbox ────────────────────────────────────────────────────────────
    inboxMessages: [],
    inboxLoading: true,
    inboxLoadingMore: false,
    inboxHasMore: true,
    inboxFilter: 'all',
    inboxSelected: null,
    inboxSheetOpen: false,
    unreadCount: 0,
    // Chat transcript lives on this._chat (chat_kernel.js). These fields are
    // what the sidebar and the run-completion toast still bind to.
    agentBots: [],
    selectedAgentBotId: '',
    isMac: /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent),
    toastText: '',
    toastKind: 'ok',
    toastVisible: false,
    _toastTimer: null,
    // ── Shorts: inline composer (replaces the old short_dialog.js overlay) ──
    shortComposeText: '',
    shortComposeSaving: false,
    // Attachments, uploaded out-of-band of typed text (Weibo/Twitter-style —
    // order in shortComposeText never matters, see saveShortCompose). Each:
    // {id, name, previewUrl (local, for instant feedback), src (server path
    // once uploaded), uploading, failed}.
    shortComposeImages: [],
    init() {
      this.initContentPane();
      window._homeData = this;
      // First render isn't an htmx swap (home.html, not htmx:afterSwap
      // below), so the drag-to-move cards need their initial pass here too.
      __hyloUpdateDraggableCards();
      window.__hyloHotkeys.register('refresh', 'r', doHomeRefresh);
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
      // No longer pushed onto the shared ESC stack — the inbox detail is a
      // docked pane (content_pane.html's inbox-detail-panel), not a floating
      // overlay, so Esc shouldn't dismiss it.
      this.$watch('inboxSheetOpen', (val) => {
        if (!val) return;
        // The note editor and the inbox detail share one pane (see
        // content_pane.html's inbox-detail-panel) — always one or the other.
        if (this.contentPaneOpen) this.contentPaneOpen = false;
        var overlayEl = document.querySelector('.content-pane');
        if (overlayEl) {
          overlayEl.classList.add('content-pane-is-opening');
          setTimeout(function () { overlayEl.classList.remove('content-pane-is-opening'); }, 320);
        }
      });
      // #home-list-pane's flex-grow (and so #graph-canvas's box) changes
      // with this class, but cytoscape caches its container size and never
      // notices — same fixup as clicking "Fit all nodes" by hand, just
      // automatic. Opening applies flex-grow with 0 delay (truly instant,
      // no transition event fires for it), so rAF is enough to let that
      // land. Closing holds flex-grow behind --content-pane-exit-ms (home.css)
      // until the panel's own slide-out finishes — a real, if zero-duration,
      // transition — so wait for its transitionend rather than guessing the
      // delay in JS; a timeout is only the fallback if that event never comes.
      this.$watch('contentPaneOpen', (isOpen) => {
        if (!this.cy) return;
        var cy = this.cy;
        var run = () => {
          if (cy !== this.cy) return;
          cy.resize();
          var node = this._focusedPath ? cy.getElementById(this._focusedPath) : null;
          if (node && !node.empty()) cy.animate({ center: { eles: node } }, { duration: 150 });
          else cy.fit(undefined, 48);
        };
        if (isOpen) { requestAnimationFrame(run); return; }
        var listEl = document.getElementById('home-list-pane');
        if (!listEl) { setTimeout(run, 220); return; }
        var done = false;
        var finish = () => { if (done) return; done = true; listEl.removeEventListener('transitionend', onEnd); run(); };
        var onEnd = (e) => { if (e.target === listEl && e.propertyName === 'flex-grow') finish(); };
        listEl.addEventListener('transitionend', onEnd);
        setTimeout(finish, 260); // covers the rare case flex-grow was already at rest and no transition ran
      });
      window._imgUpdateSelected = () => {
        this.selectedCount = document.querySelectorAll('.img-card.is-selected').length;
      };
      if (window.cytoscapeFcose && window.cytoscape) {
        try { cytoscape.use(cytoscapeFcose); } catch (_) { /* already registered */ }
      }
      this._graphTooltip = document.getElementById('graph-tooltip');
      window.addEventListener('hylo:accent', () => {
        if (!this.cy) return;
        var c = getComputedStyle(document.documentElement).getPropertyValue('--accent').trim();
        if (c) this.cy.style().selector('node:selected').style({ 'border-color': c }).update();
      });
      // The unread badge in the sidebar must stay correct regardless of
      // which section is currently open, so both the initial count and the
      // live SSE subscription start unconditionally here.
      this.refreshUnreadCount();
      this.initInboxStream();
      // Bot list lives in the sidebar, so it loads even when Chats is closed.
      // The kernel keeps a run alive if the pane is swapped away.
      var self = this;
      this._chat = window.__hyloChatCreate({
        onMates: function (mates) { self.agentBots = mates; },
        onMate: function (id) {
          self.selectedAgentBotId = id;
          // Highlight the bot whenever the chat pane is up, including a direct
          // /home?type=chat load whose sidebar key is still the default.
          var inChat = !!document.getElementById('chat-root') ||
            self.activeKey === 'chat' || String(self.activeKey).indexOf('chat:') === 0;
          if (inChat) self.activeKey = id ? ('chat:' + id) : 'chat';
        },
        onToast: function (text, kind) { self.showToast(text, kind); },
      });
      this._chat.start();
      if (document.getElementById('chat-root')) this.initChatSection();
    },
    refresh() { doHomeRefresh(); },
    // Maps the active sidebar selection to its list/grid preference bucket —
    // every folder (activeKey 'dir:...') shares one 'folder' bucket rather
    // than getting one per directory, and likewise every tag drilldown
    // (activeKey 'tags:...') shares one 'tags' bucket rather than one per tag.
    listViewKey() {
      if (this.activeKey.indexOf('dir:') === 0) return 'folder';
      if (this.activeKey.indexOf('knowledge:') === 0) return 'knowledge';
      if (this.activeKey.indexOf('tags:') === 0) return 'tags';
      return this.activeKey;
    },
    currentListView() { return this.listViewModes[this.listViewKey()] || 'list'; },
    setListView(mode) {
      var key = this.listViewKey();
      this.listViewModes[key] = mode;
      try { localStorage.setItem('hylo-list-view', JSON.stringify(this.listViewModes)); } catch (_) { /* ignore */ }
      // Unlike list<->grid (same rows, CSS-only), graph is a different
      // markup shape entirely and has to come from the server.
      if (key === 'knowledge') this._load(this._knowledgeURL());
    },
    _knowledgeURL() {
      var url = '/home/section?type=knowledge&view=' + this.currentListView();
      if (this.knowledgeIndexPath) url += '&index=' + encodeURIComponent(this.knowledgeIndexPath);
      return url;
    },

    // ── Images: select mode + bulk delete (mirrors images.js's imgCtrl) ────
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
      var ok = await window.showConfirm({
        titleHTML: '<span class="confirm-title-icon"><svg width="13" height="13" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10M5 4V3a1 1 0 011-1h2a1 1 0 011 1v1M12 4l-1 8H3L2 4"/><path d="M6 7v3M8 7v3"/></svg></span>Delete ' + n + ' image' + (n > 1 ? 's' : ''),
        message: 'Permanently delete ' + n + ' image' + (n > 1 ? 's' : '') + '? This cannot be undone.',
        confirmLabel: 'Delete',
        danger: true,
      });
      if (!ok) return;
      var results = await Promise.allSettled(cards.map((card) =>
        fetch('/api/images/delete', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ dir: card.dataset.imgDir, name: card.dataset.imgName }),
        }).then((r) => ({ ok: r.ok, card }))
      ));
      var removed = 0;
      results.forEach((r) => {
        if (r.status === 'fulfilled' && r.value.ok) { r.value.card.remove(); removed++; }
      });
      this.exitSelectMode();
      var grid = document.getElementById('img-grid');
      if (grid && !grid.querySelector('.img-card') && !grid.querySelector('.img-sentinel')) {
        var empty = document.createElement('div');
        empty.className = 'img-empty';
        empty.textContent = 'No images found';
        grid.appendChild(empty);
      }
    },
    closeLightbox() { this.lightbox = null; },
    extToType,
    async deleteLightboxImage() {
      var lb = this.lightbox;
      if (!lb || !lb.name || lb.dir == null || lb.dir === undefined) return;
      var linkHint = (lb.notes && lb.notes.length)
        ? (' It is still referenced from ' + lb.notes.length + ' note(s); those embeds will break.')
        : '';
      var ok = await window.showConfirm({
        titleHTML: '<span class="confirm-title-icon"><svg width="13" height="13" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10M5 4V3a1 1 0 011-1h2a1 1 0 011 1v1M12 4l-1 8H3L2 4"/><path d="M6 7v3M8 7v3"/></svg></span>Delete image',
        message: 'Permanently delete "' + lb.name + '" from the vault?' + linkHint,
        confirmLabel: 'Delete',
        danger: true,
      });
      if (!ok) return;
      try {
        var resp = await fetch('/api/images/delete', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ dir: lb.dir, name: lb.name }),
        });
        if (!resp.ok) {
          var msg = (await resp.text()).trim() || 'Delete failed.';
          window.showError(msg, 'Delete failed');
          return;
        }
        var dir = lb.dir, name = lb.name;
        this.closeLightbox();
        var grid = document.getElementById('img-grid');
        if (grid) {
          grid.querySelectorAll('.img-card').forEach(function (c) {
            if (c.dataset.imgDir === dir && c.dataset.imgName === name) c.remove();
          });
          if (!grid.querySelector('.img-card') && !grid.querySelector('.img-sentinel') && !grid.querySelector('.img-empty')) {
            var empty = document.createElement('div');
            empty.className = 'img-empty';
            empty.textContent = 'No images found';
            grid.appendChild(empty);
          }
        }
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
    },

    // ── Graph (mirrors graph.js's graphCtrl methods) ───────────────────────
    async loadGraph() {
      this.loading = true;
      this.empty = false;
      this._focusedPath = '';
      this.nodePanel = null;
      var url = '/api/graph/data';
      if (this.knowledgeIndexPath) url += '?index=' + encodeURIComponent(this.knowledgeIndexPath);
      try {
        var resp = await fetch(url);
        if (!resp.ok) { this.loading = false; return; }
        var data = await resp.json();
        this._renderGraph(data);
      } catch (e) {
        console.error('graph load:', e);
      }
      this.loading = false;
    },

    _applyFocus(node) {
      this._focusedPath = node.data('path');
      this.cy.stop();
      this.cy.elements().unselect();
      node.select();
      this.cy.elements().addClass('faded');
      node.removeClass('faded');
      var connected = node.connectedEdges();
      connected.removeClass('faded');
      connected.connectedNodes().removeClass('faded');
      this.cy.animate({ center: { eles: node } }, { duration: 200 });

      var neighborNodes = connected.connectedNodes().filter(function (n) {
        return n.id() !== node.id();
      });
      var connectedData = [];
      neighborNodes.forEach(function (n) {
        connectedData.push({
          path: n.data('path'),
          label: n.data('label'),
          entityType: n.data('entityType') || '',
        });
      });
      this.nodePanel = {
        path: node.data('path'),
        label: node.data('label'),
        entityType: node.data('entityType') || '',
        edgeCount: connected.length,
        connected: connectedData,
      };
      // Selecting a node now drives the editor directly — the card stays,
      // but "Open" is no longer required to see the note.
      this.openNodeInContentPane(this.nodePanel.path, this.nodePanel.label);
    },

    _clearFocus() {
      this._focusedPath = '';
      if (this.cy) {
        this.cy.stop();
        this.cy.elements().unselect();
        this.cy.elements().removeClass('faded');
      }
      this.nodePanel = null;
    },

    closeNodePanel() { this._clearFocus(); },

    openNodeInContentPane(path, label) {
      var pane = window.__hyloContentPane;
      if (!pane) return;
      pane.openNoteInContentPane(path, label || path, true, false, false, false);
    },

    _hexToRgba(hex, alpha) {
      hex = hex.replace(/^#/, '');
      if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
      var r = parseInt(hex.slice(0, 2), 16);
      var g = parseInt(hex.slice(2, 4), 16);
      var b = parseInt(hex.slice(4, 6), 16);
      return 'rgba(' + r + ',' + g + ',' + b + ',' + alpha + ')';
    },

    _entityTypeColors: {
      'concept': '#facc15', 'person': '#60a5fa', 'product': '#34d399', 'company': '#fb923c',
      'project': '#a78bfa', 'topic': '#f472b6', 'brand': '#f87171', 'business-model': '#6366f1',
      'book': '#38bdf8', 'tool': '#14b8a6', 'framework': '#06b6d4', 'technique': '#a855f7',
      'strategy': '#ef4444', 'protocol': '#22d3ee', 'product-platform': '#34d399', 'startup': '#f59e0b',
      'role': '#84cc16', 'market': '#fdba74', 'opensource-project': '#22c55e', 'service': '#2dd4bf',
      'event': '#f43f5e', 'disease': '#dc2626', 'community': '#60a5fa',
    },

    _tagPaletteColor(tag) {
      if (this._entityTypeColors[tag]) return this._entityTypeColors[tag];
      var palette = ['#60a5fa', '#f472b6', '#a78bfa', '#34d399', '#facc15', '#fb923c', '#22d3ee', '#f87171'];
      var h = 0;
      for (var i = 0; i < tag.length; i++) h = (Math.imul(31, h) + tag.charCodeAt(i)) | 0;
      return palette[Math.abs(h) % palette.length];
    },

    _tagColor(tag) {
      if (!tag) {
        var s = getComputedStyle(document.documentElement);
        return s.getPropertyValue('--bg').trim() || '#1a1a1a';
      }
      return this._hexToRgba(this._tagPaletteColor(tag), 0.30);
    },

    _tagBorder(tag) {
      if (!tag) {
        var s = getComputedStyle(document.documentElement);
        return s.getPropertyValue('--border-strong').trim() || 'rgba(244,244,245,0.09)';
      }
      return this._tagPaletteColor(tag);
    },

    _clusterPalette: ['#60a5fa', '#f472b6', '#34d399', '#fb923c', '#a78bfa', '#facc15', '#22d3ee', '#f87171', '#84cc16', '#e879f9'],

    // Communities past the palette (and singletons) share one neutral color —
    // they're the small fringe, and reusing a hue would imply a relation.
    _nodeColors(entityType, community, communitySize) {
      if (this.graphColorMode === 'type') {
        return { fill: this._tagColor(entityType), stroke: this._tagBorder(entityType) };
      }
      var hex = communitySize > 1 ? this._clusterPalette[community] || '#a1a1aa' : '#a1a1aa';
      return { fill: this._hexToRgba(hex, 0.30), stroke: hex };
    },

    setGraphColorMode(mode) {
      this.graphColorMode = mode;
      try { localStorage.setItem('hylo-graph-color', mode); } catch (_) { /* ignore */ }
      if (!this.cy) return;
      var self = this;
      this.cy.batch(function () {
        self.cy.nodes().forEach(function (n) {
          n.data(self._nodeColors(n.data('entityType'), n.data('community'), n.data('communitySize')));
        });
      });
    },

    _renderGraph(data) {
      var container = document.getElementById('graph-canvas');
      if (!container) return;

      var oldCy = this.cy; this.cy = null; if (oldCy) oldCy.destroy();
      this._focusedPath = '';

      if (!data.nodes || data.nodes.length === 0) {
        this.empty = true;
        return;
      }
      this.empty = false;

      var self = this;

      var css = getComputedStyle(document.documentElement);
      var nodeLabelColor = css.getPropertyValue('--fg').trim() || '#f4f4f5';
      var bgColor = css.getPropertyValue('--bg').trim() || '#0f0f0f';
      var accentColor = css.getPropertyValue('--accent').trim() || '#cc785c';
      var mutedHex = css.getPropertyValue('--muted').trim() || '#71717a';
      var edgeColor = /^#[0-9a-f]{6}$/i.test(mutedHex)
        ? self._hexToRgba(mutedHex, 0.32)
        : 'rgba(113,113,122,0.32)';

      var prep = __hyloGraphPrepare(data.nodes, data.edges || []);
      var hubs = new Set(__hyloGraphLabeledHubs(prep.degree, data.nodes.length));
      var communitySize = {};
      data.nodes.forEach(function (n) {
        var c = prep.community[n.id];
        communitySize[c] = (communitySize[c] || 0) + 1;
      });
      function nodeSize(id) {
        return Math.round(Math.min(64, 16 + Math.log2(prep.degree[id] + 1) * 8));
      }

      var elements = [];
      data.nodes.forEach(function (n) {
        var c = prep.community[n.id];
        var et = n.entity_type || '';
        elements.push({
          data: Object.assign({
            id: n.id, label: n.label, path: n.path,
            entityType: et, tags: n.tags || [],
            degree: prep.degree[n.id], nodeSize: nodeSize(n.id),
            community: c, communitySize: communitySize[c],
          }, self._nodeColors(et, c, communitySize[c])),
          position: prep.positions[n.id],
          classes: hubs.has(n.id) ? 'hub' : '',
        });
      });
      prep.links.forEach(function (l) {
        elements.push({ data: { source: l.source, target: l.target }, classes: l.bidir ? 'bidir' : '' });
      });

      this.cy = cytoscape({
        container: container,
        elements: elements,
        style: [
          {
            selector: 'node',
            style: {
              'background-color': 'data(fill)',
              'border-color': 'data(stroke)',
              'border-width': 1.5,
              'label': 'data(label)',
              'color': nodeLabelColor,
              'font-size': function (ele) {
                return Math.max(10, Math.min(13, 10 + ele.data('degree') * 0.2)) + 'px';
              },
              'font-family': 'Inter,-apple-system,sans-serif',
              'text-valign': 'bottom',
              'text-halign': 'center',
              'text-margin-y': 5,
              'text-max-width': '120px',
              'text-wrap': 'ellipsis',
              'min-zoomed-font-size': 12,
              'text-background-opacity': 0,
              'text-shadow-blur': 6,
              'text-shadow-color': bgColor,
              'text-shadow-opacity': 0.9,
              'text-shadow-offset-x': 0,
              'text-shadow-offset-y': 0,
              'width': 'data(nodeSize)',
              'height': 'data(nodeSize)',
              'z-index': 10,
              'cursor': 'pointer',
            }
          },
          {
            selector: 'node.hub',
            style: {
              'min-zoomed-font-size': 0,
              'font-size': function (ele) { return Math.round(11 + Math.log2(ele.data('degree') + 1)) + 'px'; },
              'font-weight': 600,
              'z-index': 15,
            }
          },
          { selector: 'node:selected', style: { 'border-color': accentColor, 'border-width': 3.5, 'z-index': 20 } },
          { selector: 'node.faded', style: { 'opacity': 0.18 } },
          {
            selector: 'edge',
            style: {
              'width': 1,
              'line-color': edgeColor,
              'target-arrow-color': edgeColor,
              'target-arrow-shape': 'triangle',
              'curve-style': 'straight',
              'arrow-scale': 0.6,
              'z-index': 1,
            }
          },
          { selector: 'edge.bidir', style: { 'target-arrow-shape': 'none' } },
          { selector: 'edge.faded', style: { 'opacity': 0.05 } },
        ],
        layout: { name: 'preset', fit: false },
        wheelSensitivity: 0.3,
        minZoom: 0.05,
        maxZoom: 4,
      });

      var cy = this.cy;
      var isolated = cy.nodes().filter(function (n) { return prep.degree[n.id()] === 0; });
      var connected = cy.elements().not(isolated);
      if (connected.nonempty()) connected.layout(__hyloGraphLayoutOptions(prep, data.nodes.length)).run();
      var bb = connected.nonempty() ? connected.boundingBox() : { x1: 0, y2: 0 };
      isolated.forEach(function (n, i) { n.position({ x: bb.x1 + i * 40, y: bb.y2 + 60 }); });

      var hubNodes = cy.nodes('.hub').sort(function (a, b) { return b.data('degree') - a.data('degree'); });
      var keep = new Set(__hyloGraphPickLabels(hubNodes.map(function (n) {
        var b = n.boundingBox({ includeNodes: false, includeEdges: false, includeLabels: true });
        return { id: n.id(), x1: b.x1, y1: b.y1, x2: b.x2, y2: b.y2 };
      })));
      hubNodes.forEach(function (n) { if (!keep.has(n.id())) n.removeClass('hub'); });
      cy.fit(undefined, 48);

      this.cy.on('tap', 'node', function (evt) {
        var node = evt.target;
        var path = node.data('path');
        if (!path) return;
        if (self._focusedPath === path) {
          self._clearFocus();
          setTimeout(function () { node.unselect(); }, 0);
        } else {
          self._applyFocus(node);
        }
      });

      this.cy.on('tap', function (evt) {
        if (evt.target === self.cy) self._clearFocus();
      });

      var tooltip = this._graphTooltip;
      if (tooltip) {
        this.cy.on('mouseover', 'node', function (evt) {
          tooltip.textContent = evt.target.data('label') || '';
          tooltip.classList.add('visible');
        });
        this.cy.on('mousemove', 'node', function (evt) {
          tooltip.style.left = (evt.originalEvent.clientX + 14) + 'px';
          tooltip.style.top = (evt.originalEvent.clientY - 8) + 'px';
        });
        this.cy.on('mouseout', 'node', function () {
          tooltip.classList.remove('visible');
        });
      }
    },

    zoomIn() {
      if (!this.cy) return;
      var cx = this.cy.width() / 2, cy = this.cy.height() / 2;
      this.cy.zoom({ level: this.cy.zoom() * 1.3, renderedPosition: { x: cx, y: cy } });
    },
    zoomOut() {
      if (!this.cy) return;
      var cx = this.cy.width() / 2, cy = this.cy.height() / 2;
      this.cy.zoom({ level: this.cy.zoom() / 1.3, renderedPosition: { x: cx, y: cy } });
    },
    zoomFit() {
      if (!this.cy) return;
      this.cy.fit(undefined, 48);
    },

    toggleKnowledge() { this.knowledgeOpen = !this.knowledgeOpen; },

    selectKnowledgeIndex(path) {
      this.activeKey = 'knowledge:' + path;
      this.knowledgeIndexPath = path;
      this._load(this._knowledgeURL());
    },

    toggleFolders() { this.foldersOpen = !this.foldersOpen; },

    selectSection(key, url) {
      this.activeKey = key;
      this._load(url);
    },

    selectFolder(path, url) {
      this.activeKey = 'dir:' + path;
      this.foldersOpen = true;
      this._load(url);
    },

    // Tags: switching to another tag from within the filtered note list
    // (the "Switch tag" picker dropdown above it, see homeTagsSectionHTML)
    // calls this directly — same as selectSection/selectFolder, it never
    // has to route back through the tag wall first.
    selectTag(tag) {
      this.activeKey = 'tags:' + tag;
      this._load('/home/section?type=tags&tag=' + encodeURIComponent(tag));
    },

    _load(url) {
      this._lastURL = url;
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
      __hyloSectionFetch(this._lastURL);
    },

    // ── Inbox: message list + unread badge + read-only sheet ───────────────
    initInboxStream() {
      // Subscribes to /api/inbox/notifications (internal/inbox.Bus) so a
      // message created while home is open updates the badge/list live
      // instead of waiting for a manual refresh. Runs regardless of which
      // sidebar section is active since the unread badge is always visible.
      if (typeof EventSource === 'undefined') return;
      var es = new EventSource('/api/inbox/notifications');
      es.addEventListener('message', (e) => {
        var msg;
        try { msg = JSON.parse(e.data); } catch (err) { return; }
        if (this.inboxMessages.some((m) => m.id === msg.id)) return;
        this.inboxMessages.unshift(msg);
        if (!msg.isRead) this.unreadCount++;
      });
    },
    visibleInboxMessages() {
      if (this.inboxFilter === 'all') return this.inboxMessages;
      return this.inboxMessages.filter((m) => this.inboxFilter === 'unread' ? !m.isRead : m.isRead);
    },
    setInboxFilter(f) {
      this.inboxFilter = f;
      // A filter can shrink the rendered list below the container's height,
      // which would never fire a scroll event — pull in more pages until
      // either the list overflows or the server runs dry.
      this.maybeFillInboxList();
    },
    loadInbox() {
      this.inboxLoading = true;
      this.inboxHasMore = true;
      return fetch('/api/inbox').then(function (r) { return r.json(); }).then((data) => {
        this.inboxMessages = data.messages || [];
        this.inboxHasMore = this.inboxMessages.length >= INBOX_PAGE_SIZE;
      }).catch(function () { }).then(() => {
        this.inboxLoading = false;
        this.refreshUnreadCount();
        this.maybeFillInboxList();
      });
    },
    loadMoreInbox() {
      if (this.inboxLoadingMore || !this.inboxHasMore) return Promise.resolve();
      this.inboxLoadingMore = true;
      var offset = this.inboxMessages.length;
      return fetch('/api/inbox?offset=' + offset).then(function (r) { return r.json(); }).then((data) => {
        var page = data.messages || [];
        var seen = new Set(this.inboxMessages.map((m) => m.id));
        page.forEach((m) => { if (!seen.has(m.id)) this.inboxMessages.push(m); });
        this.inboxHasMore = page.length >= INBOX_PAGE_SIZE;
      }).catch(function () { }).then(() => {
        this.inboxLoadingMore = false;
      });
    },
    onInboxListScroll(e) {
      if (this.inboxLoadingMore || !this.inboxHasMore) return;
      var el = e.target;
      if (el.scrollTop + el.clientHeight >= el.scrollHeight - 120) this.loadMoreInbox();
    },
    async maybeFillInboxList() {
      await this.$nextTick();
      var el = document.getElementById('home-inbox-list');
      if (!el || !this.inboxHasMore || this.inboxLoadingMore) return;
      if (el.scrollHeight <= el.clientHeight + 4) {
        await this.loadMoreInbox();
        this.maybeFillInboxList();
      }
    },
    refreshUnreadCount() {
      return fetch('/api/inbox/unread-count').then(function (r) { return r.json(); }).then((data) => {
        this.unreadCount = data.count || 0;
      }).catch(function () { });
    },
    selectInboxMessage(m) {
      this.inboxSelected = m;
      this.inboxSheetOpen = true;
      if (!m.isRead) this.markInboxRead(m);
    },
    closeInboxSheet() {
      this.inboxSheetOpen = false;
    },
    async markInboxRead(m) {
      m.isRead = true;
      this.unreadCount = Math.max(0, this.unreadCount - 1);
      try { await fetch('/api/inbox/' + m.id + '/read', { method: 'POST' }); } catch (e) { /* ignore */ }
    },
    async markAllInboxRead() {
      this.inboxMessages.forEach((m) => { m.isRead = true; });
      this.unreadCount = 0;
      try { await fetch('/api/inbox/read-all', { method: 'POST' }); } catch (e) { /* ignore */ }
    },
    relTime(iso) {
      const d = new Date(iso);
      const diffSec = Math.floor((Date.now() - d.getTime()) / 1000);
      if (diffSec < 60) return 'now';
      if (diffSec < 3600) return Math.floor(diffSec / 60) + 'm';
      if (diffSec < 86400) return Math.floor(diffSec / 3600) + 'h';
      return Math.floor(diffSec / 86400) + 'd';
    },
    fullTime(iso) {
      try { return new Date(iso).toLocaleString(); } catch (e) { return iso; }
    },
    renderMarkdown(text) {
      return window.__hyloRenderMarkdown(text);
    },

    autoResize(el) { window.__hyloAutoResize(el); },

    toggleChats() { this.chatsOpen = !this.chatsOpen; },

    selectChatAgentBot(agentBotId) {
      if (!this._chat) return;
      var opened = this._chat.tryOpen(agentBotId);
      if (opened) this.activeKey = 'chat:' + agentBotId;
      else if (this.selectedAgentBotId) this.activeKey = 'chat:' + this.selectedAgentBotId;
      if (document.getElementById('chat-root')) this._lastURL = '/home/section?type=chat';
      else this._load('/home/section?type=chat');
    },

    initChatSection() {
      if (!this._chat) return;
      var root = document.getElementById('chat-root');
      if (root) this._chat.attach(root);
      if ((this.activeKey === 'chat' || String(this.activeKey).indexOf('chat:') === 0) && this.selectedAgentBotId) {
        this.activeKey = 'chat:' + this.selectedAgentBotId;
      }
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
      var opened = await this._chat.openFreshChat(mateId);
      if (opened) this.activeKey = 'chat:' + mateId;
      else if (this.selectedAgentBotId) this.activeKey = 'chat:' + this.selectedAgentBotId;
      if (document.getElementById('chat-root')) this._lastURL = '/home/section?type=chat';
      else this._load('/home/section?type=chat');
      this._chat.insertWikilink(stem);
    },

    // ── Shorts: inline composer ─────────────────────────────────────────
    handleShortComposeKeydown(e) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault();
        void this.saveShortCompose();
      }
    },
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
    },

    // Generic version of the above — same .run-toast element (home.go), any
    // caller (e.g. home.js's drag-to-move flow) can use it, not just agent
    // run completions.
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

// Every #home-list-pane request passes through here. Ones issued by
// __hyloSectionFetch (source = the pane) that were superseded while queued
// are dropped. Declarative ones (the shorts month
// buttons' hx-get) get the same cache restore _load does.
document.body.addEventListener('htmx:beforeRequest', function (e) {
  var d = e.detail;
  if (!d || !d.target || d.target.id !== 'home-list-pane') return;
  var key = __hyloSectionKey(d.pathInfo && (d.pathInfo.finalRequestPath || d.pathInfo.requestPath));
  if (e.target === d.target) {
    if (key !== __hyloSectionWant) { e.preventDefault(); return; }
  } else if (__hyloSectionTryRestore(key)) {
    e.preventDefault();
    return;
  } else {
    __hyloSectionWant = key;
  }
  if (d.xhr) d.xhr.__hyloSectionKey = key;
});

document.body.addEventListener('htmx:beforeSwap', function (e) {
  var d = e.detail;
  if (!d || !d.target || d.target.id !== 'home-list-pane') return;
  var xhr = d.xhr;
  if (xhr && xhr.__hyloSectionKey != null && xhr.__hyloSectionKey !== __hyloSectionWant) {
    d.shouldSwap = false;
    return;
  }
  // Parking waits for a response that will swap, so the old section stays
  // on screen while loading and survives a failed request.
  if (!d.shouldSwap) return;
  // A refetch of the shown section replaces it; anything else parks it.
  if (xhr && xhr.__hyloSectionKey && xhr.__hyloSectionKey !== __hyloSectionShownURL) __hyloSectionPark();
  else __hyloSectionTeardown(d.target);
});

document.body.addEventListener('htmx:afterSwap', function (e) {
  var target = e.detail && e.detail.target;
  if (!target || target.id !== 'home-list-pane' || !window._homeData) return;
  var xhr = e.detail.xhr;
  __hyloUpdateDraggableCards();
  if (!xhr || !xhr.responseURL) return;
  try {
    var u = new URL(xhr.responseURL);
    window._homeData._lastURL = u.pathname + u.search;
    __hyloSectionShownURL = __hyloSectionKey(xhr.responseURL);
    __hyloSectionShownAt = Date.now();
    __hyloSectionShownStale = false;
    // Graph, inbox, and chat markup is static; the client fetch starts here
    // because Alpine doesn't re-run init() on an htmx swap. A cache restore
    // never reaches this — those sections come back already initialized.
    if (document.getElementById('graph-canvas')) {
      window._homeData.knowledgeIndexPath = u.searchParams.get('index') || '';
      window._homeData.loadGraph();
    }
    if (document.getElementById('home-inbox-list')) {
      window._homeData.loadInbox();
    }
    if (document.getElementById('chat-root')) {
      window._homeData.initChatSection();
    }
    if (document.getElementById('tag-cloud')) {
      __hyloRenderTagCloud();
    }
  } catch (err) { /* ignore */ }
});

// ── Tags wall: word-cloud layout via d3-cloud (vendor/d3-cloud.min.js) ────
// Packed tightly like a classic word cloud — plain text sized by tag
// frequency, not the pill/count-badge treatment used elsewhere in this app,
// since d3-cloud's interlocking placement is computed from each word's own
// glyph shape (rendered to a hidden canvas to measure it) and stops being
// tight the moment you wrap that measured shape in a padded, bordered pill.
// d3-cloud silently drops words that don't fit inside the box it's given
// (no wrapping/scrolling within the layout itself), so the box height is
// estimated from total glyph area rather than a fixed guess.
//
// Re-laid-out on panel resize (split-pane drag, reading-pane toggle, window
// resize) via a ResizeObserver on #home-list-body — never on #tag-cloud
// itself, since this function sets #tag-cloud's own height as part of every
// run, which would just retrigger the observer on itself. __hyloTagCloudGen
// guards against a slow, still-running layout (d3-cloud places words in
// timed async batches) finishing after a newer resize already started a
// fresh one and clearing the container first.
var __hyloTagCloudObserver = null;
var __hyloTagCloudResizeTimer = null;
var __hyloTagCloudGen = 0;

function __hyloRenderTagCloud() {
  var el = document.getElementById('tag-cloud');
  if (!el || typeof d3 === 'undefined' || !d3.layout || !d3.layout.cloud) return;
  var tags;
  try { tags = JSON.parse(el.dataset.tags || '[]'); } catch (e) { return; }
  if (!tags.length) return;

  var maxCount = 0;
  for (var i = 0; i < tags.length; i++) { if (tags[i].count > maxCount) maxCount = tags[i].count; }
  var MIN_SIZE = 14, MAX_SIZE = 56;
  var words = tags.map(function (t) {
    var ratio = maxCount > 0 ? t.count / maxCount : 0;
    var size = MIN_SIZE + (MAX_SIZE - MIN_SIZE) * Math.sqrt(ratio);
    return {
      text: t.name,
      count: t.count,
      size: size,
      weight: size > 34 ? '600' : (size > 20 ? '500' : '400')
    };
  });
  var fontFamily = (getComputedStyle(document.documentElement).getPropertyValue('--font-ui') || 'sans-serif').trim();

  function layoutAndRender() {
    var gen = ++__hyloTagCloudGen;
    var width = el.clientWidth || 800;
    el.dataset.layoutWidth = String(width);
    // Rough glyph-area estimate (avg glyph ~0.6x its font-size square) to
    // size the box. The multiplier controls the cloud's silhouette, not
    // just how much empty margin there is: d3-cloud's spiral starts at the
    // box center and grows outward until it finds room for each word, so
    // slack this close to the true minimum forces later words out toward
    // the box's corners — the filled shape reads as roughly rectangular.
    // More headroom lets everything settle near the center before that
    // happens, so the outline stays a rounded/elliptical blob instead.
    var totalArea = 0;
    for (var j = 0; j < words.length; j++) {
      totalArea += words[j].size * words[j].size * words[j].text.length * 0.6;
    }
    var height = Math.max(420, Math.ceil((totalArea * 2.9) / width));
    el.style.height = height + 'px';
    el.innerHTML = '';

    d3.layout.cloud()
      .size([width, height])
      .words(words)
      .padding(5)
      .rotate(0)
      .font(fontFamily)
      .fontWeight(function (d) { return d.weight; })
      .fontSize(function (d) { return d.size; })
      .on('end', function (placed) {
        if (gen !== __hyloTagCloudGen) return; // a newer resize already re-ran this
        placed.forEach(function (d) {
          var btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'tag-cloud-word';
          btn.textContent = d.text;
          btn.title = d.text + ' · ' + d.count;
          btn.style.fontSize = d.size + 'px';
          btn.style.fontWeight = d.weight;
          btn.style.left = (width / 2 + d.x) + 'px';
          btn.style.top = (height / 2 + d.y) + 'px';
          btn.addEventListener('click', function () {
            if (window._homeData) window._homeData.selectTag(d.text);
          });
          el.appendChild(btn);
        });
      })
      .start();
  }

  // d3-cloud measures each word's collision box by rendering it to a hidden
  // canvas at the resolved font — but fonts.css loads Inter with
  // font-display: swap, so a layout run before it's actually downloaded
  // measures against whatever fallback font the browser is showing that
  // instant. The real DOM text swaps to Inter once it lands, wider or
  // narrower than the fallback, and the collision boxes computed earlier no
  // longer match — small words showing that mismatch first since a couple
  // px of error is proportionally much bigger at 14px than at 56px.
  // document.fonts.ready resolves once every requested font has settled, so
  // waiting on it here keeps what's measured and what's rendered in sync.
  function watch() {
    if (__hyloTagCloudObserver) { __hyloTagCloudObserver.disconnect(); __hyloTagCloudObserver = null; }
    var body = el.parentElement;
    if (!body || typeof ResizeObserver === 'undefined') return;
    var lastWidth = body.clientWidth;
    __hyloTagCloudObserver = new ResizeObserver(function () {
      var w = body.clientWidth;
      if (Math.abs(w - lastWidth) < 4) return; // sub-pixel/height-only noise
      lastWidth = w;
      clearTimeout(__hyloTagCloudResizeTimer);
      __hyloTagCloudResizeTimer = setTimeout(layoutAndRender, 150);
    });
    __hyloTagCloudObserver.observe(body);
  }

  // Restoring a parked tag wall already has its words. Re-hook resize only,
  // so coming back doesn't wipe the cloud and jump the scroll position —
  // unless the width changed while it was parked.
  var laidOut = Number(el.dataset.layoutWidth);
  if (el.querySelector('.tag-cloud-word') && Math.abs((el.clientWidth || 800) - laidOut) < 4) { watch(); return; }

  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(layoutAndRender).catch(layoutAndRender);
  } else {
    layoutAndRender();
  }
  watch();
}

// ── Shorts entries embedded in the list pane: intercept internal links ────
// Wikilinks (/notes?…) → open in the content pane; external → new tab.
// (Mirrors shorts.js's listener for the standalone /shorts page.)
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

// ── Drag a note card onto a sidebar folder to move it ──────────────────────
// Folder-view only (activeKey 'dir:...') — Knowledge/Shorts have their own
// list views and no drop path here, by design (backend Vault.MoveNote has no
// such restriction; this is a UI-only choice). Cards are htmx-swapped in/out
// of #home-list-pane on every section/page change, so this is delegated on
// document rather than bound per-card — a per-card listener would silently
// stop working the moment htmx replaces the card it was attached to.
var __hyloDragSourcePath = null;
var __hyloDragSourceCard = null;
var __hyloDragOverTarget = null;

function __hyloDragNoteEligible(card) {
  return !!(card && card.dataset.notePath &&
    card.dataset.noteIsKnowledge !== 'true' && card.dataset.noteIsIndex !== 'true');
}

// Reflects the current section onto every visible card's draggable property
// (not just gating it at dragstart) so the cursor/affordance is honest —
// hovering a card outside folder view no longer shows a grab cursor for a
// drag that was never going to start. Native `draggable` is a reflected
// attribute, so home.css's [draggable="true"] selectors track this for free.
// Called after every #home-list-pane swap (htmx:afterSwap, below) and once
// on init for the page's first, non-htmx render.
function __hyloUpdateDraggableCards() {
  var pane = document.getElementById('home-list-pane');
  if (!pane) return;
  var isFolderView = !!(window._homeData && String(window._homeData.activeKey || '').indexOf('dir:') === 0);
  var cards = pane.querySelectorAll('.home-note-row');
  for (var i = 0; i < cards.length; i++) {
    cards[i].draggable = isFolderView && __hyloDragNoteEligible(cards[i]);
  }
}

document.addEventListener('dragstart', function (e) {
  var card = e.target.closest ? e.target.closest('.home-note-row') : null;
  if (!card) return;
  // A card mid-move (is-move-pending) is also pointer-events:none (home.css),
  // so it can't be the drag source here — this check is defense in depth.
  if (!card.draggable || card.classList.contains('is-move-pending')) { e.preventDefault(); return; }
  __hyloDragSourcePath = card.dataset.notePath;
  __hyloDragSourceCard = card;
  e.dataTransfer.effectAllowed = 'move';
  try { e.dataTransfer.setData('text/plain', __hyloDragSourcePath); } catch (_) { /* ignore */ }
  card.classList.add('is-dragging');
  // Without this, the drag image defaults to a full-size snapshot of the
  // card (as wide as the whole list) — swap in a small title-only chip that
  // sizes to its own text instead. Has to be in the DOM (even off-screen)
  // for setDragImage to snapshot it, then comes back out next tick.
  var ghost = document.createElement('div');
  ghost.className = 'home-drag-ghost';
  ghost.textContent = card.dataset.noteTitle || 'Note';
  document.body.appendChild(ghost);
  e.dataTransfer.setDragImage(ghost, 12, 14);
  setTimeout(function () { ghost.remove(); }, 0);
});

document.addEventListener('dragend', function () {
  // Only clears the *gesture* state — if drop actually kicked off a move,
  // the card keeps is-move-pending (added there) until the request settles,
  // so a second drag can't fire a second move on the same file mid-flight.
  var card = __hyloDragSourceCard;
  if (card && !card.classList.contains('is-move-pending')) card.classList.remove('is-dragging');
  if (__hyloDragOverTarget) { __hyloDragOverTarget.classList.remove('drag-over'); __hyloDragOverTarget = null; }
  __hyloDragSourcePath = null;
  __hyloDragSourceCard = null;
});

// Per spec, both dragenter and dragover need preventDefault() for an element
// to register as a valid drop target — dragover alone is enough in most
// browsers but not guaranteed, so both get the same handling.
document.addEventListener('dragenter', function (e) {
  if (!__hyloDragSourcePath) return;
  var target = e.target.closest ? e.target.closest('.home-drop-target') : null;
  if (target) e.preventDefault();
});

document.addEventListener('dragover', function (e) {
  if (!__hyloDragSourcePath) return;
  var target = e.target.closest ? e.target.closest('.home-drop-target') : null;
  if (target !== __hyloDragOverTarget) {
    if (__hyloDragOverTarget) __hyloDragOverTarget.classList.remove('drag-over');
    if (target) target.classList.add('drag-over');
    __hyloDragOverTarget = target;
  }
  if (target) { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; }
});

document.addEventListener('drop', function (e) {
  if (!__hyloDragSourcePath) return;
  // Always suppress the browser's own drop handling while our drag is live —
  // dropping outside a valid target (e.g. onto the open editor) would
  // otherwise insert the dragged path as plain text into it.
  e.preventDefault();
  var target = e.target.closest ? e.target.closest('.home-drop-target') : null;
  var sourcePath = __hyloDragSourcePath;
  var card = __hyloDragSourceCard;
  var hoverTarget = __hyloDragOverTarget;
  __hyloDragOverTarget = null;
  __hyloDragSourcePath = null;
  if (!target) { if (hoverTarget) hoverTarget.classList.remove('drag-over'); __hyloShakeCard(card, false); return; }
  var targetDir = target.dataset.dropDir;
  var slash = sourcePath.lastIndexOf('/');
  var currentDir = slash <= 0 ? '/' : sourcePath.slice(0, slash);
  if (currentDir === targetDir) { target.classList.remove('drag-over'); __hyloShakeCard(card, false); return; }
  if (card) { card.classList.remove('is-dragging'); card.classList.add('is-move-pending'); }
  // .drag-over stays through the request itself (its breathing glow doubles
  // as "still working on it") — __hyloDragMoveNote swaps it for a receipt
  // bounce on success or clears it on failure.
  void __hyloDragMoveNote(sourcePath, targetDir, card, target);
});

// Plain "that didn't do anything" feedback — dropped outside any folder, or
// back into the folder the note is already in. rejected=false, no color.
function __hyloShakeCard(card, rejected) {
  if (!card) return;
  card.classList.remove('is-dragging', 'is-move-pending');
  var cls = rejected ? 'is-move-rejected' : 'is-move-noop';
  card.classList.add(cls);
  // 500ms comfortably outlasts both animations home.css puts on these
  // classes (home-shake: --motion-base*2.5 = 400ms; the rejected variant's
  // extra home-reject-flash: --motion-base*3 = 480ms).
  setTimeout(function () { card.classList.remove(cls); }, 500);
}

// Move actually landed — flies the card toward the drop target and shrinks
// it away, then removes it from the DOM itself rather than waiting for
// reloadActiveSection()'s round trip to do it. Also gives the target a quick
// receipt bounce. Safe to no-op if either element isn't in the document any
// more (e.g. the section changed mid-request).
function __hyloFlyCardToTarget(card, targetEl) {
  if (targetEl && targetEl.isConnected) {
    targetEl.classList.remove('drag-over');
    targetEl.classList.add('just-received');
    // home-drop-received runs --motion-base*2 = 320ms — 340ms clears a beat after.
    setTimeout(function () { targetEl.classList.remove('just-received'); }, 340);
  }
  if (!card || !card.isConnected) return;
  card.classList.remove('is-move-pending');
  if (!targetEl || !targetEl.isConnected) { card.remove(); return; }
  var cardRect = card.getBoundingClientRect();
  var targetRect = targetEl.getBoundingClientRect();
  card.style.setProperty('--fly-dx', ((targetRect.left + targetRect.width / 2) - (cardRect.left + cardRect.width / 2)) + 'px');
  card.style.setProperty('--fly-dy', ((targetRect.top + targetRect.height / 2) - (cardRect.top + cardRect.height / 2)) + 'px');
  card.classList.add('is-move-success');
  var done = false;
  var remove = function () { if (done) return; done = true; card.remove(); };
  card.addEventListener('transitionend', remove, { once: true });
  // .is-move-success's longer transition (transform) runs --motion-base*2 =
  // 320ms — 400ms is the fallback in case transitionend never fires.
  setTimeout(remove, 400);
}

async function __hyloDragMoveNote(sourcePath, targetDir, card, targetEl) {
  if (window.__hyloContentPane) await window.__hyloContentPane.flushPendingSaveFor(sourcePath);
  var resp;
  try {
    resp = await fetch('/api/vault/move', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: sourcePath, newDir: targetDir }),
    });
  } catch (e) {
    if (targetEl) targetEl.classList.remove('drag-over');
    if (card) { card.classList.remove('is-move-pending'); __hyloShakeCard(card, true); }
    if (window.showError) window.showError((e && e.message) || 'Network error.', 'Cannot move');
    return;
  }
  if (!resp.ok) {
    if (targetEl) targetEl.classList.remove('drag-over');
    if (card) { card.classList.remove('is-move-pending'); __hyloShakeCard(card, true); }
    var msg = (await resp.text()).trim() || 'Move failed.';
    if (window.showError) window.showError(resp.status === 409 ? 'A note already exists at that location.' : msg, 'Cannot move');
    return;
  }
  var data = await resp.json();
  if (window.__hyloContentPane) window.__hyloContentPane.noteMoved(sourcePath, data.path);
  __hyloFlyCardToTarget(card, targetEl);
  if (window.__hyloVaultChanged) window.__hyloVaultChanged({
    op: 'move', path: sourcePath, newPath: data.path, fromDir: __hyloNoteDir(sourcePath), toDir: targetDir,
  });
}

