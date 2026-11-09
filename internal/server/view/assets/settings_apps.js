  function settingsAppsTab() {
    return {
      appsList: [],
      appsSignIn: false, // this page can take a browser sign-in (it runs on the server's machine)
      appsLoading: false,
      appsError: '',
      appsOpen: '',      // the app whose connect panel is open
      appsSecret: '',
      appsUseToken: false,
      appsSignInURL: '',
      appsWaiting: false, // the sign-in page is open in the browser
      appsBusy: '',
      appsMsg: '',

      get connectedApps() {
        return this.appsList.filter(function(a) { return a.connected; });
      },

      // check asks every platform whether its connection still works.
      async loadApps(check) {
        this.appsLoading = true;
        this.appsError = '';
        try {
          const r = await fetch(check ? '/api/apps/check' : '/api/apps', { method: check ? 'POST' : 'GET' });
          if (!r.ok) throw new Error('HTTP ' + r.status);
          const d = await r.json();
          this.appsList = d.apps || [];
          this.appsSignIn = !!d.signIn;
          const open = this.appsList.find((a) => a.id === this.appsOpen);
          if (open && open.connected && !open.reconnect) this.closeAppConnect(true);
        } catch (e) { this.appsError = e.message; }
        finally { this.appsLoading = false; }
      },

      appLine(a) {
        if (a.problem) return a.problem;
        if (!a.connected) return a.about;
        const bots = (a.bots || []).map(function(b) { return b.name; });
        return [a.account || 'Connected', bots.length ? 'On for ' + bots.join(', ') : 'On for no bot'].join(' · ');
      },

      async openAppConnect(a) {
        if (this.appsOpen) this.closeAppConnect();
        this.appsOpen = a.id;
        this.appsSecret = '';
        this.appsMsg = '';
        this.appsUseToken = false;
        this.appsWaiting = false;
        this.appsSignInURL = '';
        if (a.kind === 'mcp' && this.appsSignIn) {
          // Prepared up front, so the click opens the page itself and no popup blocker steps in.
          try {
            const r = await fetch('/api/apps/' + encodeURIComponent(a.id) + '/sign-in', { method: 'POST' });
            const d = await r.json();
            if (this.appsOpen !== a.id) return;
            if (!r.ok) {
              this.appsUseToken = true;
              this.appsMsg = d.error || 'Sign-in is unavailable';
              return;
            }
            this.appsSignInURL = d.url;
          } catch (e) {
            this.appsUseToken = true;
            this.appsMsg = e.message;
          }
          return;
        }
        this.$nextTick(() => this.$refs.appSecret && this.$refs.appSecret.focus());
      },

      // done: the app just connected, so there is no sign-in left to drop.
      closeAppConnect(done) {
        const id = this.appsOpen;
        if (!id) return;
        if (!done && this.appsSignInURL) fetch('/api/apps/' + encodeURIComponent(id) + '/sign-in', { method: 'DELETE' });
        this.appsOpen = '';
        this.appsSecret = '';
        this.appsMsg = '';
        this.appsSignInURL = '';
        this.appsWaiting = false;
      },

      async submitAppSecret(a) {
        const secret = this.appsSecret.trim();
        if (!secret || this.appsBusy === a.id) return;
        this.appsBusy = a.id;
        this.appsMsg = '';
        try {
          const r = await fetch('/api/apps/' + encodeURIComponent(a.id) + '/connect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ method: a.kind === 'skill' ? 'key' : 'token', secret: secret }),
          });
          const d = await r.json();
          if (!r.ok) {
            this.appsMsg = d.error || 'Connect failed';
            return;
          }
          this.closeAppConnect(true);
          await this.loadApps();
        } catch (e) { this.appsMsg = e.message; }
        finally { this.appsBusy = ''; }
      },

      async disconnectApp(a) {
        const bots = (a.bots || []).map(function(b) { return b.name; });
        const who = bots.length ? bots.join(', ') + ' will no longer see your ' + a.name + ' data.' : 'No bot uses it now.';
        const where = a.method === 'oauth' ? ' Hylo also signs out of ' + a.name + '.' : '';
        const ok = await window.showConfirm({ title: 'Disconnect ' + a.name, message: who + where, confirmLabel: 'Disconnect', danger: true });
        if (!ok) return;
        this.appsBusy = a.id;
        try {
          const r = await fetch('/api/apps/' + encodeURIComponent(a.id), { method: 'DELETE' });
          if (!r.ok) {
            const d = await r.json().catch(() => ({}));
            window.showError(d.error || 'Disconnect failed', 'Disconnect failed');
          }
          await this.loadApps();
        } catch (e) { window.showError(e.message, 'Disconnect failed'); }
        finally { this.appsBusy = ''; }
      },
    };
  }
