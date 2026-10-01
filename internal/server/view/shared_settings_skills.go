package view

// Skills tab: installed/available Claude skills list.
const settingsSkillsCSS = `
    .skills-desc {
      font-size: var(--text-sm); color: var(--muted); line-height: 1.5; margin: 0 0 1.25rem;
    }
    .skills-desc code {
      font-family: var(--font-mono); font-size: var(--text-xs);
      padding: 0.05rem 0.35rem; border-radius: var(--r-xs);
      background: var(--code-bg);
    }
    .skills-list { display: flex; flex-direction: column; gap: 0.45rem; }
    .skills-empty { font-size: var(--text-sm); color: var(--muted); padding: 1.5rem 0; }
    /* Box/hover/surface come from the shared .list-card.list-card--hover
       .list-card--soft (base.css); see .agent-bot-card above for why no
       --clickable. */
    .skill-card {
      display: flex; align-items: center; justify-content: space-between; gap: 1rem;
    }
    .skill-card.not-installed .skill-card-left { opacity: 0.7; }
    .skill-card.not-installed:hover .skill-card-left { opacity: 1; }
    .skill-card-left { display: flex; align-items: center; gap: 0.6rem; flex: 1; min-width: 0; }
    .skill-card-right { display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0; }
    /* .skill-dot now uses the shared .dot / .dot--on / .dot--off (base.css). */
    .skill-name { font-size: var(--text-sm); font-weight: 600; color: var(--fg); }
    /* .skill-default-badge now uses the shared .badge (base.css). */
    .skill-toggling { opacity: 0.55; pointer-events: none; }
    .skill-repo-link {
      font-size: var(--text-xs); color: var(--muted); font-family: var(--font-mono);
      text-decoration: none; opacity: 0.65;
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 260px;
      border-radius: 0;
      transition: color var(--motion-fast), opacity var(--motion-fast);
    }
    .skill-repo-link:hover { color: var(--accent-text); opacity: 1; }
    /* Install/Uninstall now use the shared .btn-outline /
       .btn-solid.btn-solid--danger with the .btn--xs size modifier. */
    /* Placeholder text for every field here (.agent-bot-form-input/-textarea,
       .cfg-input/-textarea, .settings-input) now comes from the shared
       .field-input::placeholder (base.css) — they all carry that class
       already, so this no longer needs its own copy. */
`

func settingsSkillsTabHTML() string {
	return `
          <!-- Skills tab -->
          <div class="settings-scroll-pane" x-show="tab === 'skills'">
            <div class="agent-bots-toolbar">
` + toolbarRefreshBtnHTML("skillsLoading", "loadSkills()", "Loading…") + `
            </div>
            <p class="skills-desc">
              Manage skills for agents. Built-in skills are always enabled.
              External skills can be installed or removed here. Source directory: <code>~/.hylo/skills/</code>
            </p>
            <div class="cfg-err-msg" x-show="skillsError && !skillsLoading" x-text="'Error: ' + skillsError"></div>
            <div class="cfg-loader" x-show="skillsLoading">Loading skills…</div>
            <div x-show="!skillsLoading">
              <template x-if="skillsList.length === 0">
                <div class="skills-empty">No skills found.</div>
              </template>
              <div class="skills-list">
                <template x-for="s in sortedSkillsList" :key="s.name">
                  <div class="list-card list-card--hover list-card--soft skill-card" :class="s.installed ? '' : 'not-installed'">
                    <div class="skill-card-left">
                      <span class="dot" :class="s.installed ? 'dot--on' : 'dot--off'"></span>
                      <span class="skill-name" x-text="s.name"></span>
                      <span class="badge" x-show="s.default">built-in</span>
                      <template x-if="s.repoUrl">
                        <a class="skill-repo-link" :href="s.repoUrl" target="_blank" rel="noopener noreferrer"
                           @click.stop
                           x-text="s.repoUrl.replace('https://github.com/', '')"></a>
                      </template>
                    </div>
                    <div class="skill-card-right">
                      <template x-if="!s.default && s.installed">
                        <button class="btn-outline btn-outline--danger btn--xs"
                                :disabled="!!skillsUninstalling[s.name]"
                                @click="uninstallSkill(s.name)"
                                x-text="skillsUninstalling[s.name] ? 'Removing…' : 'Uninstall'">
                        </button>
                      </template>
                      <template x-if="!s.default && !s.installed">
                        <button class="btn-outline btn--xs"
                                :disabled="!!skillsInstalling[s.name]"
                                @click="installSkill(s.name, s.repoUrl, s.subPath)"
                                x-text="skillsInstalling[s.name] ? 'Installing…' : 'Install'">
                        </button>
                      </template>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div><!-- .skills-pane -->

`
}

const settingsSkillsJS = `
      skillsList: [],
      skillsLoading: false,
      skillsError: '',
      skillsInstalling: {},
      skillsUninstalling: {},

      get sortedSkillsList() {
        return [...this.skillsList].sort(function(a, b) {
          if (a.default !== b.default) return a.default ? -1 : 1;
          return a.name.localeCompare(b.name);
        });
      },

      async loadSkills() {
        this.skillsLoading = true;
        this.skillsError = '';
        try {
          const r = await fetch('/api/skills');
          if (!r.ok) throw new Error('HTTP ' + r.status);
          const d = await r.json();
          this.skillsList = d.skills || [];
        } catch(e) { this.skillsError = e.message; }
        finally { this.skillsLoading = false; }
      },

      async installSkill(name, repoUrl, subPath) {
        this.skillsInstalling = Object.assign({}, this.skillsInstalling, { [name]: true });
        try {
          const r = await fetch('/api/skills/install', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ repoUrl: repoUrl, subPath: subPath || '', skill: name }),
          });
          const d = await r.json();
          if (!r.ok) {
            if (typeof window.showError === 'function') window.showError(d.error || 'Install failed', 'Install failed');
            return;
          }
          this.skillsList = this.skillsList.map(function(s) {
            return s.name === name ? Object.assign({}, s, { installed: true, enabled: true }) : s;
          });
        } catch(e) {
          if (typeof window.showError === 'function') window.showError(e.message, 'Install failed');
        } finally {
          const t = Object.assign({}, this.skillsInstalling);
          delete t[name];
          this.skillsInstalling = t;
        }
      },

      async uninstallSkill(name) {
        const ok = (typeof window.showConfirm === 'function')
          ? await window.showConfirm({ title: 'Uninstall skill', message: 'Remove "' + name + '" and all its files from ~/.hylo/skills/?', confirmLabel: 'Uninstall', danger: true })
          : window.confirm('Uninstall skill "' + name + '"? This cannot be undone.');
        if (!ok) return;
        this.skillsUninstalling = Object.assign({}, this.skillsUninstalling, { [name]: true });
        try {
          const r = await fetch('/api/skills/' + encodeURIComponent(name), { method: 'DELETE' });
          const d = await r.json();
          if (!r.ok) {
            if (typeof window.showError === 'function') window.showError(d.error || 'Uninstall failed', 'Uninstall failed');
            return;
          }
          this.skillsList = this.skillsList.map(function(s) {
            return s.name === name ? Object.assign({}, s, { installed: false, enabled: false }) : s;
          });
        } catch(e) {
          if (typeof window.showError === 'function') window.showError(e.message, 'Uninstall failed');
        } finally {
          const t = Object.assign({}, this.skillsUninstalling);
          delete t[name];
          this.skillsUninstalling = t;
        }
      },

`
