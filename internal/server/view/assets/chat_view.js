// Chat pane. Mounted while #chat-root is in the document and discarded on
// HTMX swap. Each segment kind owns its signature, mount, and patch so a
// streaming token only rewrites the tail. Open and copied live here, not on
// the message.
(function (root) {
  var R = root.__hyloChatReduce;
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

  function answerText(msg) {
    var segs = msg.segments || [];
    var ai = R.answerIndex(segs, msg.status === 'running');
    return ai >= 0 ? (segs[ai].content || '').trim() : '';
  }

  function copyReply(msg, btn) {
    var text = answerText(msg);
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
    var pane = window.__hyloContentPane;
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
  var noteInfoCache = Object.create(null); // path -> row | null (not in vault); undefined = not fetched or fetch failed

  function fetchNoteInfo(paths) {
    var need = paths.filter(function (p) { return p && !(p in noteInfoCache); });
    if (!need.length) return Promise.resolve();
    var chunks = [];
    for (var i = 0; i < need.length; i += 100) chunks.push(need.slice(i, i + 100));
    return Promise.all(chunks.map(function (chunk) {
      return fetch('/api/notes/info', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ paths: chunk }),
      }).then(function (resp) { return resp.ok ? resp.json() : null; })
        .then(function (data) {
          if (!data) return;
          var got = Object.create(null);
          (data.notes || []).forEach(function (n) { noteInfoCache[n.path] = n; got[n.path] = true; });
          chunk.forEach(function (p) { if (!got[p]) noteInfoCache[p] = null; });
        })
        .catch(function () {});
    }));
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
      if (window.__hyloContentPane) {
        void window.__hyloContentPane.openNoteInContentPane(path, titleEl.textContent || fallbackTitle, false, false);
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
    return update;
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

  // A fence opener still being typed would render as an empty code block until
  // its info string is whole and the block can become its card.
  var PARTIAL_FENCE = /(^|\n)[ \t]*(`{1,3}|~{1,3})[^`\n]*$/;

  var FENCE_LINE = /^[ \t]*(`{3,}|~{3,})(.*)$/;

  // A fence still open at the end of a streaming reply has a half-written body.
  function endsInOpenFence(text) {
    var open = '';
    text.split('\n').forEach(function (line) {
      var m = FENCE_LINE.exec(line);
      if (!m) return;
      if (!open) open = m[1];
      else if (m[1][0] === open[0] && m[1].length >= open.length && !m[2].trim()) open = '';
    });
    return !!open;
  }

  function setProse(node, text, streaming) {
    if (streaming) text = text.replace(PARTIAL_FENCE, '$1');
    var openTail = !!streaming && endsInOpenFence(text);
    if (node._raw === text && node._openTail === openTail) return;
    node._raw = text;
    node._openTail = openTail;
    node.innerHTML = root.__hyloRenderMarkdown(text, { cache: !streaming });
    root.__hyloHydrateFenceCards(node, { openTail: openTail });
  }

  function setText(node, text) {
    if (!node || node._raw === text) return;
    node._raw = text;
    node.textContent = text;
  }

  var CHECK_MARK = '<svg class="seg-tool-check" fill="none" stroke="currentColor" stroke-width="2.4" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 12.5l4.5 4.5L19 7.5"/></svg>';
  var CROSS_MARK = '<svg class="seg-tool-check" fill="none" stroke="currentColor" stroke-width="2.4" viewBox="0 0 24 24"><path stroke-linecap="round" d="M7 7l10 10M17 7L7 17"/></svg>';

  var TOOL_VERBS = {
    read: ['Reading', 'Read'],
    search: ['Searching', 'Searched'], grep: ['Searching', 'Searched'], glob: ['Searching', 'Searched'],
    find: ['Searching', 'Searched'], list: ['Listing', 'Listed'], ls: ['Listing', 'Listed'],
    edit: ['Editing', 'Edited'], multiedit: ['Editing', 'Edited'], write: ['Writing', 'Wrote'],
    file_change: ['Editing', 'Edited'], patch: ['Editing', 'Edited'], delete: ['Deleting', 'Deleted'],
    bash: ['Running', 'Ran'], execute: ['Running', 'Ran'], shell: ['Running', 'Ran'],
    fetch: ['Fetching', 'Fetched'], webfetch: ['Fetching', 'Fetched'], websearch: ['Searching web', 'Searched web'],
    task: ['Delegating', 'Delegated'], agent: ['Delegating', 'Delegated'],
    todowrite: ['Planning', 'Planned'], think: ['Thinking', 'Thought'],
    view: ['Reading', 'Read'], read_file: ['Reading', 'Read'], cat: ['Reading', 'Read'],
    create: ['Writing', 'Wrote'], write_file: ['Writing', 'Wrote'],
    str_replace: ['Editing', 'Edited'], str_replace_editor: ['Editing', 'Edited'], apply_patch: ['Editing', 'Edited'],
    replace: ['Editing', 'Edited'], move: ['Moving', 'Moved'],
    grep_search: ['Searching', 'Searched'], codebase_search: ['Searching', 'Searched'], search_files: ['Searching', 'Searched'],
    list_dir: ['Listing', 'Listed'], list_directory: ['Listing', 'Listed'],
    web_fetch: ['Fetching', 'Fetched'], web_search: ['Searching web', 'Searched web'],
    run_terminal_cmd: ['Running', 'Ran'], run_command: ['Running', 'Ran']
  };

  function toolVerb(name, running) {
    var short = String(name || '').replace(/^mcp__.*__/, '');
    var key = short.toLowerCase().replace(/^codex_/, '');
    var v = TOOL_VERBS[key];
    if (v) return v[running ? 0 : 1];
    return !key || key === 'other' ? 'Tool' : short;
  }

  function fmtDur(ms) {
    if (ms < 100) return '';
    if (ms < 10000) return (ms / 1000).toFixed(1) + 's';
    var s = Math.round(ms / 1000);
    return s < 60 ? s + 's' : Math.floor(s / 60) + 'm ' + (s % 60) + 's';
  }

  function toolMs(seg) {
    var total = 0;
    var calls = seg.calls || [];
    for (var i = 0; i < calls.length; i++) if (calls[i].endedAt) total += calls[i].endedAt - calls[i].startedAt;
    return total;
  }

  var chipsShown = new WeakMap();

  // Notes the call touched, as chips that pop in as they're discovered.
  function noteChips(seg, notes) {
    var row = el('div', 'seg-tool-notes');
    var seenCount = chipsShown.get(seg) || 0;
    chipsShown.set(seg, notes.length);
    notes.forEach(function (n, idx) {
      var base = n.path.split('/').pop() || n.path;
      var info = noteInfoCache[n.path];
      var title = (info && info.title ? info.title : base).replace(/\.md$/i, '');
      var chip = el('button', 'seg-note-chip' + (n.action === 'write' ? ' is-write' : '') + (idx >= seenCount ? ' is-fresh' : ''));
      chip.type = 'button';
      chip.title = n.path;
      chip.textContent = title;
      chip.addEventListener('click', function () {
        if (window.__hyloContentPane) {
          void window.__hyloContentPane.openNoteInContentPane(n.path, title, false, false);
        }
      });
      row.appendChild(chip);
    });
    return row;
  }

  // Rows rebuild wholesale on structural change; a per-seg memo keeps one-shot
  // entrance animations from replaying on every rebuild.
  var shownState = new WeakMap();
  function firstShow(seg, state) {
    var fresh = shownState.get(seg) !== state;
    shownState.set(seg, state);
    return fresh;
  }

  // Negative delay keeps looping animations in phase across row rebuilds.
  function phase(node, period) {
    node.style.animationDelay = '-' + (Date.now() % period) + 'ms';
  }

  function segCtx(msg) {
    var segs = msg.segments || [];
    return { segs: segs, running: msg.status === 'running' };
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
      sig: function (seg, ctx) { return ['h', (ctx.running && !seg.endedAt) ? 1 : 0, seg.endedAt ? 1 : 0]; },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var live = ctx.running && !seg.endedAt;
        var open = isOpen(seg);
        var wrap = el('div', 'seg-thinking hairline');
        var btn = el('button', 'seg-thinking-toggle' + (open ? ' open' : '') + (live ? ' is-streaming' : ''));
        btn.type = 'button';
        btn.appendChild(svg(CHEVRON));
        var label = el('span', 'seg-thinking-label');
        var dur = seg.endedAt && seg.startedAt ? fmtDur(seg.endedAt - seg.startedAt) : '';
        label.textContent = live ? 'Thinking' : (dur ? 'Thought for ' + dur : 'Thought');
        if (live) phase(label, 1800);
        btn.appendChild(label);
        var body = el('div', 'seg-thinking-content' + (live && !open ? ' is-preview' : ''));
        setText(body, seg.content || '');
        show(body, open || (live && !!seg.content));
        btn.addEventListener('click', function () {
          var on = toggleOpen(seg);
          btn.classList.toggle('open', on);
          body.classList.toggle('is-preview', live && !on);
          show(body, on || (live && !!seg.content));
        });
        wrap.appendChild(btn);
        wrap.appendChild(body);
        rec.body = body;
        rec.live = live;
        slot.appendChild(wrap);
      },
      patch: function (rec, seg) {
        if (!rec.body) return false;
        setText(rec.body, seg.content || '');
        if (rec.live && seg.content && !isOpen(seg)) show(rec.body, true);
        return true;
      },
    },
    tool_use: {
      sig: function (seg, ctx) {
        var calls = seg.calls || [];
        var last = calls.length ? calls[calls.length - 1] : null;
        return ['u', seg.name || '', seg.count || 0, (seg.results || []).length, ctx.running ? 1 : 0,
          last ? last.arg : '', last && last.failed ? 1 : 0,
          (seg.notes || []).map(function (n) { return n.path + n.action; }).join('|')];
      },
      mount: function (slot, seg, ctx, i, msg, rec) {
        var results = seg.results || [];
        var calls = seg.calls || [];
        var last = calls.length ? calls[calls.length - 1] : null;
        var pending = ctx.running && results.length < seg.count;
        var failed = false;
        for (var ci = 0; ci < calls.length; ci++) if (calls[ci].failed) failed = true;
        var state = pending ? 'run' : (failed ? 'err' : (results.length ? 'ok' : 'idle'));
        var hasBody = results.some(function (r) { return !!r; });
        var open = hasBody && isOpen(seg);

        var wrap = el('div', 'seg-tool is-' + state);
        var head = el(hasBody ? 'button' : 'div', 'seg-tool-head' + (open ? ' open' : ''));
        if (hasBody) head.type = 'button';

        var icon;
        if (state === 'run') { icon = el('span', 'seg-tool-spinner'); phase(icon, 700); }
        else if (state === 'ok' || state === 'err') {
          icon = svg(state === 'ok' ? CHECK_MARK : CROSS_MARK);
          if (firstShow(seg, state + results.length)) icon.classList.add('is-fresh');
        } else icon = el('span', 'seg-tool-icon-done');
        head.appendChild(icon);

        var verb = el('span', 'seg-tool-verb');
        verb.textContent = toolVerb(seg.name, pending);
        if (pending) phase(verb, 1800);
        head.appendChild(verb);
        var notes = seg.notes || [];
        if (last && last.arg && !notes.length) {
          var arg = el('span', 'seg-tool-arg');
          arg.textContent = last.arg;
          arg.title = last.arg;
          head.appendChild(arg);
        }
        if (seg.count > 1) {
          var cnt = el('span', 'seg-tool-count');
          cnt.textContent = '×' + seg.count;
          head.appendChild(cnt);
        }
        var d = pending ? '' : fmtDur(toolMs(seg));
        if (d) {
          var t = el('span', 'seg-tool-time');
          t.textContent = d;
          head.appendChild(t);
        }
        if (hasBody) head.appendChild(svg(CHEVRON));
        wrap.appendChild(head);
        if (notes.length) wrap.appendChild(noteChips(seg, notes));
        slot.appendChild(wrap);
        if (!hasBody) return;

        var box = el('div');
        show(box, open);
        rec.pres = [];
        for (var ri = 0; ri < results.length; ri++) {
          var pre = el('pre', 'seg-tool-result-content');
          if (results.length > 1) pre.style.marginTop = '0.25rem';
          pre.textContent = results.length > 1 ? ('[' + (ri + 1) + '] ' + results[ri]) : results[ri];
          pre._raw = results[ri];
          box.appendChild(pre);
          rec.pres.push(pre);
        }
        head.addEventListener('click', function () {
          var on = toggleOpen(seg);
          head.classList.toggle('open', on);
          show(box, on);
        });
        wrap.appendChild(box);
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
        // Hidden until the vault confirms which notes exist; the wrapper is
        // appended now so it keeps its place among later segments.
        show(wrap, false);
        scroller.appendChild(row);
        wrap.appendChild(scroller);
        slot.appendChild(wrap);
        var updateArrows = wireHorizontalScroller(scroller, row);
        fetchNoteInfo(items.map(function (it) { return it.path; })).then(function () {
          var cards = items
            .filter(function (it) { return noteInfoCache[it.path] !== null; })
            .map(noteAccessCard);
          if (!cards.length) return;
          cards.forEach(function (c) { row.appendChild(c.node); c.hydrate(); });
          show(wrap, true);
          updateArrows();
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

  // A turn is one trace (thinking, tools, narration) then the reply. While
  // running the trace keeps its newest steps in a capped box; once settled it
  // folds to a one-line summary.
  var TRACE_TYPES = { thinking: 1, tool_use: 1, tool_result: 1, status: 1, text: 1 };

  function buildItems(ctx) {
    var segs = ctx.segs, ai = R.answerIndex(segs, ctx.running);
    var trace = { trace: true, segs: [], idx: [], live: ctx.running }, rest = [], steps = 0;
    for (var i = 0; i < segs.length; i++) {
      if (i !== ai && TRACE_TYPES[segs[i].type]) {
        trace.segs.push(segs[i]);
        trace.idx.push(i);
        if (segs[i].type !== 'status') steps++;
      } else if (i !== ai) rest.push({ seg: segs[i], i: i });
    }
    var items = [];
    if (steps) items.push(trace);
    else for (var k = 0; k < trace.segs.length; k++) items.push({ seg: trace.segs[k], i: trace.idx[k] });
    if (ai >= 0) items.push({ seg: segs[ai], i: ai });
    return items.concat(rest);
  }

  var openTraces = new WeakMap();
  var traceState = new WeakMap();

  function traceSummary(segs, live) {
    var think = 0, thinking = false, hasThink = false, order = [], counts = {};
    for (var i = 0; i < segs.length; i++) {
      var sg = segs[i];
      if (sg.type === 'thinking') {
        hasThink = true;
        if (sg.endedAt && sg.startedAt) think += sg.endedAt - sg.startedAt;
        else if (live) thinking = true;
      } else if (sg.type === 'tool_use') {
        var v = toolVerb(sg.name, false);
        if (!(v in counts)) { counts[v] = 0; order.push(v); }
        counts[v] += sg.count || 1;
      }
    }
    var parts = [];
    if (thinking) parts.push('Thinking');
    else if (hasThink) { var d = fmtDur(think); parts.push(d ? 'Thought ' + d : 'Thought'); }
    for (var k = 0; k < order.length; k++) parts.push(order[k] + (counts[order[k]] > 1 ? ' ×' + counts[order[k]] : ''));
    return parts.join(' · ') || (live ? 'Working' : 'Worked');
  }

  function traceText(slot, seg, ctx, rec) {
    var node = el('div', 'prose seg-trace-text');
    setProse(node, seg.content || '', ctx.running);
    rec.node = node;
    slot.appendChild(node);
  }

  // The capped box fades its top edge only once older steps are cut off.
  // They overflow upward, which scrollHeight doesn't count.
  function markOverflow(inner) {
    requestAnimationFrame(function () {
      var a = inner.firstElementChild, z = inner.lastElementChild;
      var h = a && z ? z.offsetTop + z.offsetHeight - a.offsetTop : 0;
      inner.classList.toggle('is-overflow', h > inner.clientHeight + 1);
    });
  }

  function mountTrace(slot, item, ctx, msg, rec) {
    var first = item.segs[0];
    var live = item.live;
    var open = !!openTraces.get(first);
    var was = traceState.get(first);
    traceState.set(first, live ? 'live' : 'settled');
    var folding = !live && !open && was === 'live';
    var wrap = el('div', 'seg-trace' + (live ? ' is-live' : ''));
    var shell = el('div', 'seg-trace-shell' + (open ? '' : (live || folding ? ' is-capped' : ' is-collapsed')));
    var inner = el('div', 'seg-trace-body');
    rec.kids = [];
    for (var n = 0; n < item.segs.length; n++) {
      var seg = item.segs[n];
      var kslot = el('div');
      var krec = { type: seg.type };
      if (seg.type === 'text') traceText(kslot, seg, ctx, krec);
      else if (kinds[seg.type]) kinds[seg.type].mount(kslot, seg, ctx, item.idx[n], msg, krec);
      inner.appendChild(kslot);
      rec.kids.push(krec);
    }
    shell.appendChild(inner);
    var head = el('button', 'seg-trace-head' + (open ? ' open' : ''));
    head.type = 'button';
    head.appendChild(svg(CHEVRON));
    var label = el('span');
    label.textContent = traceSummary(item.segs, live);
    if (live) phase(label, 1800);
    head.appendChild(label);
    head.addEventListener('click', function () {
      var on = !openTraces.get(first);
      openTraces.set(first, on);
      head.classList.toggle('open', on);
      shell.classList.remove('is-capped', 'is-collapsed');
      if (!on) shell.classList.add(live ? 'is-capped' : 'is-collapsed');
      if (!on && live) markOverflow(inner);
    });
    wrap.appendChild(head);
    wrap.appendChild(shell);
    slot.appendChild(wrap);
    if (live || folding) markOverflow(inner);
    if (folding) {
      requestAnimationFrame(function () {
        requestAnimationFrame(function () { shell.classList.add('is-collapsed'); });
      });
    }
    rec.n = item.segs.length;
    rec.live = live;
    rec.inner = inner;
  }

  function rowSig(msg, snap) {
    if (msg.role === 'user') {
      var ids = (msg.attachments || []).map(function (a) { return a.id; }).join(',');
      return 'u\0' + (msg.content || '') + '\0' + ids;
    }
    var ctx = segCtx(msg);
    var stopping = snap.stoppingId === msg.id ? 1 : 0;
    var bits = [msg.status || '', stopping, msg.triggerEvent || '', snap.mateName(msg), answerText(msg) ? 1 : 0];
    var items = buildItems(ctx);
    for (var n = 0; n < items.length; n++) {
      var it = items[n];
      var segs = it.trace ? it.segs : [it.seg];
      if (it.trace) bits.push('trace', it.live ? 1 : 0);
      for (var m = 0; m < segs.length; m++) {
        var kind = kinds[segs[m].type];
        if (it.trace && segs[m].type === 'text') bits.push('tt');
        else if (kind) bits.push.apply(bits, kind.sig(segs[m], ctx, it.trace ? it.idx[m] : it.i));
        else bits.push(segs[m].type || '');
      }
    }
    return bits.join('\0');
  }

  function mountSegments(body, msg) {
    var ctx = segCtx(msg);
    var items = buildItems(ctx);
    var slots = [];
    for (var n = 0; n < items.length; n++) {
      var it = items[n];
      var slot = el('div');
      var rec;
      if (it.trace) {
        rec = { type: 'trace' };
        mountTrace(slot, it, ctx, msg, rec);
      } else {
        rec = { type: it.seg.type };
        var kind = kinds[it.seg.type];
        if (kind) kind.mount(slot, it.seg, ctx, it.i, msg, rec);
      }
      body.appendChild(slot);
      slots.push(rec);
    }
    body._slots = slots;
  }

  function patchSegments(body, msg) {
    var slots = body._slots;
    if (!slots) return false;
    var ctx = segCtx(msg);
    var items = buildItems(ctx);
    if (slots.length !== items.length) return false;
    for (var n = 0; n < items.length; n++) {
      var it = items[n];
      var rec = slots[n];
      if (it.trace) {
        if (!rec || rec.type !== 'trace' || rec.n !== it.segs.length || rec.live !== it.live) return false;
        for (var m = 0; m < it.segs.length; m++) {
          var ks = it.segs[m], kr = rec.kids[m], kk = ks.type === 'text' ? kinds.text : kinds[ks.type];
          if (!kr || kr.type !== ks.type) return false;
          if (kk && kk.patch && kk.patch(kr, ks, ctx, it.idx[m]) === false) return false;
        }
        if (it.live) markOverflow(rec.inner);
        continue;
      }
      var kind = kinds[it.seg.type];
      if (!rec || rec.type !== it.seg.type) return false;
      if (kind && kind.patch && kind.patch(rec, it.seg, ctx, it.i) === false) return false;
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
    } else if (msg.status === 'running' && !segs.length) {
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
    if (msg.status !== 'running' && answerText(msg)) {
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
    var avatar = el('div', 'avatar avatar--md msg-agent-avatar');
    var mate = null;
    for (var i = 0; i < snap.mates.length; i++) if (snap.mates[i].id === msg.agentBotId) { mate = snap.mates[i]; break; }
    var colorKey = mate ? mate.name : (msg.agentBotId || '');
    if (root.agentBotAvatarSVG) avatar.innerHTML = root.agentBotAvatarSVG(colorKey, snap.mates, name);
    else avatar.textContent = initials(name);
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
    var atts = msg.attachments || [];
    if (atts.length) {
      var grid = el('div', 'msg-user-images');
      atts.forEach(function (a) {
        var src = '/api/chat/uploads/' + encodeURIComponent(a.id);
        var open = el('button', 'msg-user-image');
        open.type = 'button';
        open.title = a.name || 'image';
        open.dataset.imgSrc = src;
        open.dataset.imgName = a.name || a.id;
        open.dataset.imgExt = (a.id.match(/\.[a-z]+$/i) || [''])[0];
        open.addEventListener('click', function () {
          if (root.openImageLightbox) root.openImageLightbox(open);
        });
        var img = el('img');
        img.src = src;
        img.alt = a.name || 'image';
        open.appendChild(img);
        grid.appendChild(open);
      });
      wrap.appendChild(grid);
    }
    if ((msg.content || '').trim()) {
      var bubble = el('div', 'msg-user-bubble prose');
      setProse(bubble, msg.content || '', false);
      wrap.appendChild(bubble);
    }
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
    if (!snap.mate && snap.matesLoaded && !snap.mates.length) {
      box.className = 'chat-empty empty-state empty-state--page';
      var icon = el('div', 'empty-state-icon');
      var iconTpl = document.getElementById('chat-setup-icon');
      if (iconTpl) icon.appendChild(iconTpl.content.cloneNode(true));
      box.appendChild(icon);
      var title = el('div', 'empty-state-title');
      title.textContent = 'Meet Hylo';
      box.appendChild(title);
      var about = el('div', 'empty-state-desc');
      about.textContent = 'Your personal AI agent, powered by your notes. Connect an AI agent on this server to set it up.';
      box.appendChild(about);
      var actions = el('div', 'empty-state-actions');
      var setup = el('button', 'btn-solid');
      setup.type = 'button';
      setup.textContent = 'Set up Hylo';
      setup.addEventListener('click', function () {
        window.dispatchEvent(new CustomEvent('hylo:open-settings', { detail: { tab: 'agent-bots' } }));
      });
      actions.appendChild(setup);
      box.appendChild(actions);
      return box;
    }
    if (!snap.mate) {
      var t = el('div', 'chat-empty-text');
      t.textContent = 'Select a bot from the sidebar to start';
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

  // ── Trigger tab: a run log, not a conversation ──────────────────────
  var runOpen = new Set();

  var TRIG_ICONS = {
    clock: '<svg fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 7v5l3 2"/></svg>',
    note: '<svg fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path stroke-linecap="round" stroke-linejoin="round" d="M14 3v6h6"/></svg>',
    layers: '<svg fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3 3 8l9 5 9-5z"/><path stroke-linecap="round" stroke-linejoin="round" d="m3 13 9 5 9-5"/></svg>',
    bolt: '<svg fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M13 2 4 14h7l-1 8 9-12h-7z"/></svg>',
  };
  var READ_SVG = '<svg fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/></svg>';
  var WRITE_SVG = '<svg fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 20h9"/><path stroke-linecap="round" stroke-linejoin="round" d="M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4z"/></svg>';

  function triggerIcon(et) {
    if (et === 'scheduled') return TRIG_ICONS.clock;
    if (et === 'compile_requested') return TRIG_ICONS.layers;
    if (/note/.test(et)) return TRIG_ICONS.note;
    return TRIG_ICONS.bolt;
  }

  function triggerName(t, i) { return t.name || ('Trigger ' + (i + 1)); }

  function triggerWhen(t, snap) {
    var et = (t.eventTypes || [])[0] || '';
    var label = snap.triggerLabel(et);
    if (!t.enabled) return 'Paused';
    if (et === 'scheduled' && t.schedule) return t.schedule.charAt(0).toUpperCase() + t.schedule.slice(1);
    if ((t.pathPrefixes || []).length) return label + ' · ' + t.pathPrefixes.join(', ');
    return label;
  }

  function pad2(n) { return (n < 10 ? '0' : '') + n; }

  function dayLabel(d, now) {
    var today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    var diff = Math.round((today - new Date(d.getFullYear(), d.getMonth(), d.getDate())) / 86400000);
    if (diff === 0) return 'Today';
    if (diff === 1) return 'Yesterday';
    var s = d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
    return d.getFullYear() === now.getFullYear() ? s : s + ', ' + d.getFullYear();
  }

  // One chip per note, a write outranking a read of the same note.
  function touchedNotes(msg) {
    var byPath = {}, order = [];
    (msg.segments || []).forEach(function (s) {
      if (s.type !== 'note_access') return;
      (s.items || []).forEach(function (it) {
        if (!(it.path in byPath)) order.push(it.path);
        if (byPath[it.path] !== 'write') byPath[it.path] = it.action;
      });
    });
    return order.map(function (p) { return { path: p, write: byPath[p] === 'write' }; });
  }

  function runText(msg) {
    return (msg.segments || [])
      .filter(function (s) { return s.type === 'text'; })
      .map(function (s) { return s.content || ''; })
      .join('').trim();
  }

  root.__hyloChatMount = function (rootEl, api) {
    var isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
    var thread = rootEl.querySelector('#chat-thread');
    var scrollEl = rootEl.querySelector('#chat-scroll');
    var panel = rootEl.querySelector('#chat-input-outer');
    var ta = rootEl.querySelector('#chat-textarea');
    var hint = rootEl.querySelector('#chat-input-hint');
    var hintName = rootEl.querySelector('#chat-hint-name');
    var ctxRow = rootEl.querySelector('#chat-context');
    var sendBtn = rootEl.querySelector('#chat-send');
    var attachBtn = rootEl.querySelector('#chat-add');
    var attachMenu = rootEl.querySelector('#chat-add-menu');
    var attachImage = rootEl.querySelector('#chat-add-image');
    var fileInput = rootEl.querySelector('#chat-file-input');
    var pendingImages = rootEl.querySelector('#chat-pending-images');
    var inputCard = rootEl.querySelector('#chat-input-card');
    var imageSig = '';
    var stopBtn = rootEl.querySelector('#chat-stop');
    var newBtn = rootEl.querySelector('#chat-new-btn');
    var jumpBtn = rootEl.querySelector('#chat-jump');
    var descEl = rootEl.querySelector('#chat-bot-desc');
    var seg = rootEl.querySelector('#chat-conv-seg');
    var triggersBtn = rootEl.querySelector('#chat-triggers-btn');
    var spacer = el('div', 'chat-scroll-spacer');
    spacer.id = 'chat-scroll-spacer';
    spacer.setAttribute('aria-hidden', 'true');
    thread.appendChild(spacer);

    var rows = new Map();
    // Rows of conversations switched away from, kept built so coming back doesn't rebuild them.
    var shelf = new Map();
    var SHELF_MAX = 400;
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
    var contentRo = null;
    var scrollRaf = 0;
    var timeTimer = 0;
    var emptyKind = '';
    var runsEl = null;
    var runsSig = '';
    var triggerMode = false;

    function onCopied(id, onIcon) {
      var row = rows.get(id);
      var btn = row && row.querySelector('.msg-copy-btn');
      setCopyIcon(btn, onIcon);
    }
    copiedListener = onCopied;

    // The thread is either stuck to the bottom or holding the reader's place
    // (the top row in view) through every reflow. Only the reader's own
    // scrolling switches between them: rebuilt rows and settling cards fire
    // scroll events too, and those must not read as scrolling away.
    var intentAt = -Infinity;
    var dragging = false;
    var lastTop = 0;
    var SCROLL_KEYS = { ArrowUp: 1, ArrowDown: 1, PageUp: 1, PageDown: 1, Home: 1, End: 1, ' ': 1 };

    function markIntent() { intentAt = performance.now(); }
    function userScrolling() { return dragging || performance.now() - intentAt < 600; }

    var glide = null;

    // Each state scrolls from the end it holds still: a stuck thread from the
    // bottom (column-reverse, 0 is the bottom and up is negative) and a reader's
    // place from the top, so a streaming reply moves neither and nothing writes
    // scrollTop per token. Only switching states moves the origin.
    function fromBottom() { return reversed() ? -scrollEl.scrollTop : scrollEl.scrollHeight - scrollEl.clientHeight - scrollEl.scrollTop; }
    function fromTop() { return scrollEl.scrollHeight - scrollEl.clientHeight - fromBottom(); }
    function reversed() { return scrollEl.classList.contains('is-stuck'); }

    function syncOrigin() {
      var want = api.stick() && !triggerMode;
      if (reversed() === want) return;
      var top = fromTop();
      scrollEl.classList.toggle('is-stuck', want);
      scrollEl.scrollTop = want ? 0 : top;
    }

    // A glide in progress owns the scroll position until it lands.
    function pin() {
      if (glide) return;
      syncOrigin();
      if (scrollEl.scrollTop !== 0) scrollEl.scrollTop = 0;
    }

    function captureAnchor() {
      var top = scrollEl.getBoundingClientRect().top;
      for (var n = thread.firstElementChild; n; n = n.nextElementSibling) {
        if (!n._msgId) continue;
        var r = n.getBoundingClientRect();
        if (r.bottom > top) { api.setAnchor({ id: n._msgId, top: r.top - top }); return; }
      }
    }

    function holdAnchor() {
      var a = api.anchor();
      var row = a && rows.get(a.id);
      if (!row || row.parentNode !== thread) return false;
      var delta = row.getBoundingClientRect().top - scrollEl.getBoundingClientRect().top - a.top;
      // A relative scroll adds to WebKit's in-flight position; assigning scrollTop would
      // overwrite it and drop a compensation still on its way.
      if (Math.abs(delta) >= 1) scrollEl.scrollBy(0, delta);
      return true;
    }

    // After anything that can move content: a render, a card or image settling, a resize.
    function settle() {
      if (!scrollEl || triggerMode || detaching || !rows.size) return;
      // Rows landing mid-glide would shove the view up; the glide places it each time.
      if (glide) { placeGlide(); return; }
      // A place that's gone (another chat, a reload that dropped older pages) falls back to the bottom.
      if (!api.stick() && !holdAnchor()) api.setStick(true);
      if (api.stick()) pin();
      else syncOrigin();
      lastTop = scrollEl.scrollTop;
      updateJump();
    }

    function jumpToBottom() {
      api.setStick(true);
      settle();
    }

    // A choices card answers to the message right after its own, so it's live only on
    // the newest reply in a chat; once answered it shows what was picked.
    function choiceState(snap, card) {
      var row = card.closest('.msg-row');
      var id = row && row._msgId;
      var msgs = snap.messages;
      var i = msgs.length - 1;
      while (i >= 0 && (msgs[i].id || ('i' + i)) !== id) i--;
      if (i < 0 || msgs[i].role !== 'assistant') return { mode: 'static' };
      var next = msgs[i + 1];
      if (next) return { mode: 'answered', picked: next.role === 'user' ? next.content || '' : '' };
      if (snap.convType === 'chat' && !snap.busy && msgs[i].status !== 'running') return { mode: 'active' };
      return { mode: 'answered' };
    }
    // A receipt's undo stays usable on older replies, since its text names what it
    // undoes; it's done once a later message from the user says exactly that.
    function receiptState(snap, card) {
      var row = card.closest('.msg-row');
      var id = row && row._msgId;
      var msgs = snap.messages;
      var undo = (card._hyReceipt.undo() || '').trim();
      var i = msgs.length - 1;
      while (i >= 0 && (msgs[i].id || ('i' + i)) !== id) i--;
      if (i < 0 || msgs[i].role !== 'assistant' || !undo) return { mode: 'static' };
      for (var j = i + 1; j < msgs.length; j++) {
        if (msgs[j].role === 'user' && (msgs[j].content || '').trim() === undo) return { mode: 'done' };
      }
      if (snap.convType !== 'chat') return { mode: 'static' };
      var last = msgs[msgs.length - 1];
      return { mode: snap.busy || (last && last.status === 'running') ? 'busy' : 'active' };
    }
    function cardState(snap, card) {
      return card._hyReceipt ? receiptState(snap, card) : choiceState(snap, card);
    }
    // Scanned rather than registered: a card mounted while the thread was out of
    // the page never got to announce itself.
    function syncChoices(snap) {
      thread.querySelectorAll('.hy-choices-host').forEach(function (card) {
        if (card._hyChoices) card._hyChoices.setState(choiceState(snap, card));
      });
      thread.querySelectorAll('.hy-receipt-host').forEach(function (card) {
        if (card._hyReceipt) card._hyReceipt.setState(receiptState(snap, card));
      });
    }
    on(thread, 'hylo:choices-mount', function () {
      syncChoices(api.state());
    });
    on(thread, 'hylo:choice', function (e) {
      if (cardState(api.state(), e.target).mode !== 'active') return;
      armMorph(e.target, e.detail.text);
      void api.sendChoice(e.detail.text);
    });

    // A card answer travels: the reply bubble rises out of the button that sent it
    // while the thread scrolls smoothly to it, instead of jumping a screen. The glide
    // runs in distance from the top, toward the bottom as it is each frame, so rows
    // that land mid-way (the reply's placeholder) join the same motion.
    var morph = null;
    var GLIDE_MS = 380;
    var reduceMotion = root.matchMedia && root.matchMedia('(prefers-reduced-motion: reduce)');

    function armMorph(card, text) {
      if (!scrollEl || (reduceMotion && reduceMotion.matches)) return;
      var from = null;
      card.querySelectorAll('button').forEach(function (b) {
        if (!from && b.textContent.trim() === text) from = b;
      });
      var seen = {};
      rows.forEach(function (row, id) { seen[id] = true; });
      morph = {
        text: text, seen: seen, top: fromTop(), at: performance.now(),
        from: (from || card).getBoundingClientRect(),
      };
    }

    function startMorph() {
      if (!morph) return;
      if (performance.now() - morph.at > 2000) { morph = null; return; }
      var row = null;
      rows.forEach(function (r, id) {
        if (!morph.seen[id] && r.classList.contains('msg-row-user') && r.textContent.trim() === morph.text) row = r;
      });
      if (!row) return;
      var bubble = row.querySelector('.msg-user-bubble');
      endGlide();
      glide = { from: morph.top, t0: performance.now(), bubble: bubble, chip: morph.from, raf: 0 };
      morph = null;
      placeGlide();
      glide.raf = requestAnimationFrame(stepGlide);
    }

    function endGlide() {
      if (!glide) return;
      var g = glide;
      cancelAnimationFrame(g.raf);
      g.bubble.style.transform = '';
      g.bubble.style.transformOrigin = '';
      glide = null;
      if (!userScrolling()) settle();
    }

    function stepGlide() {
      var g = glide;
      if (!g) return;
      // Done, the reader took over, or the row left (another chat opened).
      if (performance.now() - g.t0 >= GLIDE_MS || userScrolling() || !g.bubble.isConnected) { endGlide(); return; }
      placeGlide();
      g.raf = requestAnimationFrame(stepGlide);
    }

    function placeGlide() {
      var g = glide;
      var t = Math.min(1, (performance.now() - g.t0) / GLIDE_MS);
      var e = 1 - Math.pow(1 - t, 3);
      var bottom = scrollEl.scrollHeight - scrollEl.clientHeight;
      var top = g.from + (bottom - g.from) * e;
      scrollEl.scrollTop = reversed() ? top - bottom : top;
      var left = fromBottom();
      g.bubble.style.transform = '';
      var r = g.bubble.getBoundingClientRect();
      var c = g.chip;
      // Centre to centre, toward where the bubble rests once the scroll lands. The
      // vertical leg starts slower than the horizontal one, so the path arcs.
      var ey = t < 0.5 ? 4 * t * t * t : 1 - Math.pow(2 - 2 * t, 3) / 2;
      var dx = c.left + c.width / 2 - (r.left + r.width / 2);
      var dy = c.top + c.height / 2 - (r.top - left + r.height / 2);
      var s0 = Math.min(1, c.height / r.height);
      g.bubble.style.transformOrigin = 'center';
      g.bubble.style.transform = 'translate(' + dx * (1 - e) + 'px,' + (dy * (1 - ey) - left) + 'px) scale(' + (s0 + (1 - s0) * e) + ')';
    }

    function onContentHydrated() {
      if (triggerMode) measureRuns();
      else settle();
    }
    hydrateListener = onContentHydrated;
    root.__hyloPreloadFenceCards();

    function updateJump(snap) {
      var st = snap || api.state();
      show(jumpBtn, st.convType === 'chat' && !st.stick && st.messages.length > 0);
    }

    function syncComposerBox() {
      if (!panel || !scrollEl || panel.style.display === 'none') return;
      var sbw = scrollEl.offsetWidth - scrollEl.clientWidth;
      scrollEl.style.setProperty('--chat-scrollbar-w', (Math.ceil(sbw) + 4) + 'px');
      scrollEl.style.setProperty('--chat-composer-h', panel.offsetHeight + 'px');
      settle();
    }

    function armComposerObserver() {
      if (ro) { ro.disconnect(); ro = null; }
      if (!panel || !scrollEl || typeof ResizeObserver === 'undefined') return;
      if (panel.style.display === 'none') return;
      ro = new ResizeObserver(syncComposerBox);
      ro.observe(panel);
      syncComposerBox();
    }

    function paintImages(snap) {
      if (!pendingImages) return;
      var imgs = snap.images || [];
      var sig = imgs.map(function (i) {
        return [i.localId, i.uploading ? 1 : 0, i.failed ? 1 : 0, i.id || ''].join(':');
      }).join('|');
      if (inputCard) inputCard.classList.toggle('has-images', imgs.length > 0);
      if (sig === imageSig) return;
      imageSig = sig;
      pendingImages.replaceChildren();
      pendingImages.hidden = imgs.length === 0;
      imgs.forEach(function (item) {
        var box = el('div', 'compose-thumb' + (item.uploading ? ' is-uploading' : '') + (item.failed ? ' is-failed' : ''));
        var img = el('img');
        img.src = item.previewUrl;
        img.alt = item.name || 'image';
        box.appendChild(img);
        if (item.uploading) box.appendChild(el('div', 'compose-thumb-spin'));
        var rm = el('button', 'compose-thumb-remove');
        rm.type = 'button';
        rm.title = 'Remove image';
        rm.setAttribute('aria-label', 'Remove image');
        rm.innerHTML = '<svg fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" viewBox="0 0 24 24"><path d="M18 6L6 18"/><path d="M6 6l12 12"/></svg>';
        rm.addEventListener('click', function () { api.removeImage(item.localId); });
        box.appendChild(rm);
        pendingImages.appendChild(box);
      });
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
      if (newBtn) {
        show(newBtn, snap.convType === 'chat');
        newBtn.disabled = busy || !snap.mateId || snap.messages.length === 0;
      }
      show(triggersBtn, snap.convType === 'trigger' && !!snap.mateId);
      show(panel, snap.convType === 'chat' && !(snap.matesLoaded && !snap.mates.length));
      if (spacer) {
        if (snap.convType !== 'chat') spacer.style.height = '2rem';
        else spacer.style.height = '';
      }
      if (hintName) {
        hintName.textContent = mate ? ('Ask ' + mate.name + '…') : 'Select a bot from the sidebar';
      }
      if (ta) {
        // Renders run per streamed token; the chips only depend on these two.
        var ctxStale = ctxKey === null || ta.disabled !== !snap.mateId;
        ta.disabled = !snap.mateId;
        if (document.activeElement !== ta && ta.value !== snap.draft) {
          ta.value = snap.draft;
          root.__hyloAutoResize(ta);
          ctxStale = true;
        }
        show(hint, !ta.value);
        if (ctxStale) refreshContext();
      }
      if (attachBtn) {
        attachBtn.disabled = !snap.mateId;
        if (attachBtn.disabled) setAttachOpen(false);
      }
      paintImages(snap);
      if (sendBtn) {
        show(sendBtn, !live);
        sendBtn.disabled = !snap.canSend;
      }
      if (stopBtn) {
        show(stopBtn, live);
        stopBtn.disabled = !snap.runId || snap.phase === 'stopping';
      }
      updateJump(snap);
    }

    function retire(id) {
      var row = rows.get(id);
      rows.delete(id);
      if (!row) return;
      if (row.parentNode) row.parentNode.removeChild(row);
      shelf.delete(id);
      shelf.set(id, row);
      if (shelf.size > SHELF_MAX) {
        var oldest = shelf.keys().next().value;
        root.__hyloReleaseFenceCards(shelf.get(oldest));
        shelf.delete(oldest);
      }
    }

    function unshelve(id) {
      var row = shelf.get(id);
      if (!row) return null;
      shelf.delete(id);
      rows.set(id, row);
      // Its retry button and time label missed the renders while it was away.
      lastBusy = null;
      return row;
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
      if (snap.mateId && !snap.loaded) {
        Array.from(rows.keys()).forEach(retire);
        var pendingEmpty = rootEl.querySelector('.chat-empty');
        if (pendingEmpty) pendingEmpty.remove();
        emptyKind = '';
        return;
      }
      if (!snap.messages.length) {
        Array.from(rows.keys()).forEach(retire);
        var kind = snap.mate ? ('card:' + snap.mate.name) : (snap.matesLoaded && !snap.mates.length ? 'setup' : 'plain');
        var empty = rootEl.querySelector('.chat-empty');
        if (emptyKind !== kind) {
          if (empty) empty.remove();
          empty = buildEmpty(snap, isMac);
          // The setup page is a section empty state, so it hangs off the .sec.
          if (kind === 'setup') rootEl.insertBefore(empty, scrollEl.nextSibling);
          else thread.insertBefore(empty, spacer);
          emptyKind = kind;
        }
        return;
      }
      var emptyNow = rootEl.querySelector('.chat-empty');
      if (emptyNow) emptyNow.remove();
      emptyKind = '';
      var seen = {};
      var prev = null;
      var returned = false;
      for (var i = 0; i < snap.messages.length; i++) {
        var msg = snap.messages[i];
        var id = msg.id || ('i' + i);
        seen[id] = true;
        var row = rows.get(id);
        if (!row && msg.id && (row = unshelve(id))) returned = true;
        var stopBit = snap.stoppingId === id ? 1 : 0;
        if (row && row._epoch === snap.catalogEpoch && row._rev === msg.rev && row._stop === stopBit) {
          prev = place(row, prev);
          continue;
        }
        var sig = rowSig(msg, snap);
        var same = row && row._epoch === snap.catalogEpoch && row._struct === sig;
        if (!(same && row._body && patchSegments(row._body, msg))) {
          var fresh = msg.role === 'user' ? buildUser(msg) : buildAssistant(msg, snap, api);
          if (row) root.__hyloReleaseFenceCards(row);
          if (row && row.parentNode) row.parentNode.replaceChild(fresh, row);
          row = fresh;
          rows.set(id, row);
          row._struct = sig;
          row._epoch = snap.catalogEpoch;
        }
        row._rev = msg.rev;
        row._stop = stopBit;
        row._msgId = id;
        prev = place(row, prev);
      }
      if (spacer.parentNode !== thread || (prev && prev.nextSibling !== spacer)) thread.appendChild(spacer);
      rows.forEach(function (row, id) { if (!seen[id]) retire(id); });
      syncChoices(snap);
      if (returned) updateTimes();
    }

    function openTriggerSettings() {
      window.dispatchEvent(new CustomEvent('hylo:open-settings', { detail: { tab: 'agent-bots' } }));
    }

    function buildTriggerCards(snap) {
      var grid = el('div', 'trig-cards');
      snap.triggers.forEach(function (t, i) {
        var et = (t.eventTypes || [])[0] || '';
        var card = el('button', 'trig-card' + (snap.triggerFilter === t.id ? ' active' : '') + (t.enabled ? '' : ' is-paused'));
        card.type = 'button';
        card.setAttribute('aria-pressed', snap.triggerFilter === t.id ? 'true' : 'false');
        var head = el('div', 'trig-card-head');
        var icon = el('span', 'trig-card-icon');
        icon.appendChild(svg(triggerIcon(et)));
        head.appendChild(icon);
        var name = el('span', 'trig-card-name');
        name.textContent = triggerName(t, i);
        head.appendChild(name);
        card.appendChild(head);
        var when = el('div', 'trig-card-when');
        when.textContent = triggerWhen(t, snap);
        card.appendChild(when);
        card.title = (t.prompt || '').slice(0, 300);
        card.addEventListener('click', function () { api.setTriggerFilter(t.id); });
        grid.appendChild(card);
      });
      return grid;
    }

    function buildRun(msg, snap, names, last) {
      var status = msg.status || 'succeeded';
      var row = el('div', 'trig-run');
      var at = msg.createdAt || R.msgAt(msg);
      var time = el('div', 'trig-run-time');
      if (at) { var d = new Date(at); time.textContent = pad2(d.getHours()) + ':' + pad2(d.getMinutes()); }
      row.appendChild(time);
      var rail = el('div', 'trig-run-rail');
      rail.appendChild(el('span', 'trig-run-dot is-' + status));
      if (!last) rail.appendChild(el('span', 'trig-run-line'));
      row.appendChild(rail);

      var main = el('div', 'trig-run-main');
      var head = el('div', 'trig-run-head');
      var name = el('span', 'trig-run-name');
      name.textContent = names[msg.triggerId] || (msg.triggerEvent ? snap.triggerLabel(msg.triggerEvent) : 'Trigger');
      head.appendChild(name);
      if (status === 'running') {
        var st = el('span', 'trig-run-status');
        st.textContent = 'Running';
        head.appendChild(st);
      }
      main.appendChild(head);

      var text = runText(msg);
      if (status === 'running' && !text) {
        var work = el('div', 'msg-thinking-label');
        work.appendChild(document.createTextNode('Working'));
        var dots = el('span', 'thinking-dots');
        dots.appendChild(el('span')); dots.appendChild(el('span')); dots.appendChild(el('span'));
        work.appendChild(dots);
        main.appendChild(work);
      }
      var body = null;
      if (text) {
        body = el('div', 'trig-run-body prose' + (runOpen.has(msg.id) ? '' : ' is-clamped'));
        setProse(body, text, status === 'running');
        main.appendChild(body);
      }
      var banner = statusBanner(msg, snap, api);
      if (banner) main.appendChild(banner);

      if (status !== 'running') {
        var foot = el('div', 'trig-run-foot');
        if (body) {
          var more = el('button', 'trig-run-more');
          more.type = 'button';
          more.textContent = runOpen.has(msg.id) ? 'Show less' : 'Show more';
          more.hidden = true;
          more.addEventListener('click', function () {
            var on = !runOpen.has(msg.id);
            if (on) runOpen.add(msg.id); else runOpen.delete(msg.id);
            body.classList.toggle('is-clamped', !on);
            more.textContent = on ? 'Show less' : 'Show more';
          });
          foot.appendChild(more);
          body._more = more;
        }
        var notes = touchedNotes(msg);
        var chips = [];
        notes.forEach(function (n) {
          var stem = noteStem(n.path);
          var chip = el('button', 'trig-note' + (n.write ? ' is-write' : ''));
          chips.push(chip);
          chip.type = 'button';
          chip.title = (n.write ? 'Wrote ' : 'Read ') + n.path;
          chip.appendChild(svg(n.write ? WRITE_SVG : READ_SVG));
          chip.appendChild(document.createTextNode(stem));
          chip.addEventListener('click', function () {
            if (window.__hyloContentPane) void window.__hyloContentPane.openNoteInContentPane(n.path, stem, false, false);
          });
          foot.appendChild(chip);
        });
        // Same rule as the chat's note cards: a note gone from the vault gets no chip.
        if (notes.length) {
          fetchNoteInfo(notes.map(function (n) { return n.path; })).then(function () {
            notes.forEach(function (n, k) { if (noteInfoCache[n.path] === null) chips[k].remove(); });
          });
        }
        var took = status === 'succeeded' && msg.createdAt && msg.completedAt ? fmtDur(msg.completedAt - msg.createdAt) : '';
        if (took) {
          var dur = el('span', 'trig-run-dur');
          dur.textContent = took;
          foot.appendChild(dur);
        }
        if (foot.firstChild) main.appendChild(foot);
      }
      row.appendChild(main);
      return row;
    }

    // "Show more" only where the clamp actually hides something.
    function measureRuns() {
      if (!runsEl) return;
      runsEl.querySelectorAll('.trig-run-body').forEach(function (body) {
        if (!body._more || !body.classList.contains('is-clamped')) return;
        body._more.hidden = body.scrollHeight <= body.clientHeight + 2;
      });
    }

    function renderRuns(snap) {
      var runs = [];
      for (var i = snap.messages.length - 1; i >= 0; i--) {
        var m = snap.messages[i];
        if (m.role !== 'assistant') continue;
        if (snap.triggerFilter && m.triggerId !== snap.triggerFilter) continue;
        runs.push(m);
      }
      var sig = [snap.mateId, snap.triggerFilter, snap.catalogEpoch, snap.busy ? 1 : 0, snap.triggersLoaded ? 1 : 0, snap.hasOlder ? 1 : 0,
        JSON.stringify(snap.triggers.map(function (t) { return [t.id, t.name, t.enabled, t.eventTypes, t.schedule, t.pathPrefixes]; })),
        new Date().toDateString()];
      runs.forEach(function (m) { sig.push(m.id + ':' + m.rev); });
      sig = sig.join('|');
      if (runsEl && runsSig === sig && runsEl.parentNode === thread) return;
      runsSig = sig;
      var scrollTop = scrollEl && runsEl && runsEl._mate === snap.mateId ? scrollEl.scrollTop : 0;
      if (runsEl) { root.__hyloReleaseFenceCards(runsEl); runsEl.remove(); }
      runsEl = el('div', 'trig-log');
      runsEl._mate = snap.mateId;
      thread.insertBefore(runsEl, spacer);
      // Names come with the triggers; drawing runs before them would relabel every row.
      if (!snap.triggersLoaded) return;

      if (!snap.triggers.length && !runs.length && !snap.hasOlder) {
        var empty = el('div', 'empty-state trig-empty');
        var title = el('div', 'empty-state-title');
        title.textContent = 'No runs yet';
        empty.appendChild(title);
        var desc = el('div', 'empty-state-desc');
        desc.textContent = 'Add a trigger to run this bot on a schedule or when notes change. Every run shows up here.';
        empty.appendChild(desc);
        var actions = el('div', 'empty-state-actions');
        var manage = el('button', 'btn-outline');
        manage.type = 'button';
        manage.textContent = 'Manage triggers';
        manage.addEventListener('click', openTriggerSettings);
        actions.appendChild(manage);
        empty.appendChild(actions);
        runsEl.appendChild(empty);
        return;
      }

      if (snap.triggers.length) runsEl.appendChild(buildTriggerCards(snap));
      var names = {};
      snap.triggers.forEach(function (t, i) { names[t.id] = triggerName(t, i); });

      var now = new Date();
      var day = null, dayKey = '';
      runs.forEach(function (m, idx) {
        var d = new Date(m.createdAt || R.msgAt(m) || Date.now());
        var key = d.toDateString();
        if (key !== dayKey) {
          dayKey = key;
          day = el('section', 'trig-day');
          var h = el('div', 'trig-day-head');
          var label = el('span', 'trig-day-label');
          label.textContent = dayLabel(d, now);
          h.appendChild(label);
          h.appendChild(el('span', 'trig-day-rule'));
          day.appendChild(h);
          runsEl.appendChild(day);
        }
        var next = runs[idx + 1];
        var last = !next || new Date(next.createdAt || R.msgAt(next) || Date.now()).toDateString() !== key;
        day.appendChild(buildRun(m, snap, names, last));
      });
      if (!runs.length && !snap.hasOlder) {
        var none = el('div', 'trig-none');
        none.textContent = snap.triggerFilter ? 'No runs for this trigger yet.' : 'No runs yet.';
        runsEl.appendChild(none);
      }
      if (scrollEl) scrollEl.scrollTop = scrollTop;
      requestAnimationFrame(measureRuns);
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

    function render() {
      paint();
      restoreLostFocus();
      takeFocus();
    }

    function paint() {
      var snap = api.state();
      renderChrome(snap);
      var wasTrigger = triggerMode;
      triggerMode = snap.convType === 'trigger';
      if (triggerMode) {
        if (scrollEl) syncOrigin();
        if (!wasTrigger) {
          Array.from(rows.keys()).forEach(retire);
          var stale = rootEl.querySelector('.chat-empty');
          if (stale) stale.remove();
          emptyKind = '';
          if (scrollEl) scrollEl.scrollTop = 0;
        }
        renderRuns(snap);
        updateJump(snap);
        // A narrowed list can come up short; keep paging until it fills or runs out.
        if (snap.hasOlder && scrollEl && scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight < 400) void api.loadOlder();
        return;
      }
      if (runsEl) { root.__hyloReleaseFenceCards(runsEl); runsEl.remove(); runsEl = null; runsSig = ''; }
      renderThread(snap);
      syncRetry(snap);
      settle();
      startMorph();
      updateJump();
    }

    function onScroll() {
      if (scrollEl && !triggerMode && rows.size) {
        var wasStuck = api.stick();
        if (fromBottom() <= 64) api.setStick(true);
        else if (api.stick() && userScrolling() && scrollEl.scrollTop < lastTop - 1) api.setStick(false);
        // Stuck but shoved off the bottom by a reflow the observer hasn't seen yet.
        else if (api.stick()) pin();
        syncOrigin();
        // The reader moves the place only by how far they scrolled. Read from layout, it
        // would also take in rows above that grew since the last settle, and the stale
        // position WebKit reports right after our own scrollTop write.
        if (!api.stick()) {
          var a = api.anchor();
          if (wasStuck || !a) captureAnchor();
          else {
            if (userScrolling()) api.setAnchor({ id: a.id, top: a.top - (scrollEl.scrollTop - lastTop) });
            if (holdAnchor() && userScrolling()) captureAnchor();
          }
        }
        lastTop = scrollEl.scrollTop;
        updateJump();
      }
      if (scrollRaf) return;
      scrollRaf = requestAnimationFrame(function () {
        scrollRaf = 0;
        if (!scrollEl) return;
        if (triggerMode) {
          if (scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight < 400) void api.loadOlder();
          return;
        }
        if (fromTop() < 400) void api.loadOlder();
      });
    }

    if (scrollEl) {
      on(scrollEl, 'scroll', onScroll, { passive: true });
      on(scrollEl, 'wheel', markIntent, { passive: true });
      on(scrollEl, 'touchstart', markIntent, { passive: true });
      on(scrollEl, 'touchmove', markIntent, { passive: true });
      // A press on the scroller itself, not its content, is a scrollbar drag.
      on(scrollEl, 'pointerdown', function (e) { if (e.target === scrollEl) dragging = true; });
      on(window, 'pointerup', function () { if (dragging) { dragging = false; markIntent(); } });
      on(window, 'keydown', function (e) {
        var t = e.target;
        if (SCROLL_KEYS[e.key] && !(t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)))) markIntent();
      });
      if (typeof ResizeObserver !== 'undefined') {
        contentRo = new ResizeObserver(settle);
        contentRo.observe(thread);
        contentRo.observe(scrollEl);
      }
      on(scrollEl, 'click', function (e) {
        var a = e.target.closest && e.target.closest('a');
        if (!a) return;
        var href = a.getAttribute('href') || '';
        if (href.indexOf('/notes?') !== 0) return;
        e.preventDefault();
        try {
          var name = new URLSearchParams(href.split('?')[1] || '').get('name') || '';
          if (name && typeof root.__hyloContentPaneOpenWikiLink === 'function') {
            void root.__hyloContentPaneOpenWikiLink(name.replace(/\.md$/, ''));
          }
        } catch (err) { /* ignore */ }
      });
    }

    if (ta && root.__hyloPathAcCreate) {
      ac = root.__hyloPathAcCreate({
        getInput: function () { return ta; },
        getList: function () { return rootEl.querySelector('#chat-path-ac'); },
        getPanel: function () { return rootEl.querySelector('#chat-ac-panel'); },
        getHint: function () { return rootEl.querySelector('#chat-path-ac-hint'); },
        parseCtx: chatAcParseCtx,
        search: chatMentionSearch,
        getDefaultItems: chatMentionDefaultItems,
        onApply: function (input, newVal, caretPos) {
          if (detaching) return;
          var cur = input.value;
          var p = 0;
          while (p < cur.length && p < newVal.length && cur[p] === newVal[p]) p++;
          var s = 0;
          while (s < cur.length - p && s < newVal.length - p && cur[cur.length - 1 - s] === newVal[newVal.length - 1 - s]) s++;
          root.__hyloTextareaInsert(input, p, cur.length - s, newVal.slice(p, newVal.length - s));
          input.setSelectionRange(caretPos, caretPos);
        },
        escKey: 'chat-ac',
      });
      on(ta, 'input', function () {
        if (detaching) return;
        api.setDraft(ta.value);
        root.__hyloAutoResize(ta);
        ac.refresh();
        show(hint, !ta.value);
        refreshContext();
        if (sendBtn) sendBtn.disabled = !api.state().canSend;
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
        if (e.isComposing || e.keyCode === 229) return;
        var hk = window.__hyloHotkeys;
        if (!hk) return;
        var combo = hk.comboOf(e);
        if (combo !== 'Enter' && !/\+Enter$/.test(combo || '')) return;
        var custom = hk.custom || {};
        if (combo === (custom['chat-send'] || 'Enter')) { e.preventDefault(); void api.send(); }
        // Enter/Shift+Enter already insert a newline natively; only that path
        // reveals a new trailing line in WebKit.
        else if (combo === (custom['chat-newline'] || 'Shift+Enter') && !/^(Shift\+)?Enter$/.test(combo)) {
          e.preventDefault();
          root.__hyloTextareaInsert(ta, ta.selectionStart, ta.selectionEnd, '\n');
        }
      });
    }

    function insertAtCaret(text, spaced) {
      var cur = ta ? ta.value : api.draft();
      var start = ta ? ta.selectionStart : cur.length;
      var end = ta ? ta.selectionEnd : start;
      var before = cur.slice(0, start);
      var after = cur.slice(end);
      var insert = text;
      if (spaced) {
        if (before.length > 0 && !/\s$/.test(before)) insert = ' ' + insert;
        if (after.length > 0 && !/^\s/.test(after)) insert += ' ';
      }
      if (ta) {
        root.__hyloTextareaInsert(ta, start, end, insert);
        return;
      }
      api.setDraft(before + insert + after);
      refreshContext();
    }
    function insertWikilink(stem) {
      if (stem) insertAtCaret('[[' + stem + ']]', true);
    }

    // Offers the note open in the pane, or the text selected in it, as one click
    // instead of an @ search. Read on demand: nothing here watches the editor.
    var usedSelection = '';
    function paneContext() {
      var pane = root.__hyloContentPane;
      var tab = pane && pane.contentPaneOpen && pane.tabs ? pane.tabs[pane.activeTab] : null;
      if (!tab || !tab.path) return null;
      var stem = noteStem(tab.path);
      var ed = root.__hyloEditor;
      var text = '';
      if (ed && ed.view && ed.currentPath === tab.path) {
        var sel = ed.view.state.selection.main;
        text = ed.view.state.sliceDoc(sel.from, sel.to).trim();
      }
      return { stem: stem, title: tab.title || stem, selection: text };
    }
    function contextChip(label, onClick) {
      var b = el('button', 'chat-context-chip');
      b.type = 'button';
      b.appendChild(el('span', 'chat-context-plus')).textContent = '+';
      b.appendChild(el('span', 'chat-context-label')).textContent = label;
      b.addEventListener('click', onClick);
      return b;
    }
    var ctxKey = null;
    function refreshContext() {
      if (!ctxRow || !ta) return;
      var c = ta.disabled ? null : paneContext();
      var draft = ta.value;
      var chips = [];
      if (c && c.selection && c.selection !== usedSelection) {
        var lines = c.selection.split('\n').filter(function (l) { return l.trim(); }).length;
        chips.push({ key: 's:' + c.selection, label: 'Selection · ' + lines + (lines === 1 ? ' line' : ' lines'), run: function () {
          usedSelection = c.selection;
          var quote = c.selection.split('\n').map(function (l) { return '> ' + l; }).join('\n');
          var lead = ta.value.slice(0, ta.selectionStart);
          insertAtCaret((lead && !/\n$/.test(lead) ? '\n' : '') + '[[' + c.stem + ']]\n' + quote + '\n\n');
        } });
      } else if (c && draft.indexOf('[[' + c.stem + ']]') === -1) {
        chips.push({ key: 'n:' + c.stem, label: c.title, run: function () { insertWikilink(c.stem); } });
      }
      var key = chips.map(function (x) { return x.key; }).join('|');
      if (key === ctxKey) return;
      ctxKey = key;
      ctxRow.replaceChildren.apply(ctxRow, chips.map(function (x) { return contextChip(x.label, x.run); }));
      ctxRow.hidden = !chips.length;
    }
    on(ta, 'focus', refreshContext);
    on(root, 'hylo:active-note', refreshContext);

    // The composer is the chat's resting focus: pointing at a control doesn't take
    // it, a plain click or typing hands it back, and a focused control that vanishes returns it.
    var lastFocus = null;
    // A request waits for the composer to be shown and enabled, unless focus goes elsewhere first.
    var wantFocus = false;
    function isEntry(el) {
      return !!(el && el.closest && el.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])'));
    }
    // WebKit leaves focus on an element that was hidden or removed, so that counts as nowhere too.
    function orphaned(a) {
      return !a || a === document.body || !a.isConnected || a.disabled || !a.getClientRects().length;
    }
    function focusIdle() {
      var a = document.activeElement;
      return orphaned(a) || (rootEl.contains(a) && !isEntry(a));
    }
    function focusComposer() {
      wantFocus = true;
      takeFocus();
    }
    function takeFocus() {
      if (!wantFocus || !ta || ta.disabled || !rootEl.isConnected || !ta.getClientRects().length) return;
      wantFocus = false;
      if (document.activeElement !== ta) ta.focus({ preventScroll: true });
    }
    on(document, 'focusin', function (e) { if (e.target !== ta) wantFocus = false; });
    // Whatever closed over the chat (a dialog, the note pane) and has nowhere to put focus hands it here.
    root.__hyloFocusHome = focusComposer;
    on(rootEl, 'mousedown', function (e) {
      if (e.button !== 0 || isEntry(e.target)) return;
      if (e.target.closest('button, a[href], [role="button"]')) e.preventDefault();
    });
    on(rootEl, 'click', function (e) {
      if (e.detail === 0) return;
      var sel = window.getSelection && window.getSelection();
      if (sel && !sel.isCollapsed) return;
      if (focusIdle()) focusComposer();
    });
    // Typing after selecting text in the thread, where focus rests on the body.
    on(document, 'keydown', function (e) {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key.length !== 1 && e.keyCode !== 229) return;
      if (!orphaned(document.activeElement) || !rootEl.isConnected) return;
      if (ta && ta.offsetParent) focusComposer();
    });
    on(rootEl, 'focusin', function (e) { lastFocus = e.target; });
    on(rootEl, 'focusout', function (e) {
      if (!orphaned(e.target)) lastFocus = null;
    });
    function restoreLostFocus() {
      var gone = lastFocus;
      if (!gone || gone === ta || !orphaned(gone)) return;
      lastFocus = null;
      if (focusIdle()) focusComposer();
    }

    function setAttachOpen(open) {
      if (!attachMenu || !attachBtn || open === !attachMenu.hidden) return;
      attachMenu.hidden = !open;
      attachBtn.classList.toggle('is-open', open);
      attachBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
      if (open) { if (root.__hyloEscPush) root.__hyloEscPush('chat-add-menu', function () { setAttachOpen(false); }); }
      else if (root.__hyloEscPop) root.__hyloEscPop('chat-add-menu');
    }
    on(sendBtn, 'click', function () { void api.send(); });
    on(attachBtn, 'click', function () { setAttachOpen(attachMenu.hidden); });
    on(attachImage, 'click', function () {
      setAttachOpen(false);
      if (fileInput) fileInput.click();
    });
    on(document, 'pointerdown', function (e) {
      if (!attachMenu || attachMenu.hidden) return;
      if (attachBtn && (attachBtn === e.target || attachBtn.contains(e.target))) return;
      if (attachMenu.contains(e.target)) return;
      setAttachOpen(false);
    });
    on(fileInput, 'change', function () {
      var files = Array.prototype.slice.call(fileInput.files || []);
      fileInput.value = '';
      if (files.length) api.addImages(files);
    });
    on(ta, 'paste', function (e) {
      var files = (root.__hyloImageFilesFromClipboard && root.__hyloImageFilesFromClipboard(e.clipboardData)) || [];
      if (!files.length) return;
      e.preventDefault();
      api.addImages(files);
    });
    on(stopBtn, 'click', function () { void api.cancel(); });
    on(newBtn, 'click', function () { void api.newChat(); });
    on(triggersBtn, 'click', openTriggerSettings);
    on(jumpBtn, 'click', jumpToBottom);
    if (seg) {
      on(seg, 'click', function (e) {
        var btn = e.target.closest && e.target.closest('.seg-btn');
        if (!btn || btn.disabled) return;
        api.setConvType(btn.getAttribute('data-conv'));
      });
    }

    if (ta) {
      ta.value = api.draft();
      root.__hyloAutoResize(ta);
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
      focusComposer: focusComposer,
      insertWikilink: insertWikilink,
      destroy: function () {
        detaching = true;
        setAttachOpen(false);
        endGlide();
        root.__hyloReleaseFenceCards(thread);
        shelf.forEach(function (row) { root.__hyloReleaseFenceCards(row); });
        shelf.clear();
        if (root.__hyloFocusHome === focusComposer) root.__hyloFocusHome = null;
        if (copiedListener === onCopied) copiedListener = null;
        if (hydrateListener === onContentHydrated) hydrateListener = null;
        listen.abort();
        if (timeTimer) { clearInterval(timeTimer); timeTimer = 0; }
        if (ro) { ro.disconnect(); ro = null; }
        if (contentRo) { contentRo.disconnect(); contentRo = null; }
        if (ac) {
          try { ac.close(); } catch (e) { /* ignore */ }
          ac = null;
        }
        detaching = false;
      },
    };
  };
})(typeof window !== 'undefined' ? window : global);
