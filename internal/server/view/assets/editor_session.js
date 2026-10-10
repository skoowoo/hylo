  // Editor session: one CodeMirror view, its mode, and save/load.
  // Pane chrome (tabs, split, rename, compile) lives in content_pane.js.
  // ── Editor state ────────────────────────────────────────────────────────────
  // One EditorView for all notes; source vs live preview swaps a decoration Compartment in place.
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
    // Heading a [[note#Heading]] click should land on once the note opens;
    // applied by __hyloEditorRestoreScroll so its scroll writes don't undo it.
    pendingHeading: null,
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
        // typing). Patch the row from the response.
        var saved = null; try { saved = await r.json(); } catch(_) {}
        if (saved) __hyloPatchNoteRowAfterSave(path, saved);
        if (window.__hyloVaultChanged) window.__hyloVaultChanged({ op: 'write' });
      } else {
        var errText = ''; try { errText = await r.text(); } catch(_) {}
        window.showError(errText || 'Server error — your changes may not be saved.', 'Save error');
      }
    } catch(e) {
      window.showError((e && e.message) || 'Network error — your changes may not be saved.', 'Save error');
    }
  }
  // Patches every rendered row, including parked sections, since a save doesn't refetch them.
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
  // A new tab has no path or server copy until its first real edit.
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
      if (window.__hyloVaultChanged) window.__hyloVaultChanged({ op: 'create' });
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
  // Only real edits reach here: tab switches use view.setState(), which skips updateListener.
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

  // "note#Heading" names "note"; a bare "#Heading" names no other note ('').
  function __hyloWikiLinkTargetName(raw) {
    var name = (raw || '').split('#')[0].trim();
    if (!name) return '';
    if (!/\.md$/i.test(name)) name += '.md';
    return name;
  }

  function __hyloExtractWikilinkNames(md) {
    var names = [], seen = {}, m;
    __hyloWikilinkRe.lastIndex = 0;
    while ((m = __hyloWikilinkRe.exec(md || ''))) {
      var name = __hyloWikiLinkTargetName(m[1]);
      if (name && !seen[name]) { seen[name] = true; names.push(name); }
    }
    return names;
  }

  // Repaints [[links]] whose target is gone; the seq guard drops responses for a note already left.
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

  function __hyloNoteStem(path) {
    var name = String(path || '').split('/').pop();
    return name.replace(/\.md$/i, '');
  }

  // ![alt](path) → served from the vault. Relative paths resolve against the
  // note's folder, "/..." against the vault root; anything with a scheme
  // (https:, data:) is left alone.
  function __hyloNoteImageSrc(url, notePath) {
    if (!url || /^[a-z][a-z0-9+.-]*:/i.test(url) || url.indexOf('//') === 0) return url;
    var path = url.replace(/[?#].*$/, '');
    try { path = decodeURI(path); } catch(_) {}
    var parts = (path.charAt(0) === '/' ? [] : String(notePath || '/').split('/').slice(1, -1)).concat(path.split('/'));
    var out = [];
    for (var i = 0; i < parts.length; i++) {
      var part = parts[i];
      if (!part || part === '.') continue;
      if (part === '..') {
        if (!out.length) return url; // climbs out of the vault
        out.pop();
      } else {
        out.push(part);
      }
    }
    var name = out.pop();
    if (!name) return url;
    return '/api/images/at?dir=' + encodeURIComponent('/' + out.join('/')) + '&name=' + encodeURIComponent(name);
  }

  // "[[" completion source. A bare "[[" offers the other open tabs (the server
  // rejects an empty query); otherwise a filename search.
  async function __hyloEditorSearchNotes(query, signal) {
    if (!query.trim()) {
      var pane = window.__hyloContentPane;
      var tabs = pane && Array.isArray(pane.tabs) ? pane.tabs : [];
      var seen = {};
      return tabs.filter(function(t) {
        if (!t || !t.path || t.path === __hyloEditor.currentPath || seen[t.path]) return false;
        seen[t.path] = true;
        return true;
      }).map(function(t) { return { label: __hyloNoteStem(t.path) }; });
    }
    var resp = await fetch('/api/search', {
      method: 'POST', headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({q: query, type: 'name', limit: 30}), signal: signal,
    });
    if (!resp.ok) return [];
    var data = await resp.json();
    return (data.results || []).map(function(r) {
      return { label: __hyloNoteStem(r.name), detail: r.dir && r.dir !== '/' ? r.dir : undefined };
    });
  }

  // A heading in the current note scrolls in place; otherwise open the note
  // and let the scroll restore land on the heading.
  async function __hyloEditorFollowWikiLink(target, heading) {
    var s = __hyloEditor;
    var here = !target || target.toLowerCase() === __hyloNoteStem(s.currentPath).toLowerCase();
    if (here) {
      if (heading && s.view) __hyloCM.jumpToHeading(s.view, heading);
      return;
    }
    s.pendingHeading = heading || null;
    await __hyloContentPaneOpenWikiLink(target);
    // No restore in flight means the open failed or bailed; don't let the
    // heading leak into whatever note opens next.
    if (!s.pendingOpenScroll) s.pendingHeading = null;
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

  // Per-tab scroll position and source-mode flag. snapshot anchors on a doc
  // position: a fresh state forgets measured line heights, so the raw
  // scrollTop lands hundreds of lines off on a long note.
  var __hyloTabStates = new Map();
  function __hyloEditorSaveTabState(tabId) {
    if (!tabId) return;
    var view = __hyloEditor.view;
    __hyloTabStates.set(tabId, {
      scrollTop: view ? view.scrollDOM.scrollTop : 0,
      snapshot: view ? view.scrollSnapshot() : null,
      inSource: __hyloEditor.inSource,
    });
  }
  function __hyloEditorRestoreTabState(tabId) { return (tabId && __hyloTabStates.get(tabId)) || null; }
  function __hyloEditorClearTabState(tabId) { if (tabId) __hyloTabStates.delete(tabId); }
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
        resolveMarkdownImageSrc: function(url) { return __hyloNoteImageSrc(url, s.currentPath); },
        onWikiLinkClick: function(target, alias, e, heading) { void __hyloEditorFollowWikiLink(target, heading); },
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

      // The compartments are seeded per state and reconfigured later via dispatch.
      s._buildExtensions = function(decoInitial, readInitial) {
        return [
          sharedLanguage,
          cm.history(),
          // Ahead of the other Prec.highest Enter bindings: an open completion takes Enter first.
          cm.wikiLinkCompletion(__hyloEditorSearchNotes),
          // Tab/Enter inside a table, before the list bindings get them.
          cm.tableKeymap,
          // Closing ``` written as the opening one is typed.
          cm.fenceAutoClose,
          // Highest precedence: lang-markdown's own Enter binding would otherwise win.
          cm.listIndentExtension,
          cm.formatKeymap,
          // Outside the mode compartments, so it works in source mode too.
          cm.selectionFormatMenu(),
          cm.cjkWordSelection,
          cm.keymap.of([].concat(cm.defaultKeymap, cm.historyKeymap)),
          cm.search({ top: true, createPanel: __hyloCreateSearchPanel }),
          cm.EditorView.lineWrapping, cmTheme,
          cm.EditorView.contentAttributes.of({spellcheck: 'false'}),
          cm.linkClickHandler(),
          // Outside the compartment so the collapsed flag survives a mode toggle.
          cm.frontmatterCollapseField,
          s.decoCompartment.of(decoInitial),
          s.readCompartment.of(readInitial),
          cm.EditorView.updateListener.of(function(update) {
            // setState() never fires this, so every docChanged here is a real edit.
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
      // One EditorState per note/tab, swapped in with setState(): a switch is not an edit,
      // so nothing lands in undo history or hits the frontmatter filter.
      s._buildState = function(content, inSource, reading) {
        return cm.EditorState.create({
          doc: content || '',
          // Land past frontmatter and any leading list/heading marker, not at offset 0.
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
  var SR_ICON = {
    close: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    expand: '<path d="m9 6 6 6-6 6"/>',
    up: '<path d="m18 15-6-6-6 6"/>',
    down: '<path d="m6 9 6 6 6-6"/>',
  };
  function srIcon(name) {
    return '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' + SR_ICON[name] + '</svg>';
  }

  function __hyloCreateSearchPanel(view) {
    var cm = __hyloCM;
    var dom = document.createElement('div');
    dom.className = 'hylo-search-panel';
    dom.innerHTML =
      '<div class="hylo-sr-header"><span class="hylo-sr-header-label">Find</span>' +
        '<button type="button" class="hylo-sr-ibtn" data-act="close" aria-label="Close">' + srIcon('close') + '</button></div>' +
      '<div class="hylo-sr-row">' +
        '<button type="button" class="hylo-sr-ibtn hylo-sr-expand-btn" data-act="expand" title="Toggle replace" aria-label="Toggle replace" aria-expanded="false">' + srIcon('expand') + '</button>' +
        '<div class="hylo-sr-input-wrap">' +
          '<input type="text" class="field-input hylo-sr-input" main-field placeholder="Find" aria-label="Find">' +
          '<div class="hylo-sr-inset-btns">' +
            '<span class="hylo-sr-count" aria-hidden="true" style="display:none"></span>' +
            '<button type="button" class="hylo-sr-ibtn" data-act="prev" title="Previous (Shift+Enter)">' + srIcon('up') + '</button>' +
            '<button type="button" class="hylo-sr-ibtn" data-act="next" title="Next (Enter)">' + srIcon('down') + '</button>' +
            '<button type="button" class="hylo-sr-ibtn hylo-sr-toggle" data-act="case" title="Match case">Aa</button>' +
          '</div></div></div>' +
      '<div class="hylo-sr-row" hidden><div class="hylo-sr-row-spacer"></div>' +
        '<div class="hylo-sr-input-wrap">' +
          '<input type="text" class="field-input hylo-sr-input" placeholder="Replace" aria-label="Replace">' +
          '<div class="hylo-sr-inset-btns">' +
            '<button type="button" class="hylo-sr-ibtn hylo-sr-text" data-act="replace" title="Replace (Enter)">Replace</button>' +
            '<button type="button" class="hylo-sr-ibtn hylo-sr-text" data-act="replaceAll" title="Replace All">All</button>' +
          '</div></div></div>';

    var inputs = dom.querySelectorAll('input');
    var findInput = inputs[0], replaceInput = inputs[1];
    var replaceRow = replaceInput.closest('.hylo-sr-row');
    var countEl = dom.querySelector('.hylo-sr-count');
    var caseSensitive = false;

    function buildQuery() {
      return new cm.SearchQuery({ search: findInput.value, caseSensitive: caseSensitive, replace: replaceInput.value });
    }
    // Walks the same cursor findNext uses, so the tally can't drift from it.
    function updateMatchCount() {
      if (!findInput.value) { countEl.style.display = 'none'; return; }
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
    function close() { cm.closeSearchPanel(view); }

    var actions = {
      close: close,
      prev: function() { cm.findPrevious(view); },
      next: function() { cm.findNext(view); },
      replace: function() { cm.replaceNext(view); },
      replaceAll: function() { cm.cmReplaceAll(view); },
      case: function(btn) {
        caseSensitive = !caseSensitive;
        btn.classList.toggle('active', caseSensitive);
        commit(); findInput.focus();
      },
      expand: function(btn) {
        var open = replaceRow.hidden;
        replaceRow.hidden = !open;
        btn.classList.toggle('is-open', open);
        btn.setAttribute('aria-expanded', String(open));
        if (open) replaceInput.focus();
      },
    };
    dom.addEventListener('click', function(e) {
      var btn = e.target.closest('[data-act]');
      if (!btn) return;
      actions[btn.dataset.act](btn);
      if (btn.dataset.act !== 'close') updateMatchCount();
    });
    findInput.addEventListener('input', commit);
    replaceInput.addEventListener('input', commit);
    dom.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') { e.preventDefault(); close(); return; }
      if (e.key !== 'Enter') return;
      e.preventDefault();
      if (e.target === replaceInput) cm.replaceNext(view);
      else if (e.shiftKey) cm.findPrevious(view); else cm.findNext(view);
      updateMatchCount();
    });

    return {
      dom: dom,
      top: true,
      mount: function() {
        var sel = view.state.selection.main;
        if (!sel.empty) {
          var txt = view.state.sliceDoc(sel.from, sel.to);
          // Dispatching inside mount() would run during CM's own update and drop the panel.
          if (txt && !txt.includes('\n')) { findInput.value = txt; queueMicrotask(commit); }
        }
        findInput.focus(); findInput.select();
        updateMatchCount();
      },
    };
  }

  // ── Editor mode ──────────────────────────────────────────────────────────────
  // Live preview / source / reading share one view and doc; only the compartments change,
  // so the document, undo stack and dirty flag are untouched.
  var editorMode = (function() {
    function reconfigure(inSource, reading, opts) {
      var s = __hyloEditor;
      s.view.dispatch({effects: s._modeEffects(inSource, reading)});
      __hyloEditorSetModeState(inSource, reading);
      if (!(opts && opts.skipFocus)) __hyloFocusEditor();
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
      // More menu's "Source / Live preview" item.
      toggle: function() {
        if (__hyloEditor.inSource) this.exitSource(); else this.enterSource();
      },
      toggleReading: function() {
        if (__hyloEditor.readingActive) this.exitSource(); else this.enterReading();
      },
    };
  })();

  function __hyloFocusEditor() {
    if (__hyloEditor.view) __hyloEditor.view.focus();
  }
  function __hyloBlurActive() {
    if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
  }
  function __hyloFocusInsideEditor() {
    var ea = document.getElementById('content-pane-edit-area');
    return !!(ea && ea.contains(document.activeElement));
  }

  // Focus return for x-trap.noreturn dialogs: x-trap's own return calls native
  // focus(), which in WebKit scrolls a focused CodeMirror back to the top.
  // Usage: x-effect="__hyloTrapReturn($el, open)", placed before x-trap so it
  // records the opener before the trap moves focus in.
  var __hyloTrapOpeners = new WeakMap();
  function __hyloTrapReturn(el, open) {
    if (open) { __hyloTrapOpeners.set(el, document.activeElement); return; }
    var prev = __hyloTrapOpeners.get(el);
    if (!prev) return;
    __hyloTrapOpeners.delete(el);
    // After x-trap's own effect has lifted inert from the page.
    setTimeout(function() {
      if (!prev.isConnected || prev === document.body) return;
      var view = __hyloEditor.view;
      if (view && view.dom.contains(prev)) view.focus();
      else prev.focus({ preventScroll: true });
    }, 0);
  }

  // Frontmatter is read-only in live preview, so this dialog is its only edit path.
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
    await __hyloEditorPreloadLanguages(content);
    if (!__hyloEditorIsActiveTabId(tabId)) return false;
    s.currentPath = path; s.currentMd = __hyloEditorTightenLists(content); s.dirty = false;
    // Apply state (will create new state if no saved state)
    return await __hyloEditorApplyState(savedState || { inSource: false, scrollTop: 0 }, tabId);
  }

  async function __hyloContentPaneSetContent(content, tabId, savedState) {
    var s = __hyloEditor;
    if (s.dirty && s.currentPath) { clearTimeout(s.saveTimer); s.saveTimer = null; await __hyloEditorDoSave(); }
    else { clearTimeout(s.saveTimer); s.saveTimer = null; }
    await __hyloEditorPreloadLanguages(content || '');
    if (!__hyloEditorIsActiveTabId(tabId)) return false;
    __hyloEditorSaveStatus('');
    s.currentPath = ''; s.currentMd = __hyloEditorTightenLists(content || ''); s.dirty = false;
    
    // Apply state
    return await __hyloEditorApplyState(savedState || { inSource: false, scrollTop: 0 }, tabId);
  }
  
  // Runs before currentPath/currentMd switch over, so typing into the old
  // note during the wait still saves to the old note. Capped so a slow chunk
  // only costs a late highlight, never a stalled open.
  function __hyloEditorPreloadLanguages(content) {
    var p = __hyloCM && __hyloCM.preloadCodeLanguages(content);
    if (!p) return null;
    return Promise.race([p, new Promise(function(r) { setTimeout(r, 150); })]);
  }

  async function __hyloEditorApplyState(state, expectedTabId) {
    var s = __hyloEditor;
    await __hyloEnsureContentPaneEditor();
    if (!__hyloEditorIsActiveTabId(expectedTabId)) return false;

    var targetInSource = state.inSource || false;
    // Reading takes priority over a saved source/live-preview mode; a
    // not-yet-materialized tab (no currentPath) stays editable even while
    // the preference is on.
    var wantReading = !!(s.reading && s.currentPath);
    var wantSource = !wantReading && targetInSource;

    // Avoid flashing the previous note's broken flags before revalidation lands.
    s.brokenWikiLinkNames = new Set();
    s.wikiLinkRevalidateSeq++;

    // A note/tab switch: brand-new EditorState (mode baked in from the
    // start) swapped in via setState(), not a dispatch()'d replace onto the
    // previous tab's state — see s._buildState's comment for why.
    s.view.setState(s._buildState(s.currentMd, wantSource, wantReading));
    // A fresh state is parsed only ~3000 chars deep; the background worker
    // finishes 100–500ms later, so the rest would paint as raw markdown first.
    __hyloCM.forceParsing(s.view, s.view.state.doc.length, 40);
    __hyloEditorSetModeState(wantSource, wantReading);
    void __hyloEditorRevalidateWikiLinks();
    // Sync write covers the tab-switch frame. Two more writes cover the
    // pane slide-in tearing down a compositing layer and resetting scrollTop.
    __hyloEditorRestoreScroll(state, { sync: true, frames: 2, afterMs: 270, tabId: expectedTabId, focus: true });
    return true;
  }

  // saved: { scrollTop, snapshot? } from __hyloEditorSaveTabState.
  function __hyloEditorRestoreScroll(saved, opts) {
    var s = __hyloEditor;
    opts = opts || {};
    var top = (saved && saved.scrollTop) || 0;
    var snap = saved && saved.snapshot;
    var tabId = opts.tabId;
    function scroller() { return document.querySelector('#content-pane-edit-area .cm-scroller'); }
    function write() {
      var el = scroller();
      if (el) el.scrollTop = top;
      // A reload can come back shorter than when the snapshot was taken.
      if (snap && s.view && snap.value.range.to <= s.view.state.doc.length) s.view.dispatch({ effects: snap });
    }
    function apply() {
      if (tabId && !__hyloEditorIsActiveTabId(tabId)) return;
      write();
      if (s.pendingHeading && s.view) __hyloCM.jumpToHeading(s.view, s.pendingHeading);
    }
    if (s.pendingScrollRaf) { cancelAnimationFrame(s.pendingScrollRaf); s.pendingScrollRaf = null; }
    clearTimeout(s.pendingOpenScroll);
    if (opts.sync) write();
    var frames = opts.frames || 1;
    function frame(n) {
      s.pendingScrollRaf = requestAnimationFrame(function() {
        if (n > 1) { frame(n - 1); return; }
        s.pendingScrollRaf = null;
        apply();
        if (opts.focus && !__hyloFocusInsideEditor()) __hyloFocusEditor();
      });
    }
    frame(frames);
    s.pendingOpenScroll = setTimeout(function() {
      s.pendingOpenScroll = null;
      apply();
      s.pendingHeading = null;
    }, opts.afterMs);
  }
