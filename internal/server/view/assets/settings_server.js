  function settingsServerTab() {
    return {
      switching: false,
      switchConfirm: '',
      switchError: '',
      serverManaged: false,
      serverRunning: false,
      serverStopping: false,
      serverStopError: '',

      schema: [],
      values: {},
      secrets: {},
      vaultConfigPath: '',
      cfgLoading: false,
      cfgError: '',
      cfgSaving: false,
      cfgSaveError: '',
      cfgSaveOk: false,
      cfgRestarting: false,
      cfgRestartError: '',
      patch: {},
      revealed: {},
      openSection: '',

      get sectionTabs() {
        const excluded = new Set(['log', 'server']);
        const seen = new Set(), tabs = [];
        for (const f of this.schema) {
          if (!seen.has(f.section) && !excluded.has(f.section)) {
            seen.add(f.section);
            tabs.push(f.section);
          }
        }
        return tabs;
      },

      sectionLabel(s) {
        const m = {
          'proxy': 'Proxy',
          'vault': 'Vault', 'agent': 'Agent',
          'plugins.search': 'Search', 'plugins.git_sync': 'Git Sync',
        };
        return m[s] || s;
      },

      sectionIntro(s) {
        const m = {
          'proxy': 'One proxy for Discord, Telegram, link previews, remote images, WeChat, and git.',
          'vault': 'Paths, layout, and core vault behavior.',
          'agent': 'Agent CLI integration and global prompt settings.',
          'plugins.search': 'Indexing and search quality.',
          'plugins.git_sync': 'Automatic git push/pull. Whether it runs, and where it pushes, is set per vault.',
        };
        return m[s] || '';
      },

      fieldsForSection(s) { return this.schema.filter(f => f.section === s); },

      openSectionDetail(s) {
        this.openSection = s;
      },

      sectionHasDirty(s) { return this.fieldsForSection(s).some(f => this.isDirty(f.key)); },

      getVal(key) {
        if (key in this.patch) return this.patch[key];
        const parts = key.split('.');
        let v = this.values;
        for (const p of parts) {
          if (v == null || typeof v !== 'object') return null;
          v = v[p];
        }
        return v ?? null;
      },

      setVal(key, val) {
        this.patch = { ...this.patch, [key]: val };
        this.cfgSaveOk = false;
        this.cfgSaveError = '';
        this.cfgRestartError = '';
      },

      isDirty(key) { return key in this.patch; },
      get hasDirty() { return Object.keys(this.patch).length > 0; },

      discardAll() {
        this.patch = {};
        this.cfgSaveOk = false;
        this.cfgSaveError = '';
        this.cfgRestartError = '';
      },

      toggleReveal(key) { this.revealed = { ...this.revealed, [key]: !this.revealed[key] }; },
      listToText(v) { if (!Array.isArray(v)) return ''; return v.join('\n'); },
      textToList(s) { return s.split('\n').map(x => x.trim()).filter(Boolean); },

      buildNested(flat) {
        const out = {};
        for (const [key, val] of Object.entries(flat)) {
          const parts = key.split('.');
          let o = out;
          for (let i = 0; i < parts.length - 1; i++) {
            if (!(parts[i] in o)) o[parts[i]] = {};
            o = o[parts[i]];
          }
          o[parts[parts.length - 1]] = val;
        }
        return out;
      },

      get heroServer() { return Alpine.store('servers').active(); },
      get heroName() { return this.heroServer ? this.heroServer.name : location.host; },
      get heroSub() {
        const s = this.heroServer;
        if (!s) return 'Web session';
        return s.kind === 'local' ? 'This computer · ' + s.url : s.url;
      },
      get heroState() { return this.heroServer ? Alpine.store('servers').state(this.heroServer.id) : 'online'; },
      // Before the registry loads, the page origin is the server being edited.
      get editingLocalServer() {
        const s = this.heroServer;
        if (s) return s.kind === 'local';
        const h = (location.hostname || '').toLowerCase();
        return h === 'localhost' || h === '127.0.0.1' || h === '::1';
      },

      // Entry point for the sidebar switcher; errors and the discard prompt surface in Settings.
      requestSwitch(id) {
        if (!id || id === Alpine.store('servers').activeId) return;
        this.switchError = '';
        if (this.hasDirty) {
          this.tab = 'server';
          Alpine.store('settingsModal').open = true;
          this.switchConfirm = id;
          return;
        }
        this.doSwitch(id);
      },

      confirmSwitch() {
        const id = this.switchConfirm;
        this.switchConfirm = '';
        this.doSwitch(id);
      },

      async doSwitch(id) {
        if (!id || id === Alpine.store('servers').activeId) {
          this.switching = false;
          this.switchConfirm = '';
          return;
        }
        this.switching = true;
        this.switchError = '';
        const err = await Alpine.store('servers').switchTo(id);
        if (err) {
          this.switching = false;
          this.switchError = this.srvErrorText(err);
          this.tab = 'server';
          Alpine.store('settingsModal').open = true;
        }
      },

      async loadServerStatus() {
        if (!window.hyloDesktop?.getServerProcessStatus) return;
        try {
          const s = await window.hyloDesktop.getServerProcessStatus();
          this.serverManaged = s.managed;
          this.serverRunning = s.alive;
        } catch { /* noop */ }
      },

      async stopServer() {
        this.serverStopping = true;
        this.serverStopError = '';
        try {
          const r = await window.hyloDesktop.stopServer();
          if (!r.ok) { this.serverStopError = r.error || 'Stop failed'; this.serverStopping = false; }
        } catch (e) { this.serverStopError = e.message; this.serverStopping = false; }
      },

      async loadConfig() {
        this.cfgLoading = true;
        this.cfgError = '';
        try {
          const [sr, vr] = await Promise.all([
            fetch('/api/config/schema'),
            fetch('/api/config'),
          ]);
          this.schema = (await sr.json()).fields || [];
          const vd = await vr.json();
          this.values = vd.values || {};
          this.secrets = vd.secrets || {};
          this.vaultConfigPath = (vd.meta && vd.meta.vault_config_path) || '';
        } catch (e) { this.cfgError = e.message; }
        finally { this.cfgLoading = false; }
      },

      async saveConfig() {
        if (!this.hasDirty || this.cfgSaving || this.cfgRestarting) return;
        this.cfgSaving = true;
        this.cfgSaveError = '';
        this.cfgSaveOk = false;
        this.cfgRestartError = '';
        let saved = false;
        try {
          const res = await fetch('/api/config', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ patch: this.buildNested(this.patch) }),
          });
          const data = await res.json();
          if (!res.ok) {
            const detail = Array.isArray(data.errors) ? data.errors.join('; ') : (data.error || 'Save failed');
            this.cfgSaveError = detail;
          } else {
            this.patch = {};
            await this.loadConfig();
            saved = true;
          }
        } catch (e) { this.cfgSaveError = e.message; }
        finally { this.cfgSaving = false; }

        if (saved) await this.restartServer();
      },

      async restartServer() {
        this.cfgRestarting = true;
        this.cfgRestartError = '';
        let before = '';
        try {
          const res = await fetch('/api/server/restart', { method: 'POST' });
          let data = {};
          try { data = await res.json(); } catch (e) { /* body lost as the listener closed */ }
          if (!res.ok) {
            this.cfgRestartError = data.error || 'Restart failed';
            this.cfgSaveOk = true;
            this.cfgRestarting = false;
            return;
          }
          before = data.started_at || '';
        } catch (e) {
          // The connection can drop once shutdown starts. Wait for the new process.
        }
        if (await this.waitForRestart(before)) {
          location.reload();
          return;
        }
        this.cfgRestartError = 'Server did not come back';
        this.cfgRestarting = false;
      },

      async waitForRestart(before) {
        // Where the server exits instead of exec'ing (Windows), the desktop
        // heartbeat takes up to ~20s to start it again.
        const deadline = Date.now() + 60000;
        let sawDown = !before;
        while (Date.now() < deadline) {
          await new Promise(r => setTimeout(r, 200));
          try {
            const res = await fetch('/api/config');
            if (!res.ok) { sawDown = true; continue; }
            const data = await res.json();
            const started = (data.meta && data.meta.started_at) || '';
            if (before) {
              if (started && started !== before) return true;
              continue;
            }
            if (sawDown && started) return true;
          } catch (e) { sawDown = true; }
        }
        return false;
      },

      srvErrorText(err) {
        switch (err.code) {
          case 'bad_key': return 'The API key was rejected. Run "hylo auth show" on that server to see it.';
          case 'not_hylo': return 'This address does not look like a Hylo server.';
          case 'unreachable': return 'Cannot reach this address. Check the URL and that the server is running.';
          case 'unavailable': return 'This address answers, but Hylo on it is not running.';
          default: return err.message;
        }
      },

      async pickFolder(key, currentVal) {
        if (!window.hyloDesktop?.pickFolder) return;
        const result = await window.hyloDesktop.pickFolder({
          title: 'Select Vault Root Folder',
          defaultPath: currentVal || undefined,
        });
        if (!result.canceled && result.path) this.setVal(key, result.path);
      },

    };
  }
