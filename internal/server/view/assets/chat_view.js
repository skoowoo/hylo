// Chat pane. Mounted while #chat-root is in the document and discarded on
// HTMX swap. Each segment kind owns its signature, mount, and patch so a
// streaming token only rewrites the tail. Open and copied live here, not on
// the message.
(function (root) {
  var R = root.__vaultrChatReduce;
  var openSegs = new WeakMap();
  var copiedUntil = new Map();
  var copiedListener = null;
  var hydrateListener = null;

  function isOpen(seg) { return !!openSegs.get(seg); }

  function toggleOpen(seg) {
    var on = !openSegs.get(seg);
    openSegs.set(seg, on);
    return on;
  }

  function copiedNow(id) {
    var exp = copiedUntil.get(id) || 0;
    if (exp > Date.now()) return true;
    if (exp) copiedUntil.delete(id);
    return false;
  }

  function rememberCopy(id) {
    var exp = Date.now() + 2000;
    copiedUntil.set(id, exp);
    setTimeout(function () {
      if (copiedUntil.get(id) !== exp) return;
      copiedUntil.delete(id);
      if (copiedListener) copiedListener(id, false);
    }, 2000);
  }

  function copyReply(msg, btn) {
    var text = (msg.segments || [])
      .filter(function (s) { return s.type === 'text'; })
      .map(function (s) { return s.content || ''; })
      .join('').trim();
    if (!text || !navigator.clipboard) return;
    navigator.clipboard.writeText(text).then(function () {
      rememberCopy(msg.id);
      setCopyIcon(btn, true);
    }).catch(function () { /* clipboard denied */ });
  }

  function normalizeVaultDir(raw) {
    var s = (raw || '').replace(/\/+/g, '/');
    if (!s.startsWith('/')) s = '/' + s;
    if (s.length > 1 && s.endsWith('/')) s = s.slice(0, -1);
    return s || '/';
  }

  function chatAcParseCtx(val, caret) {
    val = typeof val === 'string' ? val : '';
    if (caret == null || caret > val.length) caret = val.length;
    var left = val.slice(0, caret);
    var tokenStart = 0;
    for (var i = left.length - 1; i >= 0; i--) {
      if (/[\s]/.test(left[i])) { tokenStart = i + 1; break; }
    }
    var token = left.slice(tokenStart);
    var atIdx = token.indexOf('@');
    if (atIdx === -1) return null;
    if (atIdx > 0 && token[atIdx - 1] !== '/') return null;
    var atPos = tokenStart + atIdx;
    var pathPrefix = token.slice(0, atIdx);
    var body = token.slice(atIdx + 1);
    if (body.indexOf(']') !== -1) return null;
    var lastSlash = body.lastIndexOf('/');
    if (lastSlash === -1) return { kind: 'mention', partial: body, replaceStart: atPos };
    var partial = body.slice(lastSlash + 1);
    if (partial.indexOf('.') !== -1) return null;
    var dirSeg = body.slice(0, lastSlash);
    var dirPath = normalizeVaultDir((pathPrefix || '') + '/' + dirSeg);
    return {
      kind: 'path', dirPath: dirPath, partial: partial,
      tokenStart: tokenStart, atPos: atPos, replaceStart: tokenStart + atIdx + lastSlash + 2,
    };
  }

  async function chatMentionSearch(query, signal) {
    var resp = await fetch('/api/search', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ q: query, type: 'name', limit: 30 }), signal: signal,
    });
    if (!resp.ok) return [];
    var data = await resp.json();
    var results = Array.isArray(data.results) ? data.results : [];
    return results.map(function (r) {
      var stem = (typeof r.name === 'string' ? r.name : '').replace(/\.md$/i, '');
      return { label: stem, value: stem };
    });
  }

  // Vault-relative note path -> filename stem (no directory), for turning a
  // tab's path into the same "[[stem]]" value chatMentionSearch produces.
  // Folder info is left out of the list on purpose — with it, rows carried
  // two lines each and the default/search list read as too sparse.
  function noteStem(path) {
    var p = String(path || '').replace(/^\/+/, '');
    var slash = p.lastIndexOf('/');
    var name = slash === -1 ? p : p.slice(slash + 1);
    return name.replace(/\.md$/i, '');
  }

  // Default "@" list: every open editor tab, active one pinned first —
  // referencing whatever note you're already looking at is the single most
  // common case, so it lands on the row the picker pre-selects (item 0),
  // no extra badge or color needed to make the point. Tabs without a
  // materialized path (a brand-new, never-saved tab) can't become a
  // "[[stem]]" wikilink, so they're skipped.
  function chatMentionDefaultItems() {
    var pane = window.__vaultrContentPane;
    var tabs = pane && Array.isArray(pane.tabs) ? pane.tabs : [];
    var activeIdx = pane ? pane.activeTab : -1;
    var withPath = [];
    tabs.forEach(function (t, i) { if (t && t.path) withPath.push({ tab: t, active: i === activeIdx }); });
    withPath.sort(function (a, b) { return (b.active ? 1 : 0) - (a.active ? 1 : 0); });
    return withPath.map(function (x) {
      var stem = noteStem(x.tab.path);
      return { label: x.tab.title || stem, value: stem };
    });
  }

  function el(tag, cls) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    return n;
  }

  function svg(html) {
    var h = document.createElement('span');
    h.innerHTML = html.trim();
    return h.firstChild;
  }

  var CHEVRON = '<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 18l6-6-6-6"/></svg>';
  var COPY_SVG = '<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path stroke-linecap="round" stroke-linejoin="round" d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>';
  var CHECK_SVG = '<svg fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M20 6L9 17l-5-5"/></svg>';
  var RETRY_SVG = '<svg fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 1 1-3-6.7"/><path stroke-linecap="round" stroke-linejoin="round" d="M21 3v6h-6"/></svg>';

  // Batch-fetches title/preview/cover for note-touched cards, keyed by vault
  // path. Shared across every note_access segment on the page so re-mounting
  // (patch always forces a remount here) doesn't refetch what's cached.
  var noteInfoCache = Object.create(null); // path -> row | null (unknown/failed)

  function fetchNoteInfo(paths) {
    var need = paths.filter(function (p) { return p && !(p in noteInfoCache); });
    if (!need.length) return Promise.resolve();
    return fetch('/api/notes/info', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths: need }),
    }).then(function (resp) { return resp.ok ? resp.json() : { notes: [] }; })
      .then(function (data) {
        var got = Object.create(null);
        (data.notes || []).forEach(function (n) { noteInfoCache[n.path] = n; got[n.path] = true; });
        need.forEach(function (p) { if (!got[p]) noteInfoCache[p] = null; });
      })
      .catch(function () { need.forEach(function (p) { noteInfoCache[p] = null; }); });
  }

  // One card per touched note, built from the same markup/classes as the
  // real note grid (home.css/.home-list-body.is-grid) so it looks and
  // behaves identically — borderless surface, title + preview + cover.
  // Starts from the path alone (filename as title) and hydrates with the
  // note's real title/preview/cover once fetchNoteInfo resolves.
  function noteAccessCard(item) {
    var path = item.path || '';
    var base = path.split('/').pop() || path;
    var fallbackTitle = base.replace(/\.md$/i, '');
    var isWrite = item.action === 'write';

    var card = el('div', 'list-card list-card--clickable home-list-card home-note-row seg-note-access-card');
    card.title = path;

    var cover = el('div', 'home-note-cover');
    var coverImg = document.createElement('img');
    coverImg.alt = ''; coverImg.loading = 'lazy'; coverImg.decoding = 'async';
    cover.appendChild(coverImg);
    show(cover, false);
    card.appendChild(cover);

    var top = el('div', 'home-note-row-top');
    var titleblock = el('div', 'home-note-row-titleblock');
    var titlerow = el('div', 'home-note-row-titlerow');
    var titleEl = el('span', 'home-note-row-title');
    titleEl.textContent = fallbackTitle;
    titlerow.appendChild(titleEl);
    titleblock.appendChild(titlerow);
    var previewEl = el('span', 'home-note-row-preview-grid');
    show(previewEl, false);
    titleblock.appendChild(previewEl);
    top.appendChild(titleblock);

    var badges = el('div', 'home-note-row-badges');
    var actionBadge = el('span', 'badge badge--sm' + (isWrite ? ' badge--accent' : ''));
    actionBadge.textContent = isWrite ? 'write' : 'read';
    badges.appendChild(actionBadge);
    top.appendChild(badges);
    card.appendChild(top);

    var meta = el('div', 'home-note-row-meta');
    var timeEl = el('span', 'home-note-row-time');
    show(timeEl, false);
    meta.appendChild(timeEl);
    card.appendChild(meta);

    card.addEventListener('click', function () {
      if (window.__vaultrContentPane) {
        void window.__vaultrContentPane.openNoteInContentPane(path, titleEl.textContent || fallbackTitle, false, false);
      }
    });

    function hydrate() {
      var info = noteInfoCache[path];
      if (!info) return;
      if (info.title) titleEl.textContent = info.title.replace(/\.md$/i, '');
      if (info.preview) { previewEl.textContent = info.preview; show(previewEl, true); }
      if (info.updatedAt) { timeEl.textContent = R.formatTime(info.updatedAt); show(timeEl, true); }
      if (info.cover) {
        coverImg.src = info.cover;
        show(cover, true);
        card.classList.add('has-cover');
      }
    }

    return { node: card, hydrate: hydrate };
  }

  // Adds left/right arrow buttons over a horizontally-scrolling row (the
  // scrollbar itself is hidden — see .seg-note-access-row in agent_chat.css)
  // and keeps them in sync with scroll position: hidden entirely when
  // everything already fits, each one disabled at its end of the track.
  function wireHorizontalScroller(scroller, row) {
    var prev = el('button', 'icon-btn seg-note-access-arrow seg-note-access-arrow--prev');
    prev.type = 'button';
    prev.setAttribute('aria-label', 'Scroll left');
    prev.appendChild(svg(CHEVRON));
    var next = el('button', 'icon-btn seg-note-access-arrow seg-note-access-arrow--next');
    next.type = 'button';
    next.setAttribute('aria-label', 'Scroll right');
    next.appendChild(svg(CHEVRON));
    scroller.appendChild(prev);
    scroller.appendChild(next);

    function update() {
      var max = row.scrollWidth - row.clientWidth;
      var fits = max <= 1;
      show(prev, !fits && row.scrollLeft > 0);
      show(next, !fits && row.scrollLeft < max - 1);
    }
    function page(dir) {
      row.scrollBy({ left: dir * Math.max(row.clientWidth * 0.8, 220), behavior: 'smooth' });
    }
    prev.addEventListener('click', function () { page(-1); });
    next.addEventListener('click', function () { page(1); });
    row.addEventListener('scroll', update, { passive: true });
    // mount() runs before the caller inserts this subtree into the document
    // (see mountSegments), so scrollWidth/clientWidth would still read 0 if
    // checked synchronously here — defer past that insertion's layout pass.
    requestAnimationFrame(update);
  }

  function setCopyIcon(btn, on) {
    if (!btn) return;
    btn.classList.toggle('copied', on);
    while (btn.firstChild) btn.removeChild(btn.firstChild);
    btn.appendChild(svg(on ? CHECK_SVG : COPY_SVG));
  }

  function initials(name) {
    if (!name) return '?';
    return name.trim().slice(0, 1).toUpperCase();
  }

  function show(node, on) {
    if (!node) return;
    node.style.display = on ? '' : 'none';
  }

  function hasText(segs) {
    for (var i = 0; i < segs.length; i++) {
      if (segs[i].type === 'text' && segs[i].content) return true;
    }
    return false;
  }

  function hasStatus(segs) {
    for (var i = 0; i < segs.length; i++) if (segs[i].type === 'status') return true;
    return false;
  }

  function setProse(node, text, streaming) {
    if (node._raw === text) return;
    node._raw = text;
    node.innerHTML = root.__vaultrRenderMarkdown(text, { cache: !streaming });
  }

  function setText(node, text) {
    if (!node || node._raw === text) return;
    node._raw = text;
    node.textContent = text;
  }

  function segCtx(msg) {
    var segs = msg.segments || [];
    var lastThink = -1;
    for (var i = 0; i < segs.length; i++) if (segs[i].type === 'thinking') lastThink = i;
    return { segs: segs, lastThink: lastThink, running: msg.status === 'running' };
  }

  var kinds = {
    text: {
      sig: function () { return ['t']; },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var node = el('div', 'prose');
        setProse(node, seg.content || '', ctx.running);
        rec.node = node;
        slot.appendChild(node);
      },
      patch: function (rec, seg, ctx) {
        if (!rec.node) return false;
        setProse(rec.node, seg.content || '', ctx.running);
        return true;
      },
    },
    thinking: {
      sig: function (seg, ctx, i) { return ['h', (i === ctx.lastThink && ctx.running) ? 1 : 0]; },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var streamingLast = ctx.running && i === ctx.lastThink;
        var open = isOpen(seg);
        var wrap = el('div', 'seg-thinking hairline');
        var btn = el('button', 'seg-thinking-toggle' + (open ? ' open' : '') + (streamingLast ? ' is-streaming' : ''));
        btn.type = 'button';
        btn.appendChild(svg(CHEVRON));
        btn.appendChild(document.createTextNode('Thinking'));
        var body = el('div', 'seg-thinking-content');
        setText(body, seg.content || '');
        show(body, open);
        btn.addEventListener('click', function () {
          var on = toggleOpen(seg);
          btn.classList.toggle('open', on);
          show(body, on);
        });
        wrap.appendChild(btn);
        wrap.appendChild(body);
        rec.body = body;
        slot.appendChild(wrap);
      },
      patch: function (rec, seg) {
        if (!rec.body) return false;
        setText(rec.body, seg.content || '');
        return true;
      },
    },
    tool_use: {
      sig: function (seg, ctx) {
        return ['u', seg.name || '', seg.count || 0, (seg.results || []).length, ctx.running ? 1 : 0];
      },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var running = ctx.running;
        if (!seg.results || seg.results.length === 0) {
          var badge = el('div', 'badge seg-tool-use');
          badge.appendChild(el('span', running ? 'seg-tool-spinner' : 'seg-tool-icon-done'));
          var name = el('span');
          name.textContent = seg.name || '';
          badge.appendChild(name);
          slot.appendChild(badge);
          return;
        }
        var open = isOpen(seg);
        var wrap = el('div', 'seg-tool-result');
        var btn = el('button', 'seg-tool-result-toggle' + (open ? ' open' : ''));
        btn.type = 'button';
        btn.appendChild(svg(CHEVRON));
        var label = el('span');
        label.textContent = seg.count > 1 ? (seg.name + ' ×' + seg.count) : (seg.name || '');
        btn.appendChild(label);
        var box = el('div');
        show(box, open);
        rec.pres = [];
        for (var ri = 0; ri < seg.results.length; ri++) {
          var pre = el('pre', 'seg-tool-result-content');
          if (seg.results.length > 1) pre.style.marginTop = '0.25rem';
          var shown = seg.results.length > 1 ? ('[' + (ri + 1) + '] ' + seg.results[ri]) : seg.results[ri];
          pre.textContent = shown;
          pre._raw = seg.results[ri];
          box.appendChild(pre);
          rec.pres.push(pre);
        }
        btn.addEventListener('click', function () {
          var on = toggleOpen(seg);
          btn.classList.toggle('open', on);
          show(box, on);
        });
        wrap.appendChild(btn);
        wrap.appendChild(box);
        slot.appendChild(wrap);
      },
      patch: function (rec, seg) {
        var results = seg.results || [];
        if (!results.length) return rec.pres ? false : true;
        if (!rec.pres || rec.pres.length !== results.length) return false;
        for (var ri = 0; ri < results.length; ri++) {
          if (rec.pres[ri]._raw === results[ri]) continue;
          rec.pres[ri]._raw = results[ri];
          rec.pres[ri].textContent = results.length > 1 ? ('[' + (ri + 1) + '] ' + results[ri]) : results[ri];
        }
        return true;
      },
    },
    tool_result: {
      sig: function () { return ['r']; },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var open = isOpen(seg);
        var wrap = el('div', 'seg-tool-result');
        var btn = el('button', 'seg-tool-result-toggle' + (open ? ' open' : ''));
        btn.type = 'button';
        btn.appendChild(svg(CHEVRON));
        btn.appendChild(document.createTextNode('Result'));
        var pre = el('pre', 'seg-tool-result-content');
        setText(pre, seg.content || '');
        show(pre, open);
        btn.addEventListener('click', function () {
          var on = toggleOpen(seg);
          btn.classList.toggle('open', on);
          show(pre, on);
        });
        wrap.appendChild(btn);
        wrap.appendChild(pre);
        rec.pre = pre;
        slot.appendChild(wrap);
      },
      patch: function (rec, seg) {
        if (!rec.pre) return false;
        setText(rec.pre, seg.content || '');
        return true;
      },
    },
    note_access: {
      sig: function (seg) {
        var items = seg.items || [];
        var bits = ['n', items.length];
        for (var i = 0; i < items.length; i++) bits.push(items[i].path, items[i].action);
        return bits;
      },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var items = seg.items || [];
        if (!items.length) return;
        var wrap = el('div', 'seg-note-access');
        var label = el('div', 'seg-note-access-label');
        label.textContent = 'Notes touched';
        wrap.appendChild(label);
        // home-list-body/is-grid gives these the exact same card layout as
        // the real note grid (see home.css); .seg-note-access-row itself
        // turns that into a horizontal scroll strip (agent_chat.css).
        var scroller = el('div', 'seg-note-access-scroller');
        var row = el('div', 'seg-note-access-row home-list-body is-grid');
        var cards = items.map(noteAccessCard);
        cards.forEach(function (c) { row.appendChild(c.node); });
        scroller.appendChild(row);
        wrap.appendChild(scroller);
        slot.appendChild(wrap);
        wireHorizontalScroller(scroller, row);
        fetchNoteInfo(items.map(function (it) { return it.path; })).then(function () {
          cards.forEach(function (c) { c.hydrate(); });
          // Hydration grows card height after scrollToBottom already ran; re-pin if still stuck.
          if (hydrateListener) hydrateListener();
        });
      },
      patch: function () { return false; },
    },
    status: {
      sig: function (seg, ctx) { return ['s', ctx.running ? 1 : 0]; },
      mount: function (slot, seg, ctx, i, msg, rec) {
        if (!ctx.running) return;
        var node = el('div', 'seg-status is-running');
        setText(node, seg.label || '');
        rec.node = node;
        slot.appendChild(node);
      },
      patch: function (rec, seg, ctx) {
        if (!ctx.running) return rec.node ? false : true;
        if (!rec.node) return false;
        setText(rec.node, seg.label || '');
        return true;
      },
    },
    console: {
      sig: function () { return ['c']; },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var node = el('pre', 'seg-console');
        setText(node, seg.content || '');
        rec.node = node;
        slot.appendChild(node);
      },
      patch: function (rec, seg) {
        if (!rec.node) return false;
        setText(rec.node, seg.content || '');
        return true;
      },
    },
    error: {
      sig: function () { return ['e']; },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var node = el('div', 'seg-error');
        setText(node, seg.message || '');
        rec.node = node;
        slot.appendChild(node);
      },
      patch: function (rec, seg) {
        if (!rec.node) return false;
        setText(rec.node, seg.message || '');
        return true;
      },
    },
  };

  function rowSig(msg, snap) {
    if (msg.role === 'user') return 'u\0' + (msg.content || '');
    var ctx = segCtx(msg);
    var stopping = snap.stoppingId === msg.id ? 1 : 0;
    var bits = [msg.status || '', stopping, msg.triggerEvent || '', snap.mateName(msg), hasText(ctx.segs) ? 1 : 0];
    for (var i = 0; i < ctx.segs.length; i++) {
      var seg = ctx.segs[i];
      var kind = kinds[seg.type];
      if (kind) bits.push.apply(bits, kind.sig(seg, ctx, i));
      else bits.push(seg.type || '');
    }
    return bits.join('\0');
  }

  function mountSegments(body, msg) {
    var ctx = segCtx(msg);
    var slots = [];
    for (var i = 0; i < ctx.segs.length; i++) {
      var seg = ctx.segs[i];
      var kind = kinds[seg.type];
      var slot = el('div');
      var rec = { type: seg.type };
      if (kind) kind.mount(slot, seg, ctx, i, msg, rec);
      body.appendChild(slot);
      slots.push(rec);
    }
    body._slots = slots;
  }

  function patchSegments(body, msg) {
    var slots = body._slots;
    if (!slots) return false;
    var ctx = segCtx(msg);
    if (slots.length !== ctx.segs.length) return false;
    for (var i = 0; i < ctx.segs.length; i++) {
      var seg = ctx.segs[i];
      var rec = slots[i];
      var kind = kinds[seg.type];
      if (!rec || rec.type !== seg.type) return false;
      if (kind && kind.patch && kind.patch(rec, seg, ctx, i) === false) return false;
    }
    return true;
  }

  function footer(msg, snap, api) {
    var foot = el('div', 'msg-footer');
    var segs = msg.segments || [];
    var stopping = snap.stoppingId === msg.id;
    if (msg.status === 'running' && stopping) {
      var stop = el('span', 'msg-thinking-label');
      stop.appendChild(document.createTextNode('Stopping'));
      var dots = el('span', 'thinking-dots');
      dots.appendChild(el('span')); dots.appendChild(el('span')); dots.appendChild(el('span'));
      stop.appendChild(dots);
      foot.appendChild(stop);
    } else if (msg.status === 'running' && !hasText(segs) && !hasStatus(segs)) {
      var think = el('span', 'msg-thinking-label');
      think.appendChild(document.createTextNode('Thinking'));
      var dots2 = el('span', 'thinking-dots');
      dots2.appendChild(el('span')); dots2.appendChild(el('span')); dots2.appendChild(el('span'));
      think.appendChild(dots2);
      foot.appendChild(think);
    }
    if (msg.status !== 'running') {
      var ts = R.msgAt(msg);
      var label = ts ? R.formatTime(ts) : '';
      if (label) {
        var time = el('span', 'msg-agent-time');
        time.textContent = label;
        foot.appendChild(time);
        foot._timeEl = time;
      }
    }
    if (msg.status !== 'running' && hasText(segs)) {
      var copy = el('button', 'msg-copy-btn' + (copiedNow(msg.id) ? ' copied' : ''));
      copy.type = 'button';
      copy.title = 'Copy reply';
      copy.appendChild(svg(copiedNow(msg.id) ? CHECK_SVG : COPY_SVG));
      copy.addEventListener('click', function () { copyReply(msg, copy); });
      foot.appendChild(copy);
    }
    return foot;
  }

  function statusBanner(msg, snap, api) {
    if (msg.status !== 'failed' && msg.status !== 'canceled') return null;
    var banner = el('div', 'msg-status-banner ' + (msg.status === 'failed' ? 'is-failed' : 'is-stopped'));
    var span = el('span');
    span.textContent = msg.status === 'failed' ? 'Failed to respond' : 'Stopped';
    banner.appendChild(span);
    var retry = el('button', 'msg-retry-btn');
    retry.type = 'button';
    retry.title = 'Retry';
    retry.disabled = !!snap.busy;
    retry.appendChild(svg(RETRY_SVG));
    retry.appendChild(document.createTextNode('Retry'));
    retry.addEventListener('click', function () { api.retry(msg.id); });
    banner.appendChild(retry);
    return banner;
  }

  function buildAssistant(msg, snap, api) {
    var row = el('div', 'msg-row msg-row-assistant');
    var wrap = el('div', 'msg-assistant-wrap');
    var header = el('div', 'msg-agent-header');
    var name = snap.mateName(msg);
    var avatar = el('div', 'avatar avatar--md avatar--accent msg-agent-avatar');
    var mate = null;
    for (var i = 0; i < snap.mates.length; i++) if (snap.mates[i].id === msg.agentBotId) { mate = snap.mates[i]; break; }
    var colorKey = mate ? mate.name : (msg.agentBotId || '');
    if (root.agentBotColorFor) avatar.style.background = root.agentBotColorFor(colorKey, snap.mates);
    avatar.style.color = 'var(--inverse-ink)';
    avatar.textContent = initials(name);
    header.appendChild(avatar);
    var nameEl = el('div', 'msg-agent-name');
    nameEl.textContent = name;
    header.appendChild(nameEl);
    if (msg.triggerEvent) {
      var badge = el('span', 'badge badge--accent');
      badge.textContent = snap.triggerLabel(msg.triggerEvent);
      header.appendChild(badge);
    }
    wrap.appendChild(header);
    var body = el('div', 'msg-body');
    mountSegments(body, msg);
    wrap.appendChild(body);
    var banner = statusBanner(msg, snap, api);
    if (banner) wrap.appendChild(banner);
    var foot = footer(msg, snap, api);
    wrap.appendChild(foot);
    row.appendChild(wrap);
    row._body = body;
    row._foot = foot;
    return row;
  }

  function buildUser(msg) {
    var row = el('div', 'msg-row msg-row-user');
    var wrap = el('div', 'msg-user-wrap');
    var bubble = el('div', 'msg-user-bubble prose');
    setProse(bubble, msg.content || '', false);
    wrap.appendChild(bubble);
    row.appendChild(wrap);
    return row;
  }

  function kbd(text) {
    var k = el('kbd');
    k.textContent = text;
    return k;
  }

  function textSpan(s) {
    var n = el('span');
    n.textContent = s;
    return n;
  }

  function arrow() {
    var n = el('span', 'chat-empty-hint-arrow');
    n.textContent = '→';
    return n;
  }

  function buildEmpty(snap, isMac) {
    var box = el('div', 'chat-empty');
    if (!snap.mate) {
      var t = el('div', 'chat-empty-text');
      t.textContent = 'Select an agent bot from the sidebar to start';
      box.appendChild(t);
      return box;
    }
    var card = el('div', 'chat-empty-card');
    var name = el('div', 'chat-empty-name');
    name.textContent = snap.mate.name || '';
    card.appendChild(name);
    var desc = el('div', 'chat-empty-desc');
    desc.textContent = 'Ask anything or assign a task';
    card.appendChild(desc);
    var hints = el('div', 'chat-empty-hints');
    function hint(key, nodes) {
      var k = el('span', 'chat-empty-hint-key');
      k.textContent = key;
      hints.appendChild(k);
      var v = el('div', 'chat-empty-hint-val');
      for (var i = 0; i < nodes.length; i++) v.appendChild(nodes[i]);
      hints.appendChild(v);
    }
    var mod = isMac ? '⌘' : 'Ctrl';
    // Same shared label on both rows — global search and the inline "@"
    // mention are two doors into the same action (reference a note as a
    // [[stem]] wikilink), not two different features.
    hint('Reference note', [kbd(mod), kbd('K'), textSpan('search'), arrow(), kbd(mod), kbd('↵'), textSpan('insert')]);
    hint('Reference note', [kbd('@'), textSpan('search, or "/" to browse folders')]);
    card.appendChild(hints);
    box.appendChild(card);
    return box;
  }

  root.__vaultrChatMount = function (rootEl, api) {
    var isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
    var thread = rootEl.querySelector('#chat-thread');
    var scrollEl = rootEl.querySelector('#chat-scroll');
    var panel = rootEl.querySelector('#chat-input-outer');
    var ta = rootEl.querySelector('#chat-textarea');
    var hint = rootEl.querySelector('#chat-input-hint');
    var hintName = rootEl.querySelector('#chat-hint-name');
    var sendBtn = rootEl.querySelector('#chat-send');
    var stopBtn = rootEl.querySelector('#chat-stop');
    var newBtn = rootEl.querySelector('#chat-new-btn');
    var jumpBtn = rootEl.querySelector('#chat-jump');
    var descEl = rootEl.querySelector('#chat-bot-desc');
    var seg = rootEl.querySelector('#chat-conv-seg');
    var shortcut = rootEl.querySelector('#chat-shortcut-hint');
    var spacer = el('div', 'chat-scroll-spacer');
    spacer.id = 'chat-scroll-spacer';
    spacer.setAttribute('aria-hidden', 'true');
    thread.appendChild(spacer);

    var rows = new Map();
    var lastBusy = null;
    var detaching = false;
    var listen = new AbortController();
    function on(node, type, fn, extra) {
      if (!node) return;
      var opts = extra ? Object.assign({ signal: listen.signal }, extra) : { signal: listen.signal };
      node.addEventListener(type, fn, opts);
    }
    var ac = null;
    var ro = null;
    var scrollRaf = 0;
    var timeTimer = 0;
    var emptyKind = '';

    if (shortcut) shortcut.textContent = isMac ? '⌘↵' : 'Ctrl+↵';

    function onCopied(id, onIcon) {
      var row = rows.get(id);
      var btn = row && row.querySelector('.msg-copy-btn');
      setCopyIcon(btn, onIcon);
    }
    copiedListener = onCopied;

    function scrollToBottom() {
      if (scrollEl) scrollEl.scrollTop = scrollEl.scrollHeight;
    }

    function onContentHydrated() {
      if (!detaching && api.stick()) scrollToBottom();
    }
    hydrateListener = onContentHydrated;

    function updateJump(snap) {
      var st = snap || api.state();
      show(jumpBtn, !st.stick && st.messages.length > 0);
    }

    function syncComposerBox() {
      if (!panel || !scrollEl || panel.style.display === 'none') return;
      var sbw = scrollEl.offsetWidth - scrollEl.clientWidth;
      scrollEl.style.setProperty('--chat-scrollbar-w', (Math.ceil(sbw) + 4) + 'px');
      scrollEl.style.setProperty('--chat-composer-h', panel.offsetHeight + 'px');
      if (api.stick()) scrollToBottom();
    }

    function armComposerObserver() {
      if (ro) { ro.disconnect(); ro = null; }
      if (!panel || !scrollEl || typeof ResizeObserver === 'undefined') return;
      if (panel.style.display === 'none') return;
      ro = new ResizeObserver(syncComposerBox);
      ro.observe(panel);
      syncComposerBox();
    }

    function renderChrome(snap) {
      var mate = snap.mate;
      var busy = !!snap.busy;
      var live = snap.phase === 'streaming' || snap.phase === 'stopping';
      if (descEl) {
        var desc = mate && mate.description ? mate.description : '';
        descEl.textContent = desc;
        show(descEl, !!desc);
      }
      show(seg, !!snap.mateId);
      if (seg) {
        var btns = seg.querySelectorAll('.seg-btn');
        for (var i = 0; i < btns.length; i++) {
          btns[i].classList.toggle('active', btns[i].getAttribute('data-conv') === snap.convType);
          btns[i].disabled = busy;
        }
      }
      if (newBtn) newBtn.disabled = busy || !snap.mateId || snap.messages.length === 0 || snap.convType !== 'chat';
      show(panel, snap.convType === 'chat');
      if (spacer) {
        if (snap.convType !== 'chat') spacer.style.height = '2rem';
        else spacer.style.height = '';
      }
      if (hintName) {
        hintName.textContent = mate ? ('Ask ' + mate.name + '…') : 'Select an agent bot from the sidebar';
      }
      if (ta) {
        ta.disabled = busy || !snap.mateId;
        if (document.activeElement !== ta && ta.value !== snap.draft) {
          ta.value = snap.draft;
          root.__vaultrAutoResize(ta);
        }
        show(hint, !ta.value);
      }
      if (sendBtn) {
        show(sendBtn, !live);
        sendBtn.disabled = busy || !(ta && ta.value.trim()) || !snap.mateId;
      }
      if (stopBtn) {
        show(stopBtn, live);
        stopBtn.disabled = !snap.runId || snap.phase === 'stopping';
      }
      updateJump(snap);
    }

    function retire(id) {
      var row = rows.get(id);
      if (row && row.parentNode) row.parentNode.removeChild(row);
      rows.delete(id);
    }

    function syncRetry(snap) {
      var busy = !!snap.busy;
      if (busy === lastBusy) return;
      lastBusy = busy;
      rows.forEach(function (row) {
        var btn = row.querySelector && row.querySelector('.msg-retry-btn');
        if (btn) btn.disabled = busy;
      });
    }

    function place(row, prev) {
      var before = prev ? prev.nextSibling : thread.firstChild;
      if (row !== before) thread.insertBefore(row, before);
      return row;
    }

    function renderThread(snap) {
      if (!snap.messages.length) {
        Array.from(rows.keys()).forEach(retire);
        var kind = snap.mate ? ('card:' + snap.mate.name) : 'plain';
        var empty = thread.querySelector('.chat-empty');
        if (emptyKind !== kind) {
          if (empty) empty.remove();
          empty = buildEmpty(snap, isMac);
          thread.insertBefore(empty, spacer);
          emptyKind = kind;
        }
        return;
      }
      var emptyNow = thread.querySelector('.chat-empty');
      if (emptyNow) emptyNow.remove();
      emptyKind = '';
      var seen = {};
      var prev = null;
      for (var i = 0; i < snap.messages.length; i++) {
        var msg = snap.messages[i];
        var id = msg.id || ('i' + i);
        seen[id] = true;
        var row = rows.get(id);
        var stopBit = snap.stoppingId === id ? 1 : 0;
        if (row && row._epoch === snap.catalogEpoch && row._rev === msg.rev && row._stop === stopBit) {
          prev = place(row, prev);
          continue;
        }
        var sig = rowSig(msg, snap);
        var same = row && row._epoch === snap.catalogEpoch && row._struct === sig;
        if (!(same && row._body && patchSegments(row._body, msg))) {
          var fresh = msg.role === 'user' ? buildUser(msg) : buildAssistant(msg, snap, api);
          if (row && row.parentNode) row.parentNode.replaceChild(fresh, row);
          row = fresh;
          rows.set(id, row);
          row._struct = sig;
          row._epoch = snap.catalogEpoch;
        }
        row._rev = msg.rev;
        row._stop = stopBit;
        prev = place(row, prev);
      }
      if (spacer.parentNode !== thread || (prev && prev.nextSibling !== spacer)) thread.appendChild(spacer);
      rows.forEach(function (row, id) { if (!seen[id]) retire(id); });
    }

    function updateTimes() {
      var snap = api.state();
      var now = Date.now();
      for (var i = 0; i < snap.messages.length; i++) {
        var msg = snap.messages[i];
        if (!msg || msg.role !== 'assistant' || msg.status === 'running') continue;
        var row = rows.get(msg.id);
        var timeEl = row && row._foot && row._foot._timeEl;
        if (!timeEl) continue;
        var label = R.formatTime(R.msgAt(msg), now);
        if (timeEl.textContent !== label) timeEl.textContent = label;
      }
    }

    function render(mode) {
      var snap = api.state();
      renderChrome(snap);
      renderThread(snap);
      syncRetry(snap);
      if (mode === 'jump' || (mode === 'maybe' && snap.stick)) scrollToBottom();
      updateJump(snap);
    }

    function onScroll() {
      if (scrollRaf) return;
      scrollRaf = requestAnimationFrame(function () {
        scrollRaf = 0;
        if (!scrollEl) return;
        var dist = scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight;
        api.setStick(dist <= 64);
        updateJump();
      });
    }

    if (scrollEl) {
      on(scrollEl, 'scroll', onScroll, { passive: true });
      on(scrollEl, 'click', function (e) {
        var a = e.target.closest && e.target.closest('a');
        if (!a) return;
        var href = a.getAttribute('href') || '';
        if (href.indexOf('/notes?') !== 0) return;
        e.preventDefault();
        try {
          var name = new URLSearchParams(href.split('?')[1] || '').get('name') || '';
          if (name && typeof root.__vaultrContentPaneOpenWikiLink === 'function') {
            void root.__vaultrContentPaneOpenWikiLink(name.replace(/\.md$/, ''));
          }
        } catch (err) { /* ignore */ }
      });
    }

    if (ta && root.__vaultrPathAcCreate) {
      ac = root.__vaultrPathAcCreate({
        getInput: function () { return ta; },
        getList: function () { return rootEl.querySelector('#chat-path-ac'); },
        getPanel: function () { return rootEl.querySelector('#chat-ac-panel'); },
        getHint: function () { return rootEl.querySelector('#chat-path-ac-hint'); },
        parseCtx: chatAcParseCtx,
        search: chatMentionSearch,
        getDefaultItems: chatMentionDefaultItems,
        onApply: function (input, newVal, caretPos) {
          if (detaching) return;
          input.value = newVal;
          api.setDraft(newVal);
          input.dispatchEvent(new Event('input', { bubbles: true }));
          if (input.setSelectionRange) input.setSelectionRange(caretPos, caretPos);
          input.focus();
        },
        escKey: 'chat-ac',
      });
      on(ta, 'input', function () {
        if (detaching) return;
        api.setDraft(ta.value);
        root.__vaultrAutoResize(ta);
        ac.refresh();
        show(hint, !ta.value);
        var snap = api.state();
        if (sendBtn) sendBtn.disabled = snap.busy || !ta.value.trim() || !snap.mateId;
      });
      on(ta, 'click', function () { ac.refresh(); });
      on(ta, 'keyup', function (ev) {
        if (ev.key === 'ArrowLeft' || ev.key === 'ArrowRight' || ev.key === 'Home' || ev.key === 'End') ac.refresh();
      });
      on(ta, 'blur', function () {
        setTimeout(function () {
          if (!ac) return;
          var list = rootEl.querySelector('#chat-path-ac');
          if (list && !list.contains(document.activeElement)) ac.close();
        }, 180);
      });
      on(ta, 'keydown', function (e) {
        if (ac && ac.handleKeydown(e)) return;
        if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); void api.send(); }
      });
    }

    on(sendBtn, 'click', function () { void api.send(); });
    on(stopBtn, 'click', function () { void api.cancel(); });
    on(newBtn, 'click', function () { void api.newChat(); });
    on(jumpBtn, 'click', function () {
      api.setStick(true);
      scrollToBottom();
      updateJump();
    });
    if (seg) {
      on(seg, 'click', function (e) {
        var btn = e.target.closest && e.target.closest('.seg-btn');
        if (!btn || btn.disabled) return;
        api.setConvType(btn.getAttribute('data-conv'));
      });
    }

    if (ta) {
      ta.value = api.draft();
      root.__vaultrAutoResize(ta);
    }
    armComposerObserver();
    timeTimer = setInterval(updateTimes, 30000);

    return {
      render: render,
      draftValue: function () { return ta ? ta.value : api.draft(); },
      clearComposer: function () {
        if (!ta) return;
        ta.value = '';
        ta.style.height = '';
        var card = ta.closest('.chat-input-card');
        if (card) card.classList.remove('is-multiline');
        show(hint, true);
      },
      focusComposer: function () {
        if (!ta) return;
        ta.style.height = '';
        ta.selectionStart = ta.selectionEnd = ta.value.length;
        ta.focus();
      },
      insertWikilink: function (stem) {
        if (!stem) return;
        var cur = ta ? ta.value : api.draft();
        var start = ta ? ta.selectionStart : cur.length;
        var end = ta ? ta.selectionEnd : start;
        var before = cur.slice(0, start);
        var after = cur.slice(end);
        var leadSp = before.length > 0 && before[before.length - 1] !== ' ' ? ' ' : '';
        var tailSp = after.length > 0 && after[0] !== ' ' ? ' ' : '';
        var insert = leadSp + '[[' + stem + ']]' + tailSp;
        var next = before + insert + after;
        api.setDraft(next);
        if (ta) {
          ta.value = next;
          var pos = start + insert.length;
          ta.selectionStart = ta.selectionEnd = pos;
          ta.focus();
          root.__vaultrAutoResize(ta);
          show(hint, !next);
        }
      },
      destroy: function () {
        detaching = true;
        if (copiedListener === onCopied) copiedListener = null;
        if (hydrateListener === onContentHydrated) hydrateListener = null;
        listen.abort();
        if (timeTimer) { clearInterval(timeTimer); timeTimer = 0; }
        if (ro) { ro.disconnect(); ro = null; }
        if (ac) {
          try { ac.close(); } catch (e) { /* ignore */ }
          ac = null;
        }
        detaching = false;
      },
    };
  };
})(typeof window !== 'undefined' ? window : global);
