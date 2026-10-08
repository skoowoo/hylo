package view

// WeChat iLink login panel, shown under the plugins.wechat section of the
// Server tab. Its fields are schema-driven like every other section; only
// the QR login flow lives here.
const settingsWechatCSS = `
    .cfg-wechat-auth { padding: 0.88rem 0 0; margin-top: 0.5rem; }
    .cfg-wechat-auth-head {
      display: flex; align-items: center; justify-content: space-between;
      gap: 0.75rem; margin-bottom: 0.65rem;
    }
    .cfg-wechat-auth-title { font-size: var(--text-sm); font-weight: 500; color: var(--body); }
    .cfg-wechat-meta { font-size: var(--text-xs); color: var(--muted); line-height: 1.55; margin: 0 0 0.75rem; }
    .cfg-wechat-meta code {
      font-size: var(--text-2xs); padding: 0.05rem 0.3rem; border-radius: var(--r-xs);
      background: var(--code-bg);
    }
    .cfg-wechat-actions { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; }
    .cfg-wechat-qr {
      margin-top: 0.85rem; padding: 0.75rem;
      background: var(--code-bg); text-align: center;
    }
    .cfg-wechat-qr img { width: 180px; height: 180px; object-fit: contain; background: #fff; }
    .cfg-wechat-status { font-size: var(--text-xs); color: var(--muted); margin-top: 0.5rem; }
    .cfg-wechat-err { font-size: var(--text-xs); color: var(--s-err); margin-top: 0.5rem; }
    .cfg-wechat-ok { font-size: var(--text-xs); color: var(--s-ok); margin-top: 0.5rem; }
`

func settingsWechatHTML() string {
	return `
                    <div class="cfg-wechat-auth hairline" x-show="openSection === 'plugins.wechat'">
                      <div class="cfg-wechat-auth-head">
                        <span class="cfg-wechat-auth-title">WeChat login</span>
                        <span class="badge"
                              :class="{ 'badge--ok': wechatStatus.connected }"
                              x-text="wechatStatus.connected ? 'Connected' : 'Not connected'"></span>
                      </div>
                      <p class="cfg-wechat-meta">
                        Scan with WeChat to obtain an iLink bot token. Credentials are saved to
                        <code>config.toml</code>. Create an agent bot with a <code>wechat_message</code> trigger to handle replies.
                      </p>
                      <template x-if="wechatStatus.connected">
                        <div>
                          <p class="cfg-wechat-meta" x-show="wechatStatus.account_id">
                            Bot ID: <code x-text="wechatStatus.account_id"></code>
                            <span x-show="wechatStatus.saved_at"> · saved <span x-text="wechatStatus.saved_at"></span></span>
                          </p>
                          <div class="cfg-wechat-actions">
                            <button type="button" class="btn-solid btn-solid--danger"
                                    @click="wechatLogout()"
                                    :disabled="wechatAuthBusy"
                                    x-text="wechatAuthBusy ? 'Disconnecting…' : 'Disconnect'"></button>
                          </div>
                        </div>
                      </template>
                      <template x-if="!wechatStatus.connected">
                        <div>
                          <div class="cfg-wechat-actions">
                            <button type="button" class="btn-solid"
                                    @click="startWechatLogin()"
                                    :disabled="wechatAuthBusy || wechatLoginBusy"
                                    x-text="wechatLoginBusy ? 'Waiting for scan…' : 'Scan QR to log in'"></button>
                            <button type="button" class="btn-solid"
                                    x-show="wechatLoginBusy"
                                    @click="cancelWechatLogin()">Cancel</button>
                          </div>
                          <div class="cfg-wechat-qr" x-show="wechatQrcodeImg">
                            <img :src="wechatQrcodeImg" alt="WeChat login QR code">
                            <p class="cfg-wechat-status" x-show="wechatLoginStatus" x-text="wechatLoginStatus"></p>
                          </div>
                        </div>
                      </template>
                      <p class="cfg-wechat-err" x-show="wechatAuthError" x-text="wechatAuthError"></p>
                      <p class="cfg-wechat-ok" x-show="wechatLoginOk">Connected — restart the server to start the bridge.</p>
                    </div>
`
}

