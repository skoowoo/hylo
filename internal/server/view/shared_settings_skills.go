package view

// Skills tab: skills added to ~/.hylo/skills, and the catalog they can be added from.
const settingsSkillsCSS = `
    .skills-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; margin-bottom: 0.75rem; }
    .skills-tabs .seg-btn { gap: 0.4rem; }
    .skills-tab-count {
      min-width: 1.1em; font-size: var(--text-2xs); font-variant-numeric: tabular-nums;
      color: var(--muted); opacity: 0.75;
    }
    .skills-tabs .seg-btn.active .skills-tab-count { color: inherit; opacity: 0.55; }
    .skills-hint { font-size: var(--text-xs); color: var(--muted); line-height: 1.5; margin: 0 0 1rem; min-height: 1.5em; }
    .skills-hint code {
      font-family: var(--font-mono); font-size: var(--text-2xs);
      padding: 0.05rem 0.3rem; border-radius: var(--r-xs); background: var(--code-bg);
    }
    .skills-empty {
      display: flex; flex-direction: column; align-items: center; gap: 0.5rem;
      font-size: var(--text-sm); color: var(--muted); padding: 2.5rem 0; text-align: center;
    }
    .skills-empty-link {
      border: none; background: none; padding: 0; font: inherit; font-size: var(--text-sm);
      color: var(--accent-text); border-radius: 0;
    }
    .skills-empty-link:hover { text-decoration: underline; }
    .skills-builtin { margin-bottom: 0.5rem; }
    .skills-builtin:has(.skills-builtin-row[aria-expanded="true"]) {
      margin-bottom: 1.25rem; padding-bottom: 1.25rem; border-bottom: 1px solid var(--line-hair); border-radius: 0;
    }
    .skills-builtin-row {
      display: flex; align-items: center; gap: 0.6rem; width: 100%;
      padding: 0.6rem 0.75rem 0.6rem 0.9rem; text-align: left;
      font-family: inherit; color: var(--fg);
      transition: background-color var(--motion-fast);
    }
    .skills-builtin-row:hover { background: var(--card-hov); }
    .skills-builtin-row:focus-visible { outline: 2px solid rgba(var(--accent-rgb), 0.5); outline-offset: 1px; }
    .skills-builtin-text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 0.1rem; }
    .skills-builtin-title { font-size: var(--text-sm); font-weight: var(--fw-semibold); }
    .skills-builtin-names {
      font-size: var(--text-xs); color: var(--muted);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    }
    .skills-builtin-chev { width: 14px; height: 14px; flex-shrink: 0; color: var(--muted); transition: transform var(--motion-fast); }
    .skills-builtin-row[aria-expanded="true"] .skills-builtin-chev { transform: rotate(90deg); }
    .skills-builtin-grid { margin-top: 0.5rem; }
    .skills-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(232px, 1fr)); gap: 0.5rem; }

    /* Surface from the shared .list-card (base.css); not --clickable since
       the card's controls are the targets. */
    .skill-card {
      display: flex; flex-direction: column; gap: 0.5rem; min-width: 0;
      padding: 0.8rem 0.8rem 0.7rem 0.9rem;
      transition: background-color var(--motion-fast);
    }
    .skill-card:hover { background: var(--card-hov); }
    .skill-card-head { display: flex; align-items: center; gap: 0.55rem; min-width: 0; }
    .skill-icon {
      display: grid; place-items: center; width: 26px; height: 26px; flex-shrink: 0;
      border-radius: var(--r-md); background: var(--btn-bg); color: var(--muted);
      transition: background-color var(--motion-fast), color var(--motion-fast);
    }
    .skill-icon svg { width: 13px; height: 13px; }
    .skill-card .skill-icon { position: relative; }
    .skill-card.is-on .skill-icon, .skills-builtin-row .skill-icon { background: rgba(var(--accent-rgb), 0.12); color: var(--accent-text); }
    /* Same green dot as "Always on", ringed in the card color so it sits on the tile edge. */
    .skill-card .skill-icon::after {
      content: ""; position: absolute; right: -2px; bottom: -2px; width: 8px; height: 8px;
      border-radius: var(--r-full); background: var(--s-ok); box-shadow: 0 0 0 2px var(--surface-soft);
      opacity: 0; transform: scale(0.4); transition: opacity var(--motion-fast), transform var(--motion-fast), box-shadow var(--motion-fast);
    }
    /* --card-hov is translucent; mix it opaque so the ring still matches the hovered card. */
    .skill-card:hover .skill-icon::after { box-shadow: 0 0 0 2px color-mix(in srgb, rgb(var(--accent-rgb)) 10%, var(--surface-soft)); }
    .skill-card.is-on .skill-icon::after { opacity: 1; transform: none; }
    .skill-name {
      flex: 1; min-width: 0; font-size: var(--text-sm); font-weight: var(--fw-semibold); color: var(--fg);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    }
    .skill-card.is-off .skill-name, .skill-card--available .skill-name { color: var(--muted); }
    /* Remove is rare and destructive, so it stays out of sight until the card is in use. */
    .skill-remove { opacity: 0; transition: opacity var(--motion-fast), color var(--motion-fast), background-color var(--motion-fast); }
    .skill-card:hover .skill-remove, .skill-remove:focus-visible, .skill-remove:disabled { opacity: 1; }
    .skill-remove:hover:not(:disabled) { color: var(--s-err); background: var(--s-err-bg); }
    @media (hover: none) { .skill-remove { opacity: 1; } }
    .skill-desc {
      font-size: var(--text-xs); color: var(--muted); line-height: 1.5;
      min-height: 3em; overflow-wrap: anywhere;
      display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden;
    }
    .skill-desc.is-empty { opacity: 0.6; font-style: italic; }
    .skill-card-foot {
      display: flex; align-items: center; justify-content: space-between; gap: 0.5rem;
      margin-top: auto; min-height: var(--btn-h-xs);
    }
    .skill-source {
      min-width: 0; font-family: var(--font-mono); font-size: var(--text-2xs); color: var(--muted);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      text-decoration: none; opacity: 0.7; border-radius: 0;
      transition: color var(--motion-fast), opacity var(--motion-fast);
    }
    a.skill-source:hover { color: var(--accent-text); opacity: 1; }
    .skill-fixed { display: inline-flex; align-items: center; gap: 0.35rem; font-size: var(--text-2xs); color: var(--muted); flex-shrink: 0; }
    .skill-seg { padding: 2px; }
    .skill-seg .seg-btn { padding: 0.25rem 0.5rem; font-size: var(--text-xs); }
`

