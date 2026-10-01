package view

// Server tab: server/connection config, the Config section picker ↔
// detail (see cfg-section-grid/cfg-detail in the CSS below), and the
// WeChat/Discord plugin auth flows that live inside it.
const settingsServerCSS = `
    /* ── Server config ────────────────────────────────────────── */
    .cfg-content { flex: 1; min-width: 0; display: flex; flex-direction: column; overflow: hidden; }
    /* Docked, not sticky — a flex sibling *outside* .cfg-pane-area's
       scroll viewport, not a sticky child inside it. Plain sticky here
       measured against the wrong box in this nested absolute+flex
       layout (it settled short of the real bottom edge, leaving the
       last line of content peeking out below the bar) — a non-scrolling
       footer row sidesteps that whole class of bug instead of chasing it. */
    .cfg-action-bar {
      flex-shrink: 0; display: flex; align-items: center;
      justify-content: space-between; flex-wrap: wrap;
      gap: 0.75rem 1rem; padding: 1rem 1.5rem;
      background: var(--bg);
      border-top: var(--bd-w) solid var(--hairline);
    }
    .cfg-action-bar-inner { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 0.75rem 1rem; width: 100%; max-width: 640px; }
    .cfg-action-left { display: flex; align-items: center; gap: 0.75rem; flex: 1; min-width: 0; }
    .cfg-action-right { display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0; }
    .cfg-status-ok { font-size: var(--text-xs); color: var(--s-ok); font-weight: 500; }
    .cfg-status-err {
      font-size: var(--text-xs); color: var(--s-err); font-weight: 500;
      overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
      border-radius: 0;
    }
    .cfg-restart-note { font-size: var(--text-xs); color: var(--muted); }
    .cfg-pane-area { flex: 1; min-height: 0; position: relative; overflow: hidden; }
    .cfg-pane {
      position: absolute; inset: 0; overflow-y: auto;
      padding: 1.75rem 1.5rem;
    }
    .cfg-fields { max-width: 640px; display: flex; flex-direction: column; }

    /* Section picker — a grid of cards, not a stacked accordion, so every
       section is one click away instead of sitting behind however many
       other sections happen to come before it in scroll order. */
    .cfg-section-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem; max-width: 640px; }
    /* Box/hover/cursor/surface from the shared .list-card.list-card--clickable
       .list-card--soft (base.css) — same recipe as .agent-bot-template-card below. */
    .cfg-section-card {
      display: flex; flex-direction: column; gap: 0.3rem; text-align: left;
    }
    .cfg-section-card-top { display: flex; align-items: center; justify-content: space-between; gap: 0.5rem; }
    .cfg-section-card-title { font-size: var(--text-sm); font-weight: 600; color: var(--fg); }
    .cfg-section-card-desc { font-size: var(--text-xs); color: var(--muted); line-height: 1.5; }

    /* Section detail — replaces the grid (list ↔ detail, same swap Agent
       Bots uses) instead of expanding in place, so opening one section
       never pushes the others further down the page. */
    .cfg-detail { max-width: 640px; }
    .cfg-detail-header { margin-bottom: 1.1rem; }
    .cfg-detail-title { font-size: var(--text-base); font-weight: 600; color: var(--fg); }
    .cfg-detail-desc { font-size: var(--text-sm); color: var(--muted); line-height: 1.5; margin: -0.6rem 0 1.25rem; }
    .cfg-field {
      display: grid;
      grid-template-columns: 1fr 220px;
      grid-template-areas: "meta ctrl" "desc desc";
      column-gap: 1rem; padding: 0.88rem 0;
      border-bottom: var(--bd-w) solid var(--hairline); align-items: center;
    }
    .cfg-field.multiline {
      grid-template-columns: 1fr;
      grid-template-areas: "meta" "ctrl" "desc";
      align-items: start;
    }
    .cfg-field.multiline .cfg-field-ctrl { justify-content: stretch; margin-top: 0.5rem; }
    .cfg-fields .cfg-field:last-of-type { border-bottom: none; }
    .cfg-field.dirty .cfg-field-label::before {
      content: ''; display: inline-block; width: 5px; height: 5px;
      border-radius: 50%; background: var(--accent);
      margin-right: 5px; vertical-align: middle; margin-bottom: 1px;
    }
    .cfg-field-meta { grid-area: meta; min-width: 0; }
    .cfg-field-ctrl { grid-area: ctrl; display: flex; align-items: center; justify-content: flex-end; }
    .cfg-field-label { font-size: var(--text-sm); font-weight: 500; color: var(--body); display: block; margin-bottom: 0.15rem; }
    .cfg-field-key {
      font-size: var(--text-xs); color: var(--muted); opacity: 0.65;
      font-family: var(--font-mono);
    }
    .cfg-field-desc { grid-area: desc; font-size: var(--text-xs); color: var(--muted); margin: 0.35rem 0 0; line-height: 1.5; }
    .cfg-wechat-auth { padding: 0.88rem 0 0; border-top: var(--bd-w) solid var(--hairline); }
    .cfg-wechat-auth-head {
      display: flex; align-items: center; justify-content: space-between;
      gap: 0.75rem; margin-bottom: 0.65rem;
    }
    .cfg-wechat-auth-title { font-size: var(--text-sm); font-weight: 500; color: var(--body); }
    /* .cfg-wechat-badge now uses the shared .badge / .badge--ok (base.css). */
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
    /* .cfg-input / .cfg-textarea now use the shared .field-input (base.css). */
    input[type="number"].field-input.cfg-input { width: 110px; }
    .cfg-reveal-wrap { display: flex; gap: 0.375rem; align-items: center; width: 100%; }
    /* .cfg-reveal-btn now uses the shared .icon-btn.icon-btn--lg (base.css). */
    /* Bool config fields now use the shared .seg/.seg-btn Off/On toggle
       (base.css) — same picker as Theme/Line Breaks above, not the old
       pill switch. */
    .cfg-loader { font-size: var(--text-xs); color: var(--muted); padding: 2rem 0; }
    .cfg-err-msg { font-size: var(--text-xs); color: var(--s-err); padding: 2rem 0; }

`

