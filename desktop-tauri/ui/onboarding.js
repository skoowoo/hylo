// First-run flow, shown in the compact window until onboarding_finish grows
// it into the main one. It sits inside the shell's scope: the shell decides
// when it runs (onboardingPending) and takes over again at leaveOnboarding().
document.addEventListener('alpine:init', function () {
  var STEPS = ['welcome', 'vault', 'ai'];
  var INTRO_MS = 1600;
  var LEAVE_MS = 260;
  var DAY = 86400000;

  var ICONS = {
    new: '<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 10v6m-3-3h6M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>',
    obsidian: '<img src="obsidian.svg" alt="">',
    folder: '<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>',
  };

  // Reveal order on the AI step; Pi leads because it's the one we recommend.
  var AGENTS = [
    { id: 'pi', label: 'Pi' },
    { id: 'claude', label: 'Claude Code' },
    { id: 'codex', label: 'Codex' },
    { id: 'opencode', label: 'OpenCode' },
    { id: 'cursor-agent', label: 'Cursor' },
    { id: 'copilot', label: 'Copilot' },
  ].map(function (a) {
    a.logo = window.__hyloAgentLogo(a.id, 'agents/', { glyph: 'ob-glyph', dark: 'ob-on-dark', light: 'ob-on-light' });
    return a;
  });
  // The probe answers all at once; agents still appear one by one, as if found.
  var MIN_SCAN_MS = 1400;
  var SCAN_TICK_MS = 420;
  var REVEAL_MS = 520;
  var SETTLE_MS = 500;
  // About four lines of the hello card; CJK glyphs are twice as wide.
  var HELLO_BUDGET = 210;
  var SLOW_START_MS = 3000;
  var TYPE_MS = 22;
  var REDUCED_MOTION = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  // Each look runs every agent CLI; switching apps back and forth shouldn't.
  var REFOCUS_MS = 10000;

  function api() { return window.hyloDesktop; }
  // An agent that ignores the length asked for is cut at its last whole
  // sentence that fits, or at a word with an ellipsis.
  function clip(text) {
    var used = 0;
    for (var i = 0; i < text.length; i++) {
      used += /[\u2e80-\u9fff\uac00-\ud7af\uff00-\uffef]/.test(text[i]) ? 2 : 1;
      if (used > HELLO_BUDGET) break;
    }
    if (i >= text.length) return text;
    var head = text.slice(0, i);
    var end = head.search(/[.!?。！？][^.!?。！？]*$/);
    if (end > head.length / 2) return head.slice(0, end + 1);
    var space = head.lastIndexOf(' ');
    return (space > head.length / 2 ? head.slice(0, space) : head).replace(/[\s,，、;；:：]+$/, '') + '…';
  }
  function errorText(e) { return String((e && e.message) || e || 'Something went wrong.'); }
  function wait(ms) { return new Promise(function (r) { setTimeout(r, ms); }); }
  function noop() { }

  function basename(p) {
    var parts = p.replace(/[\\/]+$/, '').split(/[\\/]/);
    return parts[parts.length - 1] || p;
  }

  function shortPath(p) {
    return p.replace(/^\/(Users|home)\/[^/]+(?=\/|$)/, '~');
  }

  function openedAgo(ms) {
    var days = Math.floor((Date.now() - ms) / DAY);
    if (days < 1) return 'opened today';
    if (days < 2) return 'opened yesterday';
    if (days < 14) return 'opened ' + days + ' days ago';
    if (days < 60) return 'opened ' + Math.floor(days / 7) + ' weeks ago';
    return 'opened ' + Math.floor(days / 30) + ' months ago';
  }

  Alpine.data('hyloOnboarding', function () {
    return {
      steps: STEPS,
      icons: ICONS,
      step: -1,
      dir: 'fwd',
      intro: false,
      leaving: false,

      pathNeeded: false,
      addPath: true,

      agents: null,
      revealed: [],
      scanName: '',
      _scan: 0,
      agentPhase: 'idle',
      agentError: '',
      // Hylo's local service: nothing past this step works without it, so
      // while it's down the only ways out are Try again and Back.
      serverUp: false,
      serverError: '',
      slowStart: false,
      detecting: false,
      selectedAgent: '',
      hello: { state: 'idle', text: '', error: '' },
      typed: '',
      helloOver: false,
      copied: '',
      entering: false,
      _helloRun: 0,
      // Bumped when the vault changes; a check still out for the old one is ignored.
      _aiRun: 0,
      _typer: 0,
      _lookedAt: 0,

      vaults: null,
      custom: null,
      selected: '',
      pauseSync: true,
      saving: false,
      vaultError: '',

      init() {
        var self = this;
        // The shell learns whether setup is pending only after this has run.
        this.$watch('onboardingPending', function (v) { if (v) self.start(self.onboardingStep); });
        if (this.onboardingPending) this.start(this.onboardingStep);
        var events = window.__TAURI__ && window.__TAURI__.event;
        if (events) {
          events.listen('onboarding-hello', function (e) {
            var p = e.payload || {};
            if (self.hello.state === 'talking' && p.token === self._helloRun) self.hello.text += p.delta || '';
          });
        }
        // Installing or signing in happens in Terminal; coming back is the cue to look again.
        // So is quitting whatever kept the server from starting.
        window.addEventListener('focus', function () {
          if (!self.is('ai') || self.aiBusy()) return;
          var retry = self.serverError || (self.agentPhase === 'ready' && self.hello.state !== 'ok');
          if (!retry) return;
          if (Date.now() - self._lookedAt >= REFOCUS_MS) self.detect();
        });
      },

      start(name) {
        if (this.step >= 0) return;
        var i = STEPS.indexOf(name);
        this.step = i < 0 ? 0 : i;
        var self = this;
        this.intro = true;
        setTimeout(function () { self.intro = false; }, INTRO_MS);
        // Fetched up front so the next steps open already filled in.
        api().onboarding.pathNeeded().then(function (v) { self.pathNeeded = !!v; }).catch(noop);
        api().onboarding.vaults().then(function (v) {
          self.vaults = v;
          if (!self.selected) {
            var cur = self.options().find(function (o) { return o.current; });
            self.selected = (cur || v.default).path;
          }
        }).catch(noop);
        if (this.is('ai')) this.enterAi();
      },

      is(name) { return STEPS[this.step] === name; },

      go(i) {
        if (this.leaving || this.saving || this.entering || i < 0 || i >= STEPS.length || i === this.step) return;
        this.dir = i > this.step ? 'fwd' : 'back';
        this.step = i;
        api().onboarding.setStep(STEPS[i]).catch(noop);
        if (this.is('ai')) this.enterAi();
      },

      back() { this.go(this.step - 1); },

      // The AI step's primary button is disabled meanwhile; Enter must agree.
      aiBusy() {
        return this.entering || this.detecting || this.hello.state === 'talking';
      },

      primary() {
        if (this.is('welcome')) {
          if (this.pathNeeded && this.addPath) {
            var self = this;
            api().onboarding.addToPath().then(function () { self.pathNeeded = false; }).catch(noop);
          }
          this.go(this.step + 1);
        } else if (this.is('vault')) {
          this.saveVault();
        } else if (this.aiBusy()) {
          return;
        } else if (this.serverError) {
          this.detect();
        } else if (this.hello.state === 'ok') {
          this.enterWithAi();
        } else {
          this.detect();
        }
      },

      // The first page is the most useful one for how setup ended: the bot
      // just made, the notes a chosen folder already had, or a blank capture.
      landing(botId) {
        // intro=1 has a new bot open the chat by writing the user a guide.
        if (this.hello.state === 'ok') return '/home?type=chat' + (botId ? '&bot=' + encodeURIComponent(botId) + '&intro=1' + (this.hello.lang ? '&lang=' + encodeURIComponent(this.hello.lang) : '') : '');
        var o = this.current();
        return o && o.kind === 'new' && !o.current ? '/home?type=shorts' : '/home?type=recent';
      },

      // The server is checked once more on the way out: entering a Hylo that
      // can't answer would only strand the user on a broken page. The landing
      // must be stored before the shell enters the server.
      async finish(botId) {
        if (this.leaving) return;
        if (!(await this.ensureServer())) {
          this.entering = false;
          return;
        }
        this.leaving = true;
        await Promise.all([api().onboarding.finish(this.landing(botId)).catch(noop), wait(LEAVE_MS)]);
        this.leaveOnboarding();
      },

      skipAi() {
        if (this.entering || !this.serverUp) return;
        this.entering = true;
        this.finish();
      },

      // ── AI ─────────────────────────────────────────────
      enterAi() {
        if (this.agentPhase === 'idle') this.detect();
      },

      // Starts the server on the vault just chosen, or says why it can't.
      async ensureServer() {
        var run = this._aiRun;
        try {
          await api().onboarding.ready();
          if (run !== this._aiRun) return false;
          this.serverUp = true;
          this.serverError = '';
          return true;
        } catch (e) {
          if (run !== this._aiRun) return false;
          this.serverUp = false;
          this.serverError = errorText(e);
          this.agentPhase = 'error';
          return false;
        }
      },

      async detect() {
        if (this.detecting) return;
        var self = this;
        var run = this._aiRun;
        this.detecting = true;
        this._lookedAt = Date.now();
        this.agentError = '';
        var first = this.agentPhase !== 'ready';
        if (first) {
          this.agentPhase = 'waiting';
          this.startScan();
        }
        var since = Date.now();
        // Only the server's start can be slow enough to explain; the probe has its own timeout.
        var slow = setTimeout(function () { self.slowStart = true; }, SLOW_START_MS);
        try {
          if (!(await this.ensureServer())) return;
          clearTimeout(slow);
          this.slowStart = false;
          var agents = await api().onboarding.agents();
          if (run !== this._aiRun) return;
          this.agents = agents;
          if (first) await wait(MIN_SCAN_MS - (Date.now() - since));
          if (run !== this._aiRun) return;
          this.stopScan();
          await this.reveal();
          if (this.leaving || run !== this._aiRun) return;
          this.agentPhase = 'ready';
          if (!this.isFound(this.selectedAgent)) this.selectedAgent = this.revealed[0] || '';
          if (this.selectedAgent && this.hello.state !== 'ok') this.sayHello(this.selectedAgent);
        } catch (e) {
          if (run !== this._aiRun) return;
          this.agentPhase = 'error';
          this.agentError = errorText(e);
        } finally {
          clearTimeout(slow);
          if (run === this._aiRun) {
            this.stopScan();
            this.slowStart = false;
            this.detecting = false;
          }
        }
      },

      // Cycles through the agents being looked for while the probe runs.
      startScan() {
        var self = this;
        var i = 0;
        this.scanName = AGENTS[0].label;
        clearInterval(this._scan);
        this._scan = setInterval(function () { self.scanName = AGENTS[++i % AGENTS.length].label; }, SCAN_TICK_MS);
      },

      stopScan() {
        clearInterval(this._scan);
        this.scanName = '';
      },

      async reveal() {
        var self = this;
        this.revealed = this.revealed.filter(function (id) { return self.isFound(id); });
        var fresh = AGENTS.filter(function (a) { return self.isFound(a.id) && self.revealed.indexOf(a.id) < 0; });
        if (!fresh.length) return;
        this.agentPhase = 'revealing';
        for (var i = 0; i < fresh.length; i++) {
          if (i) await wait(REVEAL_MS);
          this.revealed.push(fresh[i].id);
        }
        await wait(SETTLE_MS);
      },

      scanText() {
        if (this.agentPhase === 'waiting') {
          return this.slowStart ? 'Indexing your notes. Large folders can take a minute…' : 'Looking for AI agents on this computer…';
        }
        if (this.agentPhase === 'error') return this.serverError ? "Hylo couldn't start" : "Couldn't finish checking";
        var n = this.revealed.length;
        if (!n) return this.agentPhase === 'ready' ? 'No AI agent found on this computer yet.' : '';
        return 'Found ' + n + ' AI agent' + (n === 1 ? '' : 's') + ' on this computer';
      },

      isFound(id) {
        return !!(this.agents && this.agents.some(function (a) { return a.id === id && a.available; }));
      },

      agentLabel(id) {
        var a = AGENTS.find(function (x) { return x.id === id; });
        return a ? a.label : id;
      },

      logo(id) {
        var a = AGENTS.find(function (x) { return x.id === id; });
        return a ? a.logo : '';
      },

      pickAgent(id) {
        if (this.agentPhase !== 'ready' || !this.isFound(id) || this.entering || (id === this.selectedAgent && this.hello.state !== 'fail')) return;
        this.selectedAgent = id;
        this.sayHello(id);
      },

      async sayHello(id) {
        if (this.leaving) return;
        var run = ++this._helloRun;
        this.hello = { state: 'talking', text: '', error: '' };
        this.typed = '';
        this.startTyping();
        var res;
        var ctx = { lang: navigator.language, now: new Date().toLocaleString('en-US', { weekday: 'long', hour: 'numeric', minute: '2-digit' }) };
        try { res = await api().onboarding.hello(id, run, ctx); } catch (e) { res = { ok: false, error: String((e && e.message) || e) }; }
        if (run !== this._helloRun) return;
        if (res.ok) {
          this.hello.text = clip(res.text.trim());
          this.hello.lang = res.lang || '';
          this.hello.state = 'ok';
        } else {
          this.hello.error = res.error || '';
          this.hello.state = 'fail';
        }
      },

      // Streamed chunks arrive in bursts; reveal them a character at a time.
      startTyping() {
        var self = this;
        clearInterval(this._typer);
        this._typer = setInterval(function () {
          var target = clip(self.hello.text);
          if (self.typed.length < target.length) {
            var step = REDUCED_MOTION ? target.length : target.length - self.typed.length > 40 ? 3 : 1;
            self.typed = target.slice(0, self.typed.length + step);
            self.$nextTick(function () {
              var el = self.$refs.helloText;
              self.helloOver = !!el && el.scrollHeight > el.clientHeight + 1;
            });
          } else if (self.hello.state !== 'talking') {
            self.typed = target;
            clearInterval(self._typer);
          }
        }, TYPE_MS);
      },

      primaryLabel() {
        if (this.entering) return 'Opening Hylo…';
        if (this.aiBusy()) return 'Checking…';
        if (this.serverError) return 'Try again';
        return this.hello.state === 'ok' ? 'Enter Hylo' : 'Check again';
      },

      async enterWithAi() {
        if (this.entering) return;
        this.entering = true;
        var botId = await api().onboarding.createBot(this.selectedAgent, this.hello.text).catch(noop);
        this.finish(botId);
      },

      async copy(text) {
        try {
          await navigator.clipboard.writeText(text);
        } catch (e) {
          var ta = document.createElement('textarea');
          ta.value = text;
          document.body.appendChild(ta);
          ta.select();
          document.execCommand('copy');
          ta.remove();
        }
        var self = this;
        this.copied = text;
        setTimeout(function () { if (self.copied === text) self.copied = ''; }, 1600);
      },

      resetAi() {
        this._aiRun++;
        this._helloRun++;
        this.detecting = false;
        this.slowStart = false;
        this.stopScan();
        clearInterval(this._typer);
        this.hello = { state: 'idle', text: '', error: '' };
        this.typed = '';
        this.agentPhase = 'idle';
        this.serverUp = false;
        this.serverError = '';
      },

      // ── Vault ──────────────────────────────────────────
      options() {
        if (!this.vaults) return [];
        var list = [Object.assign({ kind: 'new' }, this.vaults.default)];
        if (this.vaults.current) list.push(Object.assign({ kind: 'folder' }, this.vaults.current));
        this.vaults.obsidian.forEach(function (v, i) {
          list.push(Object.assign({ kind: 'obsidian', first: i === 0 }, v));
        });
        if (this.custom) list.push(this.custom);
        return list;
      },

      current() {
        var self = this;
        return this.options().find(function (o) { return o.path === self.selected; });
      },

      meta(o) {
        if (o.current) return 'Current vault · ' + o.display;
        if (o.kind === 'new') return 'New folder · ' + o.display;
        if (o.kind === 'obsidian' && o.lastOpened) return o.display + ' · ' + openedAgo(o.lastOpened);
        return o.display;
      },

      vaultHint() {
        if (this.vaultError) return this.vaultError;
        var o = this.current();
        if (!o) return '';
        if (o.current) return 'Hylo keeps using this folder.';
        return o.kind === 'new'
          ? 'Hylo will create this folder for you.'
          : 'Hylo only adds a hidden .hylo folder here. Your notes stay as they are.';
      },

      // Sync is global: switching folders would back the new one up to the old one's remote.
      syncConflict() {
        var o = this.current();
        return !!(this.vaults && this.vaults.gitSync && o && !o.current);
      },

      choose(path) {
        if (this.saving) return;
        this.selected = path;
        this.vaultError = '';
        var root = this.$root;
        this.$nextTick(function () {
          var el = root.querySelector('.ob-card--option.is-selected');
          if (el) el.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
        });
      },

      moveSelection(delta) {
        var list = this.options();
        var i = list.findIndex(function (o) { return o.path === this.selected; }, this);
        var next = list[Math.min(list.length - 1, Math.max(0, i + delta))];
        if (next) this.choose(next.path);
      },

      async pickFolder() {
        if (this.saving) return;
        var res;
        try { res = await api().pickFolder({ title: 'Choose your notes folder' }); } catch (e) { return; }
        if (!res || res.canceled || !res.path) return;
        var known = this.options().some(function (o) { return o.path === res.path; });
        if (!known) {
          this.custom = { kind: 'folder', name: basename(res.path), path: res.path, display: shortPath(res.path) };
        }
        this.choose(res.path);
      },

      async saveVault() {
        if (this.saving || !this.selected) return;
        this.saving = true;
        this.vaultError = '';
        try {
          var changed = await api().onboarding.setVault(this.selected, this.syncConflict() && this.pauseSync);
          // A hello from before spoke about the old folder's notes; the server restarts on the new one.
          if (changed && this.agentPhase !== 'idle') this.resetAi();
          this.saving = false;
          this.go(this.step + 1);
        } catch (e) {
          this.saving = false;
          this.vaultError = errorText(e);
        }
      },

      // ── Input ──────────────────────────────────────────
      // Focus never moves on its own, so no ring shows: Enter on the page acts
      // as the primary button, and a focused control keeps its own Enter.
      onKey(e) {
        if (this.view !== 'onboarding' || this.step < 0 || this.leaving || e.isComposing) return;
        if (e.key === 'Escape') {
          e.preventDefault();
          this.back();
        } else if (e.key === 'Enter' && !e.target.closest('button, a, input, textarea, select')) {
          e.preventDefault();
          this.primary();
        } else if (this.is('vault') && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
          e.preventDefault();
          this.moveSelection(e.key === 'ArrowDown' ? 1 : -1);
        }
      },

      // The whole window is a drag handle except its controls.
      drag(e) {
        if (e.button !== 0 || e.target.closest('button, a, input, textarea, select, label, [data-no-drag]')) return;
        var win = window.__TAURI__ && window.__TAURI__.window;
        if (win) win.getCurrentWindow().startDragging().catch(noop);
      },
    };
  });
});
