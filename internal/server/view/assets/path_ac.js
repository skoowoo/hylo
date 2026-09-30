// __vaultrPathAcCreate — shared autocomplete factory for the chat textarea.
// Creates a self-contained autocomplete controller bound to a specific
// input element and dropdown list element. Agnostic of which character(s)
// actually trigger it and of how a kind's dirPath/partial get built up —
// that's entirely parseCtx's call (home.js currently routes both under
// "@": a bare query searches notes, typing "/" switches to browsing
// folders). Supports two ctx.kind values:
//   'path'    (default/back-compat) — lists a directory's children, fetched
//             once per dirPath and cached, then filtered client-side as the
//             partial segment changes. Two distinct ways to pick one:
//               Tab   → applyContinue(): appends "name/" right after the
//                       partial (ctx.replaceStart) and reopens for the next
//                       level — a plain, unlimited-depth directory browser,
//                       "@" and all, exactly like typing a path anywhere
//                       else. Nothing about a Tab pick is terminal.
//               Enter/click → applyFinal(): rewrites the *whole* token (from
//                       ctx.tokenStart — dirPath already has every earlier
//                       "/"-segment folded into it) into a clean, "@"-free
//                       vault path ("/journal/2026/lym/"), and closes.
//             Either way, once the session genuinely ends — finalized,
//             Escape'd, blurred, or just typed past — close() quietly
//             strips whatever "@" is still sitting at ctx.atPos (tracked in
//             st.pendingStripAt), so a Tab-heavy browse never leaves the
//             trigger character behind even if the user never hits Enter.
//   'mention' → live-searches notes by filename via opts.search() on every
//             debounced keystroke (no caching — the query itself changes,
//             not just a filter over a fixed list). Both Tab and Enter/click
//             finalize the same way — replaces from ctx.replaceStart with
//             "[[stem]] " — there's no next level to browse into.
//
// opts:
//   getInput()            → HTMLInputElement | HTMLTextAreaElement
//   getList()             → HTMLUListElement  (the dropdown)
//   getPanel()            → HTMLElement  (optional — outer box that owns
//                            open/close visibility; falls back to getList()
//                            when omitted, so callers that never wrapped the
//                            list in a panel keep working unchanged)
//   getHint()             → HTMLElement  (optional — persistent footer row
//                            below the list; mention mode only)
//   parseCtx(val, caret)  → { kind: 'path', dirPath, partial, tokenStart, atPos, replaceStart } |
//                            { kind: 'mention', partial, replaceStart } | null
//   search(query, signal) → Promise<Array<{label, value, sub}>>  (mention mode only)
//   getDefaultItems()     → Array<{label, value, sub}>  (optional — shown
//                            for a bare "@" instead of the hint alone, e.g.
//                            the editor's currently open tabs; put whichever
//                            item should read as "the pick" first — it gets
//                            the same aria-selected highlight item 0 always
//                            gets, no separate marker needed)
//   onApply(el, newVal, caretPos) → void  (update element value + cursor)
//   escKey                → string key for __vaultrEscPush / __vaultrEscPop
//
function __vaultrPathAcCreate(opts) {
  var st = { seq: 0, abort: null, tick: null, active: -1, fetchedDir: null, cachedDirs: [], pendingStripAt: -1 };
  // Shown for a bare "@" only when there's nothing else to list (no open
  // tabs to default to) — the one moment neither mode has anything to show
  // results for, so it's the natural place to surface both things "@" can
  // do (mirrors the empty-chat onboarding hint in home.go). Once there's an
  // actual list on screen (default tabs or search results), the same text
  // moves to the persistent footer via getHint() instead.
  var EMPTY_MENTION_HINT = 'Type to search notes, or "/" to browse folders';

  function writeValue(input, newVal, caretPos) {
    if (opts.onApply) {
      opts.onApply(input, newVal, caretPos);
    } else {
      input.value = newVal;
      if (input.setSelectionRange) input.setSelectionRange(caretPos, caretPos);
      input.focus();
    }
  }

  // Silently removes a lingering "@" left over from a path session that
  // ended without an explicit finalize (Tab-ed a level or two, then the
  // user Escaped/blurred/typed past it instead of pressing Enter). Called
  // from close() so every exit path gets it for free. The character check
  // is a sanity guard, not a real race — if the text at that position isn't
  // "@" any more, something else already changed it; leave it alone.
  function stripPendingAt() {
    if (st.pendingStripAt < 0) return;
    var pos = st.pendingStripAt;
    st.pendingStripAt = -1;
    var input = opts.getInput();
    if (!input || input.value[pos] !== '@') return;
    var newVal = input.value.slice(0, pos) + input.value.slice(pos + 1);
    var caret = input.selectionStart;
    writeValue(input, newVal, caret > pos ? caret - 1 : caret);
  }

  // The panel is whatever owns open/close visibility — a dedicated wrapper
  // when the caller gave one (so a footer hint can sit beside the list
  // inside the same box), otherwise the list itself, unchanged from before
  // getPanel existed.
  function panelEl() { return (opts.getPanel && opts.getPanel()) || opts.getList(); }

  function setHint(text) {
    var hintEl = opts.getHint && opts.getHint();
    if (!hintEl) return;
    if (text) { hintEl.textContent = text; hintEl.hidden = false; }
    else { hintEl.textContent = ''; hintEl.hidden = true; }
  }

  function close() {
    if (st.abort) { st.abort.abort(); st.abort = null; }
    clearTimeout(st.tick); st.tick = null;
    stripPendingAt();
    var panel = panelEl();
    if (panel) { panel.classList.remove('open'); panel.hidden = true; panel.setAttribute('aria-expanded', 'false'); }
    var list = opts.getList();
    if (list) list.innerHTML = '';
    setHint('');
    st.active = -1;
    if (window.__vaultrEscPop && opts.escKey) window.__vaultrEscPop(opts.escKey);
  }

  function listItems() {
    var list = opts.getList();
    return list ? list.querySelectorAll('li[role="option"]') : [];
  }

  function setActive(ix) {
    var items = listItems(); if (!items.length) return;
    if (ix < 0) ix = 0; if (ix >= items.length) ix = items.length - 1;
    st.active = ix;
    items.forEach(function(el, j) { el.setAttribute('aria-selected', j === ix ? 'true' : 'false'); });
    var sel = null;
    items.forEach(function(el) { if (el.getAttribute('aria-selected') === 'true') sel = el; });
    if (sel) sel.scrollIntoView({ block: 'nearest' });
  }

  function openList() {
    var panel = panelEl();
    if (panel) { panel.classList.add('open'); panel.hidden = false; panel.setAttribute('aria-expanded', 'true'); }
    if (window.__vaultrEscPush && opts.escKey) window.__vaultrEscPush(opts.escKey, close);
  }

  // items: Array<{label, value, sub?}> — sub (mention mode's folder line)
  // renders as a second block line under label; dir mode leaves it unset and
  // gets the original single-line row, byte-for-byte unchanged. Item 0 always
  // gets aria-selected="true" (the same highlight ArrowUp/Down move around),
  // so a caller wanting a particular row to read as "the pick" — e.g. the
  // default mention list putting the editor's active tab first — gets that
  // for free just by ordering, no separate badge or dedicated color needed.
  function render(items, emptyMsg) {
    var list = opts.getList(); if (!list) return;
    list.innerHTML = '';
    if (!items.length) {
      var li0 = document.createElement('li');
      li0.className = 'path-ac-muted'; li0.textContent = emptyMsg || 'No results';
      li0.setAttribute('role', 'presentation'); list.appendChild(li0);
      st.active = -1; return;
    }
    items.forEach(function(item, i) {
      var li = document.createElement('li');
      li.setAttribute('role', 'option'); li.setAttribute('data-name', item.value);
      li.setAttribute('aria-selected', i === 0 ? 'true' : 'false');
      if (item.sub) {
        var main = document.createElement('span'); main.className = 'path-ac-main'; main.textContent = item.label;
        var sub = document.createElement('span'); sub.className = 'path-ac-sub'; sub.textContent = item.sub;
        li.appendChild(main); li.appendChild(sub);
      } else {
        li.textContent = item.label;
      }
      // A click is a decisive pick, same as Enter — finalize, don't drill on.
      li.addEventListener('mousedown', function(ev) { ev.preventDefault(); applyFinal(item.value); });
      list.appendChild(li);
    });
    st.active = 0;
  }

  function doFilterDir(ctx) {
    st.pendingStripAt = ctx.atPos;
    var pref = (ctx.partial || '').toLowerCase();
    var names = st.cachedDirs.filter(function(d) { return !pref || d.toLowerCase().indexOf(pref) === 0; });
    var items = names.map(function(name) { return { label: name + '/', value: name, sub: null }; });
    var noMatch = !st.cachedDirs.length ? 'No folders' : 'No match';
    render(items, items.length ? '' : noMatch);
    setHint(''); // footer text is mention-only; folder browsing never shows it
    openList();
  }

  async function doFetchDir(ctx0) {
    var mySeq = st.seq;
    st.abort = new AbortController();
    try {
      var resp = await fetch('/api/vault/list-dirs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: ctx0.dirPath }), signal: st.abort.signal,
      });
      if (mySeq !== st.seq) return;
      if (!resp.ok) { close(); return; }
      var data = await resp.json();
      st.fetchedDir = typeof data.path === 'string' ? data.path : ctx0.dirPath;
      st.cachedDirs = Array.isArray(data.dirs) ? data.dirs : [];
      var input = opts.getInput(); if (!input) return;
      var ctxNow = opts.parseCtx(input.value, input.selectionStart);
      if (!ctxNow || ctxNow.kind === 'mention' || ctxNow.dirPath !== st.fetchedDir) return;
      doFilterDir(ctxNow);
    } catch(e) { if (e.name !== 'AbortError') close(); }
  }

  // Bare "@" (no query yet): show the open-tabs default list when there is
  // one, otherwise fall back to the plain hint row. Shared by refresh() and
  // doFetchSearch's empty-partial short-circuit below.
  function showMentionDefault() {
    var items = (opts.getDefaultItems && opts.getDefaultItems()) || [];
    if (items.length) {
      render(items, '');
      setHint(EMPTY_MENTION_HINT); // list is now occupied, so the "how to use this" text moves to the footer
    } else {
      render([], EMPTY_MENTION_HINT);
      setHint('');
    }
    openList();
  }

  async function doFetchSearch(ctx0) {
    // The debounced timer can still fire after the partial was deleted back
    // to empty (e.g. typed "@x" then backspaced to "@" within the debounce
    // window) — the search API 400s on an empty q, so short-circuit to the
    // same default refresh() shows instead of firing a doomed request.
    if (!(ctx0.partial || '')) { showMentionDefault(); return; }
    var mySeq = st.seq;
    st.abort = new AbortController();
    try {
      var items = await opts.search(ctx0.partial, st.abort.signal);
      if (mySeq !== st.seq) return;
      var input = opts.getInput(); if (!input) return;
      var ctxNow = opts.parseCtx(input.value, input.selectionStart);
      if (!ctxNow || ctxNow.kind !== 'mention') return;
      render(items || [], (items && items.length) ? '' : 'No matching notes');
      setHint(''); // once real search results (or a "no match" state) are showing, the footer hint is no longer needed
      openList();
    } catch (e) { if (!e || e.name !== 'AbortError') close(); }
  }

  function schedule() {
    if (st.abort) { st.abort.abort(); st.abort = null; }
    clearTimeout(st.tick); st.seq++;
    st.tick = setTimeout(function() {
      st.tick = null;
      var input = opts.getInput(); if (!input) return;
      var ctx = opts.parseCtx(input.value, input.selectionStart);
      if (!ctx) { close(); return; }
      if (ctx.kind === 'mention') { void doFetchSearch(ctx); } else { void doFetchDir(ctx); }
    }, 160);
  }

  function refresh() {
    var input = opts.getInput(); if (!input) return;
    var ctx = opts.parseCtx(input.value, input.selectionStart);
    if (!ctx) { close(); return; }
    if (ctx.kind === 'mention') {
      // Bare "@" (no query yet) — show the default (open tabs) list instead
      // of firing a search for an empty query.
      if (!(ctx.partial || '')) {
        st.seq++;
        if (st.abort) { st.abort.abort(); st.abort = null; }
        clearTimeout(st.tick); st.tick = null;
        showMentionDefault();
        return;
      }
      schedule();
      return;
    }
    if (st.fetchedDir !== null && ctx.dirPath === st.fetchedDir) { doFilterDir(ctx); return; }
    schedule();
  }

  // Finalize: mention or path, always closes. For a path pick this rebuilds
  // the entire token from scratch (tokenStart), producing a clean "@"-free
  // vault path regardless of how many Tab-picks led up to it.
  function applyFinal(value) {
    var input = opts.getInput(); if (!input) return;
    var ctx = opts.parseCtx(input.value, input.selectionStart);
    if (!ctx) { close(); return; }
    var suffix = input.value.slice(input.selectionStart);
    var prefix, insert;
    if (ctx.kind === 'mention') {
      prefix = input.value.slice(0, ctx.replaceStart);
      insert = '[[' + value + ']] ';
    } else {
      prefix = input.value.slice(0, ctx.tokenStart);
      insert = (ctx.dirPath === '/' ? '/' : ctx.dirPath + '/') + value + '/';
    }
    var nextCaret = prefix.length + insert.length;
    st.pendingStripAt = -1; // we just wrote the clean form ourselves
    writeValue(input, prefix + insert + suffix, nextCaret);
    close();
  }

  // Continue: path mode only — Tab keeps browsing. Replaces just the
  // partial segment (ctx.replaceStart) with "name/", leaving the "@" (and
  // any pathPrefix before it) untouched, then reopens for the next level.
  // Falls back to applyFinal for mention (there's nothing to continue).
  function applyContinue(value) {
    var input = opts.getInput(); if (!input) return;
    var ctx = opts.parseCtx(input.value, input.selectionStart);
    if (!ctx || ctx.kind !== 'path') { applyFinal(value); return; }
    var prefix = input.value.slice(0, ctx.replaceStart);
    var suffix = input.value.slice(input.selectionStart);
    var insert = value + '/';
    var nextCaret = prefix.length + insert.length;
    st.pendingStripAt = ctx.atPos;
    writeValue(input, prefix + insert + suffix, nextCaret);
    refresh();
  }

  function isOpen() {
    var panel = panelEl();
    return !!(panel && panel.classList.contains('open'));
  }

  // Call from a keydown handler. Returns true if the event was consumed.
  function handleKeydown(ev) {
    // Never intercept keys mid IME composition (e.g. the Enter that commits
    // pinyin → hanzi) — keyCode 229 is the classic fallback for engines that
    // don't reliably flip isComposing off in time for that same keydown.
    if (ev.isComposing || ev.keyCode === 229) return false;
    var open = isOpen();
    // Escape always works, even with zero selectable items (e.g. browsing
    // into an empty folder) — it's the one guaranteed way out of a
    // Tab-started session, so it can't be gated behind having options to
    // navigate.
    if (open && ev.key === 'Escape') { close(); return true; }
    var items = open ? listItems() : [];
    if (open && items.length) {
      if (ev.key === 'ArrowDown') {
        ev.preventDefault();
        var nDown = st.active < 0 ? 0 : st.active + 1;
        setActive(nDown >= items.length ? 0 : nDown); return true;
      }
      if (ev.key === 'ArrowUp') {
        ev.preventDefault();
        var pUp = st.active < 0 ? items.length - 1 : st.active - 1;
        setActive(pUp < 0 ? items.length - 1 : pUp); return true;
      }
      if (ev.key === 'Tab') {
        ev.preventDefault();
        var pickT = items[st.active < 0 ? 0 : st.active];
        var nmT = pickT && pickT.getAttribute('data-name');
        if (nmT) applyContinue(nmT); return true;
      }
      if (ev.key === 'Enter') {
        var pickE = items[st.active < 0 ? 0 : st.active];
        var nmE = pickE && pickE.getAttribute('data-name');
        if (nmE) { ev.preventDefault(); applyFinal(nmE); return true; }
      }
    }
    return false;
  }

  return { close: close, refresh: refresh, schedule: schedule, handleKeydown: handleKeydown, isOpen: isOpen };
}
