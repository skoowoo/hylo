package view

// Telegram pairing panel, shown under the plugins.telegram section of the
// Server tab: the owner pastes a bot token, then opens the bot's /start link
// on their phone, which binds the bot to their account.
func settingsTelegramHTML() string {
	return `
                    <div class="cfg-link hairline" x-show="openSection === 'plugins.telegram'">
                      <div class="cfg-link-head">
                        <span class="cfg-link-title">Telegram bot</span>
                        <span class="badge"
                              :class="{ 'badge--ok': telegramStatus.connected }"
                              x-text="telegramStatus.connected ? 'Connected' : 'Not connected'"></span>
                      </div>
                      <template x-if="telegramStatus.connected">
                        <div>
                          <p class="cfg-link-meta">
                            Paired with <code x-text="'@' + telegramStatus.username"></code>. Only your Telegram account can talk to it.
                          </p>
                          <div class="cfg-link-actions">
                            <button type="button" class="btn-solid btn-solid--danger"
                                    @click="telegramUnpair()"
                                    :disabled="telegramBusy"
                                    x-text="telegramBusy ? 'Disconnecting…' : 'Disconnect'"></button>
                          </div>
                        </div>
                      </template>
                      <template x-if="!telegramStatus.connected">
                        <div>
                          <p class="cfg-link-meta">
                            Create a bot with <a href="https://t.me/BotFather" target="_blank" rel="noopener noreferrer">@BotFather</a>
                            and paste its token. Then open the link it gives you on your phone and tap Start:
                            the account that does becomes the only one the bot answers.
                          </p>
                          <div class="cfg-link-actions" x-show="!telegramCode">
                            <input type="password" class="field-input cfg-input" autocomplete="off" spellcheck="false"
                                   placeholder="123456:ABC-DEF…" x-model="telegramToken"
                                   @keydown.enter.prevent="startTelegramPairing()">
                            <button type="button" class="btn-solid"
                                    @click="startTelegramPairing()"
                                    :disabled="telegramBusy || !telegramToken.trim()"
                                    x-text="telegramBusy ? 'Checking…' : 'Connect'"></button>
                          </div>
                          <div x-show="telegramCode">
                            <div class="cfg-link-qr">
                              <img :src="telegramQrcodeImg" alt="Telegram bot link QR code" x-show="telegramQrcodeImg">
                              <p class="cfg-link-status">
                                Scan with your phone, or open
                                <a :href="telegramLink" target="_blank" rel="noopener noreferrer" x-text="'@' + telegramUsername"></a>
                                in Telegram and tap Start.
                              </p>
                            </div>
                            <div class="cfg-link-actions">
                              <button type="button" class="btn-solid" @click="cancelTelegramPairing()">Cancel</button>
                            </div>
                          </div>
                        </div>
                      </template>
                      <p class="cfg-link-err" x-show="telegramError" x-text="telegramError"></p>
                      <p class="cfg-link-ok" x-show="telegramOk">Connected — restart the server to start the bridge.</p>
                    </div>
`
}

const settingsTelegramJS = `
      telegramStatus: { connected: false, username: '' },
      telegramToken: '',
      telegramBusy: false,
      telegramError: '',
      telegramOk: false,
      telegramCode: '',
      telegramLink: '',
      telegramUsername: '',
      telegramQrcodeImg: '',
      telegramPollTimer: null,

      async loadTelegramStatus() {
        try {
          const r = await fetch('/api/telegram/status');
          if (!r.ok) return;
          const d = await r.json();
          this.telegramStatus = { connected: !!d.connected, username: d.username || '' };
        } catch { /* noop */ }
      },

      cancelTelegramPairing() {
        if (this.telegramPollTimer) { clearInterval(this.telegramPollTimer); this.telegramPollTimer = null; }
        this.telegramCode = '';
        this.telegramLink = '';
        this.telegramQrcodeImg = '';
      },

      async startTelegramPairing() {
        if (this.telegramBusy || !this.telegramToken.trim()) return;
        this.telegramError = '';
        this.telegramOk = false;
        this.telegramBusy = true;
        try {
          const r = await fetch('/api/telegram/pair/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bot_token: this.telegramToken.trim(), proxy_url: this.getVal('plugins.telegram.proxy_url') || '' }),
          });
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Failed to start pairing');
          this.telegramCode = d.code;
          this.telegramLink = d.link;
          this.telegramUsername = d.username;
          this.telegramQrcodeImg = d.qrcode_image || '';
          if (this.telegramPollTimer) clearInterval(this.telegramPollTimer);
          this.telegramPollTimer = setInterval(() => this.pollTelegramPairing(), 2000);
        } catch (e) { this.telegramError = e.message; }
        finally { this.telegramBusy = false; }
      },

      async pollTelegramPairing() {
        const code = this.telegramCode;
        if (!code) return;
        try {
          const r = await fetch('/api/telegram/pair/status?code=' + encodeURIComponent(code));
          const d = await r.json();
          if (code !== this.telegramCode) return;
          if (!r.ok) throw new Error(d.error || 'Pairing failed');
          if (d.status === 'expired') throw new Error('The link expired — connect again.');
          if (d.status === 'confirmed') {
            this.cancelTelegramPairing();
            this.telegramToken = '';
            this.telegramOk = true;
            await this.loadTelegramStatus();
            await this.loadConfig();
          }
        } catch (e) { this.telegramError = e.message; this.cancelTelegramPairing(); }
      },

      async telegramUnpair() {
        this.telegramError = '';
        this.telegramOk = false;
        this.telegramBusy = true;
        try {
          const r = await fetch('/api/telegram/unpair', { method: 'POST' });
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Disconnect failed');
          this.telegramStatus = { connected: false, username: '' };
          await this.loadConfig();
        } catch (e) { this.telegramError = e.message; }
        finally { this.telegramBusy = false; }
      },

`