func settingsServerTabHTML() string {
	return `
          <!-- Server tab -->
          <div class="cfg-content" x-show="tab === 'server'">
            <div class="cfg-pane-area">
              <div class="cfg-pane">

                <div x-show="isElectron" style="max-width:640px; margin-bottom:1.75rem;">
                  <label class="settings-field-label">Connection</label>
                  <div class="settings-field-row">
                    <input type="url" class="field-input settings-input"
                           x-model="serverUrl"
                           @keydown.enter="applyServerUrl()"
                           placeholder="http://localhost:54321"
                           spellcheck="false">
                    <button class="btn-solid"
                            @click="applyServerUrl()"
                            :disabled="urlSaving"
                            x-text="urlSaving ? 'Applying…' : 'Apply'"></button>
                  </div>
                  <div class="settings-error" x-show="urlError" x-text="urlError"></div>
                  <p class="settings-field-desc">Hylo server address used by the desktop app. Changes reload immediately.</p>
                </div>

                <div x-show="isElectron && serverManaged" style="max-width:640px; margin-bottom:1.75rem;">
                  <label class="settings-field-label">Process</label>
                  <div>
                    <button class="btn-solid btn-solid--danger"
                            @click="stopServer()"
                            :disabled="!serverRunning || serverStopping"
                            x-text="serverStopping ? 'Stopping…' : 'Stop Server'"></button>
                  </div>
                  <div class="settings-error" x-show="serverStopError" x-text="serverStopError"></div>
                  <p class="settings-field-desc">Server process started and managed by this desktop app.</p>
                </div>

                <div x-show="cfgLoading" class="cfg-loader">Loading configuration…</div>
                <div x-show="cfgError && !cfgLoading" class="cfg-err-msg" x-text="'Error: ' + cfgError"></div>

                <!-- Section picker — every section is a card, one click away,
                     instead of a stacked accordion you scroll past. -->
                <template x-if="!cfgLoading && !cfgError && !openSection">
                  <div>
                    <label class="settings-field-label" style="margin-bottom:0.75rem;display:block;">Config</label>
                    <div class="cfg-section-grid">
                      <template x-for="section in sectionTabs" :key="section">
                        <button type="button" class="list-card list-card--clickable list-card--soft cfg-section-card" @click="openSectionDetail(section)">
                          <div class="cfg-section-card-top">
                            <span class="cfg-section-card-title" x-text="sectionLabel(section)"></span>
                            <span class="dot dot--accent" x-show="sectionHasDirty(section)" title="Unsaved changes in this section"></span>
                          </div>
                          <span class="cfg-section-card-desc" x-text="sectionIntro(section)"></span>
                        </button>
                      </template>
                    </div>
                  </div>
                </template>

                <!-- Section detail — replaces the picker (same list↔detail
                     swap Agent Bots uses) so opening one section never
                     pushes the others further down the page. -->
                <template x-if="!cfgLoading && !cfgError && openSection">
                  <div class="cfg-detail">
                    <div class="settings-detail-header cfg-detail-header">
` + agentBotBackBtnHTML("openSection = ''", "Config") + `
                      <span class="cfg-detail-title" x-text="sectionLabel(openSection)"></span>
                    </div>
                    <p class="cfg-detail-desc" x-show="sectionIntro(openSection)" x-text="sectionIntro(openSection)"></p>

                    <div class="cfg-fields">
                      <template x-for="field in fieldsForSection(openSection)" :key="field.key">
                        <div class="cfg-field hairline" :class="{dirty: isDirty(field.key), multiline: field.multiline}">
                          <div class="cfg-field-meta">
                            <span class="cfg-field-label" x-text="field.label"></span>
                            <code class="cfg-field-key" x-text="field.key"></code>
                          </div>
                          <div class="cfg-field-ctrl">
                            <template x-if="field.type === 'bool'">
                              <div class="seg">
                                <button type="button" class="seg-btn" :class="{active: !getVal(field.key)}" @click="setVal(field.key, false)">Off</button>
                                <button type="button" class="seg-btn" :class="{active: !!getVal(field.key)}" @click="setVal(field.key, true)">On</button>
                              </div>
                            </template>
                            <template x-if="field.type === 'string' && field.enum && field.enum.length">
                              <select class="field-input cfg-input" @change="setVal(field.key, $event.target.value)">
                                <template x-for="opt in (field.enum || [])" :key="opt">
                                  <option :value="opt" :selected="getVal(field.key) === opt" x-text="opt"></option>
                                </template>
                              </select>
                            </template>
                            <template x-if="field.type === 'int'">
                              <input type="number" class="field-input cfg-input"
                                     :value="getVal(field.key)"
                                     @change="setVal(field.key, Number($event.target.value))"
                                     :min="field.constraints ? field.constraints.min : undefined"
                                     :max="field.constraints ? field.constraints.max : undefined">
                            </template>
                            <template x-if="field.type === 'string_list'">
                              <textarea class="field-input cfg-textarea" rows="3" placeholder="One entry per line"
                                        :value="listToText(getVal(field.key))"
                                        @change="setVal(field.key, textToList($event.target.value))"></textarea>
                            </template>
                            <template x-if="field.sensitive && field.type !== 'bool' && field.type !== 'string_list'">
                              <div class="cfg-reveal-wrap">
                                <input class="field-input cfg-input"
                                       :type="revealed[field.key] ? 'text' : 'password'"
                                       :value="getVal(field.key) || ''"
                                       :placeholder="(secrets[field.key] && !getVal(field.key)) ? '••• set •••' : (field.default != null ? String(field.default) : '')"
                                       @input="setVal(field.key, $event.target.value)">
                                <button type="button" class="icon-btn icon-btn--lg"
                                        @click.stop="toggleReveal(field.key)"
                                        :title="revealed[field.key] ? 'Hide' : 'Reveal'">
                                  <svg x-show="!revealed[field.key]" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/>
                                  </svg>
                                  <svg x-show="revealed[field.key]" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M10.733 5.076a10.744 10.744 0 0 1 11.205 6.575 1 1 0 0 1 0 .696 10.747 10.747 0 0 1-1.444 2.49"/>
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M14.084 14.158a3 3 0 0 1-4.242-4.242"/>
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M17.479 17.499a10.75 10.75 0 0 1-15.417-5.151 1 1 0 0 1 0-.696 10.75 10.75 0 0 1 4.446-5.143"/>
                                    <path stroke-linecap="round" stroke-linejoin="round" d="m2 2 20 20"/>
                                  </svg>
                                </button>
                              </div>
                            </template>
                            <template x-if="!field.sensitive && field.type === 'string' && field.multiline && !(field.enum && field.enum.length)">
                              <textarea class="field-input cfg-textarea" rows="6"
                                        :value="getVal(field.key) ?? ''"
                                        :placeholder="field.default != null ? String(field.default) : ''"
                                        @input="setVal(field.key, $event.target.value)"></textarea>
                            </template>
                            <template x-if="!field.sensitive && (field.type === 'string' || field.type === 'duration') && !field.multiline && !(field.enum && field.enum.length)">
                              <div style="display:flex;gap:0.375rem;align-items:center;width:100%;">
                                <input type="text" class="field-input cfg-input" style="flex:1;min-width:0;"
                                       :value="getVal(field.key) ?? ''"
                                       :placeholder="field.default != null ? String(field.default) : ''"
                                       @input="setVal(field.key, $event.target.value)">
                                <template x-if="field.key === 'vault.path' && isElectron">
                                  <button type="button" class="icon-btn icon-btn--lg"
                                          title="Browse for folder"
                                          style="font-size:13px;letter-spacing:0.05em;padding:0 6px;width:auto;"
                                          @click="pickFolder(field.key, getVal(field.key))">···</button>
                                </template>
                              </div>
                            </template>
                          </div>
                          <p class="cfg-field-desc" x-text="field.description"></p>
                        </div>
                      </template>
                    </div>

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

                    <div class="cfg-wechat-auth hairline" x-show="openSection === 'plugins.discord'">
                      <div class="cfg-wechat-auth-head">
                        <span class="cfg-wechat-auth-title">Discord Bot status</span>
                        <span class="badge"
                              :class="{ 'badge--ok': !!getVal('plugins.discord.bot_token') }"
                              x-text="getVal('plugins.discord.bot_token') ? 'Token set' : 'Not configured'"></span>
                      </div>
                      <p class="cfg-wechat-meta">
                        Paste your Bot token above, then restart the server. Create an agent bot with a
                        <code>discord_message</code> trigger to handle replies. The bot must share a server
                        with you before it can send proactive DMs.
                      </p>
                      <template x-if="getVal('plugins.discord.user_id')">
                        <p class="cfg-wechat-meta">
                          Owner ID: <code x-text="getVal('plugins.discord.user_id')"></code>
                        </p>
                      </template>
                    </div>
                  </div>
                </template>

              </div>
            </div>

            <div class="cfg-action-bar hairline" x-show="!cfgLoading && !cfgError">
              <div class="cfg-action-bar-inner">
                <div class="cfg-action-left">
                  <span class="cfg-status-ok" x-show="cfgSaveOk && !hasDirty">Saved — restart server to apply</span>
                  <span class="cfg-status-err" x-show="cfgSaveError" x-text="cfgSaveError"></span>
                  <span class="cfg-status-err" x-show="cfgRestartError" x-text="cfgRestartError"></span>
                  <span class="cfg-restart-note" x-show="cfgRestarting">Restarting server…</span>
                  <span class="badge" x-show="hasDirty"
                        x-text="Object.keys(patch).length + ' unsaved change' + (Object.keys(patch).length !== 1 ? 's' : '')"></span>
                  <span class="cfg-restart-note"
                        x-show="!hasDirty && !cfgSaveOk && !cfgSaveError && !cfgRestartError && !cfgRestarting">
                    Pick a section to edit. Save once when done; restart server to apply.
                  </span>
                </div>
                <div class="cfg-action-right">
                  <button class="btn-outline" x-show="hasDirty" @click="discardAll()">Discard</button>
                  <button class="btn-solid"
                          @click="saveConfig()"
                          :disabled="!hasDirty || cfgSaving || cfgRestarting"
                          x-text="cfgRestarting ? 'Restarting…' : cfgSaving ? 'Saving…' : 'Save all'"></button>
                </div>
              </div>
            </div>
          </div><!-- .cfg-content server -->

`
}

