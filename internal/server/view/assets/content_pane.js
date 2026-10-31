  // Content-pane chrome: tabs, split, rename, compile.
  // The editor session is editor_session.js, concatenated just before this file.
  var __hyloEditorTabSeq = 0;
  function __hyloEditorNewTabId() {
    __hyloEditorTabSeq = (__hyloEditorTabSeq + 1) % 1000;
    return Date.now() * 1000 + __hyloEditorTabSeq;
  }

  var CONTENT_PANE_MAX_TABS = 10;
  // Local reads land well inside this; it only caps a slow server.
  var CONTENT_PANE_OPEN_WAIT_MS = 120;
  // Oldest evictable tab index, or -1. Skips the active tab and any
  // unmaterialized tab that still has real content (_pendingContent) — an
  // empty one is free to evict, or it'd sit protected forever and push out
  // real notes instead.
  function __hyloOldestEvictableTabIdx(tabs, activeTab) {
    var oldestIdx = -1, oldestId = Infinity;
    for (var j = 0; j < tabs.length; j++) {
      if (j !== activeTab && !tabs[j]._pendingContent && tabs[j].id < oldestId) { oldestIdx = j; oldestId = tabs[j].id; }
    }
    return oldestIdx;
  }

  // ── Global note-card helper ───────────────────────────────────────────────────
  function __hyloOpenNote(el) {
    var d = el && el.dataset; if (!d || !d.notePath) return;
    var pane = window.__hyloContentPane;
    if (pane) void pane.openNoteInContentPane(d.notePath, d.noteTitle || 'Note',
      d.noteIsKnowledge === 'true', d.notePinned === 'true', d.noteIsIndex === 'true',
      d.noteCanCompile === 'true');
  }

  // Note actions by path, shared by the editor and every note card; open tabs
  // and lists then follow the live feed.
  async function __hyloNoteActionFailed(resp, title) {
    var text = ''; try { text = (await resp.text()).trim(); } catch (_) {}
    window.showError(text || 'Server error (' + resp.status + ').', title);
  }

  async function __hyloSetNotePinned(path, pinned) {
    var resp = await fetch('/api/vault/pin', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path:path,pinned:!!pinned})});
    if (!resp.ok) { await __hyloNoteActionFailed(resp, pinned ? 'Could not pin' : 'Could not unpin'); return false; }
    if (window.__hyloContentPane) window.__hyloContentPane.notePinned(path, !!pinned);
    return true;
  }

  async function __hyloDeleteNote(path, title) {
    var confirmed = await window.showConfirm({titleHTML:'<span class="confirm-title-icon">' + hyloIcon('trash', 'width="13" height="13"') + '</span>Delete note',message:'"'+title+'" will be permanently deleted.',confirmLabel:'Delete',danger:true});
    if (!confirmed) return false;
    var resp = await fetch('/api/vault/delete', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path:path})});
    if (!resp.ok) { await __hyloNoteActionFailed(resp, 'Could not delete'); return false; }
    if (window.__hyloContentPane) window.__hyloContentPane.noteDeleted(path);
    return true;
  }

  function __hyloNoteCardMenu(btn, e) {
    var d = btn.closest('.note-card').dataset;
    var path = d.notePath, title = d.noteTitle, pinned = d.notePinned === 'true';
    __hyloCardMenu.toggle(btn, [
      { icon: 'bookmark', label: pinned ? 'Unpin' : 'Pin', run: function() { __hyloSetNotePinned(path, !pinned); } },
      'divider',
      { icon: 'trash', label: 'Delete', danger: true, run: function() { __hyloDeleteNote(path, title); } },
    ], e);
  }

  // Strips a trailing .md/.markdown (case-insensitive) — mirrors the server's
  // accepted extensions (internal/util/markdown.go's markdownExts), so the
  // rename box's "bare name" and its no-op check agree with what actually
  // counts as a name change.
  function __hyloStripMdExt(name) {
    return (name || '').replace(/\.(md|markdown)$/i, '');
  }

  // "/a/b/Note.md" -> "a/b"; "" at the vault root.
  function __hyloNoteDir(path) {
    var p = String(path || '').replace(/^\/+/, '');
    var slash = p.lastIndexOf('/');
    return slash > 0 ? p.slice(0, slash) : '';
  }

  // Vault-relative path -> filename stem (no directory, no extension) — the
  // wikilink target format (internal/util/mdhtml.go's wikilinkRe resolves by
  // stem, not full path), so this can't just reuse tab.title: a knowledge
  // note's title may come from frontmatter and differ from its filename.
  function __hyloNoteStem(path) {
    var p = String(path || '');
    var slash = p.lastIndexOf('/');
    return __hyloStripMdExt(slash === -1 ? p : p.slice(slash + 1));
  }

  // ── Content-pane controller factory ────────────────────────────────────────────────
  function contentPaneCtrl() {
    return {
      contentPaneOpen: false, tabs: [], activeTab: -1,
      // The agent is typing into the open note: no switching, closing or toolbar actions.
      agentWriting: false,
      renaming: false, // true while #content-pane-rename-input replaces the active tab's title
      _renameSubmitting: false, // reentrancy guard — see submitRenameActiveNote's leading comment
      _renameCheckTimer: null, // debounce handle for checkRenameNameAvailable's /api/vault/check-name call
      splitRatio: 0.5, isPaneResizing: false, _prevSplitRatio: 0.5,
      // What the shell lays out (home.html): the open state and ratio above,
      // committed a frame after the list has prepared for the move.
      layoutOpen: false, layoutRatio: 0.5,

      // Maximized is just splitRatio 0: the sidebar stays.
      toggleMaximizeEditor() {
        if (this.splitRatio > 0.02) {
          this._prevSplitRatio = this.splitRatio;
          this.splitRatio = 0;
        } else {
          this.splitRatio = this._prevSplitRatio || 0.5;
        }
      },

      revealListPane() {
        if (this.contentPaneOpen && this.splitRatio <= 0.02) this.toggleMaximizeEditor();
      },

      // Drags the divider. Ratio is the list's share of .home-main; 0 is
      // maximized, and the editor is capped so it never vanishes while open.
      // The vars go straight onto the shell once per frame and splitRatio
      // syncs on mouseup, so Alpine doesn't re-run :style per mousemove. The
      // capture layer keeps a fast drag over CodeMirror from selecting text.
      startPaneResize(e) {
        e.preventDefault();
        var self = this;
        // The capture layer below takes the second click, so dblclick never reaches the handle.
        if (e.detail === 2) {
          this.splitRatio = 0.5;
          try { localStorage.setItem('hylo.splitRatio', '0.5'); } catch(_) {}
          return;
        }
        var shell = document.querySelector('.home-shell');
        var main = shell && shell.querySelector('.home-main');
        if (!main) return;
        var mainRect = main.getBoundingClientRect();
        var avail = mainRect.width;
        if (avail <= 0) return;

        self.isPaneResizing = true;
        if (self._beginSplitMotion) self._beginSplitMotion();
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
          var ratio = (ev.clientX - mainRect.left) / avail;
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
          var moved = latestRatio !== self.splitRatio;
          self.isPaneResizing = false;
          self.splitRatio = latestRatio;
          // A moved ratio settles through the watcher's commit; an unmoved one has nothing to wait for.
          if (!moved && self._settleSplit) self._settleSplit();
          try { localStorage.setItem('hylo.splitRatio', String(latestRatio)); } catch(_) {}
        }
        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
      },

      // Per vault: switching vaults keeps the origin, and the old tabs would
      // point at notes the new vault doesn't have.
      _storeKey: 'hylo.content-pane',

      _persist() {
        try {
          localStorage.setItem(this._storeKey, JSON.stringify({
            // A not-yet-materialized tab (no path) has nothing worth
            // persisting across a reload: if it's still empty, dropping it
            // is a no-op; if the user typed something,
            // __hyloEditorMaterializeTab already gave it a real path
            // before this next persist call fires.
            tabs: this.tabs.filter(function(t){ return !!t.path; }).map(function(t){
              return {id:t.id, title:t.title, path:t.path, isKnowledge:!!t.isKnowledge, pinned:!!t.pinned, isIndex:!!t.isIndex, canCompile:!!t.canCompile};
            }),
            // By id: drafts are left out above, so an index would point at the wrong tab.
            activeId: this.tabs[this.activeTab] ? this.tabs[this.activeTab].id : null,
            activeTab: this.activeTab,
          }));
        } catch(_) {}
      },

      // The sidebar lists every tab: changes to the list slide its rows (home.js).
      _sideReflow() {
        if (!this._sideLive || typeof __hyloSideFlipCapture !== 'function') return;
        var snap = __hyloSideFlipCapture();
        this.$nextTick(function() { __hyloSideFlipPlay(snap); });
      },

      _restore() {
        try {
          var raw = localStorage.getItem(this._storeKey); if (!raw) return;
          var data = JSON.parse(raw);
          if (!data || !Array.isArray(data.tabs) || !data.tabs.length) return;
          var tabs = data.tabs;
          // Drafts live only in memory; keep this view's where they were, the store never had them.
          var drafts = [];
          this.tabs.forEach(function(t, i){ if (!t.path) drafts.push({ at: i, tab: t }); });
          var cap = Math.max(CONTENT_PANE_MAX_TABS - drafts.length, 0);
          if (tabs.length > cap) {
            var byAge = tabs.slice().sort(function(a,b){return a.id-b.id;});
            var remove = new Set(byAge.slice(0, tabs.length-cap).map(function(t){return t.id;}));
            tabs = tabs.filter(function(t){return !remove.has(t.id);});
          }
          this._sideReflow();
          this.tabs = tabs.map(function(t) {
            var path = t.path || '';
            if (!path && t.fragmentUrl) { try { path = new URL(t.fragmentUrl, location.origin).searchParams.get('path') || ''; } catch(_){} }
            if (!path && t.pageUrl)     { try { path = new URL(t.pageUrl,     location.origin).searchParams.get('path') || ''; } catch(_){} }
            return {id:t.id, title:t.title||'Note', path:path, isKnowledge:!!t.isKnowledge, pinned:!!t.pinned, isIndex:!!t.isIndex, canCompile:!!t.canCompile};
          }).filter(function(t){ return !!t.path; });
          var merged = this.tabs;
          drafts.forEach(function(d){ merged.splice(Math.min(d.at, merged.length), 0, d.tab); });
          if (!this.tabs.length) return;
          var at = this.tabs.findIndex(function(t){ return t.id === data.activeId; });
          this.activeTab = at >= 0 ? at : Math.min(Math.max(data.activeTab||0, 0), this.tabs.length-1);
        } catch(_) {}
      },

      initContentPane() {
        window.__hyloContentPane = this;
        try { this._storeKey += ':' + JSON.parse(document.getElementById('home-boot').textContent).vault; } catch(_) {}
        this._restore();
        this._sideLive = true;
        try {
          var savedRatio = parseFloat(localStorage.getItem('hylo.splitRatio'));
          if (!isNaN(savedRatio) && savedRatio >= 0 && savedRatio <= 0.9) this.splitRatio = savedRatio;
        } catch(_) {}
        this.layoutRatio = this.splitRatio;

        var self = this;
        // Esc still closes whatever's layered on top of the editor (the
        // more/bot dropdowns, CodeMirror's search panel) — it just no
        // longer closes the editor itself once those are all closed.
        var _paneEscClose = function() {
          var moreMenu = document.querySelector('.content-pane-more-menu');
          if (moreMenu && moreMenu.style.display !== 'none') {
            document.dispatchEvent(new CustomEvent('content-pane:close-more'));
            __hyloBlurActive();
            return;
          }
          var botMenu = document.querySelector('.content-pane-bot-menu');
          if (botMenu && botMenu.style.display !== 'none') {
            document.dispatchEvent(new CustomEvent('content-pane:close-bot'));
            __hyloBlurActive();
            return;
          }
          if (document.querySelector('.hylo-search-panel')) {
            var _s = __hyloEditor;
            if (_s.view && __hyloCM) __hyloCM.closeSearchPanel(_s.view);
            return;
          }
        };
        // Built while idle, so the first open doesn't import CodeMirror mid-slide.
        var warmEditor = function() { void __hyloEnsureContentPaneEditor(); };
        if (window.requestIdleCallback) requestIdleCallback(warmEditor, { timeout: 2000 });
        else setTimeout(warmEditor, 300);
        // Every split change is one width transition on .content-pane
        // (home.css), or a drag. As the state changes, is-pane-moving goes on
        // and hylo:split-moving lets the list prepare (with its width at both
        // ends); the layout follows in the next frame, so the transition's
        // clock starts after that frame is painted, however long it took.
        // hylo:split-settled marks the end.
        var paneEl = document.querySelector('.content-pane');
        var shellEl = document.querySelector('.home-shell');
        if (paneEl && shellEl) {
          // getAnimations() flushes style, so a transition just started counts.
          this._settleSplit = function() {
            if (self.isPaneResizing || paneEl.getAnimations().length || !shellEl.classList.contains('is-pane-moving')) return;
            shellEl.classList.remove('is-pane-moving');
            window.dispatchEvent(new CustomEvent('hylo:split-settled'));
          };
          this._beginSplitMotion = function(detail) {
            shellEl.classList.add('is-pane-moving');
            window.dispatchEvent(new CustomEvent('hylo:split-moving', { detail: detail || {} }));
          };
          var commitLayout = function() {
            self.layoutOpen = self.contentPaneOpen;
            self.layoutRatio = self.splitRatio;
            // Settles now if the width didn't change (a drag's commit); otherwise transitionend does.
            self.$nextTick(self._settleSplit);
          };
          this.$watch(function() { return self.contentPaneOpen + ':' + self.splitRatio; }, function() {
            if (self.isPaneResizing) return;
            var list = document.getElementById('home-list-pane');
            var mainW = list ? list.parentElement.clientWidth : 0;
            self._beginSplitMotion(list ? {
              listFrom: list.getBoundingClientRect().width,
              listTo: self.contentPaneOpen ? mainW * self.splitRatio : mainW,
            } : null);
            requestAnimationFrame(commitLayout);
          });
          paneEl.addEventListener('transitionend', function(e) { if (e.target === paneEl) self._settleSplit(); });
          paneEl.addEventListener('transitioncancel', function(e) { if (e.target === paneEl) self._settleSplit(); });
        }
        this.$watch('contentPaneOpen', function(isOpen) {
          if (!isOpen) {
            if (window.__hyloEscPop) window.__hyloEscPop('content-pane');
            // Save state BEFORE blur — blur can trigger scrollIntoView which resets scrollTop
            var currentTab = self.tabs[self.activeTab];
            if (currentTab) __hyloEditorSaveTabState(currentTab.id);
            __hyloBlurActive();
            return;
          }
          if (window.__hyloEscPush) window.__hyloEscPush('content-pane', _paneEscClose, document.querySelector('.content-pane'));
          if (self.activeTab >= 0 && !__hyloFocusInsideEditor()) __hyloFocusEditor();
        });
        this.$watch('tabs', function(){ self._persist(); });
        this.$watch('activeTab', function(i) { self._persist(); });
        // Watches the note itself, not the index: closing, renaming or first
        // saving a tab changes it while activeTab stays put.
        this.$watch(function() {
          var t = self.contentPaneOpen ? self.tabs[self.activeTab] : null;
          return t && t.path ? t.path + '\n' + (t.title || '') : '';
        }, function() { window.dispatchEvent(new CustomEvent('hylo:active-note')); });

        // Sync pane state from other section views (each section is a separate
        // WebContentsView with its own JS context; storage events cross view boundaries).
        window.addEventListener('storage', function(e) {
          if (e.key !== self._storeKey || !e.newValue || self.contentPaneOpen) return;
          self._restore();
        });
        // Notes change from anywhere (CLI, bots, disk, other views), not just
        // this pane's own actions, so tabs follow the feed.
        if (window.__hyloLive) __hyloLive.on('notes', function(changes) {
          changes.forEach(function(c) {
            if (c.op === 'rename') self.noteRenamed(c.oldPath, c.path);
            else if (c.op === 'delete') self.noteDeleted(c.path);
            else if (c.op === 'pin' && c.data) self.notePinned(c.path, !!c.data.pinned);
          });
        });
      },

      // Open an existing note in the pane.
      async openNoteInContentPane(path, title, isKnowledge, pinned, isIndex, canCompile) {
        if (!path || this.agentWriting) return;
        this.cancelRenameActiveNote(); // opening any note (even re-opening the active one) always discards an in-progress rename on whatever tab was showing
        var prevTab = this.tabs[this.activeTab];
        if (this.contentPaneOpen && prevTab) __hyloEditorSaveTabState(prevTab.id);
        this.upsertTab(path, title, isKnowledge, pinned, isIndex, canCompile);
        __hyloEditorResetCompileBtn();
        // Already loaded with unsaved edits — don't clobber with a server fetch;
        // a heading asked for is still where it goes.
        if (__hyloEditor.currentPath === path && __hyloEditor.dirty) {
          this.contentPaneOpen = true;
          var heading = __hyloEditor.pendingHeading;
          __hyloEditor.pendingHeading = null;
          if (heading) __hyloCM.jumpToHeading(__hyloEditor.view, heading);
          return;
        }
        var tab = this.tabs[this.activeTab];
        var savedState = tab ? __hyloEditorRestoreTabState(tab.id) : null;
        await this._openWith(__hyloContentPaneLoadNote(path, tab ? tab.id : null, savedState));
      },

      // Open the pane on a brand-new, empty tab. Nothing is created
      // server-side yet — the first real edit materializes it (see
      // __hyloEditorMaterializeTab) with an auto-generated name; there's
      // no filename to ask for up front and no draft/publish step.
      async openNewInContentPane() {
        if (this.agentWriting) return;
        this.cancelRenameActiveNote(); // see openNoteInContentPane
        var s = __hyloEditor;
        var prevTab = this.tabs[this.activeTab];
        if (this.contentPaneOpen && prevTab) __hyloEditorSaveTabState(prevTab.id);
        if (s.dirty && s.currentPath) { clearTimeout(s.saveTimer); s.saveTimer = null; await __hyloEditorDoSave(); }
        else { clearTimeout(s.saveTimer); s.saveTimer = null; }
        __hyloEditorSaveStatus('');

        var newTab = {id:__hyloEditorNewTabId(), title:'Untitled', path:'', isKnowledge:false, pinned:false};
        this._sideReflow();
        this.tabs.push(newTab);
        var idx = this.tabs.length - 1;

        if (this.tabs.length > CONTENT_PANE_MAX_TABS) {
          var oldestIdx = __hyloOldestEvictableTabIdx(this.tabs, this.activeTab);
          if (oldestIdx >= 0) {
            var evictedTab = this.tabs[oldestIdx];
            __hyloEditorClearTabState(evictedTab.id);
            this.tabs.splice(oldestIdx,1);
            if (oldestIdx < idx) idx--;
          }
        }
        this.activeTab = idx;

        s.currentPath = ''; s.currentMd = ''; s.dirty = false;
        await this._openWith(__hyloEditorApplyState({ inSource: false }, newTab.id));
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
          this._sideReflow();
          this.tabs.push({id:__hyloEditorNewTabId(), title:title||'Note', path:path, isKnowledge:!!isKnowledge, pinned:!!pinned, isIndex:!!isIndex, canCompile:!!canCompile});
          idx = this.tabs.length - 1;
          if (this.tabs.length > CONTENT_PANE_MAX_TABS) {
            var oldestIdx = __hyloOldestEvictableTabIdx(this.tabs, this.activeTab);
            if (oldestIdx >= 0) {
              var evicted = this.tabs[oldestIdx];
              __hyloEditorClearTabState(evicted.id);
              this.tabs.splice(oldestIdx,1);
              if (oldestIdx < idx) idx--;
            }
          }
        }
        this.activeTab = idx;
      },

      closeContentPane() {
        if (!this.agentWriting) this.contentPaneOpen = false;
      },

      // The closed editor is still laid out at its open width, so a note
      // loaded first slides in already painted. A slow load doesn't hold the
      // slide past CONTENT_PANE_OPEN_WAIT_MS; it lands in the open pane instead.
      async _openWith(load) {
        if (this.contentPaneOpen) return load;
        var waited = new Promise(function(r) { setTimeout(r, CONTENT_PANE_OPEN_WAIT_MS); });
        // A failed load still opens, as before; the caller sees the failure.
        await Promise.race([load, waited]).catch(function() {});
        this.contentPaneOpen = true;
        return load;
      },

      // Reopen on the active tab, or on tabId. A note other than the one the
      // editor holds loads first; the one it holds slides in at once and
      // re-reads behind it.
      async showContentPane(tabId) {
        if (this.contentPaneOpen) return;
        // Other views of this vault may have changed the tabs since.
        this._restore();
        var picked = this.tabs.findIndex(function(t) { return t.id === tabId; });
        if (picked >= 0) this.activeTab = picked;
        var tab = this.tabs[this.activeTab];
        if (!tab) { this.contentPaneOpen = true; return; }
        var savedState = __hyloEditorRestoreTabState(tab.id);
        if (!tab.path) return this._openWith(this._openUntitledTab(tab, savedState));
        if (__hyloEditor.currentPath === tab.path) {
          if (savedState) __hyloEditorRestoreScroll(savedState);
          this.contentPaneOpen = true;
          if (!__hyloEditor.dirty) void __hyloContentPaneRefreshNote(tab.path, tab.id);
          return;
        }
        return this._openWith(__hyloContentPaneLoadNote(tab.path, tab.id, savedState));
      },

      markTabCompiled(path) {
        for (var i = 0; i < this.tabs.length; i++) {
          if (this.tabs[i].path === path) { this.tabs[i].canCompile = false; return; }
        }
      },

      // Re-open a not-yet-materialized tab: nothing to load from the server
      // (it doesn't exist there yet) — just restore whatever was typed
      // before the user switched away (see __hyloEditorHandleContentChange's
      // tab._pendingContent) and focus the editor.
      async _openUntitledTab(tab, savedState) {
        await __hyloContentPaneSetContent(tab._pendingContent || '', tab.id, savedState);
      },

      // A sidebar row works with the pane closed too: it opens on that tab.
      selectTab(i) {
        var tab = this.tabs[i];
        if (!tab || this.agentWriting) return;
        if (this.contentPaneOpen) return this.contentPaneSwitchTab(i);
        return this.showContentPane(tab.id);
      },

      activeTabDir() {
        var tab = this.tabs[this.activeTab];
        return tab ? __hyloNoteDir(tab.path) : '';
      },

      // Parent folder, shown only when another open tab has the same title.
      tabHint(tab) {
        if (!tab.path || !this.tabs.some(function(t) { return t !== tab && t.title === tab.title; })) return '';
        return __hyloNoteDir(tab.path).split('/').pop() || '/';
      },

      async contentPaneSwitchTab(i) {
        if (this.agentWriting) return;
        this.cancelRenameActiveNote(); // see openNoteInContentPane
        // A sidebar row keeps no focus. The editor keeps its own: regaining it would make
        // WebKit scroll to the caret (cm-live/focus.js).
        if (!__hyloFocusInsideEditor()) __hyloBlurActive();
        if (i === this.activeTab) return;

        var prevTab = this.tabs[this.activeTab];
        var nextTab = this.tabs[i];
        if (!nextTab) return;

        // Save current tab's state
        if (prevTab) {
          __hyloEditorSaveTabState(prevTab.id);
        }

        // Switch active tab
        this.activeTab = i;
        __hyloEditorResetCompileBtn();

        // Load next tab's content with saved state
        var savedState = __hyloEditorRestoreTabState(nextTab.id);

        if (!nextTab.path) {
          await this._openUntitledTab(nextTab, savedState);
        } else {
          await __hyloContentPaneLoadNote(nextTab.path, nextTab.id, savedState);
        }
      },

      async contentPaneCloseTab(i) {
        if (i < 0 || i >= this.tabs.length || this.agentWriting) return;
        this.cancelRenameActiveNote(); // see openNoteInContentPane
        var wasActive = (i === this.activeTab);
        var closingTab = this.tabs[i];
        if (wasActive && closingTab.path && __hyloEditor.dirty && __hyloEditor.currentPath === closingTab.path) {
          clearTimeout(__hyloEditor.saveTimer); __hyloEditor.saveTimer = null;
          await __hyloEditorDoSave();
          var liveIdx = this.tabs.indexOf(closingTab);
          if (liveIdx < 0) return;
          i = liveIdx;
        }

        this._removeTab(i);
      },

      _removeTab(i) {
        var wasActive = (i === this.activeTab);
        __hyloEditorClearTabState(this.tabs[i].id);
        this._sideReflow();
        this.tabs.splice(i, 1);

        if (this.tabs.length === 0) {
          this.contentPaneOpen = false; this.activeTab = -1;
          clearTimeout(__hyloEditor.saveTimer); __hyloEditor.saveTimer = null;
          __hyloEditor.currentPath = ''; __hyloEditor.currentMd = ''; __hyloEditor.dirty = false;
          __hyloEditorSaveStatus('');
          if (__hyloEditor.view) {
            __hyloEditor.view.setState(__hyloEditor._buildState('', false, false));
            __hyloEditorSetModeState(false, false);
          }
          return;
        }

        if (i < this.activeTab) {
          this.activeTab -= 1;
        } else if (wasActive) {
          // The next one down inherits activation, as browsers do on close.
          this.activeTab = Math.min(i, this.tabs.length-1);
        }
        // A closed pane loads its active tab when it opens.
        if (wasActive && this.contentPaneOpen) {
          var t = this.tabs[this.activeTab];
          if (!t) return;
          var savedState = __hyloEditorRestoreTabState(t.id);
          if (t.path) {
            void __hyloContentPaneLoadNote(t.path, t.id, savedState);
          } else {
            void this._openUntitledTab(t, savedState);
          }
        }
      },

      // Opens the given mate's chat panel with a brand-new conversation
      // (not its most recent one — this is a one-shot "hand this note to a
      // bot", distinct from the sidebar's tryOpen which deliberately resumes
      // where that bot's chat left off) and drops a `[[stem]]` wikilink for
      // the active note into its composer. home.js is what actually owns
      // the chat panel/agentBots list, so this only dispatches an event
      // rather than reaching into window._homeData's chat internals directly.
      sendActiveNoteToAgentBot(mateId) {
        var tab = this.tabs[this.activeTab];
        if (!tab || !tab.path) return;
        var stem = __hyloNoteStem(tab.path);
        window.dispatchEvent(new CustomEvent('hylo:send-note-to-agent', { detail: { mateId: mateId, stem: stem } }));
      },

      async togglePinActiveNote() {
        var tab = this.tabs[this.activeTab];
        if (tab && tab.path) await __hyloSetNotePinned(tab.path, !tab.pinned);
      },

      async deleteActiveNote() {
        var tab = this.tabs[this.activeTab];
        if (tab && tab.path) await __hyloDeleteNote(tab.path, tab.title);
      },

      notePinned(path, pinned) {
        var tab = this.tabs.find(function(t) { return t.path === path; });
        if (tab) tab.pinned = pinned;
      },

      noteDeleted(path) {
        var idx = this.tabs.findIndex(function(t) { return t.path === path; });
        if (idx < 0) return;
        if (idx === this.activeTab) this.cancelRenameActiveNote();
        if (__hyloEditor.currentPath === path) {
          // Nothing left to save to; a pending autosave would recreate the note.
          clearTimeout(__hyloEditor.saveTimer); __hyloEditor.saveTimer = null; __hyloEditor.dirty = false;
        }
        this._removeTab(idx);
      },

      // Moving a note is driven from the home list (drag a card onto a
      // sidebar folder — see home_drag.js). It flushes a dirty active tab
      // first so the file that moves has the latest content; the moved note's
      // tab then closes like a deleted one.
      async flushPendingSaveFor(path) {
        if (__hyloEditor.dirty && __hyloEditor.currentPath === path) {
          clearTimeout(__hyloEditor.saveTimer); __hyloEditor.saveTimer = null;
          await __hyloEditorDoSave();
        }
      },
      // Renaming only ever changes the active tab's filename, never its
      // directory (see storage.Vault.RenameNote) — dir+move is drag-and-drop
      // only, from the sidebar (home_drag.js).
      renameActiveNote() {
        var tab = this.tabs[this.activeTab];
        if (!tab || !tab.path || this.renaming) return;
        this.renaming = true;
        // Focus-independent fallback: @keydown.escape only fires while the
        // input has focus, which tryFocus below isn't always able to land.
        var self = this;
        if (window.__hyloEscPush) window.__hyloEscPush('content-pane-rename', function() { self.cancelRenameActiveNote(); });

        var base = __hyloStripMdExt(tab.path.split('/').pop());
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
        if (window.__hyloEscPop) window.__hyloEscPop('content-pane-rename');
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
        var currentBase = __hyloStripMdExt(tab.path.split('/').pop());
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
        var currentBase = __hyloStripMdExt(tab.path.split('/').pop());
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
          var oldPath = tab.path;
          this.noteRenamed(oldPath, data.path);
          this.cancelRenameActiveNote(); // renamed successfully — same box-closing path as a cancel, just after committing
          if (data.renameJobId) void __hyloPollRenameJob(data.renameJobId);
        } catch (e) {
          window.showError((e && e.message) || 'Network error.', 'Cannot rename');
          input.disabled = false;
        } finally {
          this._renameSubmitting = false;
        }
      },

      // Autosave targets __hyloEditor.currentPath, not tab.path — miss it and
      // the next edit recreates the note at the old path.
      noteRenamed(oldPath, newPath) {
        var tab = this.tabs.find(function(t) { return t.path === oldPath; });
        if (!tab) return;
        tab.path = newPath;
        tab.title = __hyloStripMdExt(newPath.split('/').pop()) || newPath;
        if (__hyloEditor.currentPath === oldPath) __hyloEditor.currentPath = newPath;
      },
    };
  }

  // Best-effort poll for the async wikilink/source_notes sweep a rename
  // enqueues (internal/plugins/renamesync) — purely informational, never
  // blocks or retries the rename itself, which has already fully committed
  // by the time this runs. Silent on success (matches Pin/Unpin's existing
  // no-toast convention here); only speaks up if the sweep itself failed, so
  // the user knows some [[wikilinks]] elsewhere may still say the old name.
  async function __hyloPollRenameJob(jobId) {
    var deadline = Date.now() + 5 * 60 * 1000;
    await new Promise(function(r) { setTimeout(r, 600); });
    while (Date.now() < deadline) {
      var resp;
      try { resp = await fetch('/api/vault/rename-status?id=' + encodeURIComponent(jobId)); }
      catch (_) { return; }
      if (resp.ok) {
        var st = await resp.json();
        if (st.status === 'done') {
          return;
        }
        if (st.status === 'failed') {
          if (window.showError) window.showError('Some [[wikilinks]] to the old name may not have been updated automatically.', 'Rename cleanup incomplete');
          return;
        }
      }
      await new Promise(function(r) { setTimeout(r, 1500); });
    }
  }

  // ── Compile raw note from pane ─────────────────────────────────────────────
  function __hyloEditorCompileLabel(btn, text) {
    var label = btn.querySelector('.content-pane-compile-label');
    if (label) label.textContent = text;
  }
  function __hyloEditorCloseMoreMenu() {
    document.dispatchEvent(new CustomEvent('content-pane:close-more'));
  }
  function __hyloEditorResetCompileBtn() {
    var btn = document.querySelector('.content-pane-compile-btn');
    if (!btn) return;
    btn.classList.remove('is-compiling', 'success');
    btn.disabled = false;
    btn.title = 'Compile to knowledge note';
    __hyloEditorCompileLabel(btn, 'Compile');
  }

  async function compileContentPaneNote(event) {
    var btn = event && event.currentTarget;
    if (!btn || btn.disabled) return;
    var pane = window.__hyloContentPane;
    var tab = pane ? pane.tabs[pane.activeTab] : null;
    if (!tab || !tab.path || !tab.canCompile) return;
    var rawPath = tab.path;

    if (__hyloEditor.dirty && __hyloEditor.currentPath === rawPath) {
      clearTimeout(__hyloEditor.saveTimer);
      __hyloEditor.saveTimer = null;
      await __hyloEditorDoSave();
    }

    var originalTitle = btn.title;
    btn.disabled = true;
    btn.classList.add('is-compiling');
    btn.title = 'Compiling…';
    __hyloEditorCompileLabel(btn, 'Compiling…');

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
        __hyloEditorCompileLabel(btn, 'Compile');
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
            __hyloEditorCompileLabel(btn, 'Compiled');
            setTimeout(function() {
              btn.disabled = false;
              btn.classList.remove('success');
              __hyloEditorCompileLabel(btn, 'Compile');
              __hyloEditorCloseMoreMenu();
            }, 1500);
            return;
          }
          if (st.status === 'busy') {
            throw new Error('The bot has too many tasks waiting; try again later');
          }
          if (st.status === 'failed' || st.status === 'canceled') {
            throw new Error('Compile agent ' + st.status);
          }
          // The bot compiles in its chat once you've paused chatting with it, so the wait doesn't count.
          var queued = st.status === 'queued';
          if (queued) deadline = Date.now() + 10 * 60 * 1000;
          btn.title = queued ? "Queued: compiles once you've paused chatting with the bot" : 'Compiling…';
          __hyloEditorCompileLabel(btn, queued ? 'Queued…' : 'Compiling…');
        }
        await new Promise(function(r) { setTimeout(r, 1500); });
      }
      throw new Error('Compile timed out');
    } catch (err) {
      btn.disabled = false;
      btn.title = err && err.message ? err.message : 'Compile failed';
      __hyloEditorCompileLabel(btn, 'Compile');
    } finally {
      btn.classList.remove('is-compiling');
    }
  }

  // Called by the desktop Edit menu. The editor keeps its own history; native
  // undo would corrupt it, but every other field only has the native one.
  function __hyloHistory(cmd, native) {
    var s = __hyloEditor;
    var ae = document.activeElement;
    // The find panel's input sits inside view.dom but has its own native history.
    var inText = s.view && s.view.contentDOM.contains(ae) && !/^(INPUT|TEXTAREA)$/.test(ae.tagName);
    if (!inText && ae && (ae.isContentEditable || /^(INPUT|TEXTAREA)$/.test(ae.tagName))) {
      document.execCommand(native);
    } else if (s.view && __hyloCM) {
      __hyloCM[cmd](s.view);
    }
  }
  window.__hyloUndo = function() { __hyloHistory('cmUndo', 'undo'); };
  window.__hyloRedo = function() { __hyloHistory('cmRedo', 'redo'); };

  window.__hyloEditorShellHref = function() {
    try {
      var seg = location.pathname.replace(/^\/+/,'').split('/')[0];
      return '/'+(seg||'home');
    } catch(_) { return '/home'; }
  };

  window.__hyloHotkeys.register('toggle-editor', 'o', function() {
    var pane = window.__hyloContentPane;
    if (!pane) return;
    if (pane.contentPaneOpen) pane.closeContentPane(); else void pane.showContentPane();
  });

  window.__hyloHotkeys.register('new-note', 'n', function() {
    if (window.__hyloContentPane) void window.__hyloContentPane.openNewInContentPane();
  });

  window.__hyloHotkeys.register('expand-editor', '\\', function() {
    var _pane = window.__hyloContentPane;
    if (_pane && _pane.contentPaneOpen) _pane.toggleMaximizeEditor();
  });

  window.__hyloHotkeys.registerRaw('content-pane-scroll', function(e, mod) {
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
    if (mod || e.altKey) return;
    var ae = document.activeElement;
    var tag = ae && ae.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;
    var _pane = window.__hyloContentPane;
    if (!_pane || !_pane.contentPaneOpen) return;
    var _editArea = document.getElementById('content-pane-edit-area');
    if (_editArea && _editArea.contains(ae)) return;
    e.preventDefault();
    var _scroller = document.querySelector('#content-pane-edit-area .cm-scroller');
    if (_scroller) _scroller.scrollBy(0, e.key === 'ArrowDown' ? 80 : -80);
    return true;
  });

  // Returning false leaves the event alone so the native close-window still works.
  window.__hyloHotkeys.register('close-tab', 'w', function() {
    var pane = window.__hyloContentPane;
    if (!pane || !pane.contentPaneOpen || pane.activeTab < 0) return false;
    var at = pane.tabs[pane.activeTab];
    if (at && at.path) pane.contentPaneCloseTab(pane.activeTab);
  });

  // Shared by the Mod-F hotkey below and the editor's "more" menu (Find
  // item, content_pane.html) so there's one place that knows how to open it.
  function __hyloEditorOpenFind() {
    var s = __hyloEditor;
    if (!__hyloCM || !s.view) return;
    __hyloCM.openSearchPanel(s.view);
  }

  async function __hyloEditorCopyText(text) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (e) {
      if (window.showError) window.showError((e && e.message) || 'Could not copy to clipboard.', 'Copy error');
      return false;
    }
  }

  function __hyloEditorCopyMarkdown() {
    var s = __hyloEditor;
    return __hyloEditorCopyText(s.view ? s.view.state.doc.toString() : s.currentMd);
  }

  // CJK counts per character, everything else per word; frontmatter excluded.
  function __hyloEditorCountWords(text) {
    var body = text.replace(/^---\r?\n[\s\S]*?\r?\n---[ \t]*(\r?\n|$)/, '');
    var cjk = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/gu;
    var n = (body.match(cjk) || []).length;
    var words = body.replace(cjk, ' ').match(/[\p{L}\p{N}]+(?:['’-][\p{L}\p{N}]+)*/gu);
    return n + (words ? words.length : 0);
  }

  var __HYLO_INFO_MAX_LINKS = 6;

  function __hyloEditorFormatTime(ms) {
    if (!ms) return '—';
    var d = new Date(ms), p = function(n) { return String(n).padStart(2, '0'); };
    return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
  }

  // Info panel model. Editor-side fields are live (include unsaved edits);
  // `meta` is the /api/notes/meta response, null until it arrives or if it fails.
  function __hyloEditorInfo(meta) {
    var s = __hyloEditor;
    var text = s.view ? s.view.state.doc.toString() : (s.currentMd || '');
    var path = s.currentPath || '';
    var links = meta ? meta.links : [];
    var label = '';
    if (meta && meta.kind !== 'index') {
      label = meta.kind === 'knowledge' ? 'Sources' : (links.length ? 'Compiled' : 'Not compiled');
    }
    return {
      path: path,
      dir: path.replace(/\/[^/]*$/, '') || '/',
      words: __hyloEditorCountWords(text).toLocaleString(),
      lines: (s.view ? s.view.state.doc.lines : text.split('\n').length).toLocaleString(),
      created: meta ? __hyloEditorFormatTime(meta.createdAt) : '…',
      modified: s.dirty ? 'Unsaved changes' : (meta ? __hyloEditorFormatTime(meta.updatedAt) : '…'),
      linksLabel: label,
      linkCount: links.length,
      links: links.slice(0, __HYLO_INFO_MAX_LINKS),
      linksMore: Math.max(0, links.length - __HYLO_INFO_MAX_LINKS),
      linksAreKnowledge: !!meta && meta.kind !== 'knowledge',
    };
  }

  async function __hyloEditorFetchNoteMeta(path) {
    try {
      var resp = await fetch('/api/notes/meta', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({path: path}),
      });
      return resp.ok ? await resp.json() : null;
    } catch (_) {
      return null;
    }
  }

  // Plain Mod-L now (no Shift needed) — .register() handles that, unlike
  // the Mod-Shift-E it replaced which needed registerRaw's manual check.
  window.__hyloHotkeys.register('reading-mode', 'l', function() {
    var pane = window.__hyloContentPane;
    if (!pane || !pane.contentPaneOpen || pane.agentWriting || !__hyloEditor.view) return;
    var tab = pane.tabs[pane.activeTab];
    if (!tab || !tab.path) return;
    editorMode.toggleReading();
  });

  window.__hyloHotkeys.register('find', 'f', function() {
    var _fContentPane = window.__hyloContentPane;
    if (!_fContentPane || !_fContentPane.contentPaneOpen || _fContentPane.agentWriting) return false;
    if (!__hyloCM || !__hyloEditor.view) return false;
    __hyloEditorOpenFind();
  });

  function __hyloOpenTabRows() {
    return Array.prototype.slice.call(document.querySelectorAll('#home-side-open .home-side-tab'));
  }

  // Mod-T moves focus to the sidebar's open tabs (and back to the editor).
  window.__hyloHotkeys.registerRaw('open-tabs-focus', function(e, mod) {
    if (!mod || e.shiftKey || e.altKey || e.key.toLowerCase() !== 't') return;
    var _pane = window.__hyloContentPane;
    if (!_pane || !_pane.tabs.length || _pane.agentWriting) return;
    e.preventDefault();
    var rows = __hyloOpenTabRows();
    if (rows.indexOf(document.activeElement) >= 0) {
      if (_pane.contentPaneOpen) __hyloFocusEditor(); else __hyloBlurActive();
      return true;
    }
    var focusRow = function() {
      var row = __hyloOpenTabRows()[Math.max(_pane.activeTab, 0)];
      if (row) row.focus();
    };
    // A collapsed group is visibility:hidden, so open it and focus once it has rendered.
    if (window._homeData && !window._homeData.openTabsOpen) {
      window._homeData.openTabsOpen = true;
      window.Alpine.nextTick(focusRow);
    } else {
      focusRow();
    }
    return true;
  });

  // While a row has focus: ↑/↓ move, Enter opens it, Esc returns to the editor.
  // Raw, so the editor's own arrow scrolling and the Esc stack don't see it.
  window.__hyloHotkeys.registerRaw('open-tabs-nav', function(e, mod) {
    if (mod || e.shiftKey || e.altKey) return;
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown' && e.key !== 'Enter' && e.key !== 'Escape') return;
    var rows = __hyloOpenTabRows();
    var at = rows.indexOf(document.activeElement);
    var _pane = window.__hyloContentPane;
    if (at < 0 || !_pane) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    var n = rows.length;
    if (e.key === 'ArrowDown') rows[(at + 1) % n].focus();
    else if (e.key === 'ArrowUp') rows[(at - 1 + n) % n].focus();
    else if (e.key === 'Escape') { if (_pane.contentPaneOpen) __hyloFocusEditor(); else __hyloBlurActive(); }
    else Promise.resolve(_pane.selectTab(at)).then(function() { __hyloFocusEditor(); });
    return true;
  });

  // Intercept mouse back/forward buttons (button 3/4) to switch pane tabs.
  window.addEventListener('mousedown', function(e) {
    var _pane = window.__hyloContentPane;
    if (!_pane || !_pane.contentPaneOpen || _pane.tabs.length <= 1) return;
    if (e.button === 3) {
      e.preventDefault();
      if (_pane.activeTab > 0) void _pane.contentPaneSwitchTab(_pane.activeTab - 1);
    } else if (e.button === 4) {
      e.preventDefault();
      if (_pane.activeTab < _pane.tabs.length - 1) void _pane.contentPaneSwitchTab(_pane.activeTab + 1);
    }
  }, true);

