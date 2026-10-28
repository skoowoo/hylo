// Chat pane. Mounted while #chat-root is in the document and discarded on
// HTMX swap. Message rows come from chat_rows.js; this keeps them in order and
// owns the scroll model, the composer and the trigger tab's run log.
(function (root) {
  var R = root.__hyloChatReduce;
  var Rows = root.__hyloChatRows;

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

  function show(node, on) {
    if (!node) return;
    node.style.display = on ? '' : 'none';
  }

  // A run's body re-renders whole; chat rows render block by block (chat_rows.js).
  function setProse(node, text, streaming) {
    if (streaming) text = Rows.streamable(text);
    var openTail = !!streaming && Rows.endsInOpenFence(text);
    if (node._raw === text && node._openTail === openTail) return;
    node._raw = text;
    node._openTail = openTail;
    node.innerHTML = root.__hyloRenderMarkdown(text, { cache: !streaming });
    root.__hyloHydrateFenceCards(node, { openTail: openTail });
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
    clock: hyloIcon('recent'),
    note: hyloIcon('file'),
    layers: hyloIcon('layers'),
    bolt: hyloIcon('zap'),
  };
  var READ_SVG = hyloIcon('eye');
  var WRITE_SVG = hyloIcon('draft');

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
    var composerCol = rootEl.querySelector('.chat-ac-wrap');
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

    var rows = new Map(); // message id -> row (chat_rows.js)
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
    var taRO = null;
    var contentRo = null;
    var scrollRaf = 0;
    var timeTimer = 0;
    var emptyKind = '';
    var runsEl = null;
    var runsSig = '';
    var triggerMode = false;

    function onCopied(id, onIcon) {
      var row = rows.get(id);
      if (row && row.copyBtn) Rows.setCopyIcon(row.copyBtn, onIcon);
    }
    Rows.onCopied(onCopied);

    // The thread is either stuck to the bottom or holding the reader's place
    // (the top row in view) through every reflow. Only the reader's own
    // scrolling switches between them: rebuilt rows and settling cards fire
    // scroll events too, and those must not read as scrolling away.
    var intentAt = -Infinity;
    var dragging = false;
    var lastTop = 0;
    var SCROLL_KEYS = { ArrowUp: 1, ArrowDown: 1, PageUp: 1, PageDown: 1, Home: 1, End: 1, ' ': 1 };

    // While the reader's gesture (or its momentum) is in flight, WKWebView's UI
    // process owns the position and keeps applying it in the coordinates it began
    // in: an absolute scrollTop write lands under it, and flipping the origin
    // mid-flight threw the reader to the top. So absolute writes wait for the
    // gesture to end; until then only relative scrollBy compensates.
    var INTENT_MS = 600;
    var quietTimer = 0;
    function markIntent() {
      intentAt = performance.now();
      clearTimeout(quietTimer);
      quietTimer = setTimeout(settle, INTENT_MS + 50);
    }
    function userScrolling() { return dragging || performance.now() - intentAt < INTENT_MS; }

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
      if (reversed() === want || (userScrolling() && !triggerMode)) return;
      var top = fromTop();
      scrollEl.classList.toggle('is-stuck', want);
      scrollEl.scrollTop = want ? 0 : top;
    }

    // A glide in progress owns the scroll position until it lands, the reader's
    // gesture until it ends.
    function pin() {
      if (glide || userScrolling()) return;
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
      if (!row || row.node.parentNode !== thread) return false;
      var delta = row.node.getBoundingClientRect().top - scrollEl.getBoundingClientRect().top - a.top;
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

    // A split change (content_pane.js) reflows the list every frame, and WebKit
    // lays out every row for each one. Rows well out of view sit it out: hidden,
    // with their span added as margin to the nearest row still shown, so nothing
    // in view moves. An animated change also holds the column's width, so the
    // text breaks once instead of on every frame: narrowing, at the end width
    // from the start; widening, at the start width until the end one fits, so
    // the one re-break happens mid-motion and the landing moves nothing.
    var SPLIT_KEEP = 8;
    var splitHeld = null;
    var splitCols = [];
    function splitMoving(d) {
      if (!scrollEl || triggerMode || !rows.size || !thread.offsetParent) return;
      if (!splitHeld) splitHeld = parkOutOfView();
      if (d.listTo == null || d.listFrom == null) return;
      var delta = d.listTo - d.listFrom;
      splitCols = [thread, composerCol].filter(Boolean).map(function (el) {
        var room = columnRoom(el);
        var max = parseFloat(getComputedStyle(el).maxWidth) || Infinity;
        var from = Math.min(max, room), to = Math.min(max, room + delta);
        el.style.width = Math.min(from, to) + 'px';
        return { el: el, growTo: to > from ? to : 0 };
      });
    }
    // The column's container, which for the composer leaves out the scrollbar gutter.
    function columnRoom(el) {
      var box = el.parentElement, cs = getComputedStyle(box);
      return box.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
    }
    function growHeldColumns() {
      splitCols.forEach(function (c) {
        if (c.growTo && columnRoom(c.el) >= c.growTo) { c.el.style.width = c.growTo + 'px'; c.growTo = 0; }
      });
    }
    function parkOutOfView() {
      var held = { hidden: [], spaced: [] };
      var kids = [];
      for (var n = thread.firstElementChild; n; n = n.nextElementSibling) kids.push(n);
      var rects = kids.map(function (k) { return k.getBoundingClientRect(); });
      var view = scrollEl.getBoundingClientRect();
      var first = -1, last = -1;
      for (var i = 0; i < kids.length; i++) {
        if (rects[i].bottom >= view.top - view.height && rects[i].top <= view.bottom + view.height) {
          if (first < 0) first = i;
          last = i;
        }
      }
      if (first < 0) return held;
      first = Math.max(0, first - SPLIT_KEEP);
      last = Math.min(kids.length - 1, last + SPLIT_KEEP);
      var lo = first, hi = last;
      while (lo > 0 && kids[lo - 1]._msgId) lo--;
      while (hi < kids.length - 1 && kids[hi + 1]._msgId) hi++;
      function space(node, side, px) {
        node.style[side] = parseFloat(getComputedStyle(node)[side]) + px + 'px';
        held.spaced.push([node, side]);
      }
      if (lo < first) space(kids[first], 'marginTop', rects[first].top - rects[lo].top);
      if (hi > last) space(kids[last], 'marginBottom', rects[hi].bottom - rects[last].bottom);
      for (var j = lo; j < first; j++) held.hidden.push(kids[j]);
      for (var k = last + 1; k <= hi; k++) held.hidden.push(kids[k]);
      held.hidden.forEach(function (node) { node.classList.add('is-split-parked'); });
      return held;
    }
    // Rows may have moved or been shelved since; restore them wherever they are.
    function splitSettled() {
      splitCols.forEach(function (c) { c.el.style.width = ''; });
      splitCols = [];
      if (!splitHeld) return;
      splitHeld.hidden.forEach(function (node) { node.classList.remove('is-split-parked'); });
      splitHeld.spaced.forEach(function (s) { s[0].style[s[1]] = ''; });
      splitHeld = null;
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
      var bubble = null;
      rows.forEach(function (r, id) {
        if (!morph.seen[id] && r.bubble && r.node.textContent.trim() === morph.text) bubble = r.bubble;
      });
      if (!bubble) return;
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
        rm.innerHTML = hyloIcon('close');
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
      if (row.node.parentNode) row.node.parentNode.removeChild(row.node);
      shelf.delete(id);
      shelf.set(id, row);
      if (shelf.size > SHELF_MAX) {
        var oldest = shelf.keys().next().value;
        shelf.get(oldest).destroy();
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
        var btn = row.node.querySelector('.msg-retry-btn');
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
        if (!row) {
          row = Rows.create(msg, snap, api);
          rows.set(id, row);
        } else if (row.rev !== msg.rev || row.epoch !== snap.catalogEpoch || row.stop !== stopBit) {
          row.update(msg, snap);
        }
        row.rev = msg.rev;
        row.epoch = snap.catalogEpoch;
        row.stop = stopBit;
        row.node._msgId = id;
        prev = place(row.node, prev);
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
      if (status === 'running' && !text) main.appendChild(Rows.dots('Working'));
      var body = null;
      if (text) {
        body = el('div', 'trig-run-body prose' + (runOpen.has(msg.id) ? '' : ' is-clamped'));
        setProse(body, text, status === 'running');
        main.appendChild(body);
      }
      var banner = Rows.statusBanner(msg, snap, api);
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
          Rows.fetchNoteInfo(notes.map(function (n) { return n.path; })).then(function () {
            notes.forEach(function (n, k) { if (Rows.noteInfo(n.path) === null) chips[k].remove(); });
          });
        }
        var took = status === 'succeeded' && msg.createdAt && msg.completedAt ? Rows.fmtDur(msg.completedAt - msg.createdAt) : '';
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
        var timeEl = row && row.timeEl;
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
        // The reader's direction decides: up leaves the bottom at once, down into
        // its last stretch rejoins it. scrollTop grows toward the bottom in both origins.
        if (userScrolling()) {
          if (scrollEl.scrollTop < lastTop - 1) api.setStick(false);
          else if (scrollEl.scrollTop > lastTop + 1 && fromBottom() <= 64) api.setStick(true);
        }
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
        contentRo = new ResizeObserver(function () { growHeldColumns(); settle(); });
        contentRo.observe(thread);
        contentRo.observe(scrollEl);
      }
      on(window, 'hylo:split-moving', function (e) { splitMoving(e.detail || {}); });
      on(window, 'hylo:split-settled', splitSettled);
      // Markdown's own images: the sanitizer strips every img class, so a card's (embed) never match.
      var REPLY_IMG = '.msg-body .prose img:not([class])';
      on(scrollEl, 'click', function (e) {
        var img = e.target.closest && e.target.closest(REPLY_IMG);
        if (img && root.openImageLightbox) {
          e.preventDefault();
          img.dataset.imgSrc = img.src;
          img.dataset.imgName = img.alt || img.src.split('/').pop();
          root.openImageLightbox(img);
          return;
        }
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
      on(ta, 'input', function (e) {
        if (detaching) return;
        api.setDraft(ta.value);
        root.__hyloAutoResize(ta, e);
        ac.refresh();
        show(hint, !ta.value);
        refreshContext();
        if (sendBtn) sendBtn.disabled = !api.state().canSend;
      });
      on(ta, 'click', function () { ac.refresh(); });
      var taScroll = ta.parentElement, snapTimer = 0;
      // Scroll origin is the bottom, so text above = range + scrollTop. Text
      // growing past the cap moves this without a scroll event, hence the observer.
      var syncFade = function () {
        var lh = parseFloat(getComputedStyle(taScroll).lineHeight) || 21;
        var above = taScroll.scrollHeight - taScroll.clientHeight + taScroll.scrollTop;
        taScroll.style.setProperty('--fade-top', Math.min(Math.max(above, 0), lh) + 'px');
      };
      if (typeof ResizeObserver !== 'undefined') {
        taRO = new ResizeObserver(syncFade);
        taRO.observe(ta);
      }
      on(taScroll, 'scroll', function () {
        syncFade();
        // A wheel or trackpad comes to rest on a line boundary, so no half line shows.
        clearTimeout(snapTimer);
        snapTimer = setTimeout(function () {
          var lh = parseFloat(getComputedStyle(taScroll).lineHeight) || 21;
          var top = Math.round(taScroll.scrollTop / lh) * lh;
          if (Math.abs(top - taScroll.scrollTop) > 0.5) taScroll.scrollTo({ top: top, behavior: 'smooth' });
        }, 150);
      }, { passive: true });
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
        root.__hyloAutoResize(ta);
        show(hint, true);
      },
      focusComposer: focusComposer,
      insertWikilink: insertWikilink,
      destroy: function () {
        detaching = true;
        splitSettled();
        setAttachOpen(false);
        endGlide();
        if (runsEl) root.__hyloReleaseFenceCards(runsEl);
        rows.forEach(function (row) { row.destroy(); });
        shelf.forEach(function (row) { row.destroy(); });
        shelf.clear();
        if (root.__hyloFocusHome === focusComposer) root.__hyloFocusHome = null;
        clearTimeout(quietTimer);
        Rows.onCopied(null);
        listen.abort();
        if (timeTimer) { clearInterval(timeTimer); timeTimer = 0; }
        if (ro) { ro.disconnect(); ro = null; }
        if (taRO) { taRO.disconnect(); taRO = null; }
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
