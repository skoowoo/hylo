package view

// Appearance tab: theme/accent/line-breaks preferences. Split out of
// shared_settings_modal.go (which still assembles every tab's CSS/HTML/JS
// into the one modal) so each tab's chunk is reviewable on its own.
const settingsAppearanceCSS = `
    /* ── Appearance fields ────────────────────────────────────── */
    .settings-fields { max-width: 640px; display: flex; flex-direction: column; gap: 1.75rem; }
    .settings-field-label {
      display: block; font-size: var(--text-sm); font-weight: 600;
      color: var(--fg); margin-bottom: 0.5rem;
    }
    .settings-field-desc { font-size: var(--text-xs); color: var(--muted); margin-top: 0.4rem; line-height: 1.5; }
    .settings-field-row { display: flex; gap: 0.5rem; align-items: center; }
    /* Box/font come from the shared .field-input (base.css); this is
       inline next to a button in a flex row, not full-width. */
    .settings-input { flex: 1; min-width: 0; }
    /* Apply/save/toolbar/danger buttons all now use the shared
       .btn-outline / .btn-solid / .btn-solid--danger classes (base.css). */
    .settings-error { margin-top: 0.4rem; font-size: var(--text-xs); color: var(--s-err); }

    /* Theme/Enter-Effect/schedule-kind pickers use the shared .seg/.seg-btn
       component (base.css) instead of their own copy. */

    .accent-swatches { display: flex; flex-wrap: wrap; gap: 0.75rem; padding: 0.4rem; }
    /* Ring is a pseudo-element border, not outline: the page-wide
       "button:focus { outline: none }" (images.css/shorts.css) would wipe an
       outline the moment a click focuses the swatch. */
    .accent-swatch {
      position: relative; width: 1.5rem; height: 1.5rem; padding: 0; border: none;
      cursor: pointer; border-radius: var(--r-full); background: var(--sw);
    }
    .accent-swatch::after {
      content: ''; position: absolute; inset: -5px; border-radius: var(--r-full);
      border: 2px solid transparent; transition: border-color var(--motion-fast);
    }
    .accent-swatch:hover::after { border-color: rgba(var(--ink-rgb), 0.25); }
    .accent-swatch.active::after { border-color: var(--sw); }

`

func settingsAppearanceTabHTML() string {
	return `
          <!-- Appearance tab -->
          <div class="settings-scroll-pane" x-show="tab === 'appearance'">
            <div class="settings-fields">
              <div>
                <label class="settings-field-label">Theme</label>
                <div class="seg">
                  <button class="seg-btn" :class="{active: themePref==='light'}" @click="setTheme('light')">Light</button>
                  <button class="seg-btn" :class="{active: themePref==='dark'}" @click="setTheme('dark')">Dark</button>
                  <button class="seg-btn" :class="{active: themePref==='auto'}" @click="setTheme('auto')">Auto</button>
                </div>
                <p class="settings-field-desc">Light, dark, or match your system setting. Takes effect immediately.</p>
              </div>
              <div>
                <label class="settings-field-label">Accent Color</label>
                <div class="accent-swatches" role="radiogroup" aria-label="Accent color">
                  <template x-for="p in accentPresets" :key="p.id">
                    <button type="button" class="accent-swatch" role="radio"
                            :class="{active: accentPref===p.id}" :aria-checked="accentPref===p.id"
                            :style="'--sw:' + p.a" :title="p.name" :aria-label="p.name"
                            @click="setAccent(p.id)"></button>
                  </template>
                </div>
                <p class="settings-field-desc">Brand color for buttons, links and text selection. Takes effect immediately.</p>
              </div>
              <div>
                <label class="settings-field-label">Line Breaks</label>
                <div class="seg">
                  <button class="seg-btn" :class="{active: lineBreaksPref==='loose'}" @click="setLineBreaks('loose')">Loose</button>
                  <button class="seg-btn" :class="{active: lineBreaksPref==='strict'}" @click="setLineBreaks('strict')">Strict</button>
                </div>
                <p class="settings-field-desc">How a single line break inside a paragraph is interpreted when opening or pasting Markdown. Loose shows it as a visible break, matching Obsidian's default. Strict merges it into flowing text like plain CommonMark — use this if you paste text that was manually wrapped to a fixed width. Applies the next time that text is parsed (reopen the note, or switch out of source view).</p>
              </div>
            </div>
          </div>

`
}

const settingsAppearanceJS = `
      // Read live by the editor's remark pipeline on every parse (breaks.js),
      // not threaded through Editor.make() config — so this takes effect on
      // the next parse without needing the editor to be recreated.
      lineBreaksPref: localStorage.getItem('hylo-line-breaks') || 'loose',
      setLineBreaks(key) { this.lineBreaksPref = key; localStorage.setItem('hylo-line-breaks', key); },

      // themePref mirrors what themeBootstrapScript already resolved at
      // first paint (light/dark/auto) — switching here just persists the
      // new choice and re-runs that same resolution logic immediately via
      // window.__hyloApplyTheme (defined once in <head>, shared by both).
      themePref: localStorage.getItem('hylo-theme') || 'auto',
      setTheme(key) {
        this.themePref = key;
        localStorage.setItem('hylo-theme', key);
        if (window.__hyloApplyTheme) window.__hyloApplyTheme(key);
      },

      // Presets come from accentBootstrapScript (shared_accent.go), which also
      // owns applying them; this just persists the pick and re-runs it.
      accentPresets: window.__hyloAccentPresets || [],
      accentPref: (function() {
        var id = localStorage.getItem('hylo-accent');
        return (window.__hyloAccentPresets || []).some(function(p) { return p.id === id; }) ? id : 'indigo';
      })(),
      setAccent(id) {
        this.accentPref = id;
        localStorage.setItem('hylo-accent', id);
        if (window.__hyloApplyAccent) window.__hyloApplyAccent(id);
      },

`
