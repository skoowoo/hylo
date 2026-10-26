  function settingsAgentCLITab() {
    return {
      agentCopied: '',

      agentDisplayModels(ag) { return (ag.models || []).slice(0, 4); },

      copyCliExample(ag) {
        if (!ag.cliExample) return;
        navigator.clipboard.writeText(ag.cliExample).catch(function() {});
        this.agentCopied = ag.id;
        setTimeout(() => { if (this.agentCopied === ag.id) this.agentCopied = ''; }, 1500);
      },

    };
  }
