package view

// WeChat iLink login panel, shown under the plugins.wechat section of the
// Server tab. Its fields are schema-driven like every other section; only
// the QR login flow lives here.
func settingsWechatHTML() string {
	return `
                    <div class="cfg-link hairline" x-show="openSection === 'plugins.wechat'">
                      <div class="cfg-link-head">
                        <span class="cfg-link-title">WeChat login</span>
                        <span class="badge"
                              :class="{ 'badge--ok': wechatStatus.connected }"
                              x-text="wechatStatus.connected ? 'Connected' : 'Not connected'"></span>
                      </div>
                      <p class="cfg-link-meta">
                        Scan with WeChat to obtain an iLink bot token. Credentials are saved to
                        <code>config.toml</code>. Create a bot with a <code>wechat_message</code> trigger to handle replies.
                      </p>
                      <template x-if="wechatStatus.connected">
                        <div>
                          <p class="cfg-link-meta" x-show="wechatStatus.account_id">
                            Bot ID: <code x-text="wechatStatus.account_id"></code>
                            <span x-show="wechatStatus.saved_at"> · saved <span x-text="wechatStatus.saved_at"></span></span>
                          </p>
                          <div class="cfg-link-actions">
                            <button type="button" class="btn-solid btn-solid--danger"
                                    @click="wechatLogout()"
                                    :disabled="wechatAuthBusy"
                                    x-text="wechatAuthBusy ? 'Disconnecting…' : 'Disconnect'"></button>
                          </div>
                        </div>
                      </template>
                      <template x-if="!wechatStatus.connected">
                        <div>
                          <div class="cfg-link-actions">
                            <button type="button" class="btn-solid"
                                    @click="startWechatLogin()"
                                    :disabled="wechatAuthBusy || wechatLoginBusy"
                                    x-text="wechatLoginBusy ? 'Waiting for scan…' : 'Scan QR to log in'"></button>
                            <button type="button" class="btn-solid"
                                    x-show="wechatLoginBusy"
                                    @click="cancelWechatLogin()">Cancel</button>
                          </div>
                          <div class="cfg-link-qr" x-show="wechatQrcodeImg">
                            <img :src="wechatQrcodeImg" alt="WeChat login QR code">
                            <p class="cfg-link-status" x-show="wechatLoginStatus" x-text="wechatLoginStatus"></p>
                          </div>
                        </div>
                      </template>
                      <p class="cfg-link-err" x-show="wechatAuthError" x-text="wechatAuthError"></p>
                      <p class="cfg-link-ok" x-show="wechatLoginOk">Connected — restart the server to start the bridge.</p>
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
