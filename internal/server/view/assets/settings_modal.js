  function settingsCtrl() {
    const ctrl = {
      isDesktop: !!window.hyloDesktop,
      tab: 'appearance',
      agents: [],
      agentsLoading: false,
      agentsError: '',
      agentsLoaded: false,
      agentsFromCache: false,
      agentsCachedAt: 0,
      // The open detail view of the current tab, which titles the pane header and
      // gets its back button; null on a tab's own list.
      detailView() {
        if (this.tab === 'server' && this.openSection) {
          return { title: this.sectionLabel(this.openSection), parent: 'Configuration', back: () => { this.openSection = ''; } };
        }
        if (this.tab === 'agent-bots' && this.agentBotFormMode) {
          const name = this.agentBotDraft.name.trim();
          return { title: name || (this.agentBotFormMode === 'create' ? 'New bot' : 'Untitled bot'), parent: 'Bots', back: () => this.closeAgentBotForm() };
        }
        return null;
      },

      async init() {
        // Another same-origin view can change the accent; keep the picker's ring in step.
        window.addEventListener('hylo:accent', () => {
          var id = localStorage.getItem('hylo-accent');
          this.accentPref = this.accentPresets.some(p => p.id === id) ? id : 'indigo';
        });
        window.addEventListener('hylo:open-settings', (e) => {
          if (e.detail && e.detail.tab) this.tab = e.detail.tab;
          Alpine.store('settingsModal').open = true;
        });
        window.addEventListener('hylo:switch-server', (e) => {
          const id = (e.detail && e.detail.id) || '';
          if (id) this.requestSwitch(id);
        });
        // Bots and skills change from anywhere (an agent, another window): keep an open tab current.
        if (window.__hyloLive) {
          __hyloLive.on('bots', () => {
            if (!(Alpine.store('settingsModal').open && this.tab === 'agent-bots' && this.botSetup.phase === 'idle')) return;
            this.loadAgentBots();
            this.syncOpenBotChannels();
          });
          __hyloLive.on('skills', () => {
            if (Alpine.store('settingsModal').open && this.tab === 'skills') this.loadSkills();
          });
        }
        window.__hyloHotkeys.register('open-settings', ',', function() {
          Alpine.store('settingsModal').open = true;
        });
        this.$watch('tab', (val, old) => {
          if (old === 'agent-bots') this.stopBotSetup();
          if (Alpine.store('settingsModal').open) this.loadTab(val);
        });
        this.$watch('$store.settingsModal.open', (open) => {
          if (!open) {
            if (window.__hyloEscPop) window.__hyloEscPop('settings');
            this.cancelChannelFlows();
            this.stopBotSetup();
            return;
          }
          if (window.__hyloEscPush) window.__hyloEscPush('settings', () => this.requestClose(), this.$el);
          this.loadTab(this.tab);
        });
      },

      async requestClose() {
        if (this.tab === 'agent-bots' && this.agentBotFormMode && !(await this.closeAgentBotForm())) return;
        if (this.hasDirty) {
          const ok = await window.showConfirm({ title: 'Discard changes?', message: 'You have unsaved server settings.', confirmLabel: 'Discard', danger: true });
          if (!ok) return;
          this.discardAll();
        }
        Alpine.store('settingsModal').open = false;
      },

      loadTab(tab) {
        if (tab === 'server') { this.loadServerStatus(); if (!this.hasDirty) this.loadConfig(); }
        else if (tab === 'agent-bots') {
          this.loadAgentBots();
          if (!this.agentsLoaded) this.loadAgents();
          if (!this.agentBotEventDefs.length) this.loadAgentBotEvents();
        }
        else if (tab === 'skills') this.loadSkills();
        else if (tab === 'agents') { if (!this.agentsLoaded) this.loadAgents(); }
        else if (tab === 'notifications') this.loadNotifySettings();
      },

      async loadAgents(force = false) {
        // Reuse the in-flight promise so a concurrent openAgentBotEdit call doesn't
        // issue a second request while the first is still pending.
        if (!force && this._agentsLoadPromise) return this._agentsLoadPromise;
        this.agentsLoading = true;
        this.agentsError = '';
        const p = (async () => {
          try {
            const url = force ? '/api/agents?force=true' : '/api/agents';
            const r = await fetch(url);
            if (!r.ok) throw new Error('HTTP ' + r.status);
            const d = await r.json();
            this.agents = d.agents || [];
            this.agentsFromCache = d.fromCache || false;
            this.agentsCachedAt = d.fetchedAt || 0;
            this.agentsLoaded = true;
          } catch (e) { this.agentsError = e.message; }
          finally { this.agentsLoading = false; this._agentsLoadPromise = null; }
        })();
        this._agentsLoadPromise = p;
        return p;
      },

      get sortedAgents() {
        return [...this.agents].sort((a, b) => (b.available ? 1 : 0) - (a.available ? 1 : 0));
      },

    };
    // Each tab contributes its members; copying descriptors keeps their getters live.
    for (const tab of [settingsAppearanceTab, settingsServerTab, settingsAgentCLITab, settingsNotificationsTab,
      settingsShortcutsTab, settingsAgentBotsTab, settingsSkillsTab]) {
      Object.defineProperties(ctrl, Object.getOwnPropertyDescriptors(tab()));
    }
    return ctrl;
  }
