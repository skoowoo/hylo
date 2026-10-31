  function settingsNotificationsTab() {
    return {
      notifySettings: { textEnabled: true, soundEnabled: true, sound: 'beep' },

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
        if (!window.hyloDesktop?.notify) return;
        try {
          this.notifySettings = await window.hyloDesktop.notify.getSettings();
        } catch(_) {}
      },

      async saveNotifySettings() {
        if (!window.hyloDesktop?.notify) return;
        try {
          // Spread to a plain object so the desktop IPC bridge's structured clone
          // doesn't silently drop the Alpine.js reactive Proxy wrapper.
          const snap = {
            textEnabled:  this.notifySettings.textEnabled,
            soundEnabled: this.notifySettings.soundEnabled,
            sound:        this.notifySettings.sound,
          };
          this.notifySettings = await window.hyloDesktop.notify.setSettings(snap);
        } catch(e) { console.error('[notify] saveNotifySettings error:', e); }
      },
    };
  }
