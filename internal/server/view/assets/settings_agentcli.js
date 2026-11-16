  function settingsAgentCLITab() {
    return {
      agentCopied: '',

      agentDisplayModels(ag) { return (ag.models || []).slice(0, 4); },

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
