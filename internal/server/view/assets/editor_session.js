  // Editor session: one CodeMirror view, its mode, and save/load.
  // Pane chrome (tabs, split, rename, compile) lives in content_pane.js.
  // ── Editor state ────────────────────────────────────────────────────────────
  // One CodeMirror 6 EditorView (s.view) for the whole editor now — the old
  // Milkdown (WYSIWYG) / CodeMirror (source) split is gone. "inSource"/
  // editorMode still exist (see below) but now mean "decorations
  // Compartment reconfigured to plain syntax highlighting" vs "live-preview
  // decorations active", toggled in place on the same view/doc instead of
  // switching between two separately-mounted editors.
  var __hyloEditor = {
    view: null,
    initPromise: null, dirty: false,
    currentPath: '', currentMd: '',
    saveTimer: null,
    // Names (with ".md") confirmed missing from the vault for the note
    // currently loaded — see __hyloEditorRevalidateWikiLinks. Cleared and
    // recomputed on every note/tab load; stale until that async check lands.
    brokenWikiLinkNames: new Set(),
    wikiLinkRevalidateSeq: 0,
    decoCompartment: null, readCompartment: null,
    inSource: false, pendingScrollRaf: null, pendingOpenScroll: null,
    // reading: global user preference, survives note/tab switches and restarts.
    // readingActive: what the view is actually showing — drafts stay editable
    // even while the preference is on.
    reading: false, readingActive: false,
  };
  // The editor.js module. Session state stays on __hyloEditor; CodeMirror
  // symbols stay here so init doesn't copy them onto the session.
  var __hyloCM = null;
  try { __hyloEditor.reading = localStorage.getItem('hylo.reading') === '1'; } catch(_) {}
  function __hyloEditorSetReadingPref(on) {
    __hyloEditor.reading = on;
    try { localStorage.setItem('hylo.reading', on ? '1' : '0'); } catch(_) {}
  }


  // ── Save helpers ─────────────────────────────────────────────────────────────
  function __hyloEditorSaveStatus(txt) {
    var el = document.getElementById('content-pane-save-status');
    if (!el) return;
    clearTimeout(el._ssiTimer);
    if (txt === '●') {
      el.dataset.state = 'pending';
    } else if (txt === 'Saved') {
      el.dataset.state = 'saved';
      el._ssiTimer = setTimeout(function() { el.dataset.state = ''; }, 2000);
    } else {
      el.dataset.state = '';
    }
  }
  function __hyloEditorScheduleSave() {
    __hyloEditorSaveStatus('●');
    clearTimeout(__hyloEditor.saveTimer);
    __hyloEditor.saveTimer = setTimeout(__hyloEditorDoSave, 800);
  }
  async function __hyloEditorDoSave() {
    if (!__hyloEditor.dirty) return;
    var path = __hyloEditor.currentPath;
    var content = __hyloEditor.currentMd;
    if (!path) return;
    try {
      var r = await fetch('/api/vault/write', {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({path: path, content: content}),
      });
      if (r.ok) {
        __hyloEditor.dirty = false; __hyloEditorSaveStatus('Saved');
        // A plain content save is the common case (every ~800ms of idle after
        // typing). Patch the row from the response. Tags and wikilinks are
        // diffed against the last loaded/saved body so unchanged typing does
        // not drop the tag cloud or the graph.
        var saved = null; try { saved = await r.json(); } catch(_) {}
        if (saved) __hyloPatchNoteRowAfterSave(path, saved);
        var diff = __hyloNoteSigDiff(path, content);
        if (window.__hyloVaultChanged) window.__hyloVaultChanged({
          op: 'write', path: path, tagsChanged: diff.tagsChanged, linksChanged: diff.linksChanged,
        });
      } else {
        var errText = ''; try { errText = await r.text(); } catch(_) {}
        window.showError(errText || 'Server error — your changes may not be saved.', 'Save error');
      }
    } catch(e) {
      window.showError((e && e.message) || 'Network error — your changes may not be saved.', 'Save error');
    }
  }
  // Mirrors the "rows" template's path-line markup (home.go): a
  // .home-note-row-preview span shown when there's an excerpt, replacing
  // the (still-present, CSS-hidden) .home-note-row-dir span — see
  // home.css's ":not(.is-grid) .home-note-row-preview ~ .home-note-row-dir"
  // rule. Patches every rendered copy of the row, including sections the
  // home pane has parked offscreen — a content save doesn't refetch those.
  function __hyloPatchNoteRowAfterSave(path, note) {
    var rows = document.querySelectorAll('.home-note-row[data-note-path="' + CSS.escape(path) + '"]');
    for (var r = 0; r < rows.length; r++) __hyloPatchOneNoteRow(rows[r], note);
  }
  function __hyloPatchOneNoteRow(row, note) {
    var meta = row.querySelector('.home-note-row-meta');
    if (!meta) return;

    var previewEl = meta.querySelector('.home-note-row-preview');
    var previewText = (note.preview && note.preview.text) || '';
    if (previewText) {
      if (!previewEl) {
        previewEl = document.createElement('span');
        previewEl.className = 'home-note-row-preview';
        meta.insertBefore(previewEl, meta.firstChild);
      }
      previewEl.textContent = previewText;
    } else if (previewEl) {
      previewEl.remove();
    }

    // formatRelativeTime (search.go) returns exactly this for anything under
    // a minute old, which a just-completed save always is.
    var timeEl = meta.querySelector('.home-note-row-time');
    if (timeEl) timeEl.textContent = 'just now';
  }

  // ── Misc helpers ─────────────────────────────────────────────────────────────
  // __hyloEditorTightenLists / __hyloEditorSameListMarker → list_tighten.js
  // (same concatenated scope; split out so it's unit-testable headlessly).
  function __hyloEditorFindImageFile(dt) {
    if (!dt) return null;
    var items = Array.from(dt.items || []);
    for (var i = 0; i < items.length; i++) {
      if (items[i].kind === 'file' && items[i].type.indexOf('image/') === 0) return items[i].getAsFile();
    }
    return null;
  }
  async function __hyloEditorUploadImage(imgFile) {
    var fd = new FormData();
    fd.append('file', imgFile);
    var resp = await fetch('/api/vault/upload-image', {method: 'POST', body: fd});
    if (!resp.ok) throw new Error(await resp.text());
    var uploaded = await resp.json();
    if (window.__hyloVaultChanged) window.__hyloVaultChanged({ op: 'images' });
    return uploaded.src;
  }

  // ── Note materialization ─────────────────────────────────────────────────────
  // A brand-new tab has no path — and nothing on the server — until its
  // first real edit (see __hyloEditorHandleContentChange below). There is
  // no draft state to load/flush/discard before that point.
  function __hyloEditorActiveTab() {
    var pane = window.__hyloContentPane;
    return pane ? pane.tabs[pane.activeTab] : null;
  }
  function __hyloEditorIsActiveTabId(tabId) {
    if (!tabId) return true;
    var pane = window.__hyloContentPane;
    var tab = pane && pane.tabs[pane.activeTab];
    return !!(tab && tab.id === tabId);
  }
  async function __hyloEditorSaveTabForLeave(tab) { await tabStateManager.saveForLeave(tab); }

  // _materializing lives on the tab itself, not a shared variable — two
  // different unmaterialized tabs can be in flight at once.
  async function __hyloEditorMaterializeTab(tab, md) {
    if (tab._materializing) return;
    tab._materializing = true;
    if (__hyloEditorIsActiveTabId(tab.id)) __hyloEditorSaveStatus('●');
    try {
      var resp = await fetch('/api/vault/create-untitled', {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({content: md}),
      });
      if (tab.path) return; // defensive — nothing else should set this first
      if (!resp.ok) {
        var errText = ''; try { errText = await resp.text(); } catch(_) {}
        window.showError(errText || 'Server error — your note may not be saved.', 'Save error');
        return;
      }
      var data = await resp.json();
      tab.path = data.path;
      tab.title = __hyloStripMdExt(data.path.split('/').pop()) || data.path;
      delete tab._pendingContent;
      __hyloNoteSigRemember(data.path, md);
      var created = __hyloNoteSigFromContent(md);
      if (window.__hyloVaultChanged) window.__hyloVaultChanged({
        op: 'create', path: data.path, dir: __hyloNoteDir(data.path),
        tagsChanged: created.tags !== '', linksChanged: created.links !== '',
      });
      if (!__hyloEditorIsActiveTabId(tab.id)) return; // switched away while this was in flight — nothing to autosave right now
      var s = __hyloEditor;
      s.currentPath = data.path;
      s.dirty = true; // re-arm autosave — more may have been typed while the create call was in flight
      __hyloEditorScheduleSave();
    } catch(e) {
      window.showError((e && e.message) || 'Network error — your note may not be saved.', 'Save error');
    } finally {
      tab._materializing = false;
    }
  }
  // Called only for real transactions — note/tab switches go through
  // view.setState() (see s._buildState), which never reaches the
  // updateListener below at all, so every docChanged transaction that does
  // arrive here is a real edit (typing, paste-image insert, frontmatter
  // dialog apply, undo/redo, …). No text comparison: the transaction having
  // happened at all is the dirty signal.
  function __hyloEditorHandleContentChange(md) {
    var s = __hyloEditor;
    var pane = window.__hyloContentPane;
    if (pane && !pane.contentPaneOpen) return;
    s.currentMd = md;
    var tab = __hyloEditorActiveTab();
    if (tab && !tab.path) {
      // In-memory only (never persisted) — lets switching back to this tab
      // while its create-untitled call is still in flight show what was
      // typed instead of a blank editor. Cleared once materialized.
      tab._pendingContent = md;
      void __hyloEditorMaterializeTab(tab, md);
      return;
    }
    s.dirty = true;
    __hyloEditorScheduleSave();
  }

  // Mirrors internal/util/mdhtml.go's wikilinkRe + name normalization so the
  // client can batch-check the same target names the server would resolve.
  var __hyloWikilinkRe = /\[\[([^\]\[|]+?)(?:\|[^\]\[]+?)?\]\]/g;

  function __hyloWikiLinkTargetName(raw) {
    var name = (raw || '').trim();
    if (!/\.md$/i.test(name)) name += '.md';
    return name;
  }

  function __hyloExtractWikilinkNames(md) {
    var names = [], seen = {}, m;
    __hyloWikilinkRe.lastIndex = 0;
    while ((m = __hyloWikilinkRe.exec(md || ''))) {
      var name = __hyloWikiLinkTargetName(m[1]);
      if (!seen[name]) { seen[name] = true; names.push(name); }
    }
    return names;
  }

  // Batch-checks which [[wikilink]] targets in the note just loaded into the
  // editor still exist, then dispatches wikiLinksRevalidated so the live
  // preview repaints any that are gone as broken (see liveOptions.isWikiLinkBroken
  // above). Fire-and-forget; wikiLinkRevalidateSeq (bumped by the caller
  // before this runs) guards against a slow response landing after the user
  // has already switched to a different note.
  async function __hyloEditorRevalidateWikiLinks() {
    var s = __hyloEditor;
    var seq = s.wikiLinkRevalidateSeq;
    var names = __hyloExtractWikilinkNames(s.currentMd);
    if (!names.length) return;
    try {
      var r = await fetch('/api/notes/exist', {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({names: names}),
      });
      if (!r.ok || seq !== s.wikiLinkRevalidateSeq) return;
      var data = await r.json();
      var existing = {};
      (data.existing || []).forEach(function(n) { existing[n] = true; });
      var broken = new Set();
      names.forEach(function(n) { if (!existing[n]) broken.add(n); });
      s.brokenWikiLinkNames = broken;
      if (s.view && __hyloCM) {
        s.view.dispatch({effects: __hyloCM.wikiLinksRevalidated.of(null)});
      }
    } catch (_) {}
  }

  // Open a wiki-link target in the pane.  value is the raw [[…]] inner text.
  async function __hyloContentPaneOpenWikiLink(value) {
    var pane = window.__hyloContentPane;
    if (!pane) return;
    // Path-like value (contains /): treat as vault-absolute path directly
    if (value.indexOf('/') !== -1) {
      var p = value.startsWith('/') ? value : '/' + value;
      if (!p.endsWith('.md')) p += '.md';
      await pane.openNoteInContentPane(p, p.split('/').pop().replace(/\.md$/, ''), false, false);
      return;
    }
    // Bare name: resolve through the server
    var nm = value.endsWith('.md') ? value : value + '.md';
    try {
      var r = await fetch('/api/notes/resolve', {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({name: nm}),
      });
      if (!r.ok) {
        if (window.showError) window.showError('Could not resolve note: ' + value, 'Note not found');
        return;
      }
      var data = await r.json();
      if (!data.matches || !data.matches.length) {
        if (window.showError) window.showError('「' + value + '」 does not exist in the vault.', 'Note not found');
        return;
      }
      var note = data.matches[0];
      var notePath = note.dir === '/' ? '/' + note.name : note.dir + '/' + note.name;
      var noteTitle = note.title || note.name.replace(/\.md$/, '');
      var _isK = note.origin === 'plugin:compile';
      var _isI = note.origin === 'plugin:index';
      await pane.openNoteInContentPane(notePath, noteTitle, _isK, !!note.pinned, _isI,
        !_isK && !_isI && !note.compile_count);
    } catch(_) {}
  }

  function __hyloEditorSaveTabState(tabId) { tabStateManager.save(tabId); }
  function __hyloEditorRestoreTabState(tabId) { return tabStateManager.restore(tabId); }
  function __hyloEditorClearTabState(tabId) { tabStateManager.clear(tabId); }
  function __hyloEditorSyncViewButtons() {
    var s = __hyloEditor;
    document.querySelectorAll('.content-pane-view-btn-wysiwyg').forEach(function(btn) {
      btn.classList.toggle('active', !s.inSource);
    });
    document.querySelectorAll('.content-pane-reading-btn').forEach(function(btn) {
      btn.classList.toggle('active', s.reading);
    });
    document.querySelectorAll('.content-pane-view-btn-source').forEach(function(btn) {
      btn.classList.toggle('active', s.inSource);
    });
    document.querySelectorAll('.content-pane-source-toggle-btn').forEach(function(btn) {
      btn.classList.toggle('active', s.inSource);
    });
  }
  // Bookkeeping only, no dispatch — shared by editorMode's reconfigure()
  // (user toggles mode on the current tab) and __hyloEditorApplyState
  // (a note/tab switch already baked the right mode into the new state).
  function __hyloEditorSetModeState(inSource, reading) {
    var s = __hyloEditor;
    s.inSource = inSource;
    s.readingActive = reading;
    __hyloEditorSyncViewButtons();
  }

  // ── Lazy editor init ─────────────────────────────────────────────────────────
  async function __hyloEnsureContentPaneEditor() {
    var s = __hyloEditor;
    if (s.view) return;
    if (s.initPromise) return s.initPromise;
    s.initPromise = (async function() {
      var cm = await import('/static/editor.js');
      __hyloCM = cm;

      var editArea = document.getElementById('content-pane-edit-area');

      var cmTheme = cm.EditorView.theme({
        '&': {height:'100%',color:'var(--prose-body)',background:'transparent'},
        '&.cm-focused': {outline:'none'},
        '.cm-content': {caretColor:'var(--accent)'},
        '.cm-cursor,.cm-dropCursor': {borderLeftColor:'var(--accent)'},
        '.cm-selectionBackground': {background:'var(--selection-bg) !important'},
        '&.cm-focused .cm-selectionBackground': {background:'var(--selection-bg)'},
        '.cm-activeLine': {background:'var(--cm-active-line)'},
        '.cm-gutters': {display:'none'},
      });
      // Source mode only. Live preview's token colors come from
      // liveDecorations(); this highlight is what replaces them.
      var sourceHighlight = cm.syntaxHighlighting(cm.HighlightStyle.define([
        {tag:cm.tags.heading1,color:'var(--h1)',fontWeight:'600'},
        {tag:cm.tags.heading2,color:'var(--h2)',fontWeight:'600'},
        {tag:cm.tags.heading3,color:'var(--h3)',fontWeight:'600'},
        {tag:cm.tags.heading4,color:'var(--h4)',fontWeight:'500'},
        {tag:cm.tags.emphasis,fontStyle:'italic',color:'var(--prose-em)'},
        {tag:cm.tags.strong,fontWeight:'600',color:'var(--prose-strong)'},
        {tag:cm.tags.link,color:'var(--p1)'},{tag:cm.tags.url,color:'var(--p1)',opacity:'0.72'},
        {tag:cm.tags.monospace,color:'var(--code-tx)'},{tag:cm.tags.meta,color:'var(--cm-md-muted)'},
        {tag:cm.tags.punctuation,color:'var(--cm-md-muted)'},
        {tag:cm.tags.processingInstruction,color:'var(--cm-md-muted)'},
        {tag:cm.tags.strikethrough,color:'var(--muted)',textDecoration:'line-through'},
      ]));

      // One language for every note. Rebuilding it per tab would reparse.
      var sharedLanguage = cm.wikiMarkdownLanguage();

      var liveOptions = {
        resolveImageSrc: function(filename) {
          return '/api/images/serve?name=' + encodeURIComponent(filename);
        },
        onWikiLinkClick: function(target) { void __hyloContentPaneOpenWikiLink(target); },
        // Populated (async, after each note load) by __hyloEditorRevalidateWikiLinks.
        // Mutating the set alone doesn't repaint; that function also dispatches
        // wikiLinksRevalidated.
        isWikiLinkBroken: function(target) { return s.brokenWikiLinkNames.has(__hyloWikiLinkTargetName(target)); },
        onEditFrontmatter: __hyloEditorEditFrontmatter,
      };
      s.decoCompartment = new cm.Compartment();
      s.readCompartment = new cm.Compartment();
      s._modeEffects = function(inSource, reading) {
        return [
          s.decoCompartment.reconfigure(inSource ? [sourceHighlight] : cm.liveDecorations(liveOptions)),
          s.readCompartment.reconfigure((!inSource && reading) ? cm.readingExtensions() : []),
        ];
      };

      // Extensions for every EditorState we build — one per note/tab switch
      // now (see s._buildState below), not just the initial mount.
      // decoInitial/readInitial seed s.decoCompartment/s.readCompartment with
      // whatever mode this particular state should start in; editorMode's
      // reconfigure() (below) swaps them later via view.dispatch() without
      // needing a new state, since Compartments are reusable across states.
      s._buildExtensions = function(decoInitial, readInitial) {
        return [
          sharedLanguage,
          cm.history(),
          // listIndentExtension is Prec.highest internally (see
          // cm-live/list-indent.js) — @codemirror/lang-markdown's own
          // language support registers a high-precedence Enter binding for
          // "continue list markup" that a plain keymap.of(...) here would
          // lose to regardless of array position. It handles Tab/Shift-Tab
          // (not bound by defaultKeymap at all — every outliner-style
          // editor claims them, same as Cmd+]/Cmd+[) and Enter on an empty
          // list item specifically (the built-in path is supposed to
          // outdent/exit there but unreliably just inserts a blank line
          // with the marker left dangling instead); a non-empty item's
          // Enter isn't handled here and falls through to defaultKeymap.
          cm.listIndentExtension,
          // Right-click selection formatting. Outside the mode compartments,
          // so it works in both live-preview and source mode. CM6 composes
          // multiple domEventHandlers independently, so this stays separate
          // from the paste handler below.
          cm.selectionFormatMenu(),
          cm.keymap.of([].concat(cm.defaultKeymap, cm.historyKeymap)),
          cm.search({ top: true, createPanel: __hyloCreateSearchPanel }),
          cm.EditorView.lineWrapping, cmTheme,
          cm.EditorView.contentAttributes.of({spellcheck: 'false'}),
          cm.linkClickHandler(),
          // Outside the compartment so the collapsed flag survives a
          // source/live-preview toggle. The header widget that reads it
          // lives inside liveDecorations() and is torn down in source mode.
          cm.frontmatterCollapseField,
          s.decoCompartment.of(decoInitial),
          s.readCompartment.of(readInitial),
          cm.EditorView.updateListener.of(function(update) {
            // Note/tab switches (s._buildState + view.setState(), see below)
            // never reach this listener at all — setState() doesn't fire
            // updateListener, confirmed against the CM6 version bundled in
            // editor.js. So every docChanged transaction that does arrive
            // here is a real edit; no "is this programmatic" flag needed.
            if (!update.docChanged) return;
            __hyloEditorHandleContentChange(update.state.doc.toString());
          }),
          cm.EditorView.domEventHandlers({
            paste: function(e, view) {
              var imgFile = __hyloEditorFindImageFile(e.clipboardData);
              if (!imgFile) return false;
              e.preventDefault();
              __hyloEditorUploadImage(imgFile).then(function(src) {
                var filename = src.split('/').pop();
                var ins = '![[' + filename + ']]'; var sel = view.state.selection.main;
                view.dispatch({changes:{from:sel.from,to:sel.to,insert:ins},selection:{anchor:sel.from+ins.length}});
              }).catch(function(e) {
                window.showError((e && e.message) || 'Image upload failed.', 'Upload error');
              });
              return true;
            },
          }),
        ];
      };
      // One EditorState per note/tab, built fresh from its saved markdown —
      // used for every note/tab switch (see __hyloEditorApplyState) via
      // view.setState(), never view.dispatch(). setState() swaps the whole
      // state in one shot: no transaction is produced, so there's nothing to
      // exclude from history and nothing for the frontmatter changeFilter to
      // block — a switch just isn't an edit to begin with, instead of being
      // one we have to talk our way around.
      //
      // This used to fall back to a dispatch()'d full-doc replace instead,
      // because LivePreviewPlugin's decorations went stale past the first
      // ~3000 chars of a freshly-built state and never recovered — traced to
      // @codemirror/language only parsing that much of a *fresh* EditorState
      // synchronously (Work.InitViewport), handing the rest to background
      // parsing that lands through a separate dispatch our decoration
      // plugins weren't listening for. Fixed at the source (live-preview.js,
      // horizontal-rule-field.js, frontmatter-collapse.js all now compare
      // syntaxTree(update.state) against the tree they last used — the same
      // check CM6's own built-in TreeHighlighter uses for exactly this), so
      // setState() is safe here again.
      s._buildState = function(content, inSource, reading) {
        return cm.EditorState.create({
          doc: content || '',
          // No saved cursor to restore (tabStateManager only tracks scroll/
          // mode) — land past frontmatter and any leading list/task/heading
          // marker instead of EditorState.create's default of offset 0.
          selection: cm.EditorSelection.cursor(cm.initialCursorOffset(content || '')),
          extensions: s._buildExtensions(
            inSource ? [sourceHighlight] : cm.liveDecorations(liveOptions),
            (!inSource && reading) ? cm.readingExtensions() : []
          ),
        });
      };

      s.view = new cm.EditorView({
        parent: editArea,
        state: s._buildState(s.currentMd, false, false),
      });

      document.querySelectorAll('.content-pane-view-btn-wysiwyg').forEach(function(btn) {
        btn.addEventListener('click', function() {
          if (__hyloEditor.inSource) editorMode.exitSource();
        });
      });
      document.querySelectorAll('.content-pane-view-btn-source').forEach(function(btn) {
        btn.addEventListener('click', function() {
          if (!__hyloEditor.inSource) editorMode.enterSource();
        });
      });
    })();
    return s.initPromise;
  }

  // ── Search panel (custom, top-anchored) ─────────────────────────────────────
  // CodeMirror's Panel API has no declarative x-show/x-transition
  // equivalent (mount() fires once, right after insertion; there's no
  // pre-removal hook), so the slide-in/out that matches the tabs/more
  // menus (content_pane.html, content_pane.css) is done by hand: mount() toggles
  // .is-entering on the .cm-panels-top wrapper CodeMirror already
  // created, and every close goes through this wrapper, which holds the
  // real close call until .is-closing's CSS transition (content_pane.css) has
  // had time to finish.
  //
  // That hold-off is exactly what made Cmd+F/"Find" occasionally look dead:
  // for the ~100ms between adding .is-closing and this timer actually
  // calling realClose(), CM6's own search state still considers the panel
  // open (that state only flips on the real close call). openSearchPanel()
  // called in that window sees "already open" and just refocuses the
  // fading-out DOM instead of reopening it — and this timer, still pending,
  // then closes that freshly-reopened panel a moment later anyway. One
  // fast Escape-then-Cmd+F (or Esc then clicking Find again) was enough to
  // hit it. __hyloEditorOpenFind cancels s._searchCloseTimer before
  // asking CM6 to (re)open, and the timer is tracked as a single id here
  // (not left to stack one per close call) so there's only ever one to
  // cancel.
  function __hyloEditorCloseSearch(view) {
    var cm = __hyloCM;
    if (!cm) return;
    var s = __hyloEditor;
    if (s._searchCloseTimer) { clearTimeout(s._searchCloseTimer); s._searchCloseTimer = null; }
    var panel = document.querySelector('#content-pane-edit-area .cm-panels-top');
    if (!panel) { cm.closeSearchPanel(view); return; }
    panel.classList.add('is-closing');
    s._searchCloseTimer = setTimeout(function() {
      s._searchCloseTimer = null;
      cm.closeSearchPanel(view);
    }, 100);
  }
  function __hyloCreateSearchPanel(view) {
    var cm = __hyloCM;
    var dom = document.createElement('div');
    dom.className = 'hylo-search-panel';

    // Header: what this floating panel is, plus its one non-search control
    // (Close) — kept off the Find row itself so that row is only ever
    // search controls, not a mix of "act on the query" and "dismiss the
    // panel" buttons.
    var headerRow = document.createElement('div');
    headerRow.className = 'hylo-sr-header';
    var headerLabel = document.createElement('span');
    headerLabel.className = 'hylo-sr-header-label';
    headerLabel.textContent = 'Find';

    var closeBtn = document.createElement('button');
    closeBtn.type = 'button'; closeBtn.className = 'hylo-sr-ibtn'; closeBtn.setAttribute('aria-label', 'Close');
    closeBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>';

    headerRow.append(headerLabel, closeBtn);

    // Row 1: Find. Leading chevron reveals/hides row 2 (Replace, below) —
    // collapsed by default since most Cmd+F visits are look-not-change.
    var findRow = document.createElement('div');
    findRow.className = 'hylo-sr-row';

    var expandBtn = document.createElement('button');
    expandBtn.type = 'button'; expandBtn.className = 'hylo-sr-ibtn hylo-sr-expand-btn';
    expandBtn.title = 'Toggle replace'; expandBtn.setAttribute('aria-label', 'Toggle replace');
    expandBtn.setAttribute('aria-expanded', 'false');
    expandBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 6 6 6-6 6"/></svg>';

    var findWrap = document.createElement('div');
    findWrap.className = 'hylo-sr-input-wrap';

    var findInput = document.createElement('input');
    findInput.type = 'text'; findInput.placeholder = 'Find';
    findInput.className = 'field-input hylo-sr-input'; findInput.setAttribute('main-field', '');
    findInput.setAttribute('aria-label', 'Find');

    var findInset = document.createElement('div');
    findInset.className = 'hylo-sr-inset-btns';

    // Match count ("2/7") — a plain label, not a button; sits ahead of the
    // nav icons so the two read together ("2/7, then ↑↓ to move"). Hidden
    // (not just empty) when the field itself is empty, so it doesn't leave
    // a dead gap before you've typed anything.
    var countEl = document.createElement('span');
    countEl.className = 'hylo-sr-count'; countEl.setAttribute('aria-hidden', 'true');
    countEl.style.display = 'none';

    var prevBtn = document.createElement('button');
    prevBtn.type = 'button'; prevBtn.className = 'hylo-sr-ibtn'; prevBtn.title = 'Previous (Shift+Enter)';
    prevBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m18 15-6-6-6 6"/></svg>';

    var nextBtn = document.createElement('button');
    nextBtn.type = 'button'; nextBtn.className = 'hylo-sr-ibtn'; nextBtn.title = 'Next (Enter)';
    nextBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>';

    var caseBtn = document.createElement('button');
    caseBtn.type = 'button'; caseBtn.className = 'hylo-sr-ibtn hylo-sr-toggle'; caseBtn.title = 'Match case';
    caseBtn.textContent = 'Aa';

    findInset.append(countEl, prevBtn, nextBtn, caseBtn);
    findWrap.append(findInput, findInset);
    findRow.append(expandBtn, findWrap);

    // Row 2: Replace — hidden by default (expandBtn toggles it). Prior
    // versions drew Replace/Replace All as bare icons (a return-style arrow
    // and a double-chevron); neither reads unambiguously at this size, so
    // they're short text labels now — same treatment as the "Aa" toggle
    // above, not a second icon language for two actions that matter.
    var replaceRow = document.createElement('div');
    replaceRow.className = 'hylo-sr-row';
    replaceRow.hidden = true;

    // Empty — just holds the column open so replaceWrap lines up under
    // findWrap instead of under expandBtn.
    var replaceSpacer = document.createElement('div');
    replaceSpacer.className = 'hylo-sr-row-spacer';

    var replaceWrap = document.createElement('div');
    replaceWrap.className = 'hylo-sr-input-wrap';

    var replaceInput = document.createElement('input');
    replaceInput.type = 'text'; replaceInput.placeholder = 'Replace';
    replaceInput.className = 'field-input hylo-sr-input'; replaceInput.setAttribute('aria-label', 'Replace');

    var replaceInset = document.createElement('div');
    replaceInset.className = 'hylo-sr-inset-btns';

    var replaceBtn = document.createElement('button');
    replaceBtn.type = 'button'; replaceBtn.className = 'hylo-sr-ibtn hylo-sr-text'; replaceBtn.title = 'Replace (Enter)';
    replaceBtn.textContent = 'Replace';

    var replaceAllBtn = document.createElement('button');
    replaceAllBtn.type = 'button'; replaceAllBtn.className = 'hylo-sr-ibtn hylo-sr-text'; replaceAllBtn.title = 'Replace All';
    replaceAllBtn.textContent = 'All';

    replaceInset.append(replaceBtn, replaceAllBtn);
    replaceWrap.append(replaceInput, replaceInset);
    replaceRow.append(replaceSpacer, replaceWrap);
    dom.append(headerRow, findRow, replaceRow);

    // State
    var caseSensitive = false;

    function buildQuery() {
      return new cm.SearchQuery({ search: findInput.value, caseSensitive: caseSensitive, replace: replaceInput.value });
    }
    // Counts matches by walking the same SearchQuery cursor CM6 itself uses
    // for find-next — no separate/approximate tally, so it can't drift from
    // what Enter/↓ would actually land on. O(doc length); fine at note size.
    function updateMatchCount() {
      if (!findInput.value) { countEl.style.display = 'none'; countEl.textContent = ''; return; }
      countEl.style.display = '';
      var q = buildQuery();
      if (!q.valid) { countEl.textContent = '0/0'; return; }
      var pos = view.state.selection.main.head;
      var cur = q.getCursor(view.state);
      var total = 0, idx = 0, r = cur.next();
      while (!r.done) {
        total++;
        if (!idx && r.value.from >= pos) idx = total;
        r = cur.next();
      }
      countEl.textContent = total ? (idx || 1) + '/' + total : '0/0';
    }
    function commit() { view.dispatch({ effects: cm.setSearchQuery.of(buildQuery()) }); updateMatchCount(); }

    findInput.addEventListener('input', commit);
    replaceInput.addEventListener('input', commit);

    caseBtn.addEventListener('click', function() {
      caseSensitive = !caseSensitive;
      caseBtn.classList.toggle('active', caseSensitive);
      commit(); findInput.focus();
    });
    expandBtn.addEventListener('click', function() {
      var open = replaceRow.hidden;
      replaceRow.hidden = !open;
      expandBtn.classList.toggle('is-open', open);
      expandBtn.setAttribute('aria-expanded', String(open));
      if (open) replaceInput.focus();
    });
    findInput.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') { e.preventDefault(); if (e.shiftKey) cm.findPrevious(view); else cm.findNext(view); updateMatchCount(); }
      if (e.key === 'Escape') { e.preventDefault(); __hyloEditorCloseSearch(view); }
    });
    replaceInput.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') { e.preventDefault(); cm.replaceNext(view); updateMatchCount(); }
      if (e.key === 'Escape') { e.preventDefault(); __hyloEditorCloseSearch(view); }
    });
    prevBtn.addEventListener('click', function() { cm.findPrevious(view); updateMatchCount(); });
    nextBtn.addEventListener('click', function() { cm.findNext(view); updateMatchCount(); });
    replaceBtn.addEventListener('click', function() { cm.replaceNext(view); updateMatchCount(); });
    replaceAllBtn.addEventListener('click', function() { cm.cmReplaceAll(view); updateMatchCount(); });
    closeBtn.addEventListener('click', function() { __hyloEditorCloseSearch(view); });

    return {
      dom: dom,
      top: true,
      mount: function() {
        var sel = view.state.selection.main;
        if (!sel.empty) {
          var txt = view.state.sliceDoc(sel.from, sel.to);
          if (txt && !txt.includes('\n')) { findInput.value = txt; commit(); }
        }
        findInput.focus(); findInput.select();
        updateMatchCount();
        var panel = dom.closest('.cm-panels-top');
        if (panel) {
          panel.classList.add('is-entering');
          requestAnimationFrame(function() {
            requestAnimationFrame(function() { panel.classList.remove('is-entering'); });
          });
        }
      },
    };
  }

  // ── Editor mode state machine ────────────────────────────────────────────────
  // Single authority for live-preview ↔ source ↔ reading transitions *on the
  // currently active tab's document*. All three are the same EditorView/doc —
  // reconfigure() only swaps s.decoCompartment (+ s.readCompartment for
  // reading, which layers on live preview), a pure decoration change with no
  // `changes`, so it never touches the document, the undo stack, or dirty
  // tracking. Switching to a *different* note/tab is a different operation
  // entirely — see __hyloEditorApplyState, which builds a fresh EditorState
  // (mode baked in from the start) and swaps it in with view.setState().
  var editorMode = (function() {
    function reconfigure(inSource, reading, opts) {
      var s = __hyloEditor;
      s.view.dispatch({effects: s._modeEffects(inSource, reading)});
      __hyloEditorSetModeState(inSource, reading);
      if (!(opts && opts.skipFocus)) focusManager.focusEditor();
    }
    return {
      // User-triggered: live preview → source
      enterSource: function() {
        if (__hyloEditor.readingActive) __hyloEditorSetReadingPref(false);
        reconfigure(true, false);
      },
      // User-triggered: source / reading → live preview
      exitSource: function() {
        var s = __hyloEditor;
        if (s.readingActive) __hyloEditorSetReadingPref(false);
        s.currentMd = s.view.state.doc.toString();
        reconfigure(false, false);
      },
      // User-triggered: any mode → reading view
      enterReading: function() {
        var s = __hyloEditor;
        __hyloEditorSetReadingPref(true);
        s.currentMd = s.view.state.doc.toString();
        reconfigure(false, true, {skipFocus: true});
      },
      toggle: function() {
        if (__hyloEditor.inSource) this.exitSource(); else this.enterSource();
      },
      toggleReading: function() {
        if (__hyloEditor.readingActive) this.exitSource(); else this.enterReading();
      },
    };
  })();

  // ── Focus manager ────────────────────────────────────────────────────────────
  // Single authority for all editor focus/blur decisions.
  var focusManager = {
    focusEditor: function() {
      var s = __hyloEditor;
      if (s.view) s.view.focus();
    },
    // Blur whatever currently has focus.
    blurActive: function() {
      if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
    },
    // True when focus is inside the editor content area.
    isInsideEditor: function() {
      var ae = document.activeElement;
      var ea = document.getElementById('content-pane-edit-area');
      return !!(ea && ea.contains(ae));
    },
  };

  function __hyloEditorEnterSource() { editorMode.enterSource(); }
  function __hyloEditorExitSource() { editorMode.exitSource(); }

  // "Metadata" header's pencil button (cm-live/frontmatter-collapse.js) —
  // frontmatter is read-only in live-preview mode (frontmatterReadOnly()),
  // so this dialog + allowFrontmatterEdit-annotated dispatch is the only
  // way to change it there. from/to span the whole node including the
  // "---" delimiters; only the interior YAML is shown/edited.
  function __hyloEditorEditFrontmatter(view, from, to) {
    var raw = view.state.doc.sliceString(from, to);
    var lines = raw.split('\n');
    var hasClose = lines.length > 1 && lines[lines.length - 1].trim() === '---';
    var interior = (hasClose ? lines.slice(1, lines.length - 1) : lines.slice(1)).join('\n');
    if (!window.__hyloEditFrontmatter) return;
    window.__hyloEditFrontmatter(interior, function(newInterior) {
      var body = newInterior.replace(/\s+$/, '');
      var newBlock = '---\n' + (body ? body + '\n' : '') + '---';
      view.dispatch({
        changes: { from: from, to: to, insert: newBlock },
        annotations: __hyloCM.allowFrontmatterEdit.of(true),
      });
    });
  }

  // ── Tab state manager ─────────────────────────────────────────────────────────
  // Owns the per-tab saved state (scroll position, source mode).
  var tabStateManager = (function() {
    var _states = new Map();  // tabId → { scrollTop, inSource }
    return {
      save: function(tabId) {
        if (!tabId) return;
        var s = __hyloEditor;
        var scroller = document.querySelector('#content-pane-edit-area .cm-scroller');
        _states.set(tabId, { scrollTop: scroller ? scroller.scrollTop : 0, inSource: s.inSource });
      },
      restore: function(tabId) {
        if (!tabId) return null;
        return _states.get(tabId) || null;
      },
      clear: function(tabId) {
        if (!tabId) return;
        _states.delete(tabId);
      },
      saveForLeave: async function(tab) {
        if (!tab) return;
        this.save(tab.id);
      },
    };
  })();

  // ── Content loaders ──────────────────────────────────────────────────────────
  async function __hyloContentPaneLoadNote(path, tabId, savedState) {
    var s = __hyloEditor;
    if (s.dirty && s.currentPath && s.currentPath !== path) {
      clearTimeout(s.saveTimer); s.saveTimer = null; await __hyloEditorDoSave();
    } else { clearTimeout(s.saveTimer); s.saveTimer = null; }
    if (!__hyloEditorIsActiveTabId(tabId)) return false;
    __hyloEditorSaveStatus('');

    // Load from server
    var resp = await fetch('/api/vault/read', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({path: path}),
    });
    if (!resp.ok) {
      if (resp.status === 404 && window.showError) {
        window.showError('「' + path.split('/').pop().replace(/\.md$/, '') + '」 does not exist in the vault.', 'Note not found');
      }
      return false;
    }
    var content = await resp.text();
    if (!__hyloEditorIsActiveTabId(tabId)) return false;
    s.currentPath = path; s.currentMd = __hyloEditorTightenLists(content); s.dirty = false;
    __hyloNoteSigRemember(path, s.currentMd);
    
    // Apply state (will create new state if no saved state)
    return await __hyloEditorApplyState(savedState || { inSource: false, scrollTop: 0 }, tabId);
  }

  async function __hyloContentPaneSetContent(content, tabId, savedState) {
    var s = __hyloEditor;
    if (s.dirty && s.currentPath) { clearTimeout(s.saveTimer); s.saveTimer = null; await __hyloEditorDoSave(); }
    else { clearTimeout(s.saveTimer); s.saveTimer = null; }
    if (!__hyloEditorIsActiveTabId(tabId)) return false;
    __hyloEditorSaveStatus('');
    s.currentPath = ''; s.currentMd = __hyloEditorTightenLists(content || ''); s.dirty = false;
    
    // Apply state
    return await __hyloEditorApplyState(savedState || { inSource: false, scrollTop: 0 }, tabId);
  }
  
  async function __hyloEditorApplyState(state, expectedTabId) {
    var s = __hyloEditor;
    await __hyloEnsureContentPaneEditor();
    if (!__hyloEditorIsActiveTabId(expectedTabId)) return false;

    var targetInSource = state.inSource || false;
    var targetScroll = state.scrollTop || 0;
    // Reading takes priority over a saved source/live-preview mode; a
    // not-yet-materialized tab (no currentPath) stays editable even while
    // the preference is on.
    var wantReading = !!(s.reading && s.currentPath);
    var wantSource = !wantReading && targetInSource;

    // Clear the previous note's broken-wikilink set before the new one's
    // decorations render, so a stale "broken" flag can't flash on a target
    // that's perfectly fine in *this* note — __hyloEditorRevalidateWikiLinks
    // repopulates it (and repaints) once the batch check comes back.
    s.brokenWikiLinkNames = new Set();
    s.wikiLinkRevalidateSeq++;

    // A note/tab switch: brand-new EditorState (mode baked in from the
    // start) swapped in via setState(), not a dispatch()'d replace onto the
    // previous tab's state — see s._buildState's comment for why.
    s.view.setState(s._buildState(s.currentMd, wantSource, wantReading));
    __hyloEditorSetModeState(wantSource, wantReading);
    void __hyloEditorRevalidateWikiLinks();
    // Sync write covers the tab-switch frame. Two more writes cover the
    // pane slide-in tearing down a compositing layer and resetting scrollTop.
    __hyloEditorRestoreScroll(targetScroll, { sync: true, frames: 2, afterMs: 270, tabId: expectedTabId, focus: true });
    return true;
  }

  function __hyloEditorRestoreScroll(scrollTop, opts) {
    var s = __hyloEditor;
    opts = opts || {};
    var top = scrollTop || 0;
    var tabId = opts.tabId;
    function scroller() { return document.querySelector('#content-pane-edit-area .cm-scroller'); }
    function apply() {
      if (tabId && !__hyloEditorIsActiveTabId(tabId)) return;
      var el = scroller();
      if (el) el.scrollTop = top;
    }
    if (s.pendingScrollRaf) { cancelAnimationFrame(s.pendingScrollRaf); s.pendingScrollRaf = null; }
    clearTimeout(s.pendingOpenScroll);
    if (opts.sync) {
      var now = scroller();
      if (now) now.scrollTop = top;
    }
    var frames = opts.frames || 1;
    function frame(n) {
      s.pendingScrollRaf = requestAnimationFrame(function() {
        if (n > 1) { frame(n - 1); return; }
        s.pendingScrollRaf = null;
        apply();
        if (opts.focus && !focusManager.isInsideEditor()) focusManager.focusEditor();
      });
    }
    frame(frames);
    s.pendingOpenScroll = setTimeout(function() {
      s.pendingOpenScroll = null;
      apply();
    }, opts.afterMs);
  }
