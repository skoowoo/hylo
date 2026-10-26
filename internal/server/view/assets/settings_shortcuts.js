  function settingsShortcutsTab() {
    return {
      isMac: /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent),
      customKeys: Object.assign({}, window.__hyloHotkeys.custom),
      recordingId: null,
      shortcutError: '',
      shortcutDefs: [
        { id: 'dismiss',        label: 'Dismiss',                desc: 'Close any open overlay or dialog',          key: 'Esc' },
        { id: 'toggle-search',  label: 'Search',                 desc: 'Open the quick search overlay',             key: 'Mod+K' },
        { id: 'new-note',       label: 'New Note',               desc: 'Open a new blank note in the editor',       key: 'Mod+N', editable: true },
        { id: 'toggle-editor',  label: 'Toggle Editor',          desc: 'Open or close the editor panel',            key: 'Mod+O', editable: true },
        { id: 'close-tab',      label: 'Close Editor Tab',       desc: 'Close the active tab in the editor',        key: 'Mod+W', editable: true },
        { id: 'expand-editor',  label: 'Expand / Shrink Editor', desc: 'Toggle editor between 80% and 100% width',  key: 'Mod+\\', editable: true },
        { id: 'reading-mode',   label: 'Reading Mode',           desc: 'Toggle read-only view for saved notes',     key: 'Mod+L', editable: true },
        { id: 'refresh',        label: 'Refresh',                desc: 'Reload the current page',                   key: 'Mod+R' },
        { id: 'open-settings',  label: 'Settings',               desc: 'Open the settings dialog',                  key: 'Mod+,' },
        { id: 'chat-send',      label: 'Chat: Send',             desc: 'Send the message in the agent chat input',  key: 'Enter',       editable: true, chat: true },
        { id: 'chat-newline',   label: 'Chat: New Line',         desc: 'Insert a line break in the agent chat input', key: 'Shift+Enter', editable: true, chat: true },
      ],
      // Taken by the OS/webview menus or hard-wired elsewhere (find).
      reservedKeys: ['Mod+Q', 'Mod+H', 'Mod+Alt+H', 'Mod+M', 'Mod+X', 'Mod+C', 'Mod+V', 'Mod+A', 'Mod+Z', 'Mod+Y', 'Mod+Shift+Z', 'Mod+F'],
      getCombo(s) { return this.customKeys[s.id] || s.key; },
      fmtCombo(combo) {
        const parts = combo.split('+');
        const key = parts.pop();
        if (!this.isMac) return parts.map(p => p === 'Mod' ? 'Ctrl' : p).concat(key).join('+');
        const sym = { Mod: '⌘', Alt: '⌥', Shift: '⇧', Ctrl: '⌃' };
        const order = ['Ctrl', 'Alt', 'Shift', 'Mod'];
        return order.filter(p => parts.includes(p)).map(p => sym[p]).join('') + key.replace('Arrow', '');
      },
      startRecord(s) {
        this.shortcutError = '';
        this.recordingId = s.id;
        window.__hyloHotkeys.capture((e, combo) => this.onRecorded(s, combo));
      },
      cancelRecord() {
        this.recordingId = null;
        window.__hyloHotkeys.capture(null);
      },
      onRecorded(s, combo) {
        if (!combo || /^(Mod|Alt|Shift|Ctrl|Meta)$/.test(combo)) return;
        if (combo === 'Esc') return this.cancelRecord();
        const err = this.validateCombo(s, combo);
        if (err) { this.shortcutError = err; return; }
        this.cancelRecord();
        this.setCombo(s, combo);
      },
      validateCombo(s, combo) {
        if (s.chat) {
          if (!/(^|\+)Enter$/.test(combo)) return 'Chat shortcuts must use Enter';
        } else if (!combo.startsWith('Mod+') && !/^F\d+$/.test(combo)) return 'Include ' + (this.isMac ? '⌘' : 'Ctrl') + ' or use a function key';
        if (/(^|\+)(Ctrl|Meta)\+/.test(combo)) return 'Unsupported modifier';
        if (this.reservedKeys.includes(combo)) return this.fmtCombo(combo) + ' is reserved by the system';
        const other = this.shortcutDefs.find(d => d.id !== s.id && this.getCombo(d) === combo);
        if (other) return this.fmtCombo(combo) + ' is already used by ' + other.label;
        return '';
      },
      setCombo(s, combo) {
        const next = Object.assign({}, this.customKeys);
        if (combo && combo !== s.key) next[s.id] = combo; else delete next[s.id];
        this.customKeys = next;
        window.__hyloHotkeys.setCustom(s.id, next[s.id] || null);
      },
      resetCombo(s) {
        this.shortcutError = '';
        this.setCombo(s, null);
      },

    };
  }
