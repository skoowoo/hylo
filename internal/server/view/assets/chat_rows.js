// Chat message rows. A row is built once and from then on only patched: every
// part of it (trace steps, the reply's markdown blocks, cards, notes, footer)
// is a long-lived node keyed by what it shows, so content already on screen
// never comes back as a new node. Anything that would appear in one step at
// full height reveals instead.
(function (root) {
  var R = root.__hyloChatReduce;

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

  function setText(node, text) {
    if (!node || node._raw === text) return;
    node._raw = text;
    node.textContent = text;
  }

  var CHEVRON = hyloIcon('chevron-right');
  var COPY_SVG = hyloIcon('copy');
  var CHECK_SVG = hyloIcon('check');
  var RETRY_SVG = hyloIcon('retry');
  var CHECK_MARK = hyloIcon('check', 'seg-tool-check');
  var CROSS_MARK = hyloIcon('close', 'seg-tool-check');

  // ── View state the kernel's messages don't carry ────────────────────
  // Keyed by the message, which the kernel keeps across merges and remounts.
  var openState = new WeakMap();
  // Undefined until the reader toggles it, so a part can default by itself.
  function openPick(msg, key) { var o = openState.get(msg); return o ? o[key] : undefined; }
  function isOpen(msg, key) { return !!openPick(msg, key); }
  function toggleOpen(msg, key, was) {
    var o = openState.get(msg) || {};
    o[key] = !(o[key] === undefined ? was : o[key]);
    openState.set(msg, o);
    return o[key];
  }

  var copiedUntil = new Map();
  var copiedListener = null;

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

  function setCopyIcon(btn, on) {
    if (!btn) return;
    btn.classList.toggle('copied', on);
    btn.replaceChildren(svg(on ? CHECK_SVG : COPY_SVG));
  }

  function answerText(msg) {
    var segs = msg.segments || [];
    var ai = R.answerIndex(segs, msg.status === 'running');
    return ai >= 0 ? (segs[ai].content || '').trim() : '';
  }

  // ── Note info, shared by every row and the trigger log ──────────────
  // path -> row | null (not in vault); absent = not fetched yet. Every fetch
  // settles its paths, a failed one to a bare row (shown under its file name),
  // so nothing waiting on a path can ask for it again in a loop.
  var noteInfoCache = Object.create(null);
  var noteInfoWaiting = Object.create(null);

  function fetchNoteInfo(paths) {
    var need = paths.filter(function (p) { return p && !(p in noteInfoCache); });
    var waits = need.filter(function (p) { return noteInfoWaiting[p]; }).map(function (p) { return noteInfoWaiting[p]; });
    need = need.filter(function (p) { return !noteInfoWaiting[p]; });
    var chunks = [];
    for (var i = 0; i < need.length; i += 100) chunks.push(need.slice(i, i + 100));
    var fetches = chunks.map(function (chunk) {
      var p = fetch('/api/notes/info', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ paths: chunk }),
      }).then(function (resp) { return resp.ok ? resp.json() : null; })
        .catch(function () { return null; })
        .then(function (data) {
          var got = Object.create(null);
          if (data) (data.notes || []).forEach(function (n) { noteInfoCache[n.path] = n; got[n.path] = true; });
          chunk.forEach(function (q) {
            if (!got[q]) noteInfoCache[q] = data ? null : { path: q };
            delete noteInfoWaiting[q];
          });
        });
      chunk.forEach(function (q) { noteInfoWaiting[q] = p; });
      return p;
    });
    return Promise.all(fetches.concat(waits));
  }

  function noteTitle(path) {
    var info = noteInfoCache[path];
    var base = path.split('/').pop() || path;
    return (info && info.title ? info.title : base).replace(/\.md$/i, '');
  }

  function openNote(path, title) {
    if (window.__hyloContentPane) void window.__hyloContentPane.openNoteInContentPane(path, title, false, false);
  }

  // ── Markdown, one node group per block ──────────────────────────────
  // A fence opener still being typed would render as an empty code block until
  // its info string is whole and the block can become its card.
  var PARTIAL_FENCE = /(^|\n)[ \t]*(`{1,3}|~{1,3})[^`\n]*$/;
  var FENCE_LINE = /^[ \t]*(`{3,}|~{3,})(.*)$/;
  // A table reads as a paragraph of pipes until its separator row is whole.
  var PIPE_TAIL = /(^|\n)((?:[ \t]*\|[^\n]*(?:\n|$))+)$/;
  var TABLE_SEP = /^[ \t]*\|?[ \t]*:?-+:?[ \t]*(\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*\n/m;

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

  // While streaming, the tail is held back until it can't change shape.
  function streamable(text) {
    text = text.replace(PARTIAL_FENCE, '$1');
    if (endsInOpenFence(text)) return text;
    var m = PIPE_TAIL.exec(text);
    if (m && !TABLE_SEP.test(m[2])) text = text.slice(0, m.index + m[1].length);
    return text;
  }

  function Prose(cls) {
    var node = el('div', 'prose' + (cls ? ' ' + cls : ''));
    var blocks = []; // { html, card, partial, nodes, handle }
    var last = null;

    function nodesOf(b) { return b.handle ? [b.handle.el] : b.nodes; }

    function mountCard(b) {
      if (b.handle || !b.card || blocks.indexOf(b) === -1) return;
      b.handle = root.__hyloFenceBlock(b.nodes[0], b.partial);
    }

    function build(nb, partial) {
      var tpl = document.createElement('template');
      tpl.innerHTML = nb.html;
      var nodes = Array.prototype.filter.call(tpl.content.childNodes, function (n) {
        return n.nodeType === 1 || n.textContent.trim();
      });
      return { html: nb.html, card: nb.card, source: nb.source, partial: partial, nodes: nodes, handle: null };
    }

    function drop(b) {
      if (b.handle) b.handle.destroy();
      nodesOf(b).forEach(function (n) { if (n.parentNode) n.parentNode.removeChild(n); });
    }

    function update(text, streaming) {
      text = text || '';
      if (streaming) text = streamable(text);
      var key = (streaming ? 's' : 'f') + text;
      if (key === last) return;
      last = key;
      var next = root.__hyloMarkdownBlocks(text);
      var open = streaming && endsInOpenFence(text);
      var fresh = [];
      for (var i = 0; i < next.length; i++) {
        var nb = next[i], ob = blocks[i];
        var partial = open && i === next.length - 1;
        if (ob && ob.html === nb.html && ob.partial === partial) continue;
        if (ob && ob.handle && ob.card && ob.card === nb.card) {
          ob.handle.update(nb.source, { partial: partial });
          ob.html = nb.html;
          ob.partial = partial;
          continue;
        }
        var b = build(nb, partial);
        var anchor = ob ? nodesOf(ob)[0] : null;
        b.nodes.forEach(function (n) { node.insertBefore(n, anchor); });
        if (ob) drop(ob);
        blocks[i] = b;
        if (b.card) fresh.push(b);
      }
      while (blocks.length > next.length) drop(blocks.pop());
      // A card measures on mount, so it mounts once its row is in the page.
      fresh.forEach(function (b) {
        if (node.isConnected) mountCard(b);
        else queueMicrotask(function () { mountCard(b); });
      });
    }

    function destroy() {
      blocks.forEach(function (b) { if (b.handle) b.handle.destroy(); });
      blocks = [];
    }

    return { node: node, update: update, destroy: destroy };
  }

  // ── Tool and trace labels ───────────────────────────────────────────
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

  // Negative delay keeps looping animations in phase across remounts.
  function phase(node, period) {
    node.style.animationDelay = '-' + (Date.now() % period) + 'ms';
  }

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

  // ── Steps: one long-lived component per segment ─────────────────────
  // A folding body: its grid row animates between 0fr and 1fr, so opening or
  // closing slides the content below instead of dropping it. The clip between
  // has no box of its own, so closed really is zero.
  function fold(inner, cls) {
    var box = el('div', cls || 'chat-fold');
    var clip = el('div', 'chat-clip');
    clip.appendChild(inner);
    box.appendChild(clip);
    return box;
  }
  function setFold(box, on) { box.classList.toggle('is-open', !!on); }

  var steps = {
    text: function () {
      var p = Prose();
      return {
        node: p.node,
        update: function (seg, ctx, role) {
          p.node.classList.toggle('seg-trace-text', role === 'narration');
          p.update(seg.content || '', ctx.running);
        },
        destroy: p.destroy,
      };
    },

    thinking: function (msg, key) {
      var wrap = el('div', 'seg-thinking hairline');
      var btn = el('button', 'seg-thinking-toggle');
      btn.type = 'button';
      btn.appendChild(svg(CHEVRON));
      var label = el('span', 'seg-thinking-label');
      btn.appendChild(label);
      var body = el('div', 'seg-thinking-content');
      var box = fold(body);
      wrap.appendChild(btn);
      wrap.appendChild(box);
      var live = null, running = false, seg = null;
      // The preview stays until the run ends: closing it mid-run would pull the
      // rest of the turn back up under the reader.
      function paint() {
        var open = isOpen(msg, key);
        btn.classList.toggle('open', open);
        body.classList.toggle('is-preview', running && !open);
        setFold(box, open || (running && !!seg.content));
      }
      btn.addEventListener('click', function () { toggleOpen(msg, key); paint(); });
      return {
        node: wrap,
        update: function (s, ctx) {
          seg = s;
          running = ctx.running;
          var nowLive = ctx.running && !s.endedAt;
          if (nowLive !== live) {
            live = nowLive;
            btn.classList.toggle('is-streaming', live);
            if (live) phase(label, 1800);
          }
          var dur = s.endedAt && s.startedAt ? fmtDur(s.endedAt - s.startedAt) : '';
          setText(label, live ? 'Thinking' : (dur ? 'Thought for ' + dur : 'Thought'));
          setText(body, s.content || '');
          paint();
        },
      };
    },

    tool_use: function (msg, key) {
      var wrap = el('div', 'seg-tool');
      var head = el('button', 'seg-tool-head');
      head.type = 'button';
      var iconSlot = el('span', 'seg-tool-icon');
      var verb = el('span', 'seg-tool-verb');
      var arg = el('span', 'seg-tool-arg');
      var cnt = el('span', 'seg-tool-count');
      var time = el('span', 'seg-tool-time');
      var chev = svg(CHEVRON);
      chev.classList.add('seg-tool-chevron');
      [iconSlot, verb, arg, cnt, time, chev].forEach(function (n) { head.appendChild(n); });
      var chips = el('div', 'seg-tool-notes');
      var chipBy = Object.create(null);
      var box = el('div', 'seg-tool-results');
      var pres = [];
      wrap.appendChild(head);
      wrap.appendChild(chips);
      wrap.appendChild(box);
      var state = '', hasBody = false;
      head.addEventListener('click', function () {
        if (!hasBody) return;
        var on = toggleOpen(msg, key);
        head.classList.toggle('open', on);
        show(box, on);
      });
      return {
        node: wrap,
        update: function (seg, ctx) {
          var results = seg.results || [];
          var calls = seg.calls || [];
          var last = calls.length ? calls[calls.length - 1] : null;
          var pending = ctx.running && results.length < seg.count;
          var failed = calls.some(function (c) { return c.failed; });
          var next = pending ? 'run' : (failed ? 'err' : (results.length ? 'ok' : 'idle'));
          if (next !== state) {
            var settled = state === 'run';
            state = next;
            wrap.className = 'seg-tool is-' + state;
            var icon;
            if (state === 'run') { icon = el('span', 'seg-tool-spinner'); phase(icon, 700); }
            else if (state === 'ok' || state === 'err') {
              icon = svg(state === 'ok' ? CHECK_MARK : CROSS_MARK);
              if (settled) icon.classList.add('is-fresh');
            } else icon = el('span', 'seg-tool-icon-done');
            iconSlot.replaceChildren(icon);
            if (pending) phase(verb, 1800);
          }
          setText(verb, toolVerb(seg.name, pending));
          var notes = seg.notes || [];
          var argText = last && last.arg && !notes.length ? last.arg : '';
          setText(arg, argText);
          arg.title = argText;
          show(arg, !!argText);
          setText(cnt, seg.count > 1 ? '×' + seg.count : '');
          show(cnt, seg.count > 1);
          var d = pending ? '' : fmtDur(toolMs(seg));
          setText(time, d);
          show(time, !!d);

          notes.forEach(function (n) {
            var chip = chipBy[n.path];
            if (!chip) {
              chip = chipBy[n.path] = el('button', 'seg-note-chip');
              chip.type = 'button';
              chip.title = n.path;
              chip.addEventListener('click', function () { openNote(n.path, chip.textContent); });
              if (ctx.live) chip.classList.add('is-fresh');
              chips.appendChild(chip);
            }
            chip.classList.toggle('is-write', n.action === 'write');
            setText(chip, noteTitle(n.path));
          });
          show(chips, notes.length > 0);

          hasBody = results.some(function (r) { return !!r; });
          head.disabled = !hasBody;
          show(chev, hasBody);
          var open = hasBody && isOpen(msg, key);
          head.classList.toggle('open', open);
          show(box, open);
          for (var ri = 0; ri < results.length; ri++) {
            var pre = pres[ri];
            if (!pre) {
              pre = pres[ri] = el('pre', 'seg-tool-result-content');
              box.appendChild(pre);
            }
            setText(pre, results.length > 1 ? ('[' + (ri + 1) + '] ' + results[ri]) : results[ri]);
          }
          box.classList.toggle('is-multi', results.length > 1);
        },
      };
    },

    tool_result: function (msg, key) {
      var wrap = el('div', 'seg-tool-result');
      var btn = el('button', 'seg-tool-result-toggle');
      btn.type = 'button';
      btn.appendChild(svg(CHEVRON));
      btn.appendChild(document.createTextNode('Result'));
      var pre = el('pre', 'seg-tool-result-content');
      wrap.appendChild(btn);
      wrap.appendChild(pre);
      function paint() {
        var open = isOpen(msg, key);
        btn.classList.toggle('open', open);
        show(pre, open);
      }
      btn.addEventListener('click', function () { toggleOpen(msg, key); paint(); });
      return {
        node: wrap,
        update: function (seg) { setText(pre, seg.content || ''); paint(); },
      };
    },

    status: function () {
      var node = el('div', 'seg-status is-running');
      return {
        node: node,
        update: function (seg, ctx) { setText(node, seg.label || ''); show(node, ctx.running); },
      };
    },

    console: function () {
      var node = el('pre', 'seg-console');
      return { node: node, update: function (seg) { setText(node, seg.content || ''); } };
    },

    error: function () {
      var node = el('div', 'seg-error');
      return { node: node, update: function (seg) { setText(node, seg.message || ''); } };
    },
  };

  // Children of `parent` become exactly `nodes`, in order, moving only what's out of place.
  function arrange(parent, nodes) {
    var at = parent.firstChild;
    for (var i = 0; i < nodes.length; i++) {
      if (nodes[i] === at) { at = at.nextSibling; continue; }
      parent.insertBefore(nodes[i], at);
    }
    while (at) { var nx = at.nextSibling; parent.removeChild(at); at = nx; }
  }

  // ── Trace: the run's steps, open while live, folded once settled ──
  // Until the reader opens it, the rail is capped and overflows upward: the
  // newest steps stay in view and the box stops growing once full, so the reply
  // below holds still. The cap stays through the fold so it shrinks from there.
  function Trace(msg) {
    var wrap = el('div', 'seg-trace');
    var head = el('button', 'seg-trace-head');
    head.type = 'button';
    head.appendChild(svg(CHEVRON));
    var label = el('span');
    head.appendChild(label);
    var rail = el('div', 'seg-trace-rail');
    var shell = fold(rail, 'chat-fold seg-trace-shell');
    var clip = shell.firstChild;
    wrap.appendChild(head);
    wrap.appendChild(shell);
    var live = null;

    // The top edge fades only once older steps are cut off.
    function syncOverflow() {
      shell.classList.toggle('is-overflow', rail.offsetHeight > clip.clientHeight + 1);
    }
    var ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(syncOverflow) : null;
    if (ro) { ro.observe(rail); ro.observe(clip); }

    function paint() {
      var pick = openPick(msg, 'trace');
      var open = pick === undefined ? live : pick;
      head.classList.toggle('open', open);
      shell.classList.toggle('is-open', open);
      shell.classList.toggle('is-capped', pick === undefined);
    }
    head.addEventListener('click', function () {
      var pick = openPick(msg, 'trace');
      toggleOpen(msg, 'trace', pick === undefined ? live : pick);
      paint();
    });

    return {
      node: wrap,
      rail: rail,
      update: function (segs, running) {
        if (running !== live) {
          live = running;
          wrap.classList.toggle('is-live', live);
          if (live) phase(label, 1800);
        }
        setText(label, traceSummary(segs, live));
        paint();
      },
      destroy: function () { if (ro) ro.disconnect(); },
    };
  }

  // ── Notes touched: a strip of fixed-height cards, built as notes are touched ──
  function noteCard(path, action) {
    var base = path.split('/').pop() || path;
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
    titleEl.textContent = base.replace(/\.md$/i, '');
    titlerow.appendChild(titleEl);
    titleblock.appendChild(titlerow);
    var previewEl = el('span', 'home-note-row-preview-grid');
    titleblock.appendChild(previewEl);
    top.appendChild(titleblock);
    var badges = el('div', 'home-note-row-badges');
    var badge = el('span', 'badge badge--sm');
    badges.appendChild(badge);
    top.appendChild(badges);
    card.appendChild(top);
    var meta = el('div', 'home-note-row-meta');
    var timeEl = el('span', 'home-note-row-time');
    meta.appendChild(timeEl);
    card.appendChild(meta);
    card.addEventListener('click', function () { openNote(path, titleEl.textContent); });

    var hydrated = false;
    function update(act) {
      var write = act === 'write';
      badge.className = 'badge badge--sm' + (write ? ' badge--accent' : '');
      badge.textContent = write ? 'write' : 'read';
      var info = noteInfoCache[path];
      if (!info || hydrated) return;
      hydrated = true;
      if (info.title) titleEl.textContent = info.title.replace(/\.md$/i, '');
      previewEl.textContent = info.preview || '';
      timeEl.textContent = info.updatedAt ? R.formatTime(info.updatedAt) : '';
      if (info.cover) {
        coverImg.src = info.cover;
        show(cover, true);
        card.classList.add('has-cover');
      }
    }
    update(action);
    return { node: card, update: update };
  }

  // Arrows over the strip, hidden when everything fits, each disabled at its end.
  function wireArrows(scroller, row) {
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
    function sync() {
      var max = row.scrollWidth - row.clientWidth;
      var fits = max <= 1;
      show(prev, !fits && row.scrollLeft > 0);
      show(next, !fits && row.scrollLeft < max - 1);
    }
    function page(dir) { row.scrollBy({ left: dir * Math.max(row.clientWidth * 0.8, 220), behavior: 'smooth' }); }
    prev.addEventListener('click', function () { page(-1); });
    next.addEventListener('click', function () { page(1); });
    row.addEventListener('scroll', sync, { passive: true });
    var ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(sync) : null;
    if (ro) ro.observe(row);
    return { sync: sync, destroy: function () { if (ro) ro.disconnect(); } };
  }

  // A note gets a card once it's touched, read or written; a write wins. Live
  // runs decorate their tool calls, settled ones carry note_access from the DB.
  function touchedNotes(segs) {
    var byPath = Object.create(null), order = [];
    function add(path, action) {
      if (!path || /[*?]/.test(path)) return;
      if (!(path in byPath)) { order.push(path); byPath[path] = action; }
      else if (action === 'write') byPath[path] = 'write';
    }
    segs.forEach(function (s) {
      if (s.type === 'tool_use') (s.notes || []).forEach(function (n) { add(n.path, n.action); });
      else if (s.type === 'note_access') (s.items || []).forEach(function (n) { add(n.path, n.action); });
    });
    return order.map(function (p) { return { path: p, action: byPath[p] }; });
  }

  // trusting: show a note before the vault confirms it. A row built from history
  // trusts, so its strip is there from the first paint; a live one waits, so a
  // note the vault doesn't have never flashes in.
  function NotesStrip(trusting, onChange) {
    var wrap = el('div', 'seg-note-access');
    var label = el('div', 'seg-note-access-label');
    label.textContent = 'Notes touched';
    var scroller = el('div', 'seg-note-access-scroller');
    var row = el('div', 'seg-note-access-row home-list-body is-grid');
    scroller.appendChild(row);
    wrap.appendChild(label);
    wrap.appendChild(scroller);
    var arrows = wireArrows(scroller, row);
    var cards = Object.create(null);
    var notes = [];

    function paint() {
      var shown = [];
      notes.forEach(function (n) {
        var info = noteInfoCache[n.path];
        if (info === null || (info === undefined && !trusting)) return;
        var c = cards[n.path] || (cards[n.path] = noteCard(n.path, n.action));
        c.update(n.action);
        shown.push(c.node);
      });
      arrange(row, shown);
      arrows.sync();
      return shown.length;
    }

    return {
      node: wrap,
      // Returns how many cards show: a note the vault doesn't have gets none.
      update: function (next) {
        notes = next;
        var n = paint();
        var missing = notes.filter(function (x) { return !(x.path in noteInfoCache); });
        if (missing.length) {
          fetchNoteInfo(missing.map(function (x) { return x.path; })).then(function () {
            if (notes === next && onChange) onChange();
          });
        }
        return n;
      },
      destroy: arrows.destroy,
    };
  }

  // A block that appears after its row was built grows in from nothing; one there
  // from the start (history, a remount) shows at once.
  function Reveal(child, animate) {
    var box = fold(child, 'chat-reveal' + (animate ? '' : ' is-static'));
    return {
      node: box,
      set: function (on) { box.classList.toggle('is-open', !!on); },
    };
  }

  // ── Rows ────────────────────────────────────────────────────────────
  var TRACE_TYPES = { thinking: 1, tool_use: 1, tool_result: 1, status: 1, text: 1 };

  function header(msg, snap) {
    var h = el('div', 'msg-agent-header');
    var name = snap.mateName(msg);
    var avatar = el('div', 'avatar avatar--md msg-agent-avatar');
    var mate = null;
    for (var i = 0; i < snap.mates.length; i++) if (snap.mates[i].id === msg.agentBotId) { mate = snap.mates[i]; break; }
    var colorKey = mate ? mate.name : (msg.agentBotId || '');
    if (root.agentBotAvatarSVG) avatar.innerHTML = root.agentBotAvatarSVG(colorKey, snap.mates);
    h.appendChild(avatar);
    var nameEl = el('div', 'msg-agent-name');
    nameEl.textContent = name;
    h.appendChild(nameEl);
    if (msg.triggerEvent) {
      var badge = el('span', 'badge badge--accent');
      badge.textContent = snap.triggerLabel(msg.triggerEvent);
      h.appendChild(badge);
    }
    return h;
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

  function dots(text) {
    var label = el('span', 'msg-thinking-label');
    label.appendChild(document.createTextNode(text));
    var d = el('span', 'thinking-dots');
    d.appendChild(el('span')); d.appendChild(el('span')); d.appendChild(el('span'));
    label.appendChild(d);
    return label;
  }

  // Same height in every state: only what's in it changes.
  function Footer() {
    var foot = el('div', 'msg-footer');
    var stateEl = el('span', 'msg-footer-state');
    var time = el('span', 'msg-agent-time');
    var copy = el('button', 'msg-copy-btn');
    copy.type = 'button';
    copy.title = 'Copy reply';
    foot.appendChild(stateEl);
    foot.appendChild(time);
    foot.appendChild(copy);
    var shownState = null, msg = null;
    copy.addEventListener('click', function () {
      var text = answerText(msg);
      if (!text || !navigator.clipboard) return;
      var id = msg.id;
      navigator.clipboard.writeText(text).then(function () {
        rememberCopy(id);
        setCopyIcon(copy, true);
      }).catch(function () { /* clipboard denied */ });
    });
    return {
      node: foot,
      time: time,
      copy: copy,
      update: function (m, snap) {
        msg = m;
        var running = m.status === 'running';
        var st = running && snap.stoppingId === m.id ? 'Stopping' : (running && !(m.segments || []).length ? 'Thinking' : '');
        if (st !== shownState) {
          shownState = st;
          stateEl.replaceChildren(st ? dots(st) : '');
          show(stateEl, !!st);
        }
        var label = !running && R.msgAt(m) ? R.formatTime(R.msgAt(m)) : '';
        setText(time, label);
        show(time, !!label);
        var canCopy = !running && !!answerText(m);
        if (canCopy && !copy.firstChild) setCopyIcon(copy, copiedNow(m.id));
        show(copy, canCopy);
      },
    };
  }

  function stepKeys(segs) {
    var nth = {};
    return segs.map(function (s) {
      var t = s.type || '';
      nth[t] = (nth[t] || 0) + 1;
      return t + '#' + (nth[t] - 1);
    });
  }

  function assistantRow(msg, snap, api) {
    var row = el('div', 'msg-row msg-row-assistant');
    var wrap = el('div', 'msg-assistant-wrap');
    var head = header(msg, snap);
    var headSig = '';
    var body = el('div', 'msg-body');
    var bannerSlot = el('div', 'msg-banner-slot');
    var footer = Footer();
    wrap.appendChild(head);
    wrap.appendChild(body);
    wrap.appendChild(bannerSlot);
    wrap.appendChild(footer.node);
    row.appendChild(wrap);

    var built = false;
    var comps = Object.create(null); // step key -> { node, update, destroy }
    var trace = null, notes = null, notesReveal = null, bannerKey = '';
    var current = msg, curSnap = snap;

    function comp(key, type) {
      var c = comps[key];
      if (!c) c = comps[key] = steps[type](current, key);
      return c;
    }

    function update(m, s) {
      current = m;
      curSnap = s;
      var running = m.status === 'running';
      var segs = m.segments || [];
      var ctx = { running: running, live: built };
      var keys = stepKeys(segs);
      var ai = R.answerIndex(segs, running);

      var sig = [s.mateName(m), m.agentBotId, m.triggerEvent || '', s.catalogEpoch].join('\0');
      if (built && sig !== headSig) {
        var nh = header(m, s);
        wrap.replaceChild(nh, head);
        head = nh;
      }
      headSig = sig;

      var lead = [], rest = [], used = Object.create(null), stepCount = 0, traceSegs = [];
      var answer = null;
      for (var i = 0; i < segs.length; i++) {
        var sg = segs[i], type = sg.type;
        if (type === 'note_access' || !steps[type]) continue;
        var c = comp(keys[i], type);
        used[keys[i]] = true;
        c.update(sg, ctx, i === ai ? 'answer' : 'narration');
        if (i === ai) answer = c.node;
        else if (TRACE_TYPES[type]) {
          lead.push(c.node);
          traceSegs.push(sg);
          if (type !== 'status') stepCount++;
        } else rest.push(c.node);
      }

      var top = [];
      if (stepCount) {
        if (!trace) trace = Trace(m);
        trace.update(traceSegs, running);
        arrange(trace.rail, lead);
        top.push(trace.node);
      } else {
        if (trace) trace.destroy();
        trace = null;
        top = lead;
      }
      // The notes a turn touched sit between its steps and its reply.
      var touched = touchedNotes(segs);
      if (touched.length && !notes) {
        notes = NotesStrip(!built, function () { if (current === m) update(current, curSnap); });
        notesReveal = Reveal(notes.node, built);
      }
      if (notes) {
        notesReveal.set(notes.update(touched) > 0);
        top.push(notesReveal.node);
      }
      if (answer) top.push(answer);
      arrange(body, top.concat(rest));

      Object.keys(comps).forEach(function (k) {
        if (used[k]) return;
        if (comps[k].destroy) comps[k].destroy();
        delete comps[k];
      });

      var bk = (m.status === 'failed' || m.status === 'canceled') ? m.status : '';
      if (bk !== bannerKey) {
        bannerKey = bk;
        if (bk) {
          var banner = Reveal(statusBanner(m, s, api), built);
          bannerSlot.replaceChildren(banner.node);
          banner.set(true);
        } else bannerSlot.replaceChildren();
      }

      footer.update(m, s);
      built = true;
    }

    update(msg, snap);
    row._msgId = msg.id;
    // A reply that starts in front of the user greets them once; history stays still.
    if (msg.status === 'running' && root.agentBotAvatarPlay) root.agentBotAvatarPlay(head.querySelector('.bot-face'));
    return {
      node: row,
      update: update,
      avatar: function () { return head.firstChild; },
      timeEl: footer.time,
      copyBtn: footer.copy,
      destroy: function () {
        Object.keys(comps).forEach(function (k) { if (comps[k].destroy) comps[k].destroy(); });
        if (trace) trace.destroy();
        if (notes) notes.destroy();
      },
    };
  }

  function userSig(msg) {
    return (msg.content || '') + '\0' + (msg.attachments || []).map(function (a) { return a.id; }).join(',');
  }

  function userRow(msg) {
    var row = el('div', 'msg-row msg-row-user');
    var wrap = el('div', 'msg-user-wrap');
    row.appendChild(wrap);
    row._msgId = msg.id;
    var self = { node: row, bubble: null, sig: '', prose: null };

    function build(m) {
      if (self.prose) self.prose.destroy();
      self.prose = null;
      wrap.replaceChildren();
      var atts = m.attachments || [];
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
      if ((m.content || '').trim()) {
        self.prose = Prose('msg-user-bubble');
        self.prose.update(m.content, false);
        wrap.appendChild(self.prose.node);
      }
      self.bubble = self.prose && self.prose.node;
      self.sig = userSig(m);
    }

    build(msg);
    // A sent message only changes if the DB says it reads differently.
    self.update = function (m) { if (userSig(m) !== self.sig) build(m); };
    self.destroy = function () { if (self.prose) self.prose.destroy(); };
    return self;
  }

  root.__hyloChatRows = {
    create: function (msg, snap, api) {
      return msg.role === 'user' ? userRow(msg) : assistantRow(msg, snap, api);
    },
    onCopied: function (fn) { copiedListener = fn; },
    setCopyIcon: setCopyIcon,
    statusBanner: statusBanner,
    fetchNoteInfo: fetchNoteInfo,
    noteInfo: function (path) { return noteInfoCache[path]; },
    fmtDur: fmtDur,
    dots: dots,
    streamable: streamable,
    endsInOpenFence: endsInOpenFence,
  };
})(window);