const settingsServerJS = `
      serverUrl: '',
      urlSaving: false,
      urlError: '',
      serverManaged: false,
      serverRunning: false,
      serverStopping: false,
      serverStopError: '',

      schema: [],
      values: {},
      secrets: {},
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

      wechatStatus: { connected: false, account_id: '', saved_at: '', enabled: false },
      wechatAuthBusy: false,
      wechatAuthError: '',
      wechatLoginBusy: false,
      wechatLoginStatus: '',
      wechatQrcode: '',
      wechatQrcodeImg: '',
      wechatPollTimer: null,
      wechatLoginOk: false,

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
          'server': 'Server', 'vault': 'Vault', 'agent': 'Agent',
          'plugins.search': 'Search', 'plugins.git_sync': 'Git Sync',
          'plugins.compile': 'Compile',
          'plugins.wechat': 'WeChat',
          'plugins.discord': 'Discord',
        };
        return m[s] || s;
      },

      sectionIntro(s) {
        const m = {
          'vault': 'Paths, layout, and core vault behavior.',
          'agent': 'Agent CLI integration and global prompt settings.',
          'plugins.search': 'Indexing and search quality.',
          'plugins.git_sync': 'Automatic git push/pull for the vault.',
          'plugins.compile': 'AI knowledge compilation and related options.',
          'plugins.wechat': 'WeChat iLink bridge — poll DMs and emit wechat_message agent bot events.',
          'plugins.discord': 'Discord Bot bridge — receive DMs and emit discord_message agent bot events.',
          'server.listen': 'HTTP listen address and port.',
          'server': 'HTTP listen address and port.',
        };
        return m[s] || '';
      },

      fieldsForSection(s) { return this.schema.filter(f => f.section === s); },

      openSectionDetail(s) {
        this.openSection = s;
        if (s === 'plugins.wechat') this.loadWechatStatus();
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
          this.openSection = '';
          await this.loadWechatStatus();
        } catch (e) { this.cfgError = e.message; }
        finally { this.cfgLoading = false; }
      },

      async loadWechatStatus() {
        try {
          const r = await fetch('/api/wechat/status');
          if (!r.ok) return;
          const d = await r.json();
          this.wechatStatus = {
            connected: !!d.connected,
            account_id: d.account_id || '',
            saved_at: d.saved_at || '',
            enabled: !!d.enabled,
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
          this.wechatStatus = { connected: false, account_id: '', saved_at: '', enabled: false };
          await this.loadConfig();
        } catch (e) { this.wechatAuthError = e.message; }
        finally { this.wechatAuthBusy = false; }
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
            const prevSection = this.openSection;
            await this.loadConfig();
            this.openSection = prevSection;
            saved = true;
          }
        } catch (e) { this.cfgSaveError = e.message; }
        finally { this.cfgSaving = false; }

        if (saved && window.hyloDesktop?.restartServer) {
          this.cfgRestarting = true;
          try {
            const r = await window.hyloDesktop.restartServer();
            if (r.ok) return;
            if (r.reason === 'no_pid') { this.cfgSaveOk = true; }
            else { this.cfgRestartError = r.error || 'Restart failed'; }
          } catch (e) { this.cfgRestartError = e.message; }
          finally { this.cfgRestarting = false; }
        } else if (saved) {
          this.cfgSaveOk = true;
        }
      },

      async applyServerUrl() {
        this.urlError = '';
        const raw = this.serverUrl.trim().replace(/\/$/, '');
        try {
          const p = new URL(raw);
          if (!['http:', 'https:'].includes(p.protocol)) throw new Error('Must use http:// or https://');
          this.urlSaving = true;
          await window.hyloDesktop.setServerUrl(raw);
        } catch (e) { this.urlError = e.message; this.urlSaving = false; }
      },

      async pickFolder(key, currentVal) {
        if (!window.hyloDesktop?.pickFolder) return;
        const result = await window.hyloDesktop.pickFolder({
          title: 'Select Vault Root Folder',
          defaultPath: currentVal || undefined,
        });
        if (!result.canceled && result.path) this.setVal(key, result.path);
      },

`
