package view

// Notifications tab: desktop-only notification prefs.
const settingsNotificationsCSS = `
    .notif-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 0.35rem; }
    .notif-saved { font-size: var(--text-xs); color: var(--s-ok); margin: 0; font-weight: 500; }
    .notif-sound-row { display: flex; align-items: center; gap: 0.5rem; max-width: 280px; }
    /* .notif-play-btn now uses the shared .icon-btn.icon-btn--lg (base.css). */
`

func settingsNotificationsTabHTML() string {
	return `
          <!-- Notifications tab -->
          <div class="settings-scroll-pane" x-show="tab === 'notifications'">
            <div class="cfg-loader" x-show="!isDesktop">Available in the desktop app only.</div>
            <template x-if="isDesktop">
              <div class="settings-fields">

                <div>
                  <div class="notif-row">
                    <label class="settings-field-label" style="margin:0">Text Notifications</label>
                    <div class="seg">
                      <button type="button" class="seg-btn" :class="{active: !notifySettings.textEnabled}" @click="notifySettings.textEnabled = false; saveNotifySettings()">Off</button>
                      <button type="button" class="seg-btn" :class="{active: notifySettings.textEnabled}" @click="notifySettings.textEnabled = true; saveNotifySettings()">On</button>
                    </div>
                  </div>
                  <p class="settings-field-desc">Show a system notification when a new inbox message arrives.</p>
                </div>

                <div>
                  <div class="notif-row">
                    <label class="settings-field-label" style="margin:0">Sound</label>
                    <div class="seg">
                      <button type="button" class="seg-btn" :class="{active: !notifySettings.soundEnabled}" @click="notifySettings.soundEnabled = false; saveNotifySettings()">Off</button>
                      <button type="button" class="seg-btn" :class="{active: notifySettings.soundEnabled}" @click="notifySettings.soundEnabled = true; saveNotifySettings()">On</button>
                    </div>
                  </div>
                  <p class="settings-field-desc">Play a sound when a new inbox message arrives.</p>
                </div>

                <template x-if="notifySettings.soundEnabled">
                  <div class="settings-fields" style="gap:1.25rem">

                    <div>
                      <label class="settings-field-label">Notification Sound</label>
                      <div class="notif-sound-row">
                        <div class="cselect" x-data="{ csOpen: false }" @click.outside="csOpen = false" style="flex:1;min-width:0">
                          <button type="button" class="cselect-btn" :class="{open: csOpen}" @click="csOpen = !csOpen" @keydown.escape="csOpen = false">
                            <span class="cselect-btn-text" x-text="notifySoundLabel(notifySettings.sound)"></span>
                            <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/></svg>
                          </button>
                          <div class="cselect-dropdown" x-show="csOpen">
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='beep'?'sel':''" @click="notifySettings.sound='beep'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>System beep</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='none'?'sel':''" @click="notifySettings.sound='none'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>None</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Glass'?'sel':''" @click="notifySettings.sound='Glass'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Glass</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Ping'?'sel':''" @click="notifySettings.sound='Ping'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Ping</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Pop'?'sel':''" @click="notifySettings.sound='Pop'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Pop</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Tink'?'sel':''" @click="notifySettings.sound='Tink'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Tink</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Hero'?'sel':''" @click="notifySettings.sound='Hero'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Hero</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Purr'?'sel':''" @click="notifySettings.sound='Purr'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Purr</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Submarine'?'sel':''" @click="notifySettings.sound='Submarine'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Submarine</span></button>
                          </div>
                        </div>
                        <button type="button" class="icon-btn icon-btn--lg" title="Preview sound"
                                :disabled="notifySettings.sound === 'none'"
                                @click="window.hyloDesktop?.inboxNotify?.previewSound(notifySettings.sound)">
                          <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 4v16l13-8z"/></svg>
                        </button>
                      </div>
                      <p class="settings-field-desc">Sound played when a new inbox message arrives.</p>
                    </div>

                  </div>
                </template>

                <p class="notif-saved" x-show="notifySaveOk">Saved</p>

              </div>
            </template>
          </div><!-- .notifications-pane -->

`
}

const settingsNotificationsJS = `
      notifySettings: { textEnabled: true, soundEnabled: true, sound: 'beep' },
      notifySaveOk: false,

      notifySoundLabel(v) {
        const m = { beep: 'System beep', none: 'None', Glass: 'Glass', Ping: 'Ping', Pop: 'Pop', Tink: 'Tink', Hero: 'Hero', Purr: 'Purr', Submarine: 'Submarine' };
        return m[v] || v;
      },

      async loadNotifySettings() {
        if (!window.hyloDesktop?.inboxNotify) return;
        try {
          this.notifySettings = await window.hyloDesktop.inboxNotify.getSettings();
        } catch(_) {}
      },

      async saveNotifySettings() {
        if (!window.hyloDesktop?.inboxNotify) return;
        try {
          // Spread to a plain object so the desktop IPC bridge's structured clone
          // doesn't silently drop the Alpine.js reactive Proxy wrapper.
          const snap = {
            textEnabled:  this.notifySettings.textEnabled,
            soundEnabled: this.notifySettings.soundEnabled,
            sound:        this.notifySettings.sound,
          };
          this.notifySettings = await window.hyloDesktop.inboxNotify.setSettings(snap);
          this.notifySaveOk = true;
          setTimeout(() => { this.notifySaveOk = false; }, 1500);
        } catch(e) { console.error('[notify] saveNotifySettings error:', e); }
      },
`
