package view

// Shortcuts tab: keyboard shortcut reference list.
const settingsShortcutsCSS = `
    .shortcuts-fields { max-width: 640px; }
    .shortcuts-list { display: flex; flex-direction: column; }
    .shortcuts-row {
      display: flex; align-items: center; justify-content: space-between;
      padding: 0.55rem 0; border-bottom: var(--bd-w) solid var(--hairline); gap: 1rem;
    }
    .shortcuts-list .shortcuts-row:last-child { border-bottom: none; }
    .shortcuts-row-meta { min-width: 0; flex: 1; }
    .shortcuts-label { font-size: var(--text-sm); font-weight: 600; color: var(--fg); display: block; }
    .shortcuts-desc { font-size: var(--text-xs); color: var(--muted); margin-top: 0.1rem; }
    .shortcuts-keys { display: flex; gap: 0.3rem; align-items: center; flex-shrink: 0; }
    .kbd {
      display: inline-flex; align-items: center;
      padding: 0.18rem 0.42rem;
      border-radius: var(--r-xs);
      background: var(--bg); border: var(--bd-w) solid var(--border-strong);
      font-family: var(--font-mono);
      font-size: var(--text-xs); color: var(--fg); white-space: nowrap; line-height: 1.4;
    }
    /* .kbd-cmd sizing/alignment now comes from the shared .kbd-combo
       (base.css) — see that rule for why. */
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
                    <div class="shortcuts-keys">
                      <template x-for="k in getEffectiveKeys(s)" :key="k">
                        <span class="kbd kbd-combo" x-html="hyloKbdHTML(k)"></span>
                      </template>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div><!-- .shortcuts-pane -->

`
}

const settingsShortcutsJS = `
      isMac: /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent),
      customKeys: JSON.parse(localStorage.getItem('hylo-custom-keys') || '{}'),
      shortcutDefs: [
        { id: 'dismiss',        label: 'Dismiss',                desc: 'Close any open overlay or dialog',          mac: 'Esc',  win: 'Esc' },
        { id: 'toggle-search',  label: 'Search',                 desc: 'Open the quick search overlay',             mac: '⌘K',   win: 'Ctrl+K' },
        { id: 'new-note',       label: 'New Note',               desc: 'Open a new blank note in the editor',       mac: '⌘N',   win: 'Ctrl+N' },
        { id: 'toggle-editor',  label: 'Toggle Editor',          desc: 'Open or close the editor panel',            mac: '⌘O',   win: 'Ctrl+O' },
        { id: 'close-tab',      label: 'Close Editor Tab',       desc: 'Close the active tab in the editor',        mac: '⌘W',   win: 'Ctrl+W' },
        { id: 'expand-editor',  label: 'Expand / Shrink Editor', desc: 'Toggle editor between 80% and 100% width',  mac: '⌘\\',  win: 'Ctrl+\\' },
        { id: 'reading-mode',   label: 'Reading Mode',           desc: 'Toggle read-only view for saved notes',     mac: '⌘L',   win: 'Ctrl+L' },
        { id: 'refresh',        label: 'Refresh',               desc: 'Reload the current page',                   mac: '⌘R',   win: 'Ctrl+R' },
        { id: 'open-settings',  label: 'Settings',               desc: 'Open the settings dialog',                  mac: '⌘,',   win: 'Ctrl+,' },
      ],
      getEffectiveKeys(s) {
        const custom = this.customKeys[s.id];
        const key = this.isMac ? (custom?.mac ?? s.mac) : (custom?.win ?? s.win);
        return key.split('/').map(k => k.trim());
      },

`