func settingsSkillsTabHTML() string {
	const sparkle = `<svg fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z"/></svg>`
	const trash = `<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/></svg>`
	const plus = `<svg fill="none" stroke="currentColor" stroke-width="1.9" viewBox="0 0 24 24"><path stroke-linecap="round" d="M12 5v14M5 12h14"/></svg>`
	source := `
                      <template x-if="s.repo">
                        <a class="skill-source" :href="'https://github.com/' + s.repo" target="_blank" rel="noopener noreferrer"
                           :title="s.repo" x-text="s.repo"></a>
                      </template>
                      <template x-if="!s.repo">
                        <span class="skill-source" x-text="s.default ? 'hylo' : 'local'"></span>
                      </template>`
	desc := `
                    <p class="skill-desc" :class="{'is-empty': !s.description}" :title="s.description || ''"
                       x-text="s.description || (s.added ? 'No description.' : 'Add to use with your agents.')"></p>`
	card := `
                  <div class="list-card skill-card" :class="!s.added ? 'skill-card--available' : (s.enabled ? 'is-on' : 'is-off')">
                    <div class="skill-card-head">
                      <span class="skill-icon">` + sparkle + `</span>
                      <span class="skill-name" x-text="s.name" :title="s.name"></span>
                      <template x-if="s.added && !s.default">
                        <button type="button" class="icon-btn-ghost icon-btn-ghost--sm skill-remove"
                                :disabled="!!skillsRemoving[s.name]"
                                :title="skillsRemoving[s.name] ? 'Removing…' : 'Remove'" aria-label="Remove"
                                @click="removeSkill(s.name)">` + trash + `</button>
                      </template>
                    </div>` + desc + `
                    <div class="skill-card-foot">` + source + `
                      <template x-if="s.default">
                        <span class="skill-fixed"><span class="dot dot--on"></span>Always on</span>
                      </template>
                      <template x-if="s.added && !s.default">
                        <div class="seg skill-seg">
                          <button type="button" class="seg-btn" :class="{active: !s.enabled}"
                                  :disabled="!!skillsToggling[s.name]" @click="setSkillEnabled(s.name, false)">Off</button>
                          <button type="button" class="seg-btn" :class="{active: s.enabled}"
                                  :disabled="!!skillsToggling[s.name]" @click="setSkillEnabled(s.name, true)">On</button>
                        </div>
                      </template>
                      <template x-if="!s.added">
                        <button type="button" class="btn-outline btn--xs"
                                :disabled="!!skillsAdding[s.name]"
                                @click="addSkill(s)">
                          <template x-if="!skillsAdding[s.name]">` + plus + `</template>
                          <span x-text="skillsAdding[s.name] ? 'Adding…' : 'Add'"></span>
                        </button>
                      </template>
                    </div>
                  </div>`
	return `
          <!-- Skills tab -->
          <div class="settings-scroll-pane" x-show="tab === 'skills'">
            <div class="skills-toolbar">
              <div class="seg skills-tabs" role="tablist">
                <button type="button" class="seg-btn" role="tab" :class="{active: skillsTab === 'installed'}"
                        :aria-selected="skillsTab === 'installed'" @click="skillsTab = 'installed'">
                  Installed <span class="skills-tab-count" x-show="!skillsLoading" x-text="skillsList.length"></span>
                </button>
                <button type="button" class="seg-btn" role="tab" :class="{active: skillsTab === 'recommended'}"
                        :aria-selected="skillsTab === 'recommended'" @click="skillsTab = 'recommended'">
                  Recommended <span class="skills-tab-count" x-show="!skillsLoading" x-text="recommendedSkills.length"></span>
                </button>
              </div>
` + toolbarRefreshBtnHTML("skillsLoading", "loadSkills()", "Loading…") + `
            </div>
            <p class="skills-hint" x-show="skillsTab === 'installed'">Skills in <code>~/.hylo/skills/</code>. New ones start off.</p>
            <p class="skills-hint" x-show="skillsTab === 'recommended'">Curated skills you can add from GitHub, then turn on.</p>
            <div class="cfg-err-msg" x-show="skillsError && !skillsLoading" x-text="'Error: ' + skillsError"></div>
            <div class="cfg-loader" x-show="skillsLoading">Loading skills…</div>
            <div x-show="!skillsLoading && skillsTab === 'installed'">
              <template x-if="builtinSkills.length">
                <div class="skills-builtin">
                  <button type="button" class="list-card skills-builtin-row" :aria-expanded="skillsBuiltinOpen"
                          @click="skillsBuiltinOpen = !skillsBuiltinOpen">
                    <span class="skill-icon">` + sparkle + `</span>
                    <span class="skills-builtin-text">
                      <span class="skills-builtin-title"><span x-text="builtinSkills.length"></span> built-in skills</span>
                      <span class="skills-builtin-names" x-text="builtinSkills.map(function(s) { return s.name; }).join(', ')"></span>
                    </span>
                    <span class="skill-fixed"><span class="dot dot--on"></span>Always on</span>
                    <svg class="skills-builtin-chev" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="m9 6 6 6-6 6"/></svg>
                  </button>
                  <div class="skills-grid skills-builtin-grid" x-show="skillsBuiltinOpen">
                    <template x-for="s in builtinSkills" :key="s.name">` + card + `
                    </template>
                  </div>
                </div>
              </template>
              <template x-if="extraSkills.length === 0">
                <div class="skills-empty">
                  No other skills added yet.
                  <button type="button" class="skills-empty-link" @click="skillsTab = 'recommended'">Browse Recommended</button>
                </div>
              </template>
              <div class="skills-grid">
                <template x-for="s in extraSkills" :key="s.name">` + card + `
                </template>
              </div>
            </div>
            <div x-show="!skillsLoading && skillsTab === 'recommended'">
              <div class="skills-grid">
                <template x-for="s in recommendedSkills" :key="s.name">` + card + `
                </template>
              </div>
            </div>
          </div><!-- .skills-pane -->

`
}

