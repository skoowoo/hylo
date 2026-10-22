package view

// Skills tab: every skill added to ~/.hylo/skills, and the catalog Hylo curates.
const settingsSkillsCSS = `
    .skills-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; margin-bottom: 0.75rem; }
    .skills-toolbar-actions { display: flex; align-items: center; gap: 0.5rem; }
    .skills-repo-form { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.75rem; flex-wrap: wrap; }
    .skills-repo-form .field-input { flex: 2 1 14rem; min-width: 0; }
    .skills-repo-form .skills-repo-name { flex: 1 1 9rem; }
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
    .skill-card.is-added .skill-icon, .skills-builtin-row .skill-icon { background: rgba(var(--accent-rgb), 0.12); color: var(--accent-text); }
    .skill-name {
      flex: 1; min-width: 0; font-size: var(--text-sm); font-weight: var(--fw-semibold); color: var(--fg);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    }
    .skill-card--available .skill-name { color: var(--muted); }
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
    .skill-used {
      min-width: 0; font-size: var(--text-2xs); color: var(--muted);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex-shrink: 1; text-align: right;
    }
    .skill-used.is-unused { opacity: 0.6; }
    .skills-builtin-row .skill-used { flex-shrink: 0; }
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
                  <div class="list-card skill-card" :class="s.added ? 'is-added' : 'skill-card--available'">
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
                      <template x-if="s.added && skillsTab === 'added'">
                        <span class="skill-used" :class="{'is-unused': !s.bots.length}" :title="skillUsedBy(s)" x-text="skillUsedBy(s)"></span>
                      </template>
                      <template x-if="s.added && skillsTab === 'catalog'">
                        <span class="badge">Added</span>
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
              <div class="seg" role="tablist">
                <button type="button" class="seg-btn" role="tab" :class="{active: skillsTab === 'added'}"
                        :aria-selected="skillsTab === 'added'" @click="skillsTab = 'added'">
                  Added <span class="seg-count" x-show="!skillsLoading" x-text="skillsList.length"></span>
                </button>
                <button type="button" class="seg-btn" role="tab" :class="{active: skillsTab === 'catalog'}"
                        :aria-selected="skillsTab === 'catalog'" @click="skillsTab = 'catalog'">
                  Catalog <span class="seg-count" x-show="!skillsLoading" x-text="catalogSkills.length"></span>
                </button>
              </div>
              <div class="skills-toolbar-actions">
                <button type="button" class="btn-outline" @click="skillsRepoOpen = !skillsRepoOpen; $nextTick(() => skillsRepoOpen && $refs.skillsRepo.focus())">` + plus + `Add from GitHub</button>
` + toolbarRefreshBtnHTML("skillsLoading", "loadSkills()", "Loading…") + `
              </div>
            </div>
            <form class="skills-repo-form" x-show="skillsRepoOpen" @submit.prevent="addSkillFromRepo()">
              <input class="field-input" type="text" x-ref="skillsRepo" x-model="skillsRepo.repo" placeholder="owner/repo or GitHub URL" spellcheck="false">
              <input class="field-input skills-repo-name" type="text" x-model="skillsRepo.skill" placeholder="Skill name (optional)" spellcheck="false">
              <button type="submit" class="btn-solid" :disabled="!skillsRepo.repo.trim() || skillsRepo.adding" x-text="skillsRepo.adding ? 'Adding…' : 'Add'"></button>
              <button type="button" class="btn-outline" @click="skillsRepoOpen = false">Cancel</button>
            </form>
            <p class="skills-hint" x-show="skillsTab === 'added'">Every skill in <code>~/.hylo/skills/</code>, wherever it came from. Turn them on for a bot under Bots.</p>
            <p class="skills-hint" x-show="skillsTab === 'catalog'">Skills Hylo curates on GitHub. Add one, then turn it on for a bot.</p>
            <div class="cfg-err-msg" x-show="skillsError && !skillsLoading" x-text="'Error: ' + skillsError"></div>
            <div class="cfg-loader" x-show="skillsLoading">Loading skills…</div>
            <div x-show="!skillsLoading && skillsTab === 'added'">
              <template x-if="builtinSkills.length">
                <div class="skills-builtin">
                  <button type="button" class="list-card skills-builtin-row" :aria-expanded="skillsBuiltinOpen"
                          @click="skillsBuiltinOpen = !skillsBuiltinOpen">
                    <span class="skill-icon">` + sparkle + `</span>
                    <span class="skills-builtin-text">
                      <span class="skills-builtin-title"><span x-text="builtinSkills.length"></span> built-in skills</span>
                      <span class="skills-builtin-names" x-text="builtinSkills.map(function(s) { return s.name; }).join(', ')"></span>
                    </span>
                    <span class="skill-used">Always on for the main bot</span>
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
                  <button type="button" class="skills-empty-link" @click="skillsTab = 'catalog'">Browse the catalog</button>
                </div>
              </template>
              <div class="skills-grid">
                <template x-for="s in extraSkills" :key="s.name">` + card + `
                </template>
              </div>
            </div>
            <div x-show="!skillsLoading && skillsTab === 'catalog'">
              <div class="skills-grid">
                <template x-for="s in catalogSkills" :key="s.name">` + card + `
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
      skillsRepoOpen: false,
      skillsRepo: { repo: '', skill: '', adding: false },

      skillsTab: 'added',
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
      // An added catalog skill keeps its card here, marked as added.
      get catalogSkills() {
        const added = {};
        this.skillsList.forEach(function(s) { added[s.name] = s; });
        return this.skillsCatalog.map(function(e) {
          const a = added[e.name];
          return Object.assign({}, e, { added: !!a, bots: a ? a.bots : [], description: e.description || (a && a.description) || '' });
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

      skillUsedBy(s) {
        return s.bots.length ? s.bots.map(function(b) { return b.name; }).join(', ') : 'No bot';
      },

      // Adds a skill from a GitHub repository; true when it went in.
      async postSkillAdd(repo, skill) {
        try {
          const r = await fetch('/api/skills/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ repo: repo, skill: skill }),
          });
          const d = await r.json();
          if (!r.ok) {
            window.showError(d.error || 'Add failed', 'Add failed');
            return false;
          }
          // The description comes from SKILL.md, which exists only now.
          await this.loadSkills(true);
          return true;
        } catch(e) {
          window.showError(e.message, 'Add failed');
          return false;
        }
      },

      async addSkill(s) {
        this.skillsAdding = Object.assign({}, this.skillsAdding, { [s.name]: true });
        await this.postSkillAdd(s.repo, s.name);
        const t = Object.assign({}, this.skillsAdding);
        delete t[s.name];
        this.skillsAdding = t;
      },

      async addSkillFromRepo() {
        const f = this.skillsRepo;
        if (f.adding || !f.repo.trim()) return;
        f.adding = true;
        const ok = await this.postSkillAdd(f.repo.trim(), f.skill.trim());
        f.adding = false;
        if (!ok) return;
        this.skillsRepo = { repo: '', skill: '', adding: false };
        this.skillsRepoOpen = false;
        this.skillsTab = 'added';
      },

      async removeSkill(name) {
        const cur = this.skillsList.find(function(s) { return s.name === name; });
        const used = cur && cur.bots.length ? ' It is turned off for ' + this.skillUsedBy(cur) + '.' : '';
        const ok = await window.showConfirm({ title: 'Remove skill', message: 'Delete "' + name + '" and all its files from ~/.hylo/skills/? This can\'t be undone.' + used, confirmLabel: 'Remove', danger: true });
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
