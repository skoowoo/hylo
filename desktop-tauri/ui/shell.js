// The shell page: everything before a server's own page loads. Two views
// share it: onboarding (first launch) and servers. Every way into a server
// passes through here behind the logo (launch, onboarding, the list, a switch
// from a server's page); the list shows only when entering needs attention.
document.addEventListener('alpine:init', function () {
  var SPLASH_MS = 3000;
  // The logo's assembly (shell.css) is done by then; entering sooner cuts it off.
  var ENTER_MIN_MS = 1600;
  var SLOW_START_MS = 4000;
  var PROBE_EVERY_MS = 5000;
  var HEALTH_LABEL = { online: 'Online', offline: 'Offline', auth_required: 'Needs API key' };
  var BUSY_LABEL = { start: 'Starting…', stop: 'Stopping…', restart: 'Restarting…' };
  var ICONS = {
    local: '<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="12" rx="2"/><path stroke-linecap="round" d="M2 20h20"/></svg>',
    remote: '<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M3 12h18M12 3c2.5 2.6 3.8 5.6 3.8 9s-1.3 6.4-3.8 9c-2.5-2.6-3.8-5.6-3.8-9S9.5 5.6 12 3Z"/></svg>',
  };

  function api() { return window.hyloDesktop; }
  function noop() { }
  function wait(ms) { return new Promise(function (r) { setTimeout(r, Math.max(0, ms)); }); }

  function errorText(e) {
    var code = e && e.code;
    if (code === 'bad_key') return 'The API key was rejected. Run "hylo auth show" on that server to see it.';
    if (code === 'not_hylo') return 'This address does not look like a Hylo server.';
    if (code === 'unreachable') return 'Cannot reach this address. Check the URL and that the server is running.';
    if (code === 'unavailable') return 'This address answers, but Hylo on it is not running.';
    return String((e && e.message) || e || 'Something went wrong.');
  }

  Alpine.data('hyloShell', function () {
    // Retry bookkeeping lives outside Alpine's reactivity: none of it renders.
    var retryTimer = null;
    var splashTimer = null;
    var splashSince = 0;
    var attemptGen = 0;
    var isRetrying = false; // suppresses the "connecting" flash on background retries
    // Why the local server last failed to start. Until the user starts it again,
    // retries only look for it: a respawn would fail the same way.
    var startError = '';
    var probeTimer = null;
    var shell = null;

    return {
      view: 'boot',
      onboardingPending: false,
      // The step a pending onboarding resumes at.
      onboardingStep: '',
      servers: [],
      tryingId: '',
      health: {},
      proc: { managed: false, alive: false },
      busy: '',
      // 'launch' when the app opened on this page, 'app' when the app's pages sent us here.
      openedFrom: 'launch',
      entering: false,
      splash: false,
      status: { state: 'connecting', msg: 'connecting', hint: '', reason: '' },
      showStart: false,
      showDetails: false,

      icons: ICONS,
      menuId: '',
      editId: '',
      edit: { name: '', key: '', err: '', saving: false },
      removeArm: '',
      addOpen: false,
      add: { name: '', url: '', key: '', err: '', busy: false },

      async init() {
        var self = this;
        shell = this;
        window.addEventListener('hashchange', function () { self.route(); });
        probeTimer = setInterval(function () {
          if (self.view === 'servers' && !self.splash && !document.hidden) self.probe();
        }, PROBE_EVERY_MS);

        var a = api();
        if (!a || !a.servers) {
          this.view = 'servers';
          this.entering = true;
          this.setUI('error', 'preload api unavailable', 'Desktop API not found. Check DevTools.');
          document.documentElement.classList.remove('is-booting');
          return;
        }
        await this.checkOnboarding();
        try {
          await this.reloadServers();
        } catch (e) {
          this.view = this.onboardingPending ? 'onboarding' : 'servers';
          this.entering = true;
          this.setUI('error', 'cannot connect', String((e && e.message) || e || ''));
          document.documentElement.classList.remove('is-booting');
          return;
        }
        this.route();
        document.documentElement.classList.remove('is-booting');
      },

      // Decided before anything paints, so a first launch never flashes the servers view.
      async checkOnboarding() {
        try {
          var ob = await api().onboarding.get();
          if (!ob.pending) return;
          this.onboardingStep = ob.step;
          this.onboardingPending = true;
          if (location.hash !== '#servers') this.view = 'onboarding';
        } catch (e) { }
      },

      // Called by onboarding once its window has grown into the main one. Runs on the
      // shell's own scope: through onboarding's, its reveal/current/entering would win.
      leaveOnboarding() {
        shell.onboardingPending = false;
        shell.route();
      },

      route() {
        var manager = location.hash === '#servers';
        attemptGen++;
        clearTimeout(retryTimer);
        if (this.onboardingPending && !manager) {
          this.view = 'onboarding';
          return;
        }
        this.view = 'servers';
        this.showDetails = false;
        this.closeMenu();
        this.closeAdd();
        this.openedFrom = manager ? 'app' : 'launch';
        if (manager) {
          this.entering = false;
          this.reveal();
          this.reloadServers().then(this.probe.bind(this)).catch(noop);
          return;
        }
        this.enter(this.tryingId);
        this.probe();
      },

      // Starts entering a server; its card shows how it goes.
      enter(id, opts) {
        var o = opts || {};
        attemptGen++;
        clearTimeout(retryTimer);
        this.tryingId = id;
        this.entering = true;
        this.showDetails = false;
        isRetrying = false;
        startError = '';
        this.startSplash();
        this.attempt({ userInitiated: !!o.userInitiated });
      },

      // ── The logo ───────────────────────────────────────
      // Alone in the centre while a server is entered; the list comes back
      // only if entering takes long or needs the user.
      startSplash() {
        var self = this;
        clearTimeout(splashTimer);
        splashTimer = setTimeout(function () { self.reveal(); }, SPLASH_MS);
        if (this.splash) return;
        this.splash = true;
        splashSince = Date.now();
      },

      // Holds the logo for its minimum; false if another attempt took over meanwhile.
      async dwell(gen) {
        if (this.splash) await wait(ENTER_MIN_MS - (Date.now() - splashSince));
        return gen === attemptGen;
      },

      reveal() {
        clearTimeout(splashTimer);
        this.splash = false;
      },

      current() {
        var id = this.tryingId;
        return this.servers.find(function (s) { return s.id === id; }) || null;
      },

      setUI(state, msg, hint, reason) {
        if (isRetrying && state === 'connecting') return;
        var next = { state: state, msg: msg, hint: (hint || '').trim(), reason: reason || '' };
        var cur = this.status;
        // Same outcome again: keep the object, so nothing on screen re-renders.
        if (cur.state === next.state && cur.msg === next.msg && cur.hint === next.hint && cur.reason === next.reason) return;
        this.status = next;
      },

      // status.msg is the attempt's state code; this is what the card says for it.
      copy() {
        var remote = this.current() && this.current().kind !== 'local';
        switch (this.status.msg) {
          case 'connecting': return { title: remote ? 'Connecting…' : 'Opening…' };
          case 'connected': return { title: 'Entering…' };
          case 'waiting for server': return { title: 'Still starting…', desc: 'A large notes folder can take a minute to index.' };
          case 'server stopped': return { title: 'Stopped', desc: 'Hylo was stopped on this computer. Start it again to open your notes.' };
          case 'server start failed': return { title: "Couldn't start", desc: this.status.reason };
          case 'cannot connect':
          case 'unreachable': return { title: "Can't reach · trying again" };
          case 'unavailable': return { title: 'Not answering · trying again', desc: 'Something answers at this address, but not Hylo. It may be stopped on that server.' };
          case 'bad_key': return { title: 'API key rejected', desc: 'The server no longer accepts the saved key. Run "hylo auth show" on it to see the current one.' };
          case 'not_hylo': return { title: 'Not a Hylo server', desc: 'This address answers, but not as Hylo.' };
          case 'preload api unavailable': return { title: "Hylo couldn't load", desc: 'The desktop bridge is unavailable.' };
          default: return { title: this.status.msg };
        }
      },

      tone() {
        var msg = this.status.msg;
        if (['connecting', 'connected', 'waiting for server'].indexOf(msg) >= 0) return 'pending';
        return msg === 'server stopped' ? 'idle' : 'error';
      },

      schedule(ms) {
        var self = this;
        clearTimeout(retryTimer);
        retryTimer = setTimeout(function () { self.attempt(); }, ms);
      },

      async debugPaths() {
        try {
          if (api() && api().getShellDebugPaths) return await api().getShellDebugPaths();
        } catch (e) { }
        return {};
      },

      async attempt(opts) {
        var userInitiated = !!(opts && opts.userInitiated);
        var gen = ++attemptGen;
        clearTimeout(retryTimer);
        var s = this.current();
        // A background retry changes nothing on screen until its outcome differs.
        if (!isRetrying) {
          this.showStart = false;
          this.setUI('connecting', 'connecting');
        }
        if (!s) {
          this.reveal();
          this.setUI('error', 'cannot connect', 'No server is registered.');
          return;
        }

        var self = this;
        var a = api();
        // The local server indexes a large folder before it answers.
        var slow = s.kind === 'local' && !startError && setTimeout(function () {
          if (gen === attemptGen) self.setUI('pending', 'waiting for server');
        }, SLOW_START_MS);
        var err = null;
        try {
          await a.servers.check(s.id, { userInitiated: userInitiated, lookOnly: !!startError });
          if (gen !== attemptGen) return;
          // A background retry that finds it up goes in behind the logo too.
          this.startSplash();
          if (!(await this.dwell(gen))) return;
          // From here the logo stays until the server's page replaces it.
          clearTimeout(splashTimer);
          // Entering a server from onboarding's list ends onboarding, once it answers.
          if (this.onboardingPending) {
            this.onboardingPending = false;
            await a.onboarding.finish().catch(noop);
          }
          await a.servers.enter(s.id);
        } catch (e) {
          err = e || {};
        }
        clearTimeout(slow);
        if (gen !== attemptGen) return;
        if (err) return this.fail(s, err, gen);
        isRetrying = false;
        startError = '';
        this.setUI('ok', 'connected');
      },

      // err is the shell's { code, message }; the code says whether retrying can help.
      async fail(s, err, gen) {
        var local = s.kind === 'local';
        var message = err.message || String(err);
        if (!(await this.dwell(gen))) return;
        this.reveal();
        if (local) this.refreshProc();
        if (err.code === 'stopped_by_user') {
          isRetrying = false;
          this.showStart = true;
          this.setUI('error', 'server stopped');
          return;
        }
        if (local) {
          var paths = await this.debugPaths();
          if (gen !== attemptGen) return;
          if (!startError) startError = message;
          isRetrying = true;
          this.showStart = true;
          var log = 'Server output: ' + (paths.serverOutput || '') + '\nShell log: ' + (paths.diagnostics || '');
          this.setUI('error', 'server start failed', log, startError);
          this.schedule(3000);
          return;
        }
        // Only a new key or address helps these; another try would fail the same way.
        var final = err.code === 'bad_key' || err.code === 'not_hylo';
        isRetrying = !final;
        this.showStart = false;
        this.setUI('error', err.code || 'unreachable', message);
        if (!final) this.schedule(3000);
      },

      startServer() {
        this.enter(this.tryingId, { userInitiated: true });
      },

      choose(id) {
        if (!this.servers.some(function (s) { return s.id === id; })) return;
        // Already on its way in; a second click would only restart the attempt.
        if (this.entering && id === this.tryingId && this.tone() === 'pending') return;
        this.enter(id, { userInitiated: true });
      },

      async probe() {
        var a = api() && api().servers;
        if (!a) return;
        this.refreshProc();
        try {
          var out = await a.probe();
          var health = {};
          out.forEach(function (p) { health[p.id] = p.health; });
          this.health = health;
        } catch (e) { }
      },

      async refreshProc() {
        try {
          var p = await api().getServerProcessStatus();
          this.proc = { managed: !!p.managed, alive: !!p.alive };
        } catch (e) { }
      },

      // ── Cards ──────────────────────────────────────────
      // The server being entered carries the launch's status.
      live(s) {
        return this.entering && !this.splash && s.id === this.tryingId;
      },

      local() {
        return this.servers.find(function (s) { return s.kind === 'local'; }) || null;
      },

      // A local server someone else started (the CLI) isn't ours to stop or start.
      canStart() {
        return !this.proc.alive && this.health[this.local() && this.local().id] !== 'online';
      },

      busyOf(s) {
        return !!this.busy && s.kind === 'local';
      },

      pendingOf(s) {
        return this.busyOf(s) || (this.live(s) && this.tone() === 'pending');
      },

      panelOf(s) {
        return this.live(s) && !this.busyOf(s) && !!(this.copy().desc || this.showStart || this.status.hint);
      },

      cardClass(s) {
        var c = [];
        if (this.isCurrent(s) || this.live(s)) c.push('is-current');
        if (this.menuId === s.id) c.push('is-menu');
        if (this.live(s) && !this.busyOf(s)) c.push('is-live', 'is-' + this.tone());
        return c.join(' ');
      },

      // "This computer · Online", or the remote's host and its status.
      metaOf(s) {
        var where = s.kind === 'local' ? 'This computer' : s.url.replace(/^https?:\/\//, '');
        var h;
        if (this.busyOf(s)) h = BUSY_LABEL[this.busy];
        else if (this.live(s)) h = this.copy().title;
        else if (s.kind === 'local' && this.health[s.id] === 'offline' && !this.proc.alive) h = 'Stopped';
        else h = HEALTH_LABEL[this.health[s.id]];
        return h ? where + ' · ' + h : where;
      },

      dotOf(s) {
        if (this.pendingOf(s)) return '';
        if (this.live(s)) return this.tone() === 'idle' ? 'is-idle' : 'is-offline';
        if (s.kind === 'local' && this.health[s.id] === 'offline' && !this.proc.alive) return 'is-idle';
        return this.health[s.id] ? 'is-' + this.health[s.id] : '';
      },

      enterLabel(s) {
        var stopped = this.live(s) ? this.status.msg === 'server stopped' : s.kind === 'local' && this.canStart();
        return stopped ? 'Start' : 'Enter';
      },

      // Start, stop or restart this computer's Hylo without leaving the page.
      async control(kind) {
        var s = this.local();
        var a = api();
        if (!s || !a || this.busy) return;
        this.closeMenu();
        var live = this.entering && this.tryingId === s.id;
        if (live && kind === 'start') return this.startServer();
        if (live) {
          attemptGen++;
          clearTimeout(retryTimer);
        }
        this.busy = kind;
        try {
          if (kind === 'stop') await a.stopServer({ stay: true });
          else if (kind === 'restart') await a.restartServer({ stay: true });
          // Wait until it answers, so the card's status is true when the spinner goes.
          if (kind !== 'stop' && !live) await a.servers.check(s.id, { userInitiated: true });
        } catch (e) { }
        this.busy = '';
        if (live) {
          isRetrying = false;
          startError = '';
          this.attempt();
        }
        this.probe();
      },

      // ── Server manager ─────────────────────────────────
      async reloadServers() {
        var listed = await api().servers.list();
        this.servers = listed.servers || [];
        this.tryingId = listed.activeId || (this.servers[0] && this.servers[0].id) || '';
      },

      // replace(): the list is one page, so it adds nothing to history.
      openManager() { location.replace('#servers'); },
      backToSetup() { location.replace('#'); },

      activeServer() {
        return this.servers.find(function (s) { return s.active; }) || null;
      },

      // Opened from the app, the way back is the server it came from, while that one is up.
      backTarget() {
        if (this.openedFrom !== 'app' || this.onboardingPending) return null;
        var s = this.activeServer();
        if (!s || this.health[s.id] !== 'online') return null;
        if (this.entering && this.tryingId === s.id) return null;
        return s;
      },

      goBack() {
        var s = this.backTarget();
        if (s) this.enter(s.id, { userInitiated: true });
      },

      // "Current" is where Hylo is set to go; hidden while another server is being entered.
      isCurrent(s) {
        return s.active && !(this.entering && this.tryingId !== s.id);
      },

      // At launch the page speaks to why it is showing; opened on purpose it's just the list.
      heading() {
        if (this.openedFrom === 'launch' && this.entering && !this.onboardingPending) {
          var more = this.servers.length > 1 ? 'You can also enter another server below.' : '';
          var t = this.tone();
          if (t === 'pending') return { title: 'Opening Hylo…', lead: more };
          if (t === 'idle') return { title: 'Hylo is stopped', lead: more };
          return { title: "Hylo couldn't open", lead: more };
        }
        return { title: 'Servers', lead: 'Choose which Hylo to enter: the one on this computer, or one you run somewhere else.' };
      },

      toggleMenu(id) {
        this.removeArm = '';
        this.menuId = this.menuId === id ? '' : id;
      },

      closeMenu() {
        this.menuId = '';
        this.removeArm = '';
      },

      // Esc peels one layer: the menu, then an open form. It never enters a server.
      escape() {
        if (this.menuId) this.closeMenu();
        else if (this.editId) this.editId = '';
        else if (this.addOpen) this.closeAdd();
        else if (this.onboardingPending) this.backToSetup();
      },

      drag(e) {
        if (e.button !== 0 || e.target.closest('button, a, input, textarea, select, label, form')) return;
        var win = window.__TAURI__ && window.__TAURI__.window;
        if (win) win.getCurrentWindow().startDragging().catch(noop);
      },

      startEdit(s) {
        this.closeMenu();
        if (this.editId === s.id) {
          this.editId = '';
          return;
        }
        this.addOpen = false;
        this.editId = s.id;
        this.edit = { name: s.name, key: '', err: '', saving: false };
      },

      async saveEdit(s) {
        var opts = { name: this.edit.name };
        if (this.edit.key.trim()) opts.key = this.edit.key.trim();
        this.edit.saving = true;
        this.edit.err = '';
        try {
          await api().servers.update(s.id, opts);
          this.editId = '';
          await this.reloadServers();
          // A rejected key was what stopped it; go in with the new one.
          if (this.live(s) && opts.key) this.startServer();
          this.probe();
        } catch (e) {
          this.edit.err = errorText(e);
        } finally {
          this.edit.saving = false;
        }
      },

      async remove(s) {
        if (this.removeArm !== s.id) {
          this.removeArm = s.id;
          return;
        }
        this.closeMenu();
        try { await api().servers.remove(s.id); } catch (e) { }
        await this.reloadServers();
      },

      openAdd() {
        this.add = { name: '', url: '', key: '', err: '', busy: false };
        this.editId = '';
        this.addOpen = true;
        var self = this;
        this.$nextTick(function () {
          var el = self.$root.querySelector('#sv-add-url');
          if (el) el.focus();
        });
      },

      closeAdd() { this.addOpen = false; },

      addInsecure() {
        try {
          var u = new URL(this.add.url.trim());
          return u.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].indexOf(u.hostname) < 0;
        } catch (e) { return false; }
      },

      async submitAdd() {
        if (!this.add.url.trim() || this.add.busy) return;
        this.add.busy = true;
        this.add.err = '';
        try {
          await api().servers.add({ name: this.add.name, url: this.add.url.trim(), key: this.add.key });
          this.closeAdd();
          await this.reloadServers();
          this.probe();
        } catch (e) {
          this.add.err = errorText(e);
        } finally {
          this.add.busy = false;
        }
      },
    };
  });
});
