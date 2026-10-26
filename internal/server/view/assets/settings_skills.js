  function settingsSkillsTab() {
    return {
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

    };
  }
