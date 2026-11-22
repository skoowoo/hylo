  function settingsAgentCLITab() {
    return {
      agentCopied: '',
      agentJobs: {},

      agentFormatVersion(v) {
        if (!v) return '';
        return /^v/i.test(v) ? v : 'v' + v;
      },

      agentVersionPair(ag) {
        const cur = this.agentFormatVersion(ag.version);
        const next = this.agentFormatVersion(ag.latestVersion);
        return cur && next ? cur + ' → ' + next : (next || cur);
      },

      agentSummaryText() {
        const installed = this.agents.filter(a => a.available).length;
        const updates = this.agents.filter(a => a.available && a.updateAvailable).length;
        const missing = this.agents.length - installed;
        const parts = [installed + ' installed'];
        if (updates > 0) parts.push(updates + ' update' + (updates > 1 ? 's' : '') + ' available');
        parts.push(missing + ' not installed');
        return parts.join(' · ');
      },

      agentCliLogo(ag) {
        if (!window.__hyloAgentLogo) return '<span class="agent-logo-fallback">' + (ag.name || ag.id || '?')[0] + '</span>';
        return window.__hyloAgentLogo(ag.id, '/static/agents/', {
          glyph: 'agent-logo-glyph', dark: 'agent-logo-dark', light: 'agent-logo-light',
        }) || '<span class="agent-logo-fallback">' + (ag.name || ag.id || '?')[0] + '</span>';
      },

      syncAgentJobs(snapshot) {
        if (!snapshot || typeof snapshot !== 'object') return;
        for (const [id, action] of Object.entries(snapshot)) {
          if (!this.agentJobs[id] || this.agentJobs[id].state !== 'running') {
            this.agentJobs[id] = { action: action || 'install', state: 'running', error: '', output: '', showOutput: false };
          }
        }
      },

      isAgentJobRunning(id) {
        return !!(this.agentJobs[id] && this.agentJobs[id].state === 'running');
      },

      agentJobSummary(id) {
        const j = this.agentJobs[id];
        if (!j) return '';
        if (j.state === 'succeeded') return j.action === 'update' ? 'Updated successfully.' : 'Installed successfully.';
        if (j.state === 'canceled') return 'Canceled.';
        return j.error || (j.action === 'update' ? 'Update failed.' : 'Installation failed.');
      },

      toggleAgentJobLog(id) {
        if (this.agentJobs[id]) this.agentJobs[id].showOutput = !this.agentJobs[id].showOutput;
      },

      dismissAgentJob(id) {
        delete this.agentJobs[id];
      },

      async runAgentAction(ag, action) {
        if (this.isAgentJobRunning(ag.id)) return;
        const job = { action, state: 'running', error: '', output: '', showOutput: false };
        this.agentJobs[ag.id] = job;
        try {
          const r = await fetch('/api/agents/' + encodeURIComponent(ag.id) + '/' + action, { method: 'POST' });
          let d = {};
          try { d = await r.json(); } catch (_) {}
          if (!r.ok) {
            job.state = 'failed';
            job.error = d.error || ('HTTP ' + r.status);
            job.output = d.output || '';
            return;
          }
          if (d.canceled) {
            job.state = 'canceled';
            return;
          }
          job.state = 'succeeded';
          if (d.agent) this._replaceAgent(d.agent);
        } catch (e) {
          job.state = 'failed';
          job.error = e.message || 'Network error';
        }
      },

      async cancelAgentJob(id) {
        try {
          await fetch('/api/agents/' + encodeURIComponent(id) + '/cancel', { method: 'POST' });
        } catch (_) {}
      },

      _replaceAgent(updated) {
        const idx = this.agents.findIndex(a => a.id === updated.id);
        if (idx >= 0) this.agents.splice(idx, 1, updated);
        else this.agents.push(updated);
      },

      async copyCliExample(ag) {
        if (!ag.cliExample) return;
        try {
          await window.__hyloClipboard.writeText(ag.cliExample);
        } catch (_) { return; }
        this.agentCopied = ag.id;
        setTimeout(() => { if (this.agentCopied === ag.id) this.agentCopied = ''; }, 1500);
      },
    };
  }
