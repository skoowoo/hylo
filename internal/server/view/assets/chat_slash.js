// The composer's command palette. "/" (or "、" from a Chinese IME) opens it,
// and so does the + button, which types the same "/" so both are one path;
// "@" opens it inside the notes group. Typing filters the current level only:
// commands and groups at the top, a group's own items once inside it.
//
// command: {label, icon, sub, aliases, insert | run}
// group:   {label, icon, sub, group: true, hint, mention?, debounce?,
//           load(query, path, signal) → Promise<{items, note}>}
// insert is text that takes the token's place; run is an action after it's
// gone; a loaded item with `open` is a level to step into (folders).
(function (root) {
  var CJK = /[⺀-鿿豈-﫿＀-￯]/;
  var QUERY = /^[\p{L}\p{N}_-]*$/u;
  var NAME = /^[^\s\/\[\]@]*$/; // note names: anything a wikilink can hold
  var HINT = '↑↓ select · Enter open · Esc close';

  // { start, query, trigger } for the trigger token ending at the caret, or null.
  // "、" only counts at a word start, so a Chinese list ("苹果、香蕉") never opens it.
  function parse(val, caret) {
    var left = val.slice(0, caret);
    var i = Math.max(left.lastIndexOf('/'), left.lastIndexOf('、'), left.lastIndexOf('@'));
    if (i < 0) return null;
    var trigger = left[i] === '@' ? '@' : '/';
    var query = left.slice(i + 1);
    if (!(trigger === '@' ? NAME : QUERY).test(query)) return null;
    var prev = i > 0 ? left[i - 1] : '';
    if (prev && !/\s/.test(prev) && !(left[i] !== '、' && CJK.test(prev))) return null;
    return { start: i, query: query, trigger: trigger };
  }

  function score(c, q) {
    var name = c.label.toLowerCase();
    if (name.indexOf(q) === 0) return 0;
    if (name.indexOf(q) > 0) return 1;
    if ((c.aliases || []).some(function (a) { return a.toLowerCase().indexOf(q) !== -1; })) return 2;
    // Descriptions match at word starts, so "sk" doesn't find "disk".
    if (c.sub && (CJK.test(q) ? c.sub.indexOf(q) !== -1 : (' ' + c.sub.toLowerCase()).indexOf(' ' + q) !== -1)) return 3;
    return -1;
  }

  // Matches in rank order, registry order within a rank.
  function search(commands, q) {
    q = q.toLowerCase();
    var hits = [];
    commands.forEach(function (c) { var s = score(c, q); if (s >= 0) hits.push({ c: c, s: s }); });
    return hits.sort(function (a, b) { return a.s - b.s; }).map(function (h) { return h.c; });
  }

  root.__hyloSlashParse = parse;
  root.__hyloSlashSearch = search;

  // opts: input, panel, list, hint, load() → Promise<commands>, onOpen(open), escKey, signal
  // opts: input, panel, head, list, hint, commands() → the top level, onOpen(open), escKey, signal
  root.__hyloSlashCreate = function (opts) {
    var input = opts.input;
    var st = {
      open: false, commands: null, rseq: 0, tick: 0, abort: null, loading: false,
      composing: false, start: -1, group: null, path: [], viaAt: false, dismissed: -1, atLeft: -1, auto: null,
    };
    var ui = root.__hyloAcList({ panel: opts.panel, head: opts.head, list: opts.list, onPick: function () { pick(false); }, signal: opts.signal });
    var on = function (type, fn) { input.addEventListener(type, fn, { signal: opts.signal }); };

    // Marked text (pinyin) isn't the query yet, and WebKit selects all of it,
    // which reads as the token ending: hold everything until it commits.
    on('compositionstart', function () { st.composing = true; });
    on('compositionend', function () { st.composing = false; refresh(); });
    on('input', refresh);
    on('click', refresh);
    on('keyup', function (e) { if (/^(ArrowLeft|ArrowRight|Home|End)$/.test(e.key)) refresh(); });
    on('blur', function () {
      setTimeout(function () { if (document.activeElement !== input) close(); }, 180);
    });

    function setOpen(open) {
      if (st.open === open) return;
      st.open = open;
      opts.panel.hidden = !open;
      if (open) { if (root.__hyloEscPush) root.__hyloEscPush(opts.escKey, dismiss); }
      else if (root.__hyloEscPop) root.__hyloEscPop(opts.escKey);
      if (opts.onOpen) opts.onOpen(open);
    }

    function ctx() {
      if (input.selectionStart !== input.selectionEnd) return null;
      return parse(input.value, input.selectionStart);
    }

    function stop() {
      st.rseq++;
      st.loading = false;
      clearTimeout(st.tick);
      if (st.abort) { st.abort.abort(); st.abort = null; }
    }

    // The one way shut. It never edits the text: it also runs on blur and teardown.
    function close() {
      stop();
      st.start = -1;
      st.group = null;
      st.path = [];
      st.auto = null;
      st.commands = null; // built per opening, so skills turned on meanwhile show
      setOpen(false);
      ui.clear();
    }

    // Backing out (Esc, + again) takes back the + button's "/" if nothing was typed after it.
    function retract(a) {
      var c = ctx();
      if (a && c && !c.query && c.start === a.at + a.text.length - 1 && input.value.slice(a.at, c.start + 1) === a.text) {
        root.__hyloTextareaInsert(input, a.at, c.start + 1, '');
      }
    }

    // Whatever text stays behind doesn't reopen until the token ends.
    function dismiss() {
      var a = st.auto;
      close();
      retract(a);
      var c = ctx();
      st.dismissed = c ? c.start : -1;
    }

    function fill(cmd, li) {
      li.className = 'cmd-row';
      var ic = document.createElement('span');
      ic.className = 'cmd-icon';
      ic.innerHTML = hyloIcon(cmd.icon);
      var text = document.createElement('span');
      text.className = 'cmd-text';
      var main = document.createElement('span');
      main.className = 'ac-main';
      main.textContent = cmd.label;
      text.appendChild(main);
      if (cmd.sub) {
        var sub = document.createElement('span');
        sub.className = 'ac-sub';
        sub.textContent = cmd.sub;
        text.appendChild(sub);
      }
      li.appendChild(ic);
      li.appendChild(text);
      if (cmd.group || cmd.open) {
        var m = document.createElement('span');
        m.className = 'cmd-meta';
        m.innerHTML = hyloIcon('chevron-right');
        li.appendChild(m);
      }
    }

    function show(items, spec) {
      ui.show({ items: items, row: fill, note: spec.note, head: spec.head });
      opts.hint.textContent = spec.hint;
      opts.hint.hidden = !items.length;
      setOpen(true);
    }

    function renderTop(c) {
      var items = c.query ? search(st.commands, c.query) : st.commands;
      if (!items.length) { close(); return; }
      show(items, { hint: HINT });
    }

    function head(g) {
      return {
        crumbs: [g.label].concat(st.path),
        onBack: up,
        onCrumb: function (i) { st.path = st.path.slice(0, i); clearQuery(); },
      };
    }

    // The rows on screen stay until the new ones land, but nothing picks them meanwhile.
    function renderGroup(c) {
      var g = st.group;
      var seq = ++st.rseq;
      st.loading = true;
      var done = function (r) {
        if (seq !== st.rseq) return;
        st.loading = false;
        show(r.items, { hint: g.hint || HINT, note: r.items.length ? '' : r.note, head: head(g) });
      };
      var run = function () {
        if (st.abort) st.abort.abort();
        var ab = st.abort = new AbortController();
        g.load(c.query, st.path.slice(), ab.signal).then(done, function () {
          done({ items: [], note: 'Couldn’t load' });
        });
      };
      clearTimeout(st.tick);
      if (g.debounce && c.query) st.tick = setTimeout(run, g.debounce);
      else run();
    }

    function render(c) {
      if (st.group) renderGroup(c);
      else renderTop(c);
    }

    function refresh() {
      if (st.composing) return;
      var c = ctx();
      if (!c) { st.dismissed = -1; st.atLeft = -1; if (st.open) close(); return; }
      if (c.start === st.dismissed) return;
      st.dismissed = -1;
      if (c.start !== st.start) {
        st.start = c.start;
        st.group = null;
        st.path = [];
        st.viaAt = false;
      }
      if (!st.commands) st.commands = opts.commands();
      route(c);
    }

    // "@" lands in the notes group until the user steps back out of it.
    function route(c) {
      if (c.trigger === '@' && !st.group && st.atLeft !== c.start) {
        st.group = st.commands.filter(function (g) { return g.mention; })[0] || null;
        st.viaAt = true;
      }
      render(c);
    }

    // Swaps the token for text, the caret after it; removing it outright takes
    // the space the + button put before it too.
    function replaceToken(text, auto) {
      var c = ctx();
      if (!c) return;
      var from = !text && auto && auto.at + auto.text.length - 1 === c.start ? auto.at : c.start;
      root.__hyloTextareaInsert(input, from, input.selectionStart, text);
    }

    // Empties the query (its input event renders), or renders when it's already empty.
    function clearQuery() {
      var c = ctx();
      if (c && c.query) root.__hyloTextareaInsert(input, c.start + 1, input.selectionStart, '');
      else if (c) render(c);
    }

    function enter(group) {
      st.group = group;
      st.viaAt = false;
      st.path = [];
      clearQuery();
    }

    function up() {
      var c = ctx();
      if (st.path.length) { st.path.pop(); clearQuery(); return; }
      if (c && c.trigger === '@') st.atLeft = c.start;
      st.group = null;
      st.viaAt = false;
      if (c) render(c);
    }

    function pick(step) {
      var cmd = ui.item();
      if (!cmd || st.loading) return;
      if (cmd.group) { enter(cmd); return; }
      if (step && cmd.open) { st.path.push(cmd.open); clearQuery(); return; }
      var text = '';
      if (cmd.insert) text = /^\s/.test(input.value.slice(input.selectionStart)) ? cmd.insert : cmd.insert + ' ';
      var auto = st.auto;
      close();
      replaceToken(text, auto);
      if (cmd.run) cmd.run();
    }

    function handleKeydown(e) {
      if (!st.open || e.isComposing || e.keyCode === 229) return false;
      var k = e.key;
      var x = ui.item();
      if (k === 'ArrowDown' || k === 'ArrowUp') { e.preventDefault(); ui.move(k === 'ArrowDown' ? 1 : -1); return true; }
      if ((k === 'Enter' || k === 'Tab') && x) { e.preventDefault(); pick(k === 'Tab'); return true; }
      if (k === 'ArrowRight' && x && (x.group || x.open)) { e.preventDefault(); pick(true); return true; }
      var c = ctx();
      // Backspace after a bare "@" deletes it: the user never stepped in.
      var back = k === 'ArrowLeft' || (k === 'Backspace' && !(st.viaAt && !st.path.length));
      if (back && st.group && c && !c.query) { e.preventDefault(); up(); return true; }
      return false;
    }

    // The + button: types a "/" after the caret or selection (spaced off a
    // word), and its input event opens the palette on it.
    function toggle() {
      if (st.open) { dismiss(); return; }
      var at = input.selectionEnd;
      var prev = at > 0 ? input.value[at - 1] : '';
      var text = prev && !/\s/.test(prev) && !CJK.test(prev) ? ' /' : '/';
      st.auto = { at: at, text: text };
      root.__hyloTextareaInsert(input, at, at, text);
    }

    return { close: close, toggle: toggle, handleKeydown: handleKeydown };
  };
})(typeof window !== 'undefined' ? window : globalThis);