const settingsSkillsJS = `
      skillsList: [],
      skillsCatalog: [],
      skillsLoading: false,
      skillsError: '',
      skillsAdding: {},
      skillsRemoving: {},
      skillsToggling: {},

      skillsTab: 'installed',
      skillsBuiltinOpen: false,

      get builtinSkills() {
        return this.addedSkills.filter(function(s) { return s.default; });
      },
      get extraSkills() {
        return this.addedSkills.filter(function(s) { return !s.default; });
      },
      get addedSkills() {
        return this.skillsList.map(function(s) { return Object.assign({ added: true }, s); })
          .sort(function(a, b) { return a.name.localeCompare(b.name); });
      },
      // An added catalog skill keeps its card here, switched by the same On/Off control.
      get recommendedSkills() {
        const added = {};
        this.skillsList.forEach(function(s) { added[s.name] = s; });
        return this.skillsCatalog.map(function(e) {
          const a = added[e.name];
          return Object.assign({}, e, { added: !!a, enabled: !!(a && a.enabled), description: e.description || (a && a.description) || '' });
        }).sort(function(a, b) { return a.name.localeCompare(b.name); });
      },

      async loadSkills(quiet) {
        if (!quiet) this.skillsLoading = true;
        this.skillsError = '';
        try {
          const rs = await Promise.all([fetch('/api/skills'), fetch('/api/skills/catalog')]);
          rs.forEach(function(r) { if (!r.ok) throw new Error('HTTP ' + r.status); });
          const ds = await Promise.all(rs.map(function(r) { return r.json(); }));
          this.skillsList = ds[0].skills || [];
          this.skillsCatalog = ds[1].skills || [];
        } catch(e) { this.skillsError = e.message; }
        finally { this.skillsLoading = false; }
      },

      async addSkill(s) {
        this.skillsAdding = Object.assign({}, this.skillsAdding, { [s.name]: true });
        try {
          const r = await fetch('/api/skills/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ repo: s.repo, skill: s.name }),
          });
          const d = await r.json();
          if (!r.ok) {
            window.showError(d.error || 'Add failed', 'Add failed');
            return;
          }
          // The description comes from SKILL.md, which exists only now.
          await this.loadSkills(true);
        } catch(e) {
          window.showError(e.message, 'Add failed');
        } finally {
          const t = Object.assign({}, this.skillsAdding);
          delete t[s.name];
          this.skillsAdding = t;
        }
      },

      async setSkillEnabled(name, on) {
        const cur = this.skillsList.find(function(s) { return s.name === name; });
        if (!cur || cur.enabled === on) return;
        this.skillsToggling = Object.assign({}, this.skillsToggling, { [name]: true });
        try {
          const r = await fetch('/api/skills/' + encodeURIComponent(name) + (on ? '/enable' : '/disable'), { method: 'POST' });
          const d = await r.json();
          if (!r.ok) {
            window.showError(d.error || 'Update failed', on ? 'Enable failed' : 'Disable failed');
            return;
          }
          this.skillsList = this.skillsList.map(function(s) {
            return s.name === name ? Object.assign({}, s, { enabled: on }) : s;
          });
        } catch(e) {
          window.showError(e.message, on ? 'Enable failed' : 'Disable failed');
        } finally {
          const t = Object.assign({}, this.skillsToggling);
          delete t[name];
          this.skillsToggling = t;
        }
      },
      async removeSkill(name) {
        const ok = await window.showConfirm({ title: 'Remove skill', message: 'Delete "' + name + '" and all its files from ~/.hylo/skills/? This can\'t be undone.', confirmLabel: 'Remove', danger: true });
        if (!ok) return;
        this.skillsRemoving = Object.assign({}, this.skillsRemoving, { [name]: true });
        try {
          const r = await fetch('/api/skills/' + encodeURIComponent(name), { method: 'DELETE' });
          const d = await r.json();
          if (!r.ok) {
            window.showError(d.error || 'Remove failed', 'Remove failed');
            return;
          }
          this.skillsList = this.skillsList.filter(function(s) { return s.name !== name; });
        } catch(e) {
          window.showError(e.message, 'Remove failed');
        } finally {
          const t = Object.assign({}, this.skillsRemoving);
          delete t[name];
          this.skillsRemoving = t;
        }
      },

`
