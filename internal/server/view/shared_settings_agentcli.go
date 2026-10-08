package view

// Agent CLI tab ("Agents" in the sidebar/tab key): detected CLI agents and
// their models. Shares the `agents`/`loadAgents()` list state with Agent
// Bots' agent picker — that shared state stays in shared_settings_modal.go
// (the shell), not here, since both tabs read it.
const settingsAgentCLICSS = `
    /* ── Agents tab ───────────────────────────────────────────── */
    .agents-toolbar { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; }
    .agents-summary { font-size: var(--text-xs); color: var(--muted); }
    .agents-list { display: flex; flex-direction: column; gap: 0.45rem; }
    .agent-card {
      display: flex; flex-direction: column; gap: 0.625rem;
      min-width: 0; overflow: hidden;
    }
    .agent-card.unavailable { opacity: 0.48; }
    .agent-card-top {
      display: flex; align-items: center; gap: 0.6rem; min-width: 0;
    }
    .agent-card-name {
      font-size: var(--text-sm); font-weight: 600; color: var(--fg);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      border-radius: 0;
      flex-shrink: 0; max-width: 220px;
    }
    .agent-card-id {
      font-family: var(--font-mono);
      font-size: var(--text-xs); color: var(--fg); opacity: 0.6;
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      border-radius: 0;
      flex: 1; min-width: 0;
    }
    .agent-status-badge {
      font-size: var(--text-xs); font-weight: 500;
      padding: 2px 8px; border-radius: var(--r-full); flex-shrink: 0;
      background: var(--code-bg); color: var(--muted); white-space: nowrap;
    }
    .agent-status-badge.ok { color: var(--s-ok); background: var(--s-ok-bg); }
    /* row 2: path · version · protocol — single line, no wrap */
    .agent-card-info {
      display: flex; align-items: center; gap: 0.85rem;
      padding-left: 1.1rem; min-width: 0; overflow: hidden;
    }
    .agent-col-mono {
      font-family: var(--font-mono);
      font-size: var(--text-xs); color: var(--muted);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      border-radius: 0;
    }
    .agent-meta-path { flex: 1; min-width: 0; max-width: 340px; }
    /* row 3: model pills — always its own row */
    .agent-card-models {
      display: flex; flex-wrap: wrap; gap: 4px; align-items: center;
      padding-left: 1.1rem; min-width: 0; overflow: hidden;
    }
    /* Structure/color come from the shared .badge.badge--sm (base.css). */
    .agent-model-pill { max-width: 160px; }
    .agent-model-more { font-size: var(--text-xs); color: var(--muted); opacity: 0.6; white-space: nowrap; }
    /* row 4: cli example */
    .agent-card-cli {
      display: flex; align-items: center; gap: 0.5rem;
      padding-left: 1.1rem; min-width: 0; overflow: hidden;
    }
    .agent-cli-label {
      font-size: var(--text-2xs); font-weight: 600;
      color: var(--muted); opacity: 0.55;
      flex-shrink: 0; white-space: nowrap;
    }
    .agent-cli-code {
      font-family: var(--font-mono); font-size: var(--text-2xs);
      color: var(--muted); opacity: 0.75;
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      border-radius: 0;
      flex: 1; min-width: 0;
    }
    .agent-cli-copy {
      flex-shrink: 0; height: 20px; padding: 0 7px;
      border: none;
      background: var(--btn-bg); color: var(--muted);
      font-size: var(--text-2xs); font-family: var(--font-mono); cursor: pointer;
      white-space: nowrap;
      transition: background-color var(--motion-fast), color var(--motion-fast);
    }
    .agent-cli-copy:hover { color: var(--fg); background: var(--btn-bg-hov); }
    .agent-cli-copy.copied { color: var(--s-ok); background: var(--s-ok-bg); }

`

func settingsAgentCLITabHTML() string {
	return `
          <!-- Agents tab -->
          <div class="settings-scroll-pane" x-show="tab === 'agents'">
            <div class="agents-toolbar">
` + toolbarRefreshBtnHTML("agentsLoading", "loadAgents(true)", "Detecting…") + `
              <span class="agents-summary" x-show="!agentsLoading && agents.length">
                <span x-text="agents.filter(a=>a.available).length + ' available · ' + agents.filter(a=>!a.available).length + ' not installed'"></span>
                <span x-show="agentsFromCache && agentsCachedAt"
                      x-text="' · cached ' + Math.round((Date.now() - agentsCachedAt) / 60000) + 'm ago'"></span>
              </span>
            </div>
            <div class="cfg-loader" x-show="agentsLoading">Detecting agents on PATH — this may take a moment…</div>
            <div class="cfg-err-msg" x-show="agentsError && !agentsLoading" x-text="'Error: ' + agentsError"></div>
            <div class="agents-list" x-show="!agentsLoading && !agentsError && agents.length">
              <template x-for="ag in sortedAgents" :key="ag.id">
                <div class="list-card list-card--hover list-card--soft agent-card" :class="{unavailable: !ag.available}">
                  <div class="agent-card-top">
                    <span class="dot" :class="ag.available ? 'dot--on' : 'dot--off'"></span>
                    <span class="agent-card-name" x-text="ag.name"></span>
                    <span class="agent-card-id" x-text="ag.id"></span>
                    <span class="agent-status-badge" :class="ag.available ? 'ok' : ''"
                          x-text="ag.available ? 'available' : 'not installed'"></span>
                  </div>
                  <div class="agent-card-info">
                    <span class="agent-col-mono agent-meta-path"
                          :title="ag.path || ag.bin"
                          x-text="ag.path || ag.bin || '—'"></span>
                    <span class="agent-col-mono"
                          x-show="ag.version"
                          x-text="'v' + ag.version"></span>
                    <span class="agent-col-mono"
                          x-show="ag.streamFormat"
                          x-text="ag.streamFormat"></span>
                  </div>
                  <div class="agent-card-models">
                    <template x-for="m in agentDisplayModels(ag)" :key="m.id">
                      <span class="badge badge--sm agent-model-pill" :title="(m.label && m.label !== m.id) ? m.id + ' — ' + m.label : m.id" x-text="m.id"></span>
                    </template>
                    <span class="agent-model-more"
                          x-show="ag.models && ag.models.length > 4"
                          x-text="'+' + (ag.models.length - 4) + ' more'"></span>
                  </div>
                  <template x-if="ag.cliExample">
                    <div class="agent-card-cli">
                      <span class="agent-cli-label">Example:</span>
                      <span class="agent-cli-code" :title="ag.cliExample" x-text="ag.cliExample"></span>
                      <button type="button" class="agent-cli-copy"
                              :class="agentCopied === ag.id ? 'copied' : ''"
                              @click.stop="copyCliExample(ag)"
                              x-text="agentCopied === ag.id ? 'copied' : 'copy'"></button>
                    </div>
                  </template>
                </div>
              </template>
            </div>
          </div><!-- .agents-pane -->

`
}

const settingsAgentCLIJS = `
      agentCopied: '',

      agentDisplayModels(ag) { return (ag.models || []).slice(0, 4); },

      copyCliExample(ag) {
        if (!ag.cliExample) return;
        navigator.clipboard.writeText(ag.cliExample).catch(function() {});
        this.agentCopied = ag.id;
        setTimeout(() => { if (this.agentCopied === ag.id) this.agentCopied = ''; }, 1500);
      },

`
