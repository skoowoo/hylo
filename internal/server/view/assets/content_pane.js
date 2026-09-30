  // Content-pane chrome: tabs, split, rename, compile, and the inbox share of this shell.
  // The editor session is editor_session.js, concatenated just before this file.
  var __vaultrEditorTabSeq = 0;
  function __vaultrEditorNewTabId() {
    __vaultrEditorTabSeq = (__vaultrEditorTabSeq + 1) % 1000;
    return Date.now() * 1000 + __vaultrEditorTabSeq;
  }

  // Monotonic "when was this tab last active" clock — a plain counter
  // rather than Date.now() since all that ever matters is relative order
  // (who's older), and a counter can't collide within the same tick the
  // way two Date.now() reads sometimes do. Stamped onto tab._lastActiveSeq
  // by _activateTab below.
  var __vaultrEditorTabActivitySeq = 0;

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

  // Vault-relative path -> filename stem (no directory, no extension) — the
  // wikilink target format (internal/util/mdhtml.go's wikilinkRe resolves by
  // stem, not full path), so this can't just reuse tab.title: a knowledge
  // note's title may come from frontmatter and differ from its filename.
  function __vaultrNoteStem(path) {
    var p = String(path || '');
    var slash = p.lastIndexOf('/');
    return __vaultrStripMdExt(slash === -1 ? p : p.slice(slash + 1));
  }

  // ── Content-pane controller factory ────────────────────────────────────────────────
  function contentPaneCtrl() {
    return {
      contentPaneOpen: false, tabs: [], activeTab: -1,
      _skipNextOpenLoad: false, // see _openPane()
      renaming: false, // true while #content-pane-rename-input replaces the active tab's title
      _renameSubmitting: false, // reentrancy guard — see submitRenameActiveNote's leading comment
      _renameCheckTimer: null, // debounce handle for checkRenameNameAvailable's /api/vault/check-name call
      splitRatio: 0.5, isPaneResizing: false, _prevSplitRatio: 0.5,

      // How many header tab chips currently fit. 1 until initTabstrip
      // measures; the row clips, so a short first guess can't cover the
      // toolbar actions.
      tabVisibleCount: 1,
      // tab ids that should paint at opacity 0 on the next render, then
      // fade in. Has to be set in the same turn as the DOM insert.
      tabEntering: {},
      _enteringFrame: 0,
      // tabs' first tabVisibleCount entries are the "visible window", kept
      // in stable (open-order) position by _activateTab below — switching
      // between tabs already on screen never reorders them. The rest
      // (tabs.slice(tabVisibleCount)) is the overflow tail, MRU-ordered
      // (most recently evicted from the window sits first) — same order
      // the tabs-menu dropdown below lists everything in.
      visibleTabs() {
        return this.tabs.slice(0, Math.min(this.tabVisibleCount, this.tabs.length));
      },
      hiddenTabCount() {
        return this.tabs.length - this.visibleTabs().length;
      },
      // ── Header tab strip motion (FLIP) ──────────────────────────────
      // Flexbox has no CSS-native way to animate a reflow — a plain
      // transition can't tween "left" on a flex item, and (contrary to
      // the first instinct) neither does Alpine's own x-transition: that
      // directive only ever plays when something toggles via x-show/x-if,
      // and x-for's insertion path just splices a new node straight into
      // the DOM without going through that machinery — x-transition:enter
      // on an x-for child is silently inert. So both the reposition slide
      // *and* the new-chip pop-in are done by hand here, the same way:
      // grab each chip's position before a mutation (_flipCapture), let
      // Alpine re-render, then after the DOM has caught up
      // (_flipApply) — a chip that was already there gets snapped back to
      // its old spot with an instant transform and transitioned to zero
      // (reads as sliding to its new position); a chip that's new (not in
      // the "before" set at all) starts from scaled-down/transparent and
      // transitions to its resting state instead. Alpine's :key="tab.id"
      // (content_pane.html) keeps the same DOM node for a tab that
      // survives a re-render, so capturing by element reference and
      // re-measuring the *same* elements afterward is enough; a tab
      // that's gone (closed, or pushed into the hidden tail) just isn't
      // found again in the "after" pass and is skipped — no leave
      // animation (see the comment on the x-for template, content_pane.html).
      _tabstripMovers() {
        var row = document.getElementById('content-pane-tabstrip-row');
        if (!row) return [];
        return row.querySelectorAll('.content-pane-tab, .content-pane-tabs-overflow');
      },
      _flipCapture() {
        var rects = new Map();
        this._tabstripMovers().forEach(function(child) {
          if (!child.offsetParent) return; // display:none — no position to keep
          // Clear a transform left mid-flight so this read isn't the
          // in-between x of an earlier slide.
          child.style.transition = 'none';
          child.style.transform = '';
          rects.set(child, child.getBoundingClientRect().left);
        });
        return rects;
      },
      _flipApply(before) {
        if (!before) return;
        var self = this;
        requestAnimationFrame(function() {
          self._tabstripMovers().forEach(function(child) {
            if (!before.has(child)) return;
            var dx = before.get(child) - child.getBoundingClientRect().left;
            if (Math.abs(dx) < 0.5) { child.style.transition = ''; return; }
            child.style.transition = 'none';
            child.style.transform = 'translateX(' + dx + 'px)';
            requestAnimationFrame(function() {
              child.style.transition = 'transform var(--motion-view) var(--ease-out)';
              child.style.transform = '';
              child.addEventListener('transitionend', function te(ev) {
                if (ev.propertyName !== 'transform') return;
                child.style.transition = '';
                child.removeEventListener('transitionend', te);
              });
            });
          });
        });
      },
      _domTabIds() {
        var have = {};
        var row = document.getElementById('content-pane-tabstrip-row');
        if (!row) return have;
        var nodes = row.querySelectorAll('.content-pane-tab[data-tab-id]');
        for (var i = 0; i < nodes.length; i++) have[nodes[i].getAttribute('data-tab-id')] = true;
        return have;
      },
      // Mark chips that will be in the window after this turn's Alpine
      // flush but aren't in the DOM yet. The class is opacity 0 with no
      // extra frame of the chip at full strength.
      _prepareEntering() {
        if (!this.contentPaneOpen) return;
        var have = this._domTabIds();
        var n = Math.min(this.tabVisibleCount, this.tabs.length);
        var ids = [];
        for (var i = 0; i < n; i++) {
          var id = this.tabs[i].id;
          if (!have[String(id)]) ids.push(id);
        }
        if (!ids.length) return;
        var map = {};
        for (var k in this.tabEntering) if (this.tabEntering[k]) map[k] = true;
        for (var j = 0; j < ids.length; j++) map[String(ids[j])] = true;
        this.tabEntering = map;
        var self = this;
        if (self._enteringFrame) cancelAnimationFrame(self._enteringFrame);
        // Two frames: the first rAF can still be before the paint that
        // shows opacity 0, and clearing there would skip the fade.
        self._enteringFrame = requestAnimationFrame(function() {
          self._enteringFrame = requestAnimationFrame(function() {
            self._enteringFrame = 0;
            self.tabEntering = {};
          });
        });
      },
      // After a promotion, the incoming chip can be wider than the one it
      // replaced. Recompute until the active tab sits inside a prefix that
      // actually fits. Widths are cached, so the extra passes are arithmetic.
      _refitAroundActive() {
        for (var pass = 0; pass < 10; pass++) {
          var fitted = this._computeFit();
          if (!fitted) return;
          if (this.activeTab < fitted && fitted === this.tabVisibleCount) return;
          var active = this.tabs[this.activeTab];
          if (!active) return;
          this.tabVisibleCount = fitted;
          if (this.activeTab < fitted) return;
          this.tabs.splice(this.activeTab, 1);
          this.tabs.splice(fitted - 1, 0, active);
          this.activeTab = fitted - 1;
        }
      },
      // Count (+ active-tab placement) only. Callers that want a slide
      // capture before this and _flipApply after.
      _syncFit() {
        var fitted = this._computeFit();
        if (!fitted) return;
        this.tabVisibleCount = fitted;
        if (this.activeTab >= fitted && this.tabs[this.activeTab]) this._refitAroundActive();
      },
      // Width changed. A pure count change doesn't move the chips that
      // stay, so it doesn't slide — new chips fade via _prepareEntering.
      // A shrink that drops the active tab past the window does reorder,
      // and that one slide is the motion.
      _settleTabstrip(fade) {
        var fitted = this._computeFit();
        if (!fitted) return;
        var reorder = this.activeTab >= fitted && !!this.tabs[this.activeTab];
        if (fitted === this.tabVisibleCount && !reorder) return;
        var before = (fade && reorder) ? this._flipCapture() : null;
        this.tabVisibleCount = fitted;
        if (reorder) this._refitAroundActive();
        if (fade) this._prepareEntering();
        if (before) this._flipApply(before);
      },
      // Chip widths are a function of the title, measured on a hidden
      // probe (same classes, out of the row) and cached. __vaultrFitTabCount
      // then picks the count in one shot — the live row is never used as
      // a probe, so opening and resizing don't paint intermediate counts.
      _computeFit() { return 0; },
      initTabstrip(el) {
        var self = this;
        function px(name, fallback) {
          var v = parseFloat(getComputedStyle(el).getPropertyValue(name));
          return isNaN(v) ? fallback : v;
        }
        var chip = document.createElement('div');
        chip.className = 'content-pane-tab content-pane-tab-measure';
        chip.setAttribute('aria-hidden', 'true');
        chip.innerHTML = '<span class="content-pane-tab-draft-icon"></span><span class="content-pane-tab-status-slot"></span><span class="content-pane-tab-title"></span><button type="button" class="content-pane-tab-close" tabindex="-1"></button>';
        var ov = document.createElement('button');
        ov.type = 'button';
        ov.tabIndex = -1;
        ov.className = 'content-pane-tabs-overflow content-pane-tab-measure';
        ov.setAttribute('aria-hidden', 'true');
        ov.innerHTML = '<span></span><svg viewBox="0 0 24 24"></svg>';
        el.appendChild(chip);
        el.appendChild(ov);
        var titleEl = chip.querySelector('.content-pane-tab-title');
        var draftEl = chip.querySelector('.content-pane-tab-draft-icon');
        var ovLabel = ov.querySelector('span');
        var ovCache = {};
        function overflowWidth(hidden) {
          var label = '+' + hidden;
          if (ovCache[label]) return ovCache[label];
          ovLabel.textContent = label;
          ovCache[label] = ov.offsetWidth;
          return ovCache[label];
        }
        function measure(tab) {
          var key = (tab.path ? '1' : '0') + '\n' + (tab.title || '');
          if (tab._chipWKey === key && tab._chipW) return tab._chipW;
          titleEl.textContent = tab.title || '';
          draftEl.style.display = tab.path ? 'none' : '';
          var w = chip.offsetWidth;
          tab._chipW = w;
          tab._chipWKey = key;
          return w;
        }
        this._computeFit = function() {
          if (el.clientWidth <= 0 || !self.tabs.length) return 0;
          var widths = [];
          for (var i = 0; i < self.tabs.length; i++) widths.push(measure(self.tabs[i]));
          return __vaultrFitTabCount(widths, el.clientWidth, px('--cp-tab-gap', 4), overflowWidth);
        };
        function settleFromResize(reopening) {
          // Reopening fades with the pane itself. Dragging the split is
          // already a continuous width change — fading each new chip on
          // top of that just lags the row.
          self._settleTabstrip(!reopening && !self.isPaneResizing);
        }
        if (el.clientWidth > 0) settleFromResize(false);
        if (typeof ResizeObserver === 'undefined') return;
        var lastW = el.clientWidth;
        var ro = new ResizeObserver(function() {
          var w = el.clientWidth;
          if (Math.abs(w - lastW) < 1) return;
          // Closed pane is 0 wide, so every open (not just the first)
          // arrives as 0 → width. Settle in this callback: ResizeObserver
          // flushes microtasks before paint, so the corrected count lands
          // before the pane's fade is visible.
          var reopening = lastW === 0 && w > 0;
          lastW = w;
          settleFromResize(reopening);
        });
        ro.observe(el);
        if (document.fonts && document.fonts.ready) {
          document.fonts.ready.then(function() {
            ovCache = {};
            for (var i = 0; i < self.tabs.length; i++) self.tabs[i]._chipWKey = '';
            self._settleTabstrip(false);
          });
        }
      },

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
          var botMenu = document.querySelector('.content-pane-bot-menu');
          if (botMenu && botMenu.style.display !== 'none') {
            document.dispatchEvent(new CustomEvent('content-pane:close-bot'));
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
            if (_s.view && __vaultrCM) __vaultrEditorCloseSearch(_s.view);
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
            // Same note, unsaved edits: the doc is already showing, so skip
            // the synchronous write. 250ms matches this open path's 240ms slide.
            __vaultrEditorRestoreScroll(savedState.scrollTop || 0, { frames: 1, afterMs: 250, focus: true });
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
        var idx = this.tabs.length - 1;

        if (this.tabs.length > CONTENT_PANE_MAX_TABS) {
          var oldestIdx = __vaultrOldestEvictableTabIdx(this.tabs, this.activeTab);
          if (oldestIdx >= 0) {
            var evictedTab = this.tabs[oldestIdx];
            __vaultrEditorClearTabState(evictedTab.id);
            this.tabs.splice(oldestIdx,1);
            if (oldestIdx < idx) idx--;
          }
        }
        this._activateTab(idx);
        this._openPane();

        s.currentPath = ''; s.currentMd = ''; s.dirty = false;
        await __vaultrEditorApplyState({ inSource: false, scrollTop: 0 }, newTab.id);
      },

      // Makes tabs[idx] the active tab. A tab already inside the visible
      // window (idx < tabVisibleCount) just gets the pointer moved to it —
      // its position, and every other visible tab's, is left alone, so
      // switching between tabs you can already see never reshuffles the
      // header. A tab arriving from the hidden/overflow tail has to make
      // room instead: it evicts whichever visible tab has gone longest
      // without being active (_lastActiveSeq), drops that one to the
      // front of the hidden tail (now the most-recently-relevant thing
      // hidden), and joins the visible window at its far end — the same
      // spot a brand-new tab lands (see upsertTab/openNewInContentPane).
      //
      // _flipCapture/_flipApply wrap every path here — except while the
      // pane itself is still closed (this.contentPaneOpen is only flipped
      // true *after* openNoteInContentPane's upsertTab/_activateTab call,
      // so that's a reliable "this activation is part of opening the
      // editor, not a live interaction" signal). tabVisibleCount hasn't
      // been measured against real layout yet at that point (the pane has
      // zero width until it opens — see initTabstrip), so any eviction
      // decision made here would be off a guess anyway, and animating it
      // would just fight the panel's own open transition instead of
      // reading as one motion. Once open, every path animates
      // unconditionally, including the idx<cap branch — a brand-new tab
      // can land there too (upsertTab pushes it, and if the window wasn't
      // already full there's nothing to evict), and that's still a chip
      // appearing for the first time, same as any other. A plain switch
      // between two tabs that were already visible just measures the same
      // positions twice and finds nothing to animate — cheap enough at
      // ≤10 tabs not to bother special-casing it away.
      // animateOverride false skips the slide — used while the pane is
      // still closed (contentPaneOpen flips true only after this returns
      // on the open path) and whenever a caller already captured its own
      // before-positions.
      _activateTab(idx, animateOverride) {
        var tab = this.tabs[idx];
        if (!tab) return;
        var animate = animateOverride !== undefined ? animateOverride : this.contentPaneOpen;
        var before = animate ? this._flipCapture() : null;
        tab._lastActiveSeq = ++__vaultrEditorTabActivitySeq;
        // No real width yet (pane still closed): don't evict off the
        // placeholder count. The open resize settles the window for real.
        var measured = this._computeFit();
        if (!(measured > 0)) {
          this.activeTab = idx;
        } else if (idx < measured) {
          this.tabVisibleCount = measured;
          this.activeTab = idx;
        } else {
          this.tabVisibleCount = measured;
          this.tabs.splice(idx, 1);
          var cap = measured;
          var evictIdx = 0, evictSeq = Infinity;
          for (var j = 0; j < cap && j < this.tabs.length; j++) {
            var seq = this.tabs[j]._lastActiveSeq || 0;
            if (seq < evictSeq) { evictSeq = seq; evictIdx = j; }
          }
          var evicted = this.tabs.splice(evictIdx, 1)[0];
          this.tabs.splice(cap - 1, 0, evicted);
          this.tabs.splice(cap - 1, 0, tab);
          this.activeTab = cap - 1;
          this._refitAroundActive();
        }
        if (animate) {
          this._prepareEntering();
          this._flipApply(before);
        }
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
        } else {
          this.tabs.push({id:__vaultrEditorNewTabId(), title:title||'Note', path:path, isKnowledge:!!isKnowledge, pinned:!!pinned, isIndex:!!isIndex, canCompile:!!canCompile});
          idx = this.tabs.length - 1;
          if (this.tabs.length > CONTENT_PANE_MAX_TABS) {
            var oldestIdx = __vaultrOldestEvictableTabIdx(this.tabs, this.activeTab);
            if (oldestIdx >= 0) {
              var evicted = this.tabs[oldestIdx];
              __vaultrEditorClearTabState(evicted.id);
              this.tabs.splice(oldestIdx,1);
              if (oldestIdx < idx) idx--;
            }
          }
        }
        this._activateTab(idx);
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
        this._activateTab(i);
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

        // Captured before the splice below — if closing this tab shifts or
        // backfills any visible chip, the survivors slide into their new
        // spots instead of snapping (_flipApply at the bottom). Skipped
        // outright when the pane's about to close entirely (nothing left
        // to animate).
        var before = this.tabs.length > 1 ? this._flipCapture() : null;

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
          // Whichever tab now sits where the closed one was (or the last
          // one, if it was rightmost) inherits activation — same "next tab
          // over" convention browsers use on tab close. It was already
          // visible (closing a window member always backfills the window
          // from the hidden tail's MRU head — see _activateTab's comment),
          // so no eviction dance here, just the recency stamp so it isn't
          // the very next thing evicted for having gone "longest" untouched.
          this.activeTab = Math.min(i, this.tabs.length-1);
          var stamped = this.tabs[this.activeTab];
          if (stamped) stamped._lastActiveSeq = ++__vaultrEditorTabActivitySeq;
        }
        // Fit before loading: a wider backfill can shrink the window and
        // move which tab is active.
        this._syncFit();
        if (wasActive) {
          var t = this.tabs[this.activeTab];
          if (!t) return;
          var savedState = __vaultrEditorRestoreTabState(t.id);
          if (t.path) {
            void __vaultrContentPaneLoadNote(t.path, t.id, savedState);
          } else {
            void this._openUntitledTab(t, savedState);
          }
        }
        if (this.contentPaneOpen) this._prepareEntering();
        this._flipApply(before);
      },

      // Opens the given mate's chat panel with a brand-new conversation
      // (not its most recent one — this is a one-shot "hand this note to a
      // bot", distinct from the sidebar's tryOpen which deliberately resumes
      // where that bot's chat left off) and drops a `[[stem]]` wikilink for
      // the active note into its composer. home.js is what actually owns
      // the chat panel/agentBots list, so this only dispatches an event
      // (mirrors search_overlay.go's vaultr:reference-note) rather than
      // reaching into window._homeData's chat internals directly.
      sendActiveNoteToAgentBot(mateId) {
        var tab = this.tabs[this.activeTab];
        if (!tab || !tab.path) return;
        var stem = __vaultrNoteStem(tab.path);
        window.dispatchEvent(new CustomEvent('vaultr:send-note-to-agent', { detail: { mateId: mateId, stem: stem } }));
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
        var beforeDelete = this.tabs.length > 1 ? this._flipCapture() : null;
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
          this._syncFit();
          if (this.contentPaneOpen) this._prepareEntering();
          this._flipApply(beforeDelete);
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
        tab._chipWKey = '';
        if (__vaultrEditor.currentPath === oldPath) __vaultrEditor.currentPath = newPath;
        this._settleTabstrip(this.contentPaneOpen);
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
    if (s.view && __vaultrCM) __vaultrCM.cmUndo(s.view);
  };
  window.__vaultrRedo = function() {
    var s = __vaultrEditor;
    if (s.view && __vaultrCM) __vaultrCM.cmRedo(s.view);
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
    if (!__vaultrCM || !s.view) return;
    // Cancel a still-pending animated close and snap any fading-out panel
    // back to visible first — otherwise that stale timer would go on to
    // close the panel this call is about to (re)open.
    if (s._searchCloseTimer) { clearTimeout(s._searchCloseTimer); s._searchCloseTimer = null; }
    var panel = document.querySelector('#content-pane-edit-area .cm-panels-top');
    if (panel) panel.classList.remove('is-closing');
    __vaultrCM.openSearchPanel(s.view);
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
    if (!__vaultrCM || !__vaultrEditor.view) return;
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

