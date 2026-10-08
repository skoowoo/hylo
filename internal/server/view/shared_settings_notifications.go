package view

// Notifications tab: desktop-only notification prefs.
const settingsNotificationsCSS = `
    .notif-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 0.35rem; }
    .notif-saved { font-size: var(--text-xs); color: var(--s-ok); margin: 0; font-weight: 500; }
    .notif-sound-row { display: flex; align-items: center; gap: 0.5rem; max-width: 280px; }
`

func notifyToggleRowHTML(field, label, desc string) string {
	return `
                <div>
                  <div class="notif-row">
                    <label class="settings-field-label" style="margin:0">` + label + `</label>
                    <div class="seg">
                      <button type="button" class="seg-btn" :class="{active: !notifySettings.` + field + `}" @click="notifySettings.` + field + ` = false; saveNotifySettings()">Off</button>
                      <button type="button" class="seg-btn" :class="{active: notifySettings.` + field + `}" @click="notifySettings.` + field + ` = true; saveNotifySettings()">On</button>
                    </div>
                  </div>
                  <p class="settings-field-desc">` + desc + `</p>
                </div>`
}

func settingsNotificationsTabHTML() string {
	return `
          <!-- Notifications tab -->
          <div class="settings-scroll-pane" x-show="tab === 'notifications'">
            <div class="cfg-loader" x-show="!isDesktop">Available in the desktop app only.</div>
            <template x-if="isDesktop">
              <div class="settings-fields">
` + notifyToggleRowHTML("textEnabled", "Text Notifications", "Show a system notification when a new inbox message arrives.") +
		notifyToggleRowHTML("soundEnabled", "Sound", "Play a sound when a new inbox message arrives.") + `

                <template x-if="notifySettings.soundEnabled">
                  <div>
                    <label class="settings-field-label">Notification Sound</label>
                    <div class="notif-sound-row">
                      <div style="flex:1;min-width:0">
` + cselectHTML(
		`notifySoundLabel(notifySettings.sound)`,
		`<template x-for="o in notifySounds" :key="o.id"><button type="button" class="cselect-option" :class="notifySettings.sound===o.id?'sel':''" @click="notifySettings.sound=o.id; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span x-text="o.label"></span></button></template>`,
	) + `
                      </div>
                      <button type="button" class="icon-btn icon-btn--lg" title="Preview sound"
                              :disabled="notifySettings.sound === 'none'"
                              @click="window.hyloDesktop?.inboxNotify?.previewSound(notifySettings.sound)">
                        <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 4v16l13-8z"/></svg>
                      </button>
                    </div>
                    <p class="settings-field-desc">Sound played when a new inbox message arrives.</p>
                  </div>
                </template>

                <p class="notif-saved" x-show="notifySaveOk">Saved</p>

              </div>
            </template>
          </div>

`
}

const settingsNotificationsJS = `
      notifySettings: { textEnabled: true, soundEnabled: true, sound: 'beep' },
      notifySaveOk: false,

      notifySounds: [
        { id: 'beep', label: 'System beep' }, { id: 'none', label: 'None' },
        { id: 'Glass', label: 'Glass' }, { id: 'Ping', label: 'Ping' }, { id: 'Pop', label: 'Pop' },
        { id: 'Tink', label: 'Tink' }, { id: 'Hero', label: 'Hero' }, { id: 'Purr', label: 'Purr' },
        { id: 'Submarine', label: 'Submarine' },
      ],
      notifySoundLabel(v) {
        const o = this.notifySounds.find(x => x.id === v);
        return o ? o.label : v;
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
