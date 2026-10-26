  function settingsAgentBotsTab() {
    return {
      agentBotsList: [],
      agentBotsLoading: false,
      agentBotsError: '',
      agentBotFormMode: null,
      botSetup: { phase: 'idle', agents: [], agentId: '', model: '', hello: { state: 'idle', text: '', typed: '', error: '' }, creating: false, error: '' },
      agentBotEditId: '',
      // Full shape up front: the hidden form footer still evaluates name.trim().
      agentBotDraft: { name: '', description: '', agentId: '', model: '', cwd: '', main: false, systemPrompt: '', enabled: true },
      agentBotTriggers: [],
      // Skill name → on, for the bot's working directory; applied on save.
      agentBotSkills: {},
      agentBotSaving: false,
      agentBotSaveError: '',
      agentBotEventDefs: [],
      botFormTab: 'general',
      botChannels: { wechat: null, discord: null, telegram: null },
      channelBusy: '',
      channelError: '',
      wxImage: '',
      wxCode: '',
      wxTimer: null,
      wxPolling: false,
      dcToken: '',
      dcOwner: '',
      tgToken: '',
      tgCode: '',
      tgLink: '',
      tgUsername: '',
      tgImage: '',
      tgTimer: null,
      // One table instead of the same "which kind of trigger is this" branch
      // repeated across agentBotPromptPlaceholder and the Event block's hint
      // text. Prompt vars come from the server's event defs.
      AGENT_BOT_EVENT_KINDS: {
        scheduled: {
          placeholder: 'Review my vault and write a daily digest. Time: {Now}',
          hint: 'Scheduled triggers run on a timer — choose one schedule below',
        },
        compile_requested: {
          placeholder: 'Compile {Path} into knowledge units.',
          hint: 'Fires when the user manually triggers compilation via the API — Path carries the note',
        },
        agent_run_completed: {
          placeholder: '{Name} just finished — review its output and take action.',
          hint: "Fires when another bot's run succeeds. Filter by source bot names below",
        },
      },
      agentBotSchedulePresets: {
        every: [
          { label: 'Every hour', value: 'every 1h' },
          { label: 'Every 6 hours', value: 'every 6h' },
        ],
      },
      agentBotWeekdayDefs: [
        { abbr: 'mon', label: 'Mon' },
        { abbr: 'tue', label: 'Tue' },
        { abbr: 'wed', label: 'Wed' },
        { abbr: 'thu', label: 'Thu' },
        { abbr: 'fri', label: 'Fri' },
        { abbr: 'sat', label: 'Sat' },
        { abbr: 'sun', label: 'Sun' },
      ],
      async loadAgentBots() {
        this.agentBotsLoading = true;
        this.agentBotsError = '';
        try {
          const r = await fetch('/api/mates');
          if (!r.ok) throw new Error('HTTP ' + r.status);
          const d = await r.json();
          this.agentBotsList = d.mates || [];
          if (this.agentBotsList.length) this.botSetup.phase = 'idle';
          else if (this.botSetup.phase === 'idle') this.startBotSetup();
        } catch(e) { this.agentBotsError = e.message; }
        finally { this.agentBotsLoading = false; }
      },

      // No bots yet: the same detect → hello → create the desktop onboarding
      // runs. Hello is the proof the agent works, so a failed one creates nothing.
      async startBotSetup() {
        this.cancelSetupHello();
        this.botSetup = { phase: 'scanning', agents: [], agentId: '', model: '', hello: { state: 'idle', text: '', typed: '', error: '' }, creating: false, error: '' };
        await this.loadAgents(true);
        if (this.agentsError) {
          this.botSetup.phase = 'error';
          this.botSetup.error = "Couldn't look for AI agents: " + this.agentsError;
          return;
        }
        this.botSetup.agents = this.agents.filter(function(a) { return a.available; });
        this.botSetup.phase = 'ready';
        if (this.botSetup.agents.length) this.pickSetupAgent(this.botSetup.agents[0].id);
      },

      // Leaving the tab stops the hello's agent run; coming back starts over.
      stopBotSetup() {
        if (this.botSetup.creating) return;
        this.cancelSetupHello();
        this.botSetup.phase = 'idle';
      },

      setupAgentLogo(a) {
        if (!a) return '';
        return window.__hyloAgentLogo(a.id, '/static/agents/', { glyph: 'bot-setup-glyph', dark: 'bot-setup-on-dark', light: 'bot-setup-on-light' }) ||
          (a.name || a.id).charAt(0).toUpperCase();
      },

      setupTyping() {
        const h = this.botSetup.hello;
        return h.state === 'talking' || h.typed.length < h.text.length;
      },

      setupHelloDone() { return this.botSetup.hello.state === 'ok' && !this.setupTyping(); },

      // Words are typed out at a steady pace, however the agent streams them.
      startSetupTyping() {
        const reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        clearInterval(this._typeTimer);
        this._typeTimer = setInterval(() => {
          const h = this.botSetup.hello;
          const left = h.text.length - h.typed.length;
          if (left > 0) h.typed = h.text.slice(0, h.typed.length + (reduce ? left : left > 40 ? 3 : 1));
          else if (h.state !== 'talking') { clearInterval(this._typeTimer); this._typeTimer = null; }
        }, 22);
      },

      cancelSetupHello() {
        clearInterval(this._typeTimer);
        this._typeTimer = null;
        this._setupToken = (this._setupToken || 0) + 1;
        if (this._setupES) { this._setupES.close(); this._setupES = null; }
        if (this._setupRun) {
          fetch('/api/runs/' + this._setupRun + '/cancel', { method: 'POST' }).catch(function() {});
          this._setupRun = '';
        }
      },

      // The agent's own "default" is offered once, first.
      setupModels() {
        const models = this.agentBotModelsForAgent(this.botSetup.agentId).filter(function(m) { return m.id !== 'default'; });
        return [{ id: 'default', label: '' }].concat(models);
      },

      // Hello waits for a model, so it runs on what the bot will use.
      pickSetupAgent(id) {
        if (this.botSetup.creating) return;
        this.cancelSetupHello();
        this.botSetup.agentId = id;
        this.botSetup.model = '';
        this.botSetup.hello = { state: 'idle', text: '', typed: '', error: '' };
      },

      pickSetupModel(model) {
        if (this.botSetup.creating) return;
        this.botSetup.model = model;
        this.startSetupHello();
      },

      async startSetupHello() {
        if (this.botSetup.creating || !this.botSetup.model) return;
        this.cancelSetupHello();
        const token = this._setupToken;
        const live = () => token === this._setupToken;
        const id = this.botSetup.agentId;
        this.botSetup.hello = { state: 'talking', text: '', typed: '', error: '' };
        const hello = this.botSetup.hello;
        this.startSetupTyping();
        try {
          const r = await fetch('/api/setup/hello', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              agentId: id,
              model: this.botSetup.model,
              lang: navigator.language || '',
              now: new Date().toLocaleString('en-US', { weekday: 'long', hour: 'numeric', minute: '2-digit' }),
            }),
          });
          if (!r.ok) throw new Error((await r.text()).trim() || 'HTTP ' + r.status);
          const runId = (await r.json()).runId;
          if (!live()) { fetch('/api/runs/' + runId + '/cancel', { method: 'POST' }).catch(function() {}); return; }
          this._setupRun = runId;
          const ok = await new Promise((resolve) => {
            const es = new EventSource('/api/runs/' + runId + '/events');
            this._setupES = es;
            const timer = setTimeout(() => { hello.error = "It didn't answer in time."; finish(false); }, 90000);
            const finish = (v) => { clearTimeout(timer); es.close(); resolve(v); };
            es.addEventListener('agent', (e) => {
              try {
                const ev = JSON.parse(e.data);
                if (ev.type === 'text_delta' && ev.delta && live()) hello.text += ev.delta;
              } catch (_) {}
            });
            // A named "error" frame carries data; the browser's own error event doesn't.
            es.addEventListener('error', (e) => {
              if (!e.data) return;
              try { hello.error = JSON.parse(e.data).message || ''; } catch (_) {}
            });
            es.addEventListener('end', (e) => {
              let status = '';
              try { status = JSON.parse(e.data).status; } catch (_) {}
              finish(status === 'succeeded' && !!hello.text.trim());
            });
          });
          if (!live()) return;
          // A timed-out run may still be going.
          if (!ok) this.cancelSetupHello();
          this._setupRun = '';
          this._setupES = null;
          hello.state = ok ? 'ok' : 'fail';
        } catch (e) {
          if (!live()) return;
          hello.error = e.message;
          hello.state = 'fail';
        }
      },

      async createSetupBot() {
        const s = this.botSetup;
        if (s.creating || s.hello.state !== 'ok') return;
        s.creating = true;
        try {
          const r = await fetch('/api/setup/bot', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ agentId: s.agentId, model: s.model, greeting: s.hello.text.trim() }),
          });
          if (!r.ok) throw new Error((await r.text()).trim() || 'HTTP ' + r.status);
          const id = (await r.json()).mateId;
          if (!id) { s.creating = false; this.loadAgentBots(); return; }
          // intro=1 has the new bot open the chat by writing the user a guide.
          window.location.href = '/home?type=chat&bot=' + encodeURIComponent(id) + '&intro=1';
        } catch (e) {
          s.creating = false;
          window.showError('Setup failed: ' + e.message, 'Setup failed');
        }
      },

      async loadAgentBotEvents() {
        try {
          const r = await fetch('/api/mate-events');
          if (!r.ok) return;
          const d = await r.json();
          this.agentBotEventDefs = d.events || [];
        } catch(_) {}
      },

      async newAgentBot() {
        const first = this.agents.find(function(a){ return a.available; });
        const firstModel = (first && first.models && first.models.length) ? first.models[0].id : '';
        this.agentBotDraft = { name: '', description: '', agentId: first ? first.id : '', model: firstModel, cwd: '', main: false, systemPrompt: '', enabled: true };
        this.agentBotTriggers = [];
        this.agentBotSkills = {};
        this.agentBotEditId = '';
        this.agentBotSaveError = '';
        this.cancelChannelFlows();
        this.botFormTab = 'general';
        this.agentBotFormMode = 'create';
        this._agentBotSaved = this.agentBotSnapshot();
        await this.loadAgentBotSkills('');
      },

      // Every added skill, built-ins last since most bots leave them alone.
      get agentBotSkillList() { return this.extraSkills.concat(this.builtinSkills); },

      // Loads which skills are on for the bot, then marks the form as clean.
      async loadAgentBotSkills(botId) {
        await this.loadSkills(true);
        const on = {};
        this.agentBotSkillList.forEach(function(s) { on[s.name] = !!botId && s.bots.some(function(b) { return b.id === botId; }); });
        this.agentBotSkills = on;
        this._agentBotSaved = this.agentBotSnapshot();
      },

      // Turns skills on or off for the bot to match the form.
      async applyAgentBotSkills(bot) {
        for (const s of this.agentBotSkillList) {
          if (s.default && bot.main) continue;
          const on = !!this.agentBotSkills[s.name];
          if (on === s.bots.some(function(b) { return b.id === bot.id; })) continue;
          const r = await fetch('/api/skills/' + encodeURIComponent(s.name) + (on ? '/enable' : '/disable'), {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ botId: bot.id }),
          });
          if (!r.ok) {
            const d = await r.json().catch(function() { return {}; });
            throw new Error(d.error || 'Updating skill ' + s.name + ' failed');
          }
        }
        await this.loadSkills(true);
      },

      async openAgentBotEdit(id) {
        try {
          const [r] = await Promise.all([
            fetch('/api/mates/' + id),
            this.agentsLoaded ? Promise.resolve() : this.loadAgents(),
          ]);
          if (!r.ok) throw new Error('HTTP ' + r.status);
          const d = await r.json();
          const m = d.mate;
          const savedModel = m.model || '';
          const knownModels = this.agentBotModelsForAgent(m.agentId);
          const modelValid = !savedModel || savedModel === 'default' ||
            knownModels.length === 0 ||
            knownModels.some(function(x) { return x.id === savedModel; });
          this.agentBotDraft = { name: m.name, description: m.description || '', agentId: m.agentId, model: modelValid ? savedModel : '', cwd: m.cwd, main: m.main, systemPrompt: m.systemPrompt || '', enabled: m.enabled };
          this.agentBotTriggers = (m.triggers || []).map(function(t) {
            return Object.assign({}, t, {
              name: t.name || '',
              eventTypes: t.eventTypes || [],
              schedule: t.schedule || '',
              pathPrefixes: t.pathPrefixes || [],
              _open: false,
            });
          });
          this.agentBotSkills = {};
          this.agentBotEditId = m.id;
          this.agentBotSaveError = '';
          this.botFormTab = 'general';
          this.agentBotFormMode = 'edit';
          this._agentBotSaved = this.agentBotSnapshot();
          await this.loadAgentBotSkills(m.id);
          await this.loadBotChannels(m.id);
        } catch(e) { window.showError('Load failed: ' + e.message, 'Load failed'); }
      },

      // Fits a prompt box to its text. Measured a frame later, since a
      // trigger just expanded isn't laid out yet.
      agentBotAutoGrow(el) {
        requestAnimationFrame(() => {
          if (!el.offsetParent) return;
          el.style.height = 'auto';
          el.style.height = (el.scrollHeight + 2) + 'px';
        });
      },

      agentBotSnapshot() {
        return JSON.stringify([this.agentBotDraft, this.agentBotSkills, this.agentBotTriggers.map(function(t) {
          const copy = Object.assign({}, t);
          delete copy._open;
          return copy;
        })]);
      },
      agentBotDirty() { return !!this.agentBotFormMode && this.agentBotSnapshot() !== this._agentBotSaved; },

      async closeAgentBotForm() {
        if (this.agentBotDirty()) {
          const ok = await window.showConfirm({ title: 'Discard changes?', message: 'This bot has unsaved changes.', confirmLabel: 'Discard', danger: true });
          if (!ok) return false;
        }
        this.cancelChannelFlows();
        this.agentBotFormMode = null;
        return true;
      },

      // Channels connect to a saved bot, so a new one has to be saved first.
      agentBotFormTabs() {
        const on = (s) => this.agentBotSkills[s.name];
        const skills = this.extraSkills.filter(on).length +
          (this.agentBotDraft.main ? this.builtinSkills.length : this.builtinSkills.filter(on).length);
        return [
          { id: 'general', label: 'General', count: 0 },
          { id: 'skills', label: 'Skills', count: skills },
          { id: 'triggers', label: 'Triggers', count: this.agentBotTriggers.length },
          { id: 'channels', label: 'Channels', count: Object.values(this.botChannels).filter(Boolean).length, disabled: this.agentBotFormMode !== 'edit' },
        ];
      },

      channelLabel(kind) {
        return { wechat: 'WeChat', discord: 'Discord', telegram: 'Telegram' }[kind] || kind;
      },

      cancelChannelFlows() {
        if (this.wxTimer) { clearInterval(this.wxTimer); this.wxTimer = null; }
        if (this.tgTimer) { clearInterval(this.tgTimer); this.tgTimer = null; }
        this.wxImage = '';
        this.wxCode = '';
        this.tgCode = '';
        this.tgLink = '';
        this.tgImage = '';
        this.tgUsername = '';
        this.channelBusy = '';
      },

      async fetchBotChannels(id) {
        const r = await fetch('/api/mates/' + id + '/channels');
        if (!r.ok) throw new Error('HTTP ' + r.status);
        const d = await r.json();
        const next = { wechat: null, discord: null, telegram: null };
        (d.channels || []).forEach(function(c) { next[c.kind] = c; });
        return next;
      },

      // Refresh an open bot's channels without cancelling a scan in progress.
      async syncOpenBotChannels() {
        if (this.agentBotFormMode !== 'edit' || !this.agentBotEditId || this.wxCode || this.tgCode || this.channelBusy) return;
        const id = this.agentBotEditId;
        try {
          const next = await this.fetchBotChannels(id);
          if (id === this.agentBotEditId) this.botChannels = next;
        } catch (e) {}
      },

      async loadBotChannels(id) {
        this.cancelChannelFlows();
        this.channelError = '';
        this.botChannels = { wechat: null, discord: null, telegram: null };
        if (!id) return;
        try {
          this.botChannels = await this.fetchBotChannels(id);
        } catch (e) { this.channelError = e.message; }
      },

      async channelFail(r) {
        const d = await r.json().catch(function() { return {}; });
        throw new Error(d.error || ('HTTP ' + r.status));
      },

      async disconnectChannel(kind) {
        if (this.channelBusy || !this.agentBotEditId) return;
        this.channelError = '';
        this.channelBusy = kind;
        try {
          const r = await fetch('/api/mates/' + this.agentBotEditId + '/channels/' + kind, { method: 'DELETE' });
          if (!r.ok) await this.channelFail(r);
          await this.loadBotChannels(this.agentBotEditId);
          await this.loadAgentBots();
        } catch (e) { this.channelError = e.message; this.channelBusy = ''; }
      },

      async startWechat() {
        if (this.channelBusy || !this.agentBotEditId) return;
        this.channelError = '';
        this.channelBusy = 'wechat';
        try {
          const r = await fetch('/api/mates/' + this.agentBotEditId + '/channels/wechat/login/start', { method: 'POST' });
          if (!r.ok) await this.channelFail(r);
          const d = await r.json();
          this.wxCode = d.qrcode;
          this.wxImage = d.qrcode_image || '';
          if (this.wxTimer) clearInterval(this.wxTimer);
          this.wxTimer = setInterval(() => this.pollWechat(), 2000);
        } catch (e) { this.channelError = e.message; }
        finally { if (this.channelBusy === 'wechat') this.channelBusy = ''; }
      },

      cancelWechat() {
        if (this.wxTimer) { clearInterval(this.wxTimer); this.wxTimer = null; }
        this.wxImage = '';
        this.wxCode = '';
      },

      // The status call can be held open by WeChat, so polls never overlap.
      async pollWechat() {
        const code = this.wxCode;
        if (!code || !this.agentBotEditId || this.wxPolling) return;
        this.wxPolling = true;
        try {
          const r = await fetch('/api/mates/' + this.agentBotEditId + '/channels/wechat/login/status?qrcode=' + encodeURIComponent(code));
          if (!r.ok) await this.channelFail(r);
          const d = await r.json();
          if (code !== this.wxCode) return;
          if (d.qrcode && d.qrcode !== code) {
            this.wxCode = d.qrcode;
            if (d.qrcode_image) this.wxImage = d.qrcode_image;
          }
          if (d.status === 'confirmed' || d.connected) {
            this.cancelWechat();
            await this.loadBotChannels(this.agentBotEditId);
            await this.loadAgentBots();
          }
        } catch (e) { this.channelError = e.message; this.cancelWechat(); }
        finally { this.wxPolling = false; }
      },

      async connectDiscord() {
        if (this.channelBusy || !this.agentBotEditId || !this.dcToken.trim() || !this.dcOwner.trim()) return;
        this.channelError = '';
        this.channelBusy = 'discord';
        try {
          const r = await fetch('/api/mates/' + this.agentBotEditId + '/channels/discord', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bot_token: this.dcToken.trim(), user_id: this.dcOwner.trim() }),
          });
          if (!r.ok) await this.channelFail(r);
          this.dcToken = '';
          this.dcOwner = '';
          await this.loadBotChannels(this.agentBotEditId);
          await this.loadAgentBots();
        } catch (e) { this.channelError = e.message; }
        finally { if (this.channelBusy === 'discord') this.channelBusy = ''; }
      },

      async startTelegram() {
        if (this.channelBusy || !this.agentBotEditId || !this.tgToken.trim()) return;
        this.channelError = '';
        this.channelBusy = 'telegram';
        try {
          const r = await fetch('/api/mates/' + this.agentBotEditId + '/channels/telegram/pair/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bot_token: this.tgToken.trim() }),
          });
          if (!r.ok) await this.channelFail(r);
          const d = await r.json();
          this.tgCode = d.code;
          this.tgLink = d.link;
          this.tgUsername = d.username;
          this.tgImage = d.qrcode_image || '';
          if (this.tgTimer) clearInterval(this.tgTimer);
          this.tgTimer = setInterval(() => this.pollTelegram(), 2000);
        } catch (e) { this.channelError = e.message; }
        finally { if (this.channelBusy === 'telegram') this.channelBusy = ''; }
      },

      cancelTelegram() {
        if (this.tgTimer) { clearInterval(this.tgTimer); this.tgTimer = null; }
        this.tgCode = '';
        this.tgLink = '';
        this.tgImage = '';
        this.tgUsername = '';
      },

      async pollTelegram() {
        const code = this.tgCode;
        if (!code || !this.agentBotEditId) return;
        try {
          const r = await fetch('/api/mates/' + this.agentBotEditId + '/channels/telegram/pair/status?code=' + encodeURIComponent(code));
          if (!r.ok) await this.channelFail(r);
          const d = await r.json();
          if (code !== this.tgCode) return;
          if (d.status === 'confirmed') {
            this.tgToken = '';
            this.cancelTelegram();
            await this.loadBotChannels(this.agentBotEditId);
            await this.loadAgentBots();
          } else if (d.status === 'expired') {
            this.cancelTelegram();
            this.channelError = 'Pairing expired. Try again.';
          }
        } catch (e) { this.channelError = e.message; this.cancelTelegram(); }
      },

      agentBotModelsForAgent(agentId) {
        const a = this.agents.find(function(x) { return x.id === agentId; });
        return (a && a.models) ? a.models : [];
      },

      onAgentBotAgentChange() {
        const models = this.agentBotModelsForAgent(this.agentBotDraft.agentId);
        this.agentBotDraft.model = models.length ? models[0].id : '';
      },

      async saveAgentBot() {
        if (this.agentBotSaving || !this.agentBotDraft.name.trim()) return;
        this.agentBotSaving = true;
        this.agentBotSaveError = '';
        try {
          const payload = Object.assign({}, this.agentBotDraft, { triggers: this.agentBotTriggers.map(function(t) {
            const copy = Object.assign({}, t);
            delete copy._open;
            return copy;
          }) });
          const url = this.agentBotFormMode === 'create' ? '/api/mates' : '/api/mates/' + this.agentBotEditId;
          const method = this.agentBotFormMode === 'create' ? 'POST' : 'PUT';
          const r = await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
          if (!r.ok) { this.agentBotSaveError = await r.text(); return; }
          const d = await r.json();
          const saved = Object.assign({}, d.mate, { triggerCount: (d.mate.triggers || []).length });
          if (this.agentBotFormMode === 'create') {
            this.agentBotsList.push(saved);
          } else {
            this.agentBotsList = this.agentBotsList.map(function(m) { return m.id === saved.id ? saved : m; });
          }
          // From here a retry updates the bot rather than creating it twice.
          this.agentBotFormMode = 'edit';
          this.agentBotEditId = saved.id;
          this.agentBotDraft.cwd = saved.cwd;
          await this.applyAgentBotSkills(saved);
          this.agentBotFormMode = null;
        } catch(e) { this.agentBotSaveError = e.message; }
        finally { this.agentBotSaving = false; }
      },

      async deleteAgentBot(id) {
        const ok = await window.showConfirm({ title: 'Delete bot', message: 'This bot and all its data will be permanently deleted.', confirmLabel: 'Delete', danger: true });
        if (!ok) return;
        try {
          const r = await fetch('/api/mates/' + id, { method: 'DELETE' });
          if (!r.ok) { window.showError('Delete failed (server error)', 'Delete failed'); return; }
          this.agentBotsList = this.agentBotsList.filter(function(m) { return m.id !== id; });
          if (!this.agentBotsList.length) this.startBotSetup();
        } catch(e) { window.showError('Delete failed: ' + e.message, 'Delete failed'); }
      },

      async moveAgentBot(idx, dir) {
        var newIdx = idx + dir;
        if (newIdx < 0 || newIdx >= this.agentBotsList.length) return;
        var tmp = this.agentBotsList[idx];
        this.agentBotsList[idx] = this.agentBotsList[newIdx];
        this.agentBotsList[newIdx] = tmp;
        this.agentBotsList = this.agentBotsList.slice();
        try {
          await fetch('/api/mates/reorder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids: this.agentBotsList.map(function(m) { return m.id; }) }),
          });
        } catch(_) {}
      },

      addAgentBotTrigger() {
        this.agentBotTriggers.push({ id: '', mateId: '', name: '', eventTypes: ['note_created'], schedule: '', prompt: '', pathPrefixes: [], enabled: true, _open: true });
      },

      removeAgentBotTrigger(idx) { this.agentBotTriggers.splice(idx, 1); },

      // Collapsed-state label: event + the one extra detail (schedule, or
      // path/source-bot filter) that distinguishes it from another trigger
      // on the same event, so a glance at the closed card is still useful.
      agentBotTriggerSummary(t) {
        const def = this.agentBotEventDefs.find(function(d){ return d.type === (t.eventTypes[0] || ''); });
        const label = def ? def.label : (t.eventTypes[0] || 'Select event…');
        if (this.eventOf(t) === 'scheduled') return t.schedule ? label + ' · ' + t.schedule : label;
        if ((t.pathPrefixes || []).length) return label + ' · ' + t.pathPrefixes.join(', ');
        return label;
      },

      // UI writes exactly one event type per trigger, so it keys the table.
      eventOf(t) { return (t.eventTypes || [])[0] || ''; },
      agentBotEventKind(t) { return this.AGENT_BOT_EVENT_KINDS[this.eventOf(t)] || null; },

      // Scheduled triggers carry no path; the rest filter by path prefix
      // (or, for agent_run_completed, by source bot name — same field).
      filterKindOf(et) {
        if (et === 'scheduled') return 'none';
        return et === 'agent_run_completed' ? 'bots' : 'path';
      },
      hasPathFilter(t) { return this.filterKindOf(this.eventOf(t)) !== 'none'; },
      agentBotFilterFor(t) {
        if (this.eventOf(t) === 'agent_run_completed') {
          return {
            title: 'Source Bots',
            hint: 'Only fire when the run was completed by one of these bots. Leave empty to fire on any bot. One bot name per line.',
            placeholder: 'Summarizer\nCompiler',
          };
        }
        return {
          title: 'Path Prefixes',
          hint: 'Only fire when the event path starts with one of these prefixes. Leave empty to match all paths. One prefix per line, e.g. /journal/',
          placeholder: '/journal/\n/projects/work/',
        };
      },

      agentBotPromptVarsForTrigger(t) {
        const et = this.eventOf(t);
        const def = this.agentBotEventDefs.find(function(d){ return d.type === et; });
        return (def && def.vars) || [];
      },

      agentBotPromptPlaceholder(t) {
        const kind = this.agentBotEventKind(t);
        return kind ? kind.placeholder : 'Summarize the key points in {Path}';
      },

      agentBotTriggerHint(t) {
        const kind = this.agentBotEventKind(t);
        return kind ? kind.hint : 'Vault event that activates this trigger';
      },

      setAgentBotET(trigger, et) {
        // Paths and bot names share pathPrefixes; a value meant for the old kind
        // would linger hidden, or fail the backend's "/" check.
        if (this.filterKindOf(this.eventOf(trigger)) !== this.filterKindOf(et)) trigger.pathPrefixes = [];
        if (et === 'scheduled') {
          trigger.eventTypes = ['scheduled'];
          if (!trigger.schedule) trigger.schedule = 'daily 09:00';
        } else {
          trigger.eventTypes = [et];
          trigger.schedule = '';
        }
      },

      // Backend owns the grammar ("every 1h" / "daily HH:MM" / "weekly d1,d2 HH:MM");
      // this is the one place the UI reads it back.
      parseAgentBotSchedule(t) {
        const parts = (t.schedule || '').trim().toLowerCase().split(/\s+/);
        const kind = parts[0] === 'every' || parts[0] === 'weekly' ? parts[0] : 'daily';
        if (kind !== 'weekly') return { kind: kind, days: [], time: '09:00' };
        const days = !parts[1] || parts[1] === 'none' ? [] : parts[1].split(',').filter(Boolean);
        return { kind: 'weekly', days: days, time: parts[2] || '09:00' };
      },
      scheduleKindOf(t) { return this.parseAgentBotSchedule(t).kind; },
      scheduleTimeOf(t) {
        const parts = (t.schedule || '').trim().split(/\s+/);
        return (this.scheduleKindOf(t) === 'weekly' ? parts[2] : parts[1]) || '09:00';
      },
      setScheduleTime(t, time) {
        if (!/^\d{2}:\d{2}$/.test(time || '')) return;
        if (this.scheduleKindOf(t) === 'weekly') {
          const days = this.weeklyDaysOf(t);
          t.schedule = 'weekly ' + (days.length ? days.join(',') : 'none') + ' ' + time;
        } else {
          t.schedule = 'daily ' + time;
        }
      },
      weeklyDaysOf(t) { return this.parseAgentBotSchedule(t).days; },

      setScheduleKind(t, kind) {
        if (this.scheduleKindOf(t) === kind) return;
        t.schedule = { every: 'every 1h', daily: 'daily 09:00', weekly: 'weekly mon 09:00' }[kind];
      },

      // dayField falls back to the "none" sentinel (instead of an empty string) when the
      // last selected day is removed, so the schedule string stays prefixed "weekly " and
      // scheduleKindOf() keeps the Weekly tab active rather than falling back to Daily.
      toggleWeeklyDay(t, abbr) {
        const order = this.agentBotWeekdayDefs.map(function(d) { return d.abbr; });
        let days = this.weeklyDaysOf(t);
        if (days.indexOf(abbr) >= 0) {
          days = days.filter(function(d) { return d !== abbr; });
        } else {
          days = days.concat([abbr]);
        }
        days.sort(function(a, b) { return order.indexOf(a) - order.indexOf(b); });
        t.schedule = 'weekly ' + (days.length ? days.join(',') : 'none') + ' ' + this.parseAgentBotSchedule(t).time;
      },

      insertAgentBotVar(ti, token, event) {
        const promptBlock = event.target.closest('.agent-bot-trigger-prompt');
        const ta = promptBlock && promptBlock.querySelector('textarea');
        if (ta && typeof ta.selectionStart === 'number') {
          const start = ta.selectionStart;
          const end = ta.selectionEnd;
          const val = ta.value || '';
          ta.value = val.slice(0, start) + token + val.slice(end);
          ta.dispatchEvent(new Event('input', { bubbles: true }));
          const pos = start + token.length;
          ta.focus();
          ta.setSelectionRange(pos, pos);
          return;
        }
        const t = this.agentBotTriggers[ti];
        if (!t) return;
        t.prompt = (t.prompt || '') + token;
      },

    };
  }
