package view

// Shortcuts tab: keyboard shortcut reference list.
const settingsShortcutsCSS = `
    .shortcuts-fields { max-width: 640px; }
    .shortcuts-list { display: flex; flex-direction: column; }
    .shortcuts-row {
      display: flex; align-items: center; justify-content: space-between;
      padding: 0.6rem 0; gap: 1rem;
    }
    .shortcuts-row-meta { min-width: 0; flex: 1; }
    .shortcuts-label { font-size: var(--text-sm); font-weight: 600; color: var(--fg); display: block; }
    .shortcuts-desc { font-size: var(--text-xs); color: var(--muted); margin-top: 0.1rem; }
    .shortcuts-keys { display: flex; gap: 0.3rem; align-items: center; flex-shrink: 0; }
    .kbd {
      display: inline-flex; align-items: center;
      padding: 0.18rem 0.42rem;
      border-radius: var(--r-xs);
      background: var(--btn-bg); border: var(--bd-w) solid transparent;
      font-family: var(--font-mono);
      font-size: var(--text-xs); color: var(--fg); white-space: nowrap; line-height: 1.4;
    }
    button.kbd { cursor: pointer; }
    button.kbd:hover { background: var(--btn-bg-hov); }
    .kbd-recording { border-color: var(--accent, var(--fg)); color: var(--muted); }
    .shortcuts-btn {
      background: none; border: none; padding: 0; cursor: pointer;
      font-size: var(--text-xs); color: var(--muted);
    }
    .shortcuts-btn:hover { color: var(--fg); }
    .shortcuts-error { flex-basis: 100%; text-align: right; font-size: var(--text-xs); color: var(--danger, #d33); }
    .shortcuts-row { flex-wrap: wrap; }
`

func settingsShortcutsTabHTML() string {
	return `
          <!-- Shortcuts tab -->
          <div class="settings-scroll-pane" x-show="tab === 'shortcuts'">
            <div class="shortcuts-fields">
              <div class="shortcuts-list">
                <template x-for="s in shortcutDefs" :key="s.id">
                  <div class="shortcuts-row hairline">
                    <div class="shortcuts-row-meta">
                      <span class="shortcuts-label" x-text="s.label"></span>
                      <span class="shortcuts-desc" x-text="s.desc"></span>
                    </div>
                    <div class="shortcuts-keys" @click.outside="recordingId === s.id && cancelRecord()">
                      <template x-if="s.editable && customKeys[s.id] && recordingId !== s.id">
                        <button type="button" class="shortcuts-btn" @click="resetCombo(s)">Reset</button>
                      </template>
                      <button type="button" class="kbd kbd-combo" x-show="s.editable"
                              :class="{ 'kbd-recording': recordingId === s.id }"
                              @click="recordingId === s.id ? cancelRecord() : startRecord(s)"
                              x-html="recordingId === s.id ? 'Press shortcut…' : hyloKbdHTML(fmtCombo(getCombo(s)))"></button>
                      <span class="kbd kbd-combo" x-show="!s.editable" x-html="hyloKbdHTML(fmtCombo(getCombo(s)))"></span>
                    </div>
                    <div class="shortcuts-error" x-show="recordingId === s.id && shortcutError" x-text="shortcutError"></div>
                  </div>
                </template>
              </div>
            </div>
          </div><!-- .shortcuts-pane -->

`
}

const settingsShortcutsJS = `
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

`
