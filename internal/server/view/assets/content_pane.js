  // ── Editor state ────────────────────────────────────────────────────────────
  // One CodeMirror 6 EditorView (s.view) for the whole editor now — the old
  // Milkdown (WYSIWYG) / CodeMirror (source) split is gone. "inSource"/
  // editorMode still exist (see below) but now mean "decorations
  // Compartment reconfigured to plain syntax highlighting" vs "live-preview
  // decorations active", toggled in place on the same view/doc instead of
  // switching between two separately-mounted editors.
  var __vaultrEditor = {
    view: null,
    initPromise: null, dirty: false,
    currentPath: '', currentMd: '',
    saveTimer: null,
    EditorView: null, EditorState: null, Compartment: null, keymap: null,
    defaultKeymap: null, historyKeymap: null, history: null,
    markdown: null, HighlightStyle: null, syntaxHighlighting: null, tags: null,
    cmUndo: null, cmRedo: null,
    cmSearch: null, cmOpenSearchPanel: null, cmCloseSearchPanel: null,
    cmFindNext: null, cmFindPrev: null, cmReplaceNext: null, cmReplaceAll: null,
    SearchQuery: null, getSearchQuery: null, setSearchQuery: null,
    livePreviewPlugin: null, livePreviewAtomicRanges: null, wikiLinksRevalidated: null,
    livePreviewTheme: null, wikiMarkdownLanguage: null,
    // Names (with ".md") confirmed missing from the vault for the note
    // currently loaded — see __vaultrEditorRevalidateWikiLinks. Cleared and
    // recomputed on every note/tab load; stale until that async check lands.
    brokenWikiLinkNames: new Set(),
    wikiLinkRevalidateSeq: 0,
    decoCompartment: null, readCompartment: null, sharedLanguage: null,
    inSource: false, pendingScrollRaf: null, pendingOpenScroll: null,
    // reading: global user preference, survives note/tab switches and restarts.
    // readingActive: what the view is actually showing — drafts stay editable
    // even while the preference is on.
    reading: false, readingActive: false,
  };
  try { __vaultrEditor.reading = localStorage.getItem('vaultr.reading') === '1'; } catch(_) {}
  function __vaultrEditorSetReadingPref(on) {
    __vaultrEditor.reading = on;
    try { localStorage.setItem('vaultr.reading', on ? '1' : '0'); } catch(_) {}
  }

  var __vaultrEditorTabSeq = 0;
  function __vaultrEditorNewTabId() {
    __vaultrEditorTabSeq = (__vaultrEditorTabSeq + 1) % 1000;
    return Date.now() * 1000 + __vaultrEditorTabSeq;
  }

  var CONTENT_PANE_MAX_TABS = 10;
  // Oldest evictable tab index, or -1. Skips the active tab and any
  // unmaterialized tab that still has real content (_pendingContent) — an
  // empty one is free to evict, or it'd sit protected forever and push out
  // real notes instead.
  function __vaultrOldestEvictableTabIdx(tabs, activeTab) {
    var oldestIdx = -1, oldestId = Infinity;
    for (var j = 0; j < tabs.length; j++) {
      if (j !== activeTab && !tabs[j]._pendingContent && tabs[j].id < oldestId) { oldestIdx = j; oldestId = tabs[j].id; }
    }
    return oldestIdx;
  }

  // ── Directory-path autocomplete (kept for reuse, not currently wired in) ────
  // Parses "…/parti" into {dirPath, partial} for __vaultrPathAcCreate
  // (path_ac.js) — used by the old create-mode path input. Nothing calls
  // this today (new notes no longer ask for a path up front), but it's
  // generic and worth keeping for a future directory picker.
  function __vaultrEditorAcParseCtx(val, caret) {
    val = typeof val === 'string' ? val : '';
    if (caret == null || caret > val.length) caret = val.length;
    var left = val.slice(0, caret);
    if (left.indexOf('/') === -1) return null;
    var slash = left.lastIndexOf('/');
    var partial = left.slice(slash + 1);
    if (partial.indexOf('.') !== -1) return null;
    return { dirPath: slash === 0 ? '/' : left.slice(0, slash), partial: partial, replaceStart: slash + 1 };
  }

  // ── Save helpers ─────────────────────────────────────────────────────────────
  function __vaultrEditorSaveStatus(txt) {
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
  function __vaultrEditorScheduleSave() {
    __vaultrEditorSaveStatus('●');
    clearTimeout(__vaultrEditor.saveTimer);
    __vaultrEditor.saveTimer = setTimeout(__vaultrEditorDoSave, 800);
  }
  async function __vaultrEditorDoSave() {
    if (!__vaultrEditor.dirty) return;
    var path = __vaultrEditor.currentPath;
    var content = __vaultrEditor.currentMd;
    if (!path) return;
    try {
      var r = await fetch('/api/vault/write', {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({path: path, content: content}),
      });
      if (r.ok) {
        __vaultrEditor.dirty = false; __vaultrEditorSaveStatus('Saved');
        // A plain content save is by far the most frequent vault mutation
        // (every ~800ms of idle-after-typing) — unlike pin/delete/rename/move,
        // which go through window.__vaultrAfterVaultMutation() and refetch +
        // re-render the whole active section, this patches only the one row
        // that could have changed, straight from the save response, with no
        // extra request at all.
        var saved = null; try { saved = await r.json(); } catch(_) {}
        if (saved) __vaultrPatchNoteRowAfterSave(path, saved);
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
  // rule. Only touches the row if it's actually rendered in the list right
  // now; most saves happen while some other section is showing, and there's
  // nothing to patch then.
  function __vaultrPatchNoteRowAfterSave(path, note) {
    var row = document.querySelector('.home-note-row[data-note-path="' + CSS.escape(path) + '"]');
    if (!row) return;
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
  function __vaultrEditorTightenLists(md) {
    return md.replace(
      /(^[ \t]*(?:[-*+]|\d+[.)]) [^\n]*)\n\n(?=[ \t]*(?:[-*+]|\d+[.)]) )/gm, '$1\n');
  }
  function __vaultrEditorFindImageFile(dt) {
    if (!dt) return null;
    var items = Array.from(dt.items || []);
    for (var i = 0; i < items.length; i++) {
      if (items[i].kind === 'file' && items[i].type.indexOf('image/') === 0) return items[i].getAsFile();
    }
    return null;
  }
  async function __vaultrEditorUploadImage(imgFile) {
    var fd = new FormData();
    fd.append('file', imgFile);
    var resp = await fetch('/api/vault/upload-image', {method: 'POST', body: fd});
    if (!resp.ok) throw new Error(await resp.text());
    return (await resp.json()).src;
  }

  // ── Note materialization ─────────────────────────────────────────────────────
  // A brand-new tab has no path — and nothing on the server — until its
  // first real edit (see __vaultrEditorHandleContentChange below). There is
  // no draft state to load/flush/discard before that point.
  function __vaultrEditorActiveTab() {
    var pane = window.__vaultrContentPane;
    return pane ? pane.tabs[pane.activeTab] : null;
  }
  function __vaultrEditorIsActiveTabId(tabId) {
    if (!tabId) return true;
    var pane = window.__vaultrContentPane;
    var tab = pane && pane.tabs[pane.activeTab];
    return !!(tab && tab.id === tabId);
  }
  async function __vaultrEditorSaveTabForLeave(tab) { await tabStateManager.saveForLeave(tab); }

  // _materializing lives on the tab itself, not a shared variable — two
  // different unmaterialized tabs can be in flight at once.
  async function __vaultrEditorMaterializeTab(tab, md) {
    if (tab._materializing) return;
    tab._materializing = true;
    if (__vaultrEditorIsActiveTabId(tab.id)) __vaultrEditorSaveStatus('●');
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
      tab.title = __vaultrStripMdExt(data.path.split('/').pop()) || data.path;
      delete tab._pendingContent;
      if (!__vaultrEditorIsActiveTabId(tab.id)) return; // switched away while this was in flight — nothing to autosave right now
      var s = __vaultrEditor;
      s.currentPath = data.path;
      s.dirty = true; // re-arm autosave — more may have been typed while the create call was in flight
      __vaultrEditorScheduleSave();
      var ptEl = document.getElementById('content-pane-path-text');
      if (ptEl) ptEl.textContent = data.path;
      if (window.__vaultrAfterVaultMutation) await window.__vaultrAfterVaultMutation();
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
  function __vaultrEditorHandleContentChange(md) {
    var s = __vaultrEditor;
    var pane = window.__vaultrContentPane;
    if (pane && !pane.contentPaneOpen) return;
    s.currentMd = md;
    var tab = __vaultrEditorActiveTab();
    if (tab && !tab.path) {
      // In-memory only (never persisted) — lets switching back to this tab
      // while its create-untitled call is still in flight show what was
      // typed instead of a blank editor. Cleared once materialized.
      tab._pendingContent = md;
      void __vaultrEditorMaterializeTab(tab, md);
      return;
    }
    s.dirty = true;
    __vaultrEditorScheduleSave();
  }

  // Mirrors internal/util/mdhtml.go's wikilinkRe + name normalization so the
  // client can batch-check the same target names the server would resolve.
  var __vaultrWikilinkRe = /\[\[([^\]\[|]+?)(?:\|[^\]\[]+?)?\]\]/g;

  function __vaultrWikiLinkTargetName(raw) {
    var name = (raw || '').trim();
    if (!/\.md$/i.test(name)) name += '.md';
    return name;
  }

  function __vaultrExtractWikilinkNames(md) {
    var names = [], seen = {}, m;
    __vaultrWikilinkRe.lastIndex = 0;
    while ((m = __vaultrWikilinkRe.exec(md || ''))) {
      var name = __vaultrWikiLinkTargetName(m[1]);
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
  async function __vaultrEditorRevalidateWikiLinks() {
    var s = __vaultrEditor;
    var seq = s.wikiLinkRevalidateSeq;
    var names = __vaultrExtractWikilinkNames(s.currentMd);
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
      if (s.view && s.wikiLinksRevalidated) {
        s.view.dispatch({effects: s.wikiLinksRevalidated.of(null)});
      }
    } catch (_) {}
  }

  // Open a wiki-link target in the pane.  value is the raw [[…]] inner text.
  async function __vaultrContentPaneOpenWikiLink(value) {
    var pane = window.__vaultrContentPane;
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

  function __vaultrEditorSaveTabState(tabId) { tabStateManager.save(tabId); }
  function __vaultrEditorRestoreTabState(tabId) { return tabStateManager.restore(tabId); }
  function __vaultrEditorClearTabState(tabId) { tabStateManager.clear(tabId); }
  function __vaultrEditorSyncViewButtons() {
    var s = __vaultrEditor;
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
  // (user toggles mode on the current tab) and __vaultrEditorApplyState
  // (a note/tab switch already baked the right mode into the new state).
  function __vaultrEditorSetModeState(inSource, reading) {
    var s = __vaultrEditor;
    s.inSource = inSource;
    s.readingActive = reading;
    __vaultrEditorSyncViewButtons();
  }

  // ── Lazy editor init ─────────────────────────────────────────────────────────
  async function __vaultrEnsureContentPaneEditor() {
    var s = __vaultrEditor;
    if (s.view) return;
    if (s.initPromise) return s.initPromise;
    s.initPromise = (async function() {
      var mod = await import('/static/editor.js');
      s.EditorView = mod.EditorView; s.EditorState = mod.EditorState; s.Compartment = mod.Compartment;
      s.keymap = mod.keymap;
      s.defaultKeymap = mod.defaultKeymap; s.historyKeymap = mod.historyKeymap; s.history = mod.history;
      s.listIndentExtension = mod.listIndentExtension;
      s.toggleBold = mod.toggleBold; s.toggleItalic = mod.toggleItalic; s.toggleStrikethrough = mod.toggleStrikethrough;
      s.toggleBulletList = mod.toggleBulletList; s.toggleOrderedList = mod.toggleOrderedList; s.toggleTaskList = mod.toggleTaskList;
      s.toggleBlockquote = mod.toggleBlockquote;
      s.selectionFormatMenu = mod.selectionFormatMenu;
      s.readingExtensions = mod.readingExtensions;
      s.markdown = mod.markdown; s.HighlightStyle = mod.HighlightStyle;
      s.syntaxHighlighting = mod.syntaxHighlighting; s.tags = mod.tags;
      s.cmUndo = mod.cmUndo; s.cmRedo = mod.cmRedo;
      s.cmSearch = mod.search; s.cmOpenSearchPanel = mod.openSearchPanel;
      s.cmCloseSearchPanel = __vaultrEditorMakeAnimatedSearchClose(mod.closeSearchPanel);
      s.cmFindNext = mod.findNext; s.cmFindPrev = mod.findPrevious;
      s.cmReplaceNext = mod.replaceNext; s.cmReplaceAll = mod.cmReplaceAll;
      s.SearchQuery = mod.SearchQuery; s.getSearchQuery = mod.getSearchQuery; s.setSearchQuery = mod.setSearchQuery;
      s.livePreviewPlugin = mod.livePreviewPlugin; s.livePreviewAtomicRanges = mod.livePreviewAtomicRanges;
      s.wikiLinksRevalidated = mod.wikiLinksRevalidated;
      s.livePreviewTheme = mod.livePreviewTheme; s.codeHighlightStyle = mod.codeHighlightStyle;
      s.wikiMarkdownLanguage = mod.wikiMarkdownLanguage; s.linkClickHandler = mod.linkClickHandler;
      s.horizontalRuleField = mod.horizontalRuleField;
      s.frontmatterCollapseField = mod.frontmatterCollapseField; s.frontmatterHeaderField = mod.frontmatterHeaderField;
      s.frontmatterReadOnly = mod.frontmatterReadOnly; s.allowFrontmatterEdit = mod.allowFrontmatterEdit;

      var editArea = document.getElementById('content-pane-edit-area');

      var cmTheme = s.EditorView.theme({
        '&': {height:'100%',color:'var(--prose-body)',background:'transparent'},
        '&.cm-focused': {outline:'none'},
        '.cm-content': {caretColor:'var(--accent)'},
        '.cm-cursor,.cm-dropCursor': {borderLeftColor:'var(--accent)'},
        '.cm-selectionBackground': {background:'var(--selection-bg) !important'},
        '&.cm-focused .cm-selectionBackground': {background:'var(--selection-bg)'},
        '.cm-activeLine': {background:'var(--cm-active-line)'},
        '.cm-gutters': {display:'none'},
      });
      // Raw-markdown ("Source") mode's syntax highlighting — the live-preview
      // decorations (s.livePreviewPlugin etc.) replace this in "Live preview"
      // mode; see editorMode below for how the two are swapped.
      var cmHighlight = s.HighlightStyle.define([
        {tag:s.tags.heading1,color:'var(--h1)',fontWeight:'600'},
        {tag:s.tags.heading2,color:'var(--h2)',fontWeight:'600'},
        {tag:s.tags.heading3,color:'var(--h3)',fontWeight:'600'},
        {tag:s.tags.heading4,color:'var(--h4)',fontWeight:'500'},
        {tag:s.tags.emphasis,fontStyle:'italic',color:'var(--prose-em)'},
        {tag:s.tags.strong,fontWeight:'600',color:'var(--prose-strong)'},
        {tag:s.tags.link,color:'var(--p1)'},{tag:s.tags.url,color:'var(--p1)',opacity:'0.72'},
        {tag:s.tags.monospace,color:'var(--code-tx)'},{tag:s.tags.meta,color:'var(--cm-md-muted)'},
        {tag:s.tags.punctuation,color:'var(--cm-md-muted)'},
        {tag:s.tags.processingInstruction,color:'var(--cm-md-muted)'},
        {tag:s.tags.strikethrough,color:'var(--muted)',textDecoration:'line-through'},
      ]);

      // Shared by both modes so toggling reconfigures s.decoCompartment
      // around the same parse instead of re-parsing from scratch.
      s.sharedLanguage = s.wikiMarkdownLanguage();

      var liveOptions = {
        resolveImageSrc: function(filename) {
          return '/api/images/serve?name=' + encodeURIComponent(filename);
        },
        onWikiLinkClick: function(target) { void __vaultrContentPaneOpenWikiLink(target); },
        // Populated (async, after each note load) by __vaultrEditorRevalidateWikiLinks
        // below with the names confirmed missing from the vault. Checked by
        // decorateWikiLink on every rebuild — see that function's comment for why
        // mutating this set alone isn't enough to repaint until a
        // wikiLinksRevalidated effect is also dispatched.
        isWikiLinkBroken: function(target) { return s.brokenWikiLinkNames.has(__vaultrWikiLinkTargetName(target)); },
      };
      // Exposed on s so editorMode (a separate closure) can reconfigure
      // s.decoCompartment without rebuilding these from scratch. Frontmatter
      // now renders through s.livePreviewPlugin like everything else — see
      // cm-live/index.js's livePreviewExtensions() comment for why it no
      // longer needs its own opt-in field.
      s._liveModeExt = function() {
        return [
          s.livePreviewPlugin.of(liveOptions),
          s.livePreviewAtomicRanges(),
          s.horizontalRuleField(),
          // Live-preview only: source mode is the raw-YAML editing surface,
          // so frontmatter stays freely editable there. The Metadata
          // header's edit dialog is the only path in this mode.
          s.frontmatterReadOnly(),
          // Also live-preview only — it's a decoration (the "Metadata" row
          // widget), not doc text, but source mode must show the raw
          // bytes untouched. Inside decoCompartment so entering source
          // tears it down instead of painting the header over real YAML
          // (frontmatterCollapseField, below the compartment, keeps the
          // collapsed/expanded flag itself so it survives the round-trip).
          s.frontmatterHeaderField({ onEditFrontmatter: __vaultrEditorEditFrontmatter }),
          s.livePreviewTheme,
          // Fenced-code token colors (keyword/string/comment/...) — see
          // cm-live/theme.js's codeHighlightStyle comment for why this is a
          // separate, code-scoped style rather than cm-live/index.js's
          // livePreviewExtensions() bundling it in directly: this file
          // hand-picks pieces instead of calling that function (see the
          // comment above s._liveModeExt) so wikiMarkdownLanguage() can stay
          // shared across both modes outside s.decoCompartment — anything
          // livePreviewExtensions() adds has to be mirrored here too.
          s.syntaxHighlighting(s.codeHighlightStyle),
        ];
      };
      s._sourceModeExt = function() {
        return [s.syntaxHighlighting(cmHighlight)];
      };
      // Shared by editorMode.reconfigure() (user toggles mode on the current
      // tab — see below) so the inSource/reading → extensions mapping lives
      // in one place instead of copy-pasted at each call site.
      s._modeEffects = function(inSource, reading) {
        return [
          s.decoCompartment.reconfigure(inSource ? s._sourceModeExt() : s._liveModeExt()),
          s.readCompartment.reconfigure((!inSource && reading) ? s.readingExtensions() : []),
        ];
      };
      s.decoCompartment = new s.Compartment();
      s.readCompartment = new s.Compartment();

      // Extensions for every EditorState we build — one per note/tab switch
      // now (see s._buildState below), not just the initial mount.
      // decoInitial/readInitial seed s.decoCompartment/s.readCompartment with
      // whatever mode this particular state should start in; editorMode's
      // reconfigure() (below) swaps them later via view.dispatch() without
      // needing a new state, since Compartments are reusable across states.
      s._buildExtensions = function(decoInitial, readInitial) {
        return [
          s.sharedLanguage,
          s.history(),
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
          s.listIndentExtension,
          // Right-click selection formatting (bold/italic/strikethrough/list) —
          // see cm-live/format-menu.js. Outside the mode compartments like
          // listIndentExtension above, so it works in both live-preview and
          // source mode; not merged into the paste domEventHandlers block
          // below since CM6 composes multiple domEventHandlers extensions
          // independently and format-menu.js owns its own contextmenu handler.
          s.selectionFormatMenu(),
          s.keymap.of([...s.defaultKeymap, ...s.historyKeymap]),
          s.cmSearch({ top: true, createPanel: __vaultrCreateSearchPanel }),
          s.EditorView.lineWrapping, cmTheme,
          s.EditorView.contentAttributes.of({spellcheck: 'false'}),
          s.linkClickHandler(), // click a collapsed link to open it, Obsidian-style — works in both modes, not compartmented
          // Outside the compartment so the collapsed flag survives a
          // source/live-preview toggle — reconfigure() tears down and
          // recreates any StateField that was only inside _liveModeExt()
          // (frontmatterHeaderField, which reads this flag, lives there
          // now — see the comment on it in _liveModeExt above).
          s.frontmatterCollapseField,
          s.decoCompartment.of(decoInitial),
          s.readCompartment.of(readInitial),
          s.EditorView.updateListener.of(function(update) {
            // Note/tab switches (s._buildState + view.setState(), see below)
            // never reach this listener at all — setState() doesn't fire
            // updateListener, confirmed against the CM6 version bundled in
            // editor.js. So every docChanged transaction that does arrive
            // here is a real edit; no "is this programmatic" flag needed.
            if (!update.docChanged) return;
            __vaultrEditorHandleContentChange(update.state.doc.toString());
          }),
          s.EditorView.domEventHandlers({
            paste: function(e, view) {
              var imgFile = __vaultrEditorFindImageFile(e.clipboardData);
              if (!imgFile) return false;
              e.preventDefault();
              __vaultrEditorUploadImage(imgFile).then(function(src) {
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
      // Only used for the view's very first mount, below — note/tab switches
      // One EditorState per note/tab, built fresh from its saved markdown —
      // used for every note/tab switch (see __vaultrEditorApplyState) via
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
        return s.EditorState.create({
          doc: content || '',
          extensions: s._buildExtensions(
            inSource ? s._sourceModeExt() : s._liveModeExt(),
            (!inSource && reading) ? s.readingExtensions() : []
          ),
        });
      };

      s.view = new s.EditorView({
        parent: editArea,
        state: s._buildState(s.currentMd, false, false),
      });

      document.querySelectorAll('.content-pane-view-btn-wysiwyg').forEach(function(btn) {
        btn.addEventListener('click', function() {
          if (__vaultrEditor.inSource) editorMode.exitSource();
        });
      });
      document.querySelectorAll('.content-pane-view-btn-source').forEach(function(btn) {
        btn.addEventListener('click', function() {
          if (!__vaultrEditor.inSource) editorMode.enterSource();
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
  // hit it. __vaultrEditorOpenFind cancels s._searchCloseTimer before
  // asking CM6 to (re)open, and the timer is tracked as a single id here
  // (not left to stack one per close call) so there's only ever one to
  // cancel.
  function __vaultrEditorMakeAnimatedSearchClose(realClose) {
    return function(view) {
      var s = __vaultrEditor;
      if (s._searchCloseTimer) { clearTimeout(s._searchCloseTimer); s._searchCloseTimer = null; }
      var panel = document.querySelector('#content-pane-edit-area .cm-panels-top');
      if (!panel) { realClose(view); return; }
      panel.classList.add('is-closing');
      s._searchCloseTimer = setTimeout(function() {
        s._searchCloseTimer = null;
        realClose(view);
      }, 100);
    };
  }
  function __vaultrCreateSearchPanel(view) {
    var s = __vaultrEditor;
    var dom = document.createElement('div');
    dom.className = 'vaultr-search-panel';

    // Header: what this floating panel is, plus its one non-search control
    // (Close) — kept off the Find row itself so that row is only ever
    // search controls, not a mix of "act on the query" and "dismiss the
    // panel" buttons.
    var headerRow = document.createElement('div');
    headerRow.className = 'vaultr-sr-header';
    var headerLabel = document.createElement('span');
    headerLabel.className = 'vaultr-sr-header-label';
    headerLabel.textContent = 'Find';

    var closeBtn = document.createElement('button');
    closeBtn.type = 'button'; closeBtn.className = 'vaultr-sr-ibtn'; closeBtn.setAttribute('aria-label', 'Close');
    closeBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>';

    headerRow.append(headerLabel, closeBtn);

    // Row 1: Find. Leading chevron reveals/hides row 2 (Replace, below) —
    // collapsed by default since most Cmd+F visits are look-not-change.
    var findRow = document.createElement('div');
    findRow.className = 'vaultr-sr-row';

    var expandBtn = document.createElement('button');
    expandBtn.type = 'button'; expandBtn.className = 'vaultr-sr-ibtn vaultr-sr-expand-btn';
    expandBtn.title = 'Toggle replace'; expandBtn.setAttribute('aria-label', 'Toggle replace');
    expandBtn.setAttribute('aria-expanded', 'false');
    expandBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 6 6 6-6 6"/></svg>';

    var findWrap = document.createElement('div');
    findWrap.className = 'vaultr-sr-input-wrap';

    var findInput = document.createElement('input');
    findInput.type = 'text'; findInput.placeholder = 'Find';
    findInput.className = 'field-input vaultr-sr-input'; findInput.setAttribute('main-field', '');
    findInput.setAttribute('aria-label', 'Find');

    var findInset = document.createElement('div');
    findInset.className = 'vaultr-sr-inset-btns';

    // Match count ("2/7") — a plain label, not a button; sits ahead of the
    // nav icons so the two read together ("2/7, then ↑↓ to move"). Hidden
    // (not just empty) when the field itself is empty, so it doesn't leave
    // a dead gap before you've typed anything.
    var countEl = document.createElement('span');
    countEl.className = 'vaultr-sr-count'; countEl.setAttribute('aria-hidden', 'true');
    countEl.style.display = 'none';

    var prevBtn = document.createElement('button');
    prevBtn.type = 'button'; prevBtn.className = 'vaultr-sr-ibtn'; prevBtn.title = 'Previous (Shift+Enter)';
    prevBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m18 15-6-6-6 6"/></svg>';

    var nextBtn = document.createElement('button');
    nextBtn.type = 'button'; nextBtn.className = 'vaultr-sr-ibtn'; nextBtn.title = 'Next (Enter)';
    nextBtn.innerHTML = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>';

    var caseBtn = document.createElement('button');
    caseBtn.type = 'button'; caseBtn.className = 'vaultr-sr-ibtn vaultr-sr-toggle'; caseBtn.title = 'Match case';
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
    replaceRow.className = 'vaultr-sr-row';
    replaceRow.hidden = true;

    // Empty — just holds the column open so replaceWrap lines up under
    // findWrap instead of under expandBtn.
    var replaceSpacer = document.createElement('div');
    replaceSpacer.className = 'vaultr-sr-row-spacer';

    var replaceWrap = document.createElement('div');
    replaceWrap.className = 'vaultr-sr-input-wrap';

    var replaceInput = document.createElement('input');
    replaceInput.type = 'text'; replaceInput.placeholder = 'Replace';
    replaceInput.className = 'field-input vaultr-sr-input'; replaceInput.setAttribute('aria-label', 'Replace');

    var replaceInset = document.createElement('div');
    replaceInset.className = 'vaultr-sr-inset-btns';

    var replaceBtn = document.createElement('button');
    replaceBtn.type = 'button'; replaceBtn.className = 'vaultr-sr-ibtn vaultr-sr-text'; replaceBtn.title = 'Replace (Enter)';
    replaceBtn.textContent = 'Replace';

    var replaceAllBtn = document.createElement('button');
    replaceAllBtn.type = 'button'; replaceAllBtn.className = 'vaultr-sr-ibtn vaultr-sr-text'; replaceAllBtn.title = 'Replace All';
    replaceAllBtn.textContent = 'All';

    replaceInset.append(replaceBtn, replaceAllBtn);
    replaceWrap.append(replaceInput, replaceInset);
    replaceRow.append(replaceSpacer, replaceWrap);
    dom.append(headerRow, findRow, replaceRow);

    // State
    var caseSensitive = false;

    function buildQuery() {
      return new s.SearchQuery({ search: findInput.value, caseSensitive: caseSensitive, replace: replaceInput.value });
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
    function commit() { view.dispatch({ effects: s.setSearchQuery.of(buildQuery()) }); updateMatchCount(); }

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
      if (e.key === 'Enter') { e.preventDefault(); if (e.shiftKey) s.cmFindPrev(view); else s.cmFindNext(view); updateMatchCount(); }
      if (e.key === 'Escape') { e.preventDefault(); s.cmCloseSearchPanel(view); }
    });
    replaceInput.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') { e.preventDefault(); s.cmReplaceNext(view); updateMatchCount(); }
      if (e.key === 'Escape') { e.preventDefault(); s.cmCloseSearchPanel(view); }
    });
    prevBtn.addEventListener('click', function() { s.cmFindPrev(view); updateMatchCount(); });
    nextBtn.addEventListener('click', function() { s.cmFindNext(view); updateMatchCount(); });
    replaceBtn.addEventListener('click', function() { s.cmReplaceNext(view); updateMatchCount(); });
    replaceAllBtn.addEventListener('click', function() { s.cmReplaceAll(view); updateMatchCount(); });
    closeBtn.addEventListener('click', function() { s.cmCloseSearchPanel(view); });

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
  // entirely — see __vaultrEditorApplyState, which builds a fresh EditorState
  // (mode baked in from the start) and swaps it in with view.setState().
  var editorMode = (function() {
    function reconfigure(inSource, reading, opts) {
      var s = __vaultrEditor;
      s.view.dispatch({effects: s._modeEffects(inSource, reading)});
      __vaultrEditorSetModeState(inSource, reading);
      if (!(opts && opts.skipFocus)) focusManager.focusEditor();
    }
    return {
      // User-triggered: live preview → source
      enterSource: function() {
        if (__vaultrEditor.readingActive) __vaultrEditorSetReadingPref(false);
        reconfigure(true, false);
      },
      // User-triggered: source / reading → live preview
      exitSource: function() {
        var s = __vaultrEditor;
        if (s.readingActive) __vaultrEditorSetReadingPref(false);
        s.currentMd = s.view.state.doc.toString();
        reconfigure(false, false);
      },
      // User-triggered: any mode → reading view
      enterReading: function() {
        var s = __vaultrEditor;
        __vaultrEditorSetReadingPref(true);
        s.currentMd = s.view.state.doc.toString();
        reconfigure(false, true, {skipFocus: true});
      },
      toggle: function() {
        if (__vaultrEditor.inSource) this.exitSource(); else this.enterSource();
      },
      toggleReading: function() {
        if (__vaultrEditor.readingActive) this.exitSource(); else this.enterReading();
      },
    };
  })();

  // ── Focus manager ────────────────────────────────────────────────────────────
  // Single authority for all editor focus/blur decisions.
  var focusManager = {
    focusEditor: function() {
      var s = __vaultrEditor;
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

  function __vaultrEditorEnterSource() { editorMode.enterSource(); }
  function __vaultrEditorExitSource() { editorMode.exitSource(); }

  // "Metadata" header's pencil button (cm-live/frontmatter-collapse.js) —
  // frontmatter is read-only in live-preview mode (frontmatterReadOnly()),
  // so this dialog + allowFrontmatterEdit-annotated dispatch is the only
  // way to change it there. from/to span the whole node including the
  // "---" delimiters; only the interior YAML is shown/edited.
  function __vaultrEditorEditFrontmatter(view, from, to) {
    var raw = view.state.doc.sliceString(from, to);
    var lines = raw.split('\n');
    var hasClose = lines.length > 1 && lines[lines.length - 1].trim() === '---';
    var interior = (hasClose ? lines.slice(1, lines.length - 1) : lines.slice(1)).join('\n');
    if (!window.__vaultrEditFrontmatter) return;
    window.__vaultrEditFrontmatter(interior, function(newInterior) {
      var body = newInterior.replace(/\s+$/, '');
      var newBlock = '---\n' + (body ? body + '\n' : '') + '---';
      view.dispatch({
        changes: { from: from, to: to, insert: newBlock },
        annotations: __vaultrEditor.allowFrontmatterEdit.of(true),
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
        var s = __vaultrEditor;
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
  async function __vaultrContentPaneLoadNote(path, tabId, savedState) {
    var s = __vaultrEditor;
    if (s.dirty && s.currentPath && s.currentPath !== path) {
      clearTimeout(s.saveTimer); s.saveTimer = null; await __vaultrEditorDoSave();
    } else { clearTimeout(s.saveTimer); s.saveTimer = null; }
    if (!__vaultrEditorIsActiveTabId(tabId)) return false;
    __vaultrEditorSaveStatus('');
    var ptEl = document.getElementById('content-pane-path-text');
    if (ptEl) ptEl.textContent = path;

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
    if (!__vaultrEditorIsActiveTabId(tabId)) return false;
    s.currentPath = path; s.currentMd = __vaultrEditorTightenLists(content); s.dirty = false;
    
    // Apply state (will create new state if no saved state)
    return await __vaultrEditorApplyState(savedState || { inSource: false, scrollTop: 0 }, tabId);
  }

  async function __vaultrContentPaneSetContent(content, tabId, savedState) {
    var s = __vaultrEditor;
    if (s.dirty && s.currentPath) { clearTimeout(s.saveTimer); s.saveTimer = null; await __vaultrEditorDoSave(); }
    else { clearTimeout(s.saveTimer); s.saveTimer = null; }
    if (!__vaultrEditorIsActiveTabId(tabId)) return false;
    __vaultrEditorSaveStatus('');
    s.currentPath = ''; s.currentMd = __vaultrEditorTightenLists(content || ''); s.dirty = false;
    
    // Apply state
    return await __vaultrEditorApplyState(savedState || { inSource: false, scrollTop: 0 }, tabId);
  }
  
  async function __vaultrEditorApplyState(state, expectedTabId) {
    var s = __vaultrEditor;
    await __vaultrEnsureContentPaneEditor();
    if (!__vaultrEditorIsActiveTabId(expectedTabId)) return false;

    var targetInSource = state.inSource || false;
    var targetScroll = state.scrollTop || 0;
    // Reading takes priority over a saved source/live-preview mode; a
    // not-yet-materialized tab (no currentPath) stays editable even while
    // the preference is on.
    var wantReading = !!(s.reading && s.currentPath);
    var wantSource = !wantReading && targetInSource;

    // Clear the previous note's broken-wikilink set before the new one's
    // decorations render, so a stale "broken" flag can't flash on a target
    // that's perfectly fine in *this* note — __vaultrEditorRevalidateWikiLinks
    // repopulates it (and repaints) once the batch check comes back.
    s.brokenWikiLinkNames = new Set();
    s.wikiLinkRevalidateSeq++;

    // A note/tab switch: brand-new EditorState (mode baked in from the
    // start) swapped in via setState(), not a dispatch()'d replace onto the
    // previous tab's state — see s._buildState's comment for why.
    s.view.setState(s._buildState(s.currentMd, wantSource, wantReading));
    __vaultrEditorSetModeState(wantSource, wantReading);
    void __vaultrEditorRevalidateWikiLinks();

    if (s.pendingScrollRaf) { cancelAnimationFrame(s.pendingScrollRaf); s.pendingScrollRaf = null; }
    clearTimeout(s.pendingOpenScroll);
    var getScroller = function() { return document.querySelector('#content-pane-edit-area .cm-scroller'); };
    // Set it once synchronously, in the same tick as the state swap above
    // (setState() has already updated the DOM by the time this line runs) —
    // otherwise the scroller keeps the PREVIOUS tab's scrollTop for at
    // least one paint, since the rAF/setTimeout calls below are the only
    // other place this gets set and both are deliberately deferred (see
    // their own comments). That stale-scrollTop-against-new-content frame
    // is the tab-switch flash: this line is what removes it for the common
    // case; the deferred ones stay as-is for the one case that still needs
    // them — the pane's own slide-in animation tearing down a compositing
    // layer and resetting scrollTop out from under this synchronous set.
    var syncEl = getScroller(); if (syncEl) syncEl.scrollTop = targetScroll;
    s.pendingScrollRaf = requestAnimationFrame(function() {
      s.pendingScrollRaf = requestAnimationFrame(function() {
        s.pendingScrollRaf = null;
        if (!__vaultrEditorIsActiveTabId(expectedTabId)) return;
        var el = getScroller(); if (el) el.scrollTop = targetScroll;
        if (!focusManager.isInsideEditor()) {
          focusManager.focusEditor();
        }
      });
    });
    s.pendingOpenScroll = setTimeout(function() {
      s.pendingOpenScroll = null;
      if (!__vaultrEditorIsActiveTabId(expectedTabId)) return;
      var el = getScroller(); if (el) el.scrollTop = targetScroll;
    }, 270);
    return true;
  }


  // ── Global note-card helper ───────────────────────────────────────────────────
  function __vaultrOpenNote(el) {
    var d = el && el.dataset; if (!d || !d.notePath) return;
    var pane = window.__vaultrContentPane;
    if (pane) void pane.openNoteInContentPane(d.notePath, d.noteTitle || 'Note',
      d.noteIsKnowledge === 'true', d.notePinned === 'true', d.noteIsIndex === 'true',
      d.noteCanCompile === 'true');
  }

  // Strips a trailing .md/.markdown (case-insensitive) — mirrors the server's
  // accepted extensions (internal/util/markdown.go's markdownExts), so the
  // rename box's "bare name" and its no-op check agree with what actually
  // counts as a name change.
  function __vaultrStripMdExt(name) {
    return (name || '').replace(/\.(md|markdown)$/i, '');
  }

  // ── Content-pane controller factory ────────────────────────────────────────────────
  function contentPaneCtrl() {
    return {
      contentPaneOpen: false, tabs: [], activeTab: -1,
      _skipNextOpenLoad: false, // see _openPane()
      renaming: false, // true while #content-pane-rename-input replaces #content-pane-path-text
      _renameSubmitting: false, // reentrancy guard — see submitRenameActiveNote's leading comment
      _renameCheckTimer: null, // debounce handle for checkRenameNameAvailable's /api/vault/check-name call
      splitRatio: 0.5, isPaneResizing: false, _prevSplitRatio: 0.5,

      // Tab-bar maximize/restore button — same spot/icon the old "focus
      // mode" toggle used, but it just drives splitRatio to/from 0 now
      // instead of a separate expanded state (the sidebar always stays
      // visible; see home.css's .content-pane-open rule).
      toggleMaximizeEditor() {
        if (this.splitRatio > 0.02) {
          this._prevSplitRatio = this.splitRatio;
          this.splitRatio = 0;
        } else {
          this.splitRatio = this._prevSplitRatio || 0.5;
        }
      },

      // Drags the divider between #home-list-pane and the editor pane.
      // Ratio is list-pane's share of the space left over once the
      // sidebar and resizer are accounted for (see home.css's
      // --split-ratio/--split-ratio-inv). Dragging all the way to the
      // sidebar collapses the list pane to 0 — the editor fills 100% of
      // the content area (not the whole window; the sidebar stays put).
      // Dragging the other way is capped so the editor never fully
      // disappears while still open.
      //
      // Perf: mousemove can fire far faster than the display refreshes,
      // and going through Alpine's reactive splitRatio on every one of
      // those events would re-run the whole :style expression (string
      // concat + reparse) well more often than needed. Instead this
      // writes the two CSS vars straight to the element, coalesced to one
      // update per animation frame — self.splitRatio (and its Alpine
      // side-effects: persistence, the maximize-button icon, etc.) is
      // synced exactly once, on mouseup. A full-viewport capture layer
      // owns the drag's mousemove/mouseup for the same reason a native
      // <input type=range> thumb does: without it, a fast drag can carry
      // the pointer over CodeMirror's contenteditable mid-gesture, which
      // would otherwise start a text selection instead of resizing.
      startPaneResize(e) {
        e.preventDefault();
        var self = this;
        var shell = document.querySelector('.home-shell');
        var side = document.getElementById('home-side');
        var resizer = document.getElementById('home-pane-resizer');
        if (!shell || !side || !resizer) return;
        var shellRect = shell.getBoundingClientRect();
        var sideW = side.offsetWidth;
        var resizerW = resizer.offsetWidth;
        var avail = shellRect.width - sideW - resizerW;
        if (avail <= 0) return;

        self.isPaneResizing = true;
        var prevCursor = document.body.style.cursor;
        var prevUserSelect = document.body.style.userSelect;
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';

        var capture = document.createElement('div');
        capture.style.cssText = 'position:fixed;inset:0;z-index:9999;cursor:col-resize;';
        document.body.appendChild(capture);

        var latestRatio = self.splitRatio;
        var rafId = null;
        function applyRatio() {
          rafId = null;
          shell.style.setProperty('--split-ratio', latestRatio);
          shell.style.setProperty('--split-ratio-inv', 1 - latestRatio);
        }
        function onMove(ev) {
          var x = ev.clientX - shellRect.left - sideW - (resizerW / 2);
          var ratio = x / avail;
          latestRatio = Math.min(0.9, Math.max(0, ratio));
          if (rafId == null) rafId = requestAnimationFrame(applyRatio);
        }
        function onUp() {
          if (rafId != null) { cancelAnimationFrame(rafId); applyRatio(); }
          document.removeEventListener('mousemove', onMove);
          document.removeEventListener('mouseup', onUp);
          capture.remove();
          document.body.style.cursor = prevCursor;
          document.body.style.userSelect = prevUserSelect;
          self.isPaneResizing = false;
          self.splitRatio = latestRatio;
          try { localStorage.setItem('vaultr.splitRatio', String(latestRatio)); } catch(_) {}
        }
        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
      },

      _persist() {
        try {
          localStorage.setItem('vaultr.content-pane', JSON.stringify({
            // A not-yet-materialized tab (no path) has nothing worth
            // persisting across a reload: if it's still empty, dropping it
            // is a no-op; if the user typed something,
            // __vaultrEditorMaterializeTab already gave it a real path
            // before this next persist call fires.
            tabs: this.tabs.filter(function(t){ return !!t.path; }).map(function(t){
              return {id:t.id, title:t.title, path:t.path, isKnowledge:!!t.isKnowledge, pinned:!!t.pinned, isIndex:!!t.isIndex, canCompile:!!t.canCompile};
            }),
            activeTab: this.activeTab,
          }));
        } catch(_) {}
      },

      _restore() {
        try {
          var raw = localStorage.getItem('vaultr.content-pane'); if (!raw) return;
          var data = JSON.parse(raw);
          if (!data || !Array.isArray(data.tabs) || !data.tabs.length) return;
          var tabs = data.tabs;
          if (tabs.length > CONTENT_PANE_MAX_TABS) {
            var byAge = tabs.slice().sort(function(a,b){return a.id-b.id;});
            var remove = new Set(byAge.slice(0, tabs.length-CONTENT_PANE_MAX_TABS).map(function(t){return t.id;}));
            tabs = tabs.filter(function(t){return !remove.has(t.id);});
          }
          this.tabs = tabs.map(function(t) {
            var path = t.path || '';
            if (!path && t.fragmentUrl) { try { path = new URL(t.fragmentUrl, location.origin).searchParams.get('path') || ''; } catch(_){} }
            if (!path && t.pageUrl)     { try { path = new URL(t.pageUrl,     location.origin).searchParams.get('path') || ''; } catch(_){} }
            return {id:t.id, title:t.title||'Note', path:path, isKnowledge:!!t.isKnowledge, pinned:!!t.pinned, isIndex:!!t.isIndex, canCompile:!!t.canCompile};
          }).filter(function(t){ return !!t.path; });
          if (!this.tabs.length) return;
          this.activeTab = Math.min(Math.max(data.activeTab||0, 0), this.tabs.length-1);
        } catch(_) {}
      },

      initContentPane() {
        window.__vaultrContentPane = this;
        this._restore();
        try {
          var savedRatio = parseFloat(localStorage.getItem('vaultr.splitRatio'));
          if (!isNaN(savedRatio) && savedRatio >= 0 && savedRatio <= 0.9) this.splitRatio = savedRatio;
        } catch(_) {}

        var self = this; var prevOpen = false;
        // Esc still closes whatever's layered on top of the editor (the
        // more/tabs dropdowns, CodeMirror's search panel) — it just no
        // longer closes the editor itself once those are all closed.
        var _paneEscClose = function() {
          var moreMenu = document.querySelector('.content-pane-more-menu');
          if (moreMenu && moreMenu.style.display !== 'none') {
            document.dispatchEvent(new CustomEvent('content-pane:close-more'));
            focusManager.blurActive();
            return;
          }
          var tabsMenu = document.querySelector('.content-pane-tabs-menu');
          if (tabsMenu && tabsMenu.style.display !== 'none') {
            document.dispatchEvent(new CustomEvent('content-pane:close-tabs'));
            focusManager.blurActive();
            return;
          }
          if (document.querySelector('.vaultr-search-panel')) {
            var _s = __vaultrEditor;
            if (_s.view && _s.cmCloseSearchPanel) _s.cmCloseSearchPanel(_s.view);
            return;
          }
        };
        this.$watch('contentPaneOpen', async function(isOpen) {
          if (!isOpen) {
            if (window.__vaultrEscPop) window.__vaultrEscPop('content-pane');
            // Save state BEFORE blur — blur can trigger scrollIntoView which resets scrollTop
            var currentTab = self.tabs[self.activeTab];
            if (currentTab) await __vaultrEditorSaveTabForLeave(currentTab);
            focusManager.blurActive();
            prevOpen = false; return;
          }
          prevOpen = true;
          // The inbox detail panel shares this same pane (see content_pane.html's
          // inbox-detail-panel) — always one or the other.
          if (self.inboxSheetOpen) self.inboxSheetOpen = false;
          var _paneOverlayEl = document.querySelector('.content-pane');
          if (_paneOverlayEl) {
            _paneOverlayEl.classList.add('content-pane-is-opening');
            setTimeout(function() { _paneOverlayEl.classList.remove('content-pane-is-opening'); }, 320);
          }
          if (window.__vaultrEscPush) window.__vaultrEscPush('content-pane', _paneEscClose);
          if (self._skipNextOpenLoad) { self._skipNextOpenLoad = false; return; } // see _openPane()
          // Refresh key-behavior config on every open so settings changes take
          // effect immediately without restarting the app. Called before any
          // content loading so all code paths (same note, new note) pick it
          // up. No-op if the editor hasn't been created yet (handled later in
          // __vaultrEnsureContentPaneEditor's initPromise).
          // Always read latest tabs from localStorage before opening — other
          // WebContentsViews (same session, different JS context) may have
          // added tabs since this view last called _restore().
          self._restore();
          var tab = self.tabs[self.activeTab];
          if (!tab) return;

          var savedState = __vaultrEditorRestoreTabState(tab.id);
          
          if (!tab.path) {
            await self._openUntitledTab(tab, savedState);
            return;
          }
          if (__vaultrEditor.currentPath !== tab.path || !__vaultrEditor.dirty) {
            void __vaultrContentPaneLoadNote(tab.path, tab.id, savedState);
          } else if (savedState) {
            // Same note with unsaved edits — restore scroll only. Two passes:
            // 1) rAF: fire immediately so scroll looks right during slide-in animation
            // 2) setTimeout(250): fire after the 240ms CSS transition in case the browser
            //    resets scrollTop when the GPU compositing layer is torn down at animation end
            var s = __vaultrEditor;
            var sc = savedState.scrollTop || 0;
            function __applyContentPaneScroll() {
              var scroller = document.querySelector('#content-pane-edit-area .cm-scroller');
              if (scroller) scroller.scrollTop = sc;
            }
            if (s.pendingScrollRaf) { cancelAnimationFrame(s.pendingScrollRaf); s.pendingScrollRaf = null; }
            clearTimeout(s.pendingOpenScroll);
            s.pendingScrollRaf = requestAnimationFrame(function() {
              s.pendingScrollRaf = null;
              __applyContentPaneScroll();
              if (!focusManager.isInsideEditor()) {
                focusManager.focusEditor();
              }
            });
            s.pendingOpenScroll = setTimeout(function() {
              s.pendingOpenScroll = null;
              __applyContentPaneScroll();
            }, 250);
          }
        });
        this.$watch('tabs', function(){ self._persist(); });
        this.$watch('activeTab', function(i) {
          self._persist();
        });

        // Sync pane state from other section views (each section is a separate
        // WebContentsView with its own JS context; storage events cross view boundaries).
        window.addEventListener('storage', function(e) {
          if (e.key !== 'vaultr.content-pane' || !e.newValue || self.contentPaneOpen) return;
          self._restore();
        });
      },

      // Open an existing note in the pane.
      async openNoteInContentPane(path, title, isKnowledge, pinned, isIndex, canCompile) {
        if (!path) return;
        this.cancelRenameActiveNote(); // opening any note (even re-opening the active one) always discards an in-progress rename on whatever tab was showing
        var prevTab = this.tabs[this.activeTab];
        if (this.contentPaneOpen && prevTab) await __vaultrEditorSaveTabForLeave(prevTab);
        this.upsertTab(path, title, isKnowledge, pinned, isIndex, canCompile);
        __vaultrEditorResetCompileBtn();
        this._openPane();
        // Already loaded with unsaved edits — don't clobber with a server fetch
        if (__vaultrEditor.currentPath === path && __vaultrEditor.dirty) return;
        var tab = this.tabs[this.activeTab];
        var savedState = tab ? __vaultrEditorRestoreTabState(tab.id) : null;
        await __vaultrContentPaneLoadNote(path, tab ? tab.id : null, savedState);
      },

      // Open the pane on a brand-new, empty tab. Nothing is created
      // server-side yet — the first real edit materializes it (see
      // __vaultrEditorMaterializeTab) with an auto-generated name; there's
      // no filename to ask for up front and no draft/publish step.
      async openNewInContentPane() {
        this.cancelRenameActiveNote(); // see openNoteInContentPane
        var s = __vaultrEditor;
        var prevTab = this.tabs[this.activeTab];
        if (this.contentPaneOpen && prevTab) await __vaultrEditorSaveTabForLeave(prevTab);
        if (s.dirty && s.currentPath) { clearTimeout(s.saveTimer); s.saveTimer = null; await __vaultrEditorDoSave(); }
        else { clearTimeout(s.saveTimer); s.saveTimer = null; }
        __vaultrEditorSaveStatus('');

        var newTab = {id:__vaultrEditorNewTabId(), title:'Untitled', path:'', isKnowledge:false, pinned:false};
        this.tabs.push(newTab);
        this.activeTab = this.tabs.length - 1;

        if (this.tabs.length > CONTENT_PANE_MAX_TABS) {
          var oldestIdx = __vaultrOldestEvictableTabIdx(this.tabs, this.activeTab);
          if (oldestIdx >= 0) {
            var evictedTab = this.tabs[oldestIdx];
            __vaultrEditorClearTabState(evictedTab.id);
            this.tabs.splice(oldestIdx,1);
            if (oldestIdx < this.activeTab) this.activeTab--;
          }
        }
        this._moveActiveTabToFront();
        this._openPane();

        s.currentPath = ''; s.currentMd = ''; s.dirty = false;
        var ptEl = document.getElementById('content-pane-path-text');
        if (ptEl) ptEl.textContent = 'Untitled';
        await __vaultrEditorApplyState({ inSource: false, scrollTop: 0 }, newTab.id);
      },

      // Keeps the tabs list ordered most-recently-opened-first, so the
      // tabs panel always shows whatever was just opened/switched to at
      // the top. Called with this.activeTab already pointing at the tab
      // to promote — a plain array move, since callers key everything
      // else off tab object references (nextTab/newTab/etc.), not index.
      _moveActiveTabToFront() {
        var i = this.activeTab;
        if (i <= 0 || i >= this.tabs.length) return;
        var tab = this.tabs.splice(i, 1)[0];
        this.tabs.unshift(tab);
        this.activeTab = 0;
      },

      upsertTab(path, title, isKnowledge, pinned, isIndex, canCompile) {
        var idx = -1;
        for (var i = 0; i < this.tabs.length; i++) { if (this.tabs[i].path === path) { idx=i; break; } }
        if (idx >= 0) {
          this.tabs[idx].title = title || this.tabs[idx].title;
          if (isKnowledge !== undefined) this.tabs[idx].isKnowledge = !!isKnowledge;
          if (pinned !== undefined) this.tabs[idx].pinned = !!pinned;
          if (isIndex !== undefined) this.tabs[idx].isIndex = !!isIndex;
          if (canCompile !== undefined) this.tabs[idx].canCompile = !!canCompile;
          this.activeTab = idx;
        } else {
          this.tabs.push({id:__vaultrEditorNewTabId(), title:title||'Note', path:path, isKnowledge:!!isKnowledge, pinned:!!pinned, isIndex:!!isIndex, canCompile:!!canCompile});
          this.activeTab = this.tabs.length - 1;
          if (this.tabs.length > CONTENT_PANE_MAX_TABS) {
            var oldestIdx = __vaultrOldestEvictableTabIdx(this.tabs, this.activeTab);
            if (oldestIdx >= 0) {
              var evicted = this.tabs[oldestIdx];
              __vaultrEditorClearTabState(evicted.id);
              this.tabs.splice(oldestIdx,1);
              if (oldestIdx < this.activeTab) this.activeTab--;
            }
          }
        }
        this._moveActiveTabToFront();
      },

      // Flips contentPaneOpen false→true while telling its $watch (in
      // initContentPane) to skip self._restore() + reload: the caller
      // already pushed/upserted the right tab and will load it itself.
      _openPane() {
        if (!this.contentPaneOpen) this._skipNextOpenLoad = true;
        this.contentPaneOpen = true;
      },

      markTabCompiled(path) {
        for (var i = 0; i < this.tabs.length; i++) {
          if (this.tabs[i].path === path) { this.tabs[i].canCompile = false; return; }
        }
      },

      // Re-open a not-yet-materialized tab: nothing to load from the server
      // (it doesn't exist there yet) — just restore whatever was typed
      // before the user switched away (see __vaultrEditorHandleContentChange's
      // tab._pendingContent) and focus the editor.
      async _openUntitledTab(tab, savedState) {
        await __vaultrContentPaneSetContent(tab._pendingContent || '', tab.id, savedState);
        if (!__vaultrEditorIsActiveTabId(tab.id)) return;
        var ptEl = document.getElementById('content-pane-path-text');
        if (ptEl) ptEl.textContent = 'Untitled';
      },

      async contentPaneSwitchTab(i) {
        this.cancelRenameActiveNote(); // see openNoteInContentPane
        focusManager.blurActive();
        if (i === this.activeTab) return;

        var prevTab = this.tabs[this.activeTab];
        var nextTab = this.tabs[i];
        if (!nextTab) return;

        // Save current tab's state
        if (prevTab) {
          await __vaultrEditorSaveTabForLeave(prevTab);
        }

        // Switch active tab
        this.activeTab = i;
        this._moveActiveTabToFront();
        __vaultrEditorResetCompileBtn();

        // Load next tab's content with saved state
        var savedState = __vaultrEditorRestoreTabState(nextTab.id);

        if (!nextTab.path) {
          await this._openUntitledTab(nextTab, savedState);
        } else {
          await __vaultrContentPaneLoadNote(nextTab.path, nextTab.id, savedState);
        }
      },

      async contentPaneCloseTab(i) {
        if (i < 0 || i >= this.tabs.length) return;
        this.cancelRenameActiveNote(); // see openNoteInContentPane
        var wasActive = (i === this.activeTab);
        var closingTab = this.tabs[i];
        if (wasActive && closingTab.path && __vaultrEditor.dirty && __vaultrEditor.currentPath === closingTab.path) {
          clearTimeout(__vaultrEditor.saveTimer); __vaultrEditor.saveTimer = null;
          await __vaultrEditorDoSave();
          var liveIdx = this.tabs.indexOf(closingTab);
          if (liveIdx < 0) return;
          i = liveIdx;
          wasActive = (i === this.activeTab);
        }

        // Clear saved state for this tab
        __vaultrEditorClearTabState(closingTab.id);

        this.tabs.splice(i, 1);

        if (this.tabs.length === 0) {
          this.contentPaneOpen = false; this.activeTab = -1;
          clearTimeout(__vaultrEditor.saveTimer); __vaultrEditor.saveTimer = null;
          __vaultrEditor.currentPath = ''; __vaultrEditor.currentMd = ''; __vaultrEditor.dirty = false;
          __vaultrEditorSaveStatus('');
          if (__vaultrEditor.view) {
            __vaultrEditor.view.setState(__vaultrEditor._buildState('', false, false));
            __vaultrEditorSetModeState(false, false);
          }
          return;
        }

        if (i < this.activeTab) {
          this.activeTab -= 1;
        } else if (wasActive) {
          this.activeTab = Math.min(i, this.tabs.length-1);
          var t = this.tabs[this.activeTab];
          if (!t) return;

          var savedState = __vaultrEditorRestoreTabState(t.id);

          if (t.path) {
            void __vaultrContentPaneLoadNote(t.path, t.id, savedState);
          } else {
            void this._openUntitledTab(t, savedState);
          }
        }
      },

      async togglePinActiveNote() {
        var tab = this.tabs[this.activeTab];
        if (!tab || !tab.path) return;
        if (tab.pinned) await this.unpinActiveNote(); else await this.pinActiveNote();
      },

      async pinActiveNote() {
        var tab = this.tabs[this.activeTab]; if (!tab || !tab.path || tab.pinned) return;
        var resp = await fetch('/api/vault/pin', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path:tab.path,pinned:true})});
        if (resp.status === 409) { window.showError((await resp.text()).trim() || 'Pinned notes limit reached.', 'Cannot pin'); return; }
        if (!resp.ok) return;
        tab.pinned = true;
        if (window.__vaultrAfterVaultMutation) await window.__vaultrAfterVaultMutation();
      },

      async unpinActiveNote() {
        var tab = this.tabs[this.activeTab]; if (!tab || !tab.path || !tab.pinned) return;
        var resp = await fetch('/api/vault/pin', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path:tab.path,pinned:false})});
        if (!resp.ok) return;
        tab.pinned = false;
        if (window.__vaultrAfterVaultMutation) await window.__vaultrAfterVaultMutation();
      },

      async deleteActiveNote() {
        var tab = this.tabs[this.activeTab]; if (!tab || !tab.path) return;
        var confirmed = await window.showConfirm({titleHTML:'<span class="confirm-title-icon"><svg width="13" height="13" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10M5 4V3a1 1 0 011-1h2a1 1 0 011 1v1M12 4l-1 8H3L2 4"/><path d="M6 7v3M8 7v3"/></svg></span>Delete note',message:'"'+tab.title+'" will be permanently deleted.',confirmLabel:'Delete',danger:true});
        if (!confirmed) return;
        var reqBody = {path:tab.path};
        var resp = await fetch('/api/vault/delete', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(reqBody)});
        if (!resp.ok) return;
        clearTimeout(__vaultrEditor.saveTimer); __vaultrEditor.saveTimer = null; __vaultrEditor.dirty = false;
        var cur = this.activeTab;
        var deletedTab = this.tabs[cur];
        if (deletedTab) __vaultrEditorClearTabState(deletedTab.id);
        this.tabs.splice(cur, 1);
        if (this.tabs.length === 0) {
          this.contentPaneOpen = false; this.activeTab = -1;
          __vaultrEditor.currentPath = ''; __vaultrEditor.currentMd = ''; __vaultrEditorSaveStatus('');
          if (__vaultrEditor.view) {
            __vaultrEditor.view.setState(__vaultrEditor._buildState('', false, false));
            __vaultrEditorSetModeState(false, false);
          }
        } else {
          this.activeTab = cur > 0 ? cur-1 : 0;
          var nextTab = this.tabs[this.activeTab];
          if (nextTab && nextTab.path) void __vaultrContentPaneLoadNote(nextTab.path, nextTab.id, __vaultrEditorRestoreTabState(nextTab.id));
          else if (nextTab) {
            void this._openUntitledTab(nextTab, __vaultrEditorRestoreTabState(nextTab.id));
          }
        }
        if (window.__vaultrAfterVaultMutation) await window.__vaultrAfterVaultMutation();
      },

      // Moving a note is driven entirely from the home list (drag a card onto
      // a sidebar folder — see home.js's drag-and-drop handlers), not from
      // here. These two are the seam that side needs into the editor:
      // flush a dirty active tab before the move (so the file that actually
      // gets renamed has the latest content) and re-point an open tab at the
      // new path afterward (autosave targets __vaultrEditor.currentPath, not
      // tab.path — miss that and the next edit would silently recreate the
      // note at its old spot).
      async flushPendingSaveFor(path) {
        if (__vaultrEditor.dirty && __vaultrEditor.currentPath === path) {
          clearTimeout(__vaultrEditor.saveTimer); __vaultrEditor.saveTimer = null;
          await __vaultrEditorDoSave();
        }
      },
      noteMoved(oldPath, newPath) {
        var tab = this.tabs.find(function(t) { return t.path === oldPath; });
        if (!tab) return;
        tab.path = newPath;
        if (__vaultrEditor.currentPath === oldPath) __vaultrEditor.currentPath = newPath;
        if (this.tabs[this.activeTab] === tab) {
          var ptEl = document.getElementById('content-pane-path-text');
          if (ptEl) ptEl.textContent = newPath;
        }
      },

      // Renaming only ever changes the active tab's filename, never its
      // directory (see storage.Vault.RenameNote) — dir+move is drag-and-drop
      // only, from the sidebar (home.js), same split as noteMoved above.
      renameActiveNote() {
        var tab = this.tabs[this.activeTab];
        if (!tab || !tab.path || this.renaming) return;
        this.renaming = true;
        // Focus-independent fallback: @keydown.escape only fires while the
        // input has focus, which tryFocus below isn't always able to land.
        var self = this;
        if (window.__vaultrEscPush) window.__vaultrEscPush('content-pane-rename', function() { self.cancelRenameActiveNote(); });

        var base = __vaultrStripMdExt(tab.path.split('/').pop());
        this.$nextTick(function() {
          // Retry a few frames: something else (an Alpine transition,
          // CodeMirror) occasionally wins the focus race right after this.
          var tryFocus = function(attemptsLeft) {
            var input = document.getElementById('content-pane-rename-input');
            if (!input || !self.renaming) return;
            input.disabled = false; // a leftover disabled state silently blocks focus()
            input.value = base;
            input.classList.remove('invalid');
            input.title = '';
            input.focus();
            input.select();
            if (document.activeElement !== input && attemptsLeft > 0) {
              requestAnimationFrame(function() { tryFocus(attemptsLeft - 1); });
            }
          };
          tryFocus(3);
        });
      },

      // Single close path, so the ESC-stack push/pop above always pairs up.
      cancelRenameActiveNote() {
        if (!this.renaming) return;
        this.renaming = false;
        clearTimeout(this._renameCheckTimer);
        if (window.__vaultrEscPop) window.__vaultrEscPop('content-pane-rename');
      },

      // Advisory-only, debounced "is this name already taken?" hint while the
      // user types — never blocks Enter/submit, which still re-checks
      // server-side atomically with the actual rename (see RenameNote/
      // dbNameTaken). This can only ever be a UX nicety: the check and the
      // eventual submit are two separate requests, so another tab/rename
      // could always take the name in between.
      checkRenameNameAvailable() {
        clearTimeout(this._renameCheckTimer);
        var self = this;
        var input = document.getElementById('content-pane-rename-input');
        var tab = this.tabs[this.activeTab];
        if (!input || !tab || !tab.path) return;

        var newBase = input.value.trim();
        var currentBase = __vaultrStripMdExt(tab.path.split('/').pop());
        if (!newBase || newBase === currentBase || /[\/\\]/.test(newBase)) {
          // Unchanged, empty, or a "/" — already handled elsewhere at submit time.
          input.classList.remove('invalid');
          input.title = '';
          return;
        }

        this._renameCheckTimer = setTimeout(async function() {
          var resp;
          try {
            resp = await fetch('/api/vault/check-name?path=' + encodeURIComponent(tab.path) + '&newName=' + encodeURIComponent(newBase));
          } catch (_) { return; } // best-effort hint only — a network hiccup here must never block typing
          // The box may have closed, or the text moved on, while this was in flight.
          if (!self.renaming || input.value.trim() !== newBase) return;
          if (!resp.ok) return;
          var data = await resp.json();
          if (data.available === false) {
            input.classList.add('invalid');
            input.title = 'A note named "' + newBase + '" already exists in the vault.';
          } else {
            input.classList.remove('invalid');
            input.title = '';
          }
        }, 300);
      },

      async submitRenameActiveNote() {
        // input.disabled below synchronously blurs the input, re-entering
        // here via @blur while the first call's fetch is still in flight —
        // without this guard that fires a duplicate POST for the same old
        // path, and whichever the server processes second 404s.
        if (this._renameSubmitting) return;
        if (!this.renaming) return; // already closed (Escape, tab switch, …)
        clearTimeout(this._renameCheckTimer); // the authoritative check below supersedes the advisory one
        var tab = this.tabs[this.activeTab];
        var input = document.getElementById('content-pane-rename-input');
        if (!tab || !tab.path || !input) { this.cancelRenameActiveNote(); return; }

        var newBase = input.value.trim();
        var currentBase = __vaultrStripMdExt(tab.path.split('/').pop());
        if (!newBase || newBase === currentBase) { this.cancelRenameActiveNote(); return; }
        if (/[\/\\]/.test(newBase)) {
          window.showError('A file name cannot contain "/".', 'Cannot rename');
          input.classList.add('invalid');
          return;
        }

        this._renameSubmitting = true;
        try {
          await this.flushPendingSaveFor(tab.path); // save the latest content before the file underneath it moves

          input.disabled = true; // triggers the synchronous re-entrant blur the guard above exists for
          var resp = await fetch('/api/vault/rename', {
            method: 'POST', headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({path: tab.path, newName: newBase}),
          });
          if (!resp.ok) {
            var msg = (await resp.text()).trim() || 'Rename failed.';
            if (resp.status === 409) msg = 'A note named "' + newBase + '" already exists in the vault — note names must be unique.';
            else if (resp.status === 403) msg = 'This note can’t be renamed.';
            window.showError(msg, 'Cannot rename');
            input.classList.add('invalid');
            input.disabled = false;
            input.focus();
            return;
          }
          var data = await resp.json();
          input.disabled = false; // undo the disable a few lines up — left true after a successful rename, the *next* rename could never focus this same <input> node again
          this.noteRenamed(tab.path, data.path);
          this.cancelRenameActiveNote(); // renamed successfully — same box-closing path as a cancel, just after committing
          if (window.__vaultrAfterVaultMutation) await window.__vaultrAfterVaultMutation();
          if (data.renameJobId) void __vaultrPollRenameJob(data.renameJobId);
        } catch (e) {
          window.showError((e && e.message) || 'Network error.', 'Cannot rename');
          input.disabled = false;
        } finally {
          this._renameSubmitting = false;
        }
      },

      noteRenamed(oldPath, newPath) {
        var tab = this.tabs.find(function(t) { return t.path === oldPath; });
        if (!tab) return;
        tab.path = newPath;
        tab.title = __vaultrStripMdExt(newPath.split('/').pop()) || newPath;
        if (__vaultrEditor.currentPath === oldPath) __vaultrEditor.currentPath = newPath;
        if (this.tabs[this.activeTab] === tab) {
          var ptEl = document.getElementById('content-pane-path-text');
          if (ptEl) ptEl.textContent = newPath;
        }
      },
    };
  }

  // Best-effort poll for the async wikilink/source_notes sweep a rename
  // enqueues (internal/plugins/renamesync) — purely informational, never
  // blocks or retries the rename itself, which has already fully committed
  // by the time this runs. Silent on success (matches Pin/Unpin's existing
  // no-toast convention here); only speaks up if the sweep itself failed, so
  // the user knows some [[wikilinks]] elsewhere may still say the old name.
  async function __vaultrPollRenameJob(jobId) {
    var deadline = Date.now() + 5 * 60 * 1000;
    await new Promise(function(r) { setTimeout(r, 600); });
    while (Date.now() < deadline) {
      var resp;
      try { resp = await fetch('/api/vault/rename-status?id=' + encodeURIComponent(jobId)); }
      catch (_) { return; }
      if (resp.ok) {
        var st = await resp.json();
        if (st.status === 'done') return;
        if (st.status === 'failed') {
          if (window.showError) window.showError('Some [[wikilinks]] to the old name may not have been updated automatically.', 'Rename cleanup incomplete');
          return;
        }
      }
      await new Promise(function(r) { setTimeout(r, 1500); });
    }
  }

  // ── Compile raw note from pane ─────────────────────────────────────────────
  function __vaultrEditorCompileLabel(btn, text) {
    var label = btn.querySelector('.content-pane-compile-label');
    if (label) label.textContent = text;
  }
  function __vaultrEditorCloseMoreMenu() {
    document.dispatchEvent(new CustomEvent('content-pane:close-more'));
  }
  function __vaultrEditorResetCompileBtn() {
    var btn = document.querySelector('.content-pane-compile-btn');
    if (!btn) return;
    btn.classList.remove('is-compiling', 'success');
    btn.disabled = false;
    btn.title = 'Compile to knowledge note';
    __vaultrEditorCompileLabel(btn, 'Compile');
  }

  async function compileContentPaneNote(event) {
    var btn = event && event.currentTarget;
    if (!btn || btn.disabled) return;
    var pane = window.__vaultrContentPane;
    var tab = pane ? pane.tabs[pane.activeTab] : null;
    if (!tab || !tab.path || !tab.canCompile) return;
    var rawPath = tab.path;

    if (__vaultrEditor.dirty && __vaultrEditor.currentPath === rawPath) {
      clearTimeout(__vaultrEditor.saveTimer);
      __vaultrEditor.saveTimer = null;
      await __vaultrEditorDoSave();
    }

    var originalTitle = btn.title;
    btn.disabled = true;
    btn.classList.add('is-compiling');
    btn.title = 'Compiling…';
    __vaultrEditorCompileLabel(btn, 'Compiling…');

    try {
      var resp = await fetch('/api/compile/trigger', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({path: rawPath}),
      });
      if (!resp.ok && resp.status !== 409) {
        var msg = 'Compile failed';
        try { var data = await resp.json(); if (data && data.error) msg = data.error; } catch (_) {}
        throw new Error(msg);
      }
      if (resp.status === 409) {
        btn.disabled = false;
        btn.title = originalTitle;
        __vaultrEditorCompileLabel(btn, 'Compile');
        return;
      }

      var pollURL = '/api/runs/by-ref?path=' + encodeURIComponent(rawPath);
      var deadline = Date.now() + 10 * 60 * 1000;
      await new Promise(function(r) { setTimeout(r, 600); });
      while (Date.now() < deadline) {
        var pr = await fetch(pollURL);
        if (pr.ok) {
          var st = await pr.json();
          if (st.status === 'succeeded') {
            if (pane && pane.markTabCompiled) pane.markTabCompiled(rawPath);
            btn.classList.add('success');
            btn.title = 'Compiled';
            __vaultrEditorCompileLabel(btn, 'Compiled');
            setTimeout(function() {
              btn.disabled = false;
              btn.classList.remove('success');
              __vaultrEditorCompileLabel(btn, 'Compile');
              __vaultrEditorCloseMoreMenu();
            }, 1500);
            if (window.__vaultrAfterVaultMutation) await window.__vaultrAfterVaultMutation();
            return;
          }
          if (st.status === 'failed' || st.status === 'canceled') {
            throw new Error('Compile agent ' + st.status);
          }
        }
        await new Promise(function(r) { setTimeout(r, 1500); });
      }
      throw new Error('Compile timed out');
    } catch (err) {
      btn.disabled = false;
      btn.title = err && err.message ? err.message : 'Compile failed';
      __vaultrEditorCompileLabel(btn, 'Compile');
    } finally {
      btn.classList.remove('is-compiling');
    }
  }

  // Global undo/redo called by Electron main process via executeJavaScript.
  // Calls CodeMirror undo directly, bypassing browser native undo.
  window.__vaultrUndo = function() {
    var s = __vaultrEditor;
    if (s.view && s.cmUndo) s.cmUndo(s.view);
  };
  window.__vaultrRedo = function() {
    var s = __vaultrEditor;
    if (s.view && s.cmRedo) s.cmRedo(s.view);
  };

  window.__vaultrEditorShellHref = function() {
    try {
      var seg = location.pathname.replace(/^\/+/,'').split('/')[0];
      return '/'+(seg||'home');
    } catch(_) { return '/home'; }
  };

  window.__vaultrHotkeys.register('content-pane', 'o', function() {
    if (window.__vaultrContentPane) window.__vaultrContentPane.contentPaneOpen = !window.__vaultrContentPane.contentPaneOpen;
  });

  window.__vaultrHotkeys.register('new-note', 'n', function() {
    if (window.__vaultrContentPane) void window.__vaultrContentPane.openNewInContentPane();
  });

  window.__vaultrHotkeys.register('content-pane-maximize', '\\', function() {
    var _pane = window.__vaultrContentPane;
    if (_pane && _pane.contentPaneOpen) _pane.toggleMaximizeEditor();
  });

  window.__vaultrHotkeys.registerRaw('content-pane-scroll', function(e, mod) {
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
    if (mod || e.altKey) return;
    var ae = document.activeElement;
    var tag = ae && ae.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;
    var _pane = window.__vaultrContentPane;
    if (!_pane || !_pane.contentPaneOpen) return;
    var _editArea = document.getElementById('content-pane-edit-area');
    if (_editArea && _editArea.contains(ae)) return;
    e.preventDefault();
    var _scroller = document.querySelector('#content-pane-edit-area .cm-scroller');
    if (_scroller) _scroller.scrollBy(0, e.key === 'ArrowDown' ? 80 : -80);
    return true;
  });

  window.__vaultrHotkeys.registerRaw('content-pane-close-tab', function(e, mod) {
    if (!mod || e.shiftKey || e.altKey || e.key.toLowerCase() !== 'w') return;
    var pane = window.__vaultrContentPane;
    if (!pane || !pane.contentPaneOpen || pane.activeTab < 0) return;
    e.preventDefault();
    var at = pane.tabs[pane.activeTab];
    if (at && at.path) pane.contentPaneCloseTab(pane.activeTab);
    return true;
  });

  // Shared by the Mod-F hotkey below and the editor's "more" menu (Find
  // item, content_pane.html) so there's one place that knows how to open it.
  function __vaultrEditorOpenFind() {
    var s = __vaultrEditor;
    if (!s.cmOpenSearchPanel || !s.view) return;
    // Cancel a still-pending animated close (__vaultrEditorMakeAnimatedSearchClose)
    // and snap any fading-out panel back to visible first — otherwise that
    // stale timer would go on to close the panel this call is about to
    // (re)open. See the comment on __vaultrEditorMakeAnimatedSearchClose.
    if (s._searchCloseTimer) { clearTimeout(s._searchCloseTimer); s._searchCloseTimer = null; }
    var panel = document.querySelector('#content-pane-edit-area .cm-panels-top');
    if (panel) panel.classList.remove('is-closing');
    s.cmOpenSearchPanel(s.view);
  }

  async function __vaultrEditorCopyMarkdown() {
    var s = __vaultrEditor;
    var text = s.view ? s.view.state.doc.toString() : s.currentMd;
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (e) {
      if (window.showError) window.showError((e && e.message) || 'Could not copy to clipboard.', 'Copy error');
      return false;
    }
  }

  // Plain Mod-L now (no Shift needed) — .register() handles that, unlike
  // the Mod-Shift-E it replaced which needed registerRaw's manual check.
  window.__vaultrHotkeys.register('content-pane-reading-toggle', 'l', function() {
    var pane = window.__vaultrContentPane;
    if (!pane || !pane.contentPaneOpen || !__vaultrEditor.view) return;
    var tab = pane.tabs[pane.activeTab];
    if (!tab || !tab.path) return;
    editorMode.toggleReading();
  });

  window.__vaultrHotkeys.registerRaw('content-pane-find', function(e, mod) {
    if (!mod || e.shiftKey || e.altKey || e.key.toLowerCase() !== 'f') return;
    var _fContentPane = window.__vaultrContentPane;
    if (!_fContentPane || !_fContentPane.contentPaneOpen) return;
    if (!__vaultrEditor.cmOpenSearchPanel || !__vaultrEditor.view) return;
    e.preventDefault();
    __vaultrEditorOpenFind();
    return true;
  });

  // The tabs dropdown's own open/highlight state lives in its local x-data
  // (content_pane.html's #content-pane-tabs-wrap) rather than on window.__vaultrContentPane —
  // same "small local x-data for a popover" pattern as .content-pane-more-wrap.
  // Alpine.$data bridges into it for these plain-JS hotkey handlers.
  function __vaultrContentPaneTabsMenuScope() {
    var el = document.getElementById('content-pane-tabs-wrap');
    return (el && window.Alpine) ? window.Alpine.$data(el) : null;
  }

  // Mod-T opens/closes the tabs dropdown (mirrors the "switch tabs" meaning
  // Cmd/Ctrl+T carries in most apps).
  window.__vaultrHotkeys.registerRaw('content-pane-tabs-menu-toggle', function(e, mod) {
    if (!mod || e.shiftKey || e.altKey || e.key.toLowerCase() !== 't') return;
    var _pane = window.__vaultrContentPane;
    if (!_pane || !_pane.contentPaneOpen) return;
    var scope = __vaultrContentPaneTabsMenuScope();
    if (!scope) return;
    e.preventDefault();
    scope.toggleTabsMenu();
    if (!scope.tabsMenuOpen) document.dispatchEvent(new CustomEvent('content-pane:close-tabs'));
    return true;
  });

  // While the tabs dropdown is open: ↑/↓ move the highlighted row, Enter
  // switches to it. Registered after content-pane-scroll (below) so it's checked
  // first — see keysJS's reverse-registration-order dispatch — and takes
  // over plain arrow keys instead of letting them scroll the editor.
  window.__vaultrHotkeys.registerRaw('content-pane-tabs-menu-nav', function(e, mod) {
    if (mod || e.shiftKey || e.altKey) return;
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown' && e.key !== 'Enter') return;
    var menu = document.querySelector('.content-pane-tabs-menu');
    if (!menu || menu.style.display === 'none') return;
    var scope = __vaultrContentPaneTabsMenuScope();
    var _pane = window.__vaultrContentPane;
    if (!scope || !scope.tabsMenuOpen || !_pane) return;
    var n = _pane.tabs.length;
    if (!n) return;
    e.preventDefault();
    if (e.key === 'ArrowDown') {
      scope.tabsMenuActiveIndex = (scope.tabsMenuActiveIndex + 1 + n) % n;
    } else if (e.key === 'ArrowUp') {
      scope.tabsMenuActiveIndex = (scope.tabsMenuActiveIndex - 1 + n) % n;
    } else {
      var idx = scope.tabsMenuActiveIndex;
      if (idx >= 0 && idx < n) void _pane.contentPaneSwitchTab(idx);
      scope.tabsMenuOpen = false;
    }
    return true;
  });

  // Intercept mouse back/forward buttons (button 3/4) to switch pane tabs.
  window.addEventListener('mousedown', function(e) {
    var _pane = window.__vaultrContentPane;
    if (!_pane || !_pane.contentPaneOpen || _pane.tabs.length <= 1) return;
    if (e.button === 3) {
      e.preventDefault();
      if (_pane.activeTab > 0) void _pane.contentPaneSwitchTab(_pane.activeTab - 1);
    } else if (e.button === 4) {
      e.preventDefault();
      if (_pane.activeTab < _pane.tabs.length - 1) void _pane.contentPaneSwitchTab(_pane.activeTab + 1);
    }
  }, true);