const settingsWechatJS = `
      wechatStatus: { connected: false, account_id: '', saved_at: '' },
      wechatAuthBusy: false,
      wechatAuthError: '',
      wechatLoginBusy: false,
      wechatLoginStatus: '',
      wechatQrcode: '',
      wechatQrcodeImg: '',
      wechatPollTimer: null,
      wechatLoginOk: false,

      async loadWechatStatus() {
        try {
          const r = await fetch('/api/wechat/status');
          if (!r.ok) return;
          const d = await r.json();
          this.wechatStatus = {
            connected: !!d.connected,
            account_id: d.account_id || '',
            saved_at: d.saved_at || '',
          };
        } catch { /* noop */ }
      },

      cancelWechatLogin() {
        if (this.wechatPollTimer) { clearInterval(this.wechatPollTimer); this.wechatPollTimer = null; }
        this.wechatLoginBusy = false;
        this.wechatLoginStatus = '';
        this.wechatQrcode = '';
        this.wechatQrcodeImg = '';
      },

      wechatStatusLabel(status) {
        // Keys mirror the WeChat iLink bridge's own status vocabulary
        // (internal/wechat/auth.go) verbatim — 'scaned' is that upstream
        // API's spelling, not a typo here; don't "fix" it or the status
        // label silently stops matching.
        const m = { wait: 'Open WeChat and scan the QR code', scaned: 'QR scanned — confirm login in WeChat', expired: 'QR expired — fetching a new code…', confirmed: 'Login successful' };
        return m[status] || status;
      },

      async startWechatLogin() {
        this.wechatAuthError = '';
        this.wechatLoginOk = false;
        this.wechatAuthBusy = true;
        try {
          const r = await fetch('/api/wechat/login/start', { method: 'POST' });
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Failed to start login');
          this.wechatQrcode = d.qrcode || '';
          this.wechatQrcodeImg = d.qrcode_image || '';
          this.wechatLoginBusy = true;
          this.wechatLoginStatus = this.wechatStatusLabel('wait');
          if (this.wechatPollTimer) clearInterval(this.wechatPollTimer);
          this.wechatPollTimer = setInterval(() => this.pollWechatLogin(), 1500);
          await this.pollWechatLogin();
        } catch (e) { this.wechatAuthError = e.message; this.cancelWechatLogin(); }
        finally { this.wechatAuthBusy = false; }
      },

      async pollWechatLogin() {
        if (!this.wechatQrcode) return;
        try {
          const r = await fetch('/api/wechat/login/status?qrcode=' + encodeURIComponent(this.wechatQrcode));
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Login poll failed');
          if (d.qrcode && d.qrcode !== this.wechatQrcode) this.wechatQrcode = d.qrcode;
          if (d.qrcode_image) this.wechatQrcodeImg = d.qrcode_image;
          if (d.status) this.wechatLoginStatus = this.wechatStatusLabel(d.status);
          if (d.status === 'confirmed') {
            this.cancelWechatLogin();
            this.wechatLoginOk = true;
            await this.loadWechatStatus();
            await this.loadConfig();
          }
        } catch (e) { this.wechatAuthError = e.message; this.cancelWechatLogin(); }
      },

      async wechatLogout() {
        this.wechatAuthError = '';
        this.wechatLoginOk = false;
        this.wechatAuthBusy = true;
        try {
          const r = await fetch('/api/wechat/logout', { method: 'POST' });
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Logout failed');
          this.wechatStatus = { connected: false, account_id: '', saved_at: '' };
          await this.loadConfig();
        } catch (e) { this.wechatAuthError = e.message; }
        finally { this.wechatAuthBusy = false; }
      },

`
