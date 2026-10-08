package view

// agentBotCompilerPrompt is the shared trigger prompt for the Compiler
// template's two triggers (compile_requested and note_created+/Web Clips/).
// Built as a Go string (not inline in the JS below) because it contains
// backticks, which can't appear inside the raw-string-delimited JS blob.
// \\n (not \n) so the *generated JS source* carries a literal backslash-n
// for the browser to interpret, not a raw newline that would break the
// single-quoted JS string literal it's spliced into.
const agentBotCompilerPrompt = "Compile note `{Path}` into structured knowledge under `/_knowledge`. Steps:\\n\\n1. Use the `hylo-compile-note` skill to compile the note\\n2. Once compiled, use the `hylo-index-knowledge` skill to create or update the index\\n3. Finally, use the `humanizer` skill to polish the wording of the newly generated knowledge unit(s) — you must preserve the `frontmatter`, `link`, `wiki link`, and paragraph structure."

// agentBotRecapShortNotePrompt is the Recap template's short_note_created
// trigger prompt — same backtick-splice reason as agentBotCompilerPrompt above.
const agentBotRecapShortNotePrompt = "Use the hylo-recap skill to cross-check this short note against my knowledge base — anything related or conflicting?\\n\\nShort note content: `{Content}`"

// Agent Bots tab: the template picker, list, and the create/edit form
// (Profile/Agent/Triggers). The largest single tab, hence its own file.
const settingsAgentBotsCSS = `
    /* Outer flex column: .agent-bots-scroll (flex:1, the only scroller)
       plus the footer as a non-scrolling sibling — same docked-footer
       reasoning as .cfg-action-bar above, not a sticky child of the
       scroller. */
    .agent-bots-pane { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
    .agent-bots-scroll { flex: 1; min-height: 0; overflow-y: auto; padding: 1.75rem 1.5rem 0.5rem; }
    .agent-bots-toolbar { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; }
    .agent-bots-list { display: flex; flex-direction: column; gap: 0.45rem; }
    .agent-bots-empty { font-size: var(--text-sm); color: var(--muted); padding: 1.5rem 0; }
    /* Box/hover/surface come from the shared .list-card.list-card--hover
       .list-card--soft (base.css); the row itself isn't a click target
       (actions live in its own buttons), so no --clickable. */
    .agent-bot-card {
      display: flex; align-items: flex-start; gap: 0.875rem;
    }
    .agent-bot-card.disabled-card { opacity: 0.45; }
    .agent-bot-card-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 0.3rem; }
    .agent-bot-card-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 0.5rem; min-width: 0; }
    .agent-bot-card-name { font-size: var(--text-sm); font-weight: 600; color: var(--fg); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; border-radius: 0; flex: 1; min-width: 0; padding-top: 0.1rem; }
    .agent-bot-card-desc { font-size: var(--text-xs); color: var(--muted); line-height: 1.5; }
    .agent-bot-card-actions { display: flex; gap: 0.3rem; flex-shrink: 0; }
    .agent-bot-card-tags { display: flex; flex-wrap: wrap; gap: 0.35rem; margin-top: 0.15rem; }
    /* Structure/color come from the shared .badge / .badge--ok (base.css). */
    .agent-bot-badge { max-width: 200px; }
    .agent-bot-form-wrap { max-width: 780px; }
    .agent-bot-template-list { display: flex; flex-direction: column; gap: 0.45rem; max-width: 640px; }
    /* Box/hover/cursor/surface come from the shared .list-card.list-card--clickable
       .list-card--soft (base.css) — unlike .agent-bot-card, the whole row IS
       the click target here (pick a template), not just a container for its
       own action buttons. */
    .agent-bot-template-card {
      display: flex; flex-direction: column; gap: 0.2rem; text-align: left;
    }
    .agent-bot-template-name { font-size: var(--text-sm); font-weight: 600; color: var(--fg); }
    .agent-bot-template-desc { font-size: var(--text-xs); color: var(--muted); line-height: 1.5; }
    /* Layout (display/align/gap) shared with .cfg-detail-header via
       .settings-detail-header; only the margin below it differs per
       caller, so that part stays local. */
    .agent-bot-form-header { margin-bottom: 2rem; }
    .agent-bot-back-btn {
      display: inline-flex; align-items: center; gap: 0.3rem;
      background: none; border: none; color: var(--muted); font-size: var(--text-sm);
      cursor: pointer; padding: 0;
    }
    .agent-bot-back-btn:hover { color: var(--fg); }
    .agent-bot-back-btn svg { width: 14px; height: 14px; }
    .agent-bot-form { display: flex; flex-direction: column; }
    .agent-bot-form-section {
      padding: 1.65rem 0 1.5rem;
      display: flex; flex-direction: column; gap: 1.25rem;
    }
    .agent-bot-form-section:first-child { padding-top: 0; }
    .agent-bot-form-section-triggers { gap: 1rem; }
    .agent-bot-form-section-title {
      font-size: var(--text-xs); font-weight: var(--fw-medium); letter-spacing: 0.05em;
      text-transform: uppercase; color: var(--fg); margin: 0;
      display: flex; align-items: center; gap: 0.6rem; white-space: nowrap;
    }
    .agent-bot-trigger-section-top { display: flex; flex-direction: column; gap: 0.4rem; }
    .agent-bot-section-desc { font-size: var(--text-sm); color: var(--muted); line-height: 1.5; margin: 0; }
    .agent-bot-form-row { display: flex; gap: 1rem; }
    .agent-bot-form-row > * { flex: 1; min-width: 0; }
    .agent-bot-form-label { display: block; font-size: var(--text-sm); font-weight: 600; color: var(--fg); margin-bottom: 0.4rem; }
    /* Box/font come from the shared .field-input (base.css). This one
       auto-grows with its content (JS sets height from scrollHeight),
       so it keeps its own overflow/line-height instead of the shared
       fixed-rows textarea treatment. */
    .agent-bot-form-textarea { overflow: hidden; line-height: 1.55; }
    .agent-bot-trigger-add {
      font-size: var(--text-sm); font-weight: 600; color: var(--muted);
      background: none; border: none; cursor: pointer; padding: 0.15rem 0;
      display: block;
    }
    .agent-bot-trigger-add:hover { color: var(--fg); }
    .agent-bot-var-panel { margin-bottom: 0.5rem; }
    .agent-bot-var-panel-label {
      display: block; font-size: var(--text-xs); font-weight: 600;
      letter-spacing: 0.04em; text-transform: uppercase; color: var(--muted); margin-bottom: 0.4rem;
    }
    .agent-bot-var-chips { display: flex; flex-wrap: wrap; gap: 0.35rem; }
    .agent-bot-var-chip {
      display: inline-flex; align-items: center; height: var(--btn-h-xs); padding: 0 0.6rem;
      border: none; background: var(--btn-bg);
      border-radius: var(--r-full);
      cursor: pointer;
      transition: background-color var(--motion-fast);
    }
    .agent-bot-var-chip:hover { background: var(--btn-bg-hov); }
    .agent-bot-var-chip:active { background: var(--btn-bg-on); }
    .agent-bot-var-chip code {
      font-family: var(--font-mono);
      font-size: var(--text-xs); font-weight: 600; color: var(--fg); opacity: 0.8;
    }
    .agent-bot-trigger-list { display: flex; flex-direction: column; gap: 1rem; }
    .agent-bot-trigger-card {
      background: var(--surface-soft);
      border-radius: var(--r-lg);
      padding: 1rem 1.15rem 1.15rem;
      display: flex; flex-direction: column; gap: 1rem;
    }
    /* Clickable — collapsed by default (see .agent-bot-trigger-summary),
       so N triggers costs N header rows of scroll instead of N full
       editors. */
    .agent-bot-trigger-hdr {
      display: flex; align-items: center; justify-content: space-between;
      gap: 0.75rem; cursor: pointer;
    }
    .agent-bot-trigger-card.is-open .agent-bot-trigger-hdr {
      padding-bottom: 0.75rem;
    }
    .agent-bot-trigger-hdr-main { display: flex; align-items: center; gap: 0.55rem; min-width: 0; }
    .agent-bot-trigger-chev { flex-shrink: 0; color: var(--muted); display: flex; align-items: center; transition: transform var(--motion-fast); }
    .agent-bot-trigger-chev svg { width: 14px; height: 14px; }
    .agent-bot-trigger-chev.open { transform: rotate(90deg); }
    .agent-bot-trigger-label { font-size: var(--text-sm); font-weight: 600; color: var(--fg); flex-shrink: 0; }
    .agent-bot-trigger-summary {
      font-size: var(--text-xs); color: var(--muted);
      overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0;
    }
    .agent-bot-trigger-hdr-actions { display: flex; align-items: center; gap: 0.65rem; flex-shrink: 0; }
    .agent-bot-trigger-del {
      background: none; border: none; color: var(--s-err); font-size: var(--text-xs);
      font-weight: 500; cursor: pointer; padding: 0;
      opacity: 0.55;
    }
    .agent-bot-trigger-del:hover { opacity: 1; }
    .agent-bot-trigger-body { display: flex; flex-direction: column; gap: 1.4rem; }
    .agent-bot-block-label { margin-bottom: 0.65rem; }
    .agent-bot-block-title { display: block; font-size: var(--text-sm); font-weight: 600; color: var(--fg); }
    .agent-bot-block-hint { display: block; font-size: var(--text-xs); color: var(--muted); line-height: 1.5; margin-top: 0.2rem; }
    .agent-bot-prompt-textarea { font-family: var(--font-mono); font-size: var(--text-sm); }
    .agent-bot-schedule-presets { display: flex; flex-wrap: wrap; gap: 0.4rem; margin-bottom: 0.65rem; }
    .agent-bot-schedule-custom-label { display: block; font-size: var(--text-xs); color: var(--muted); margin-bottom: 0.3rem; }
    .agent-bot-weekday-toggles { align-items: center; margin-bottom: 0.4rem; }
    .agent-bot-schedule-kind-seg { margin-bottom: 0.85rem; }
    .agent-bot-schedule-kind-body { margin-bottom: 0.85rem; }
    /* Docked, not sticky — a flex sibling *outside* .agent-bots-scroll's
       scroll viewport (same reasoning as .cfg-action-bar above), so Save
       stays reachable no matter how many trigger cards are expanded. */
    .agent-bot-form-footer {
      flex-shrink: 0; display: flex; align-items: center; gap: 0.5rem;
      padding: 1.1rem 1.5rem;
      background: var(--bg);
      border-top: var(--bd-w) solid var(--line-hair);
      max-width: 780px;
    }
    .agent-bot-form-err { flex: 1; font-size: var(--text-xs); color: var(--s-err); }

    /* .cselect* (custom select) moved to assets/cselect.css — it's an
       app-wide primitive shared with Shorts' month picker, not a
       settings-modal detail. */
`

func settingsAgentBotsTabHTML() string {
	return `
          <!-- Agent Bots tab -->
          <div class="agent-bots-pane" x-show="tab === 'agent-bots'">
           <div class="agent-bots-scroll">

            <template x-if="!agentBotFormMode && !agentBotTemplatePicker">
              <div>
                <div class="agent-bots-toolbar">
                  <button class="btn-outline" @click="newAgentBot()">
                    <svg fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14"/>
                      <path stroke-linecap="round" stroke-linejoin="round" d="M12 5v14"/>
                    </svg>
                    New Agent Bot
                  </button>

` + toolbarRefreshBtnHTML("agentBotsLoading", "loadAgentBots()", "Loading…") + `
                </div>
                <div class="cfg-err-msg" x-show="agentBotsError && !agentBotsLoading" x-text="'Error: ' + agentBotsError"></div>
                <div class="agent-bots-list">
                  <template x-if="!agentBotsLoading && agentBotsList.length === 0">
                    <div class="agent-bots-empty">No agent bots yet — click New Agent Bot to create one.</div>
                  </template>
                  <template x-for="(m, mi) in agentBotsList" :key="m.id">
                    <div class="list-card list-card--hover list-card--soft agent-bot-card" :class="m.enabled ? '' : 'disabled-card'">
                      <div class="avatar avatar--lg avatar--neutral agent-bot-avatar"
                           :style="'background:' + agentBotColor(m.name)">
                        <span x-text="m.name ? m.name.charAt(0).toUpperCase() : '?'"></span>
                      </div>
                      <div class="agent-bot-card-body">
                        <div class="agent-bot-card-header">
                          <span class="agent-bot-card-name" x-text="m.name"></span>
                          <div class="agent-bot-card-actions">
                            <button class="btn-outline btn--xs" @click.stop="moveAgentBot(mi, -1)" :disabled="mi === 0" type="button" title="Move up">↑</button>
                            <button class="btn-outline btn--xs" @click.stop="moveAgentBot(mi, 1)" :disabled="mi === agentBotsList.length - 1" type="button" title="Move down">↓</button>
                            <button class="btn-outline btn--xs" @click.stop="openAgentBotEdit(m.id)" type="button">Edit</button>
                            <button class="btn-solid btn-solid--danger btn--xs" @click.stop="deleteAgentBot(m.id)" type="button">Delete</button>
                          </div>
                        </div>
                        <p class="agent-bot-card-desc" x-show="m.description" x-text="m.description"></p>
                        <div class="agent-bot-card-tags">
                          <span class="badge agent-bot-badge" x-text="m.agentId || '—'"></span>
                          <template x-if="m.model">
                            <span class="badge agent-bot-badge" x-text="m.model"></span>
                          </template>
                          <template x-if="m.triggerCount > 0">
                            <span class="badge badge--ok agent-bot-badge"
                                  x-text="m.triggerCount + (m.triggerCount === 1 ? ' trigger' : ' triggers')"></span>
                          </template>
                        </div>
                      </div>
                    </div>
                  </template>
                </div>
              </div>
            </template>

            <template x-if="agentBotTemplatePicker">
              <div class="agent-bot-form-wrap">
                <div class="settings-detail-header agent-bot-form-header">
` + agentBotBackBtnHTML("agentBotTemplatePicker = false", "Agent Bots") + `
                </div>
                <p class="agent-bot-section-desc" style="margin-bottom:1.1rem;">Start from a template, or build one from scratch.</p>
                <div class="agent-bot-template-list">
                  <template x-for="tpl in agentBotTemplates" :key="tpl.id">
                    <button type="button" class="list-card list-card--clickable list-card--soft agent-bot-template-card" @click="chooseAgentBotTemplate(tpl)">
                      <span class="agent-bot-template-name" x-text="tpl.name"></span>
                      <span class="agent-bot-template-desc" x-text="tpl.description"></span>
                    </button>
                  </template>
                  <button type="button" class="list-card list-card--clickable list-card--soft agent-bot-template-card"
                          @click="chooseAgentBotTemplate({id: 'blank', name: '', description: '', triggers: []})">
                    <span class="agent-bot-template-name">Blank</span>
                    <span class="agent-bot-template-desc">Start from an empty agent bot and configure everything yourself.</span>
                  </button>
                </div>
              </div>
            </template>

            <template x-if="agentBotFormMode">
              <div class="agent-bot-form-wrap">
                <div class="settings-detail-header agent-bot-form-header">
` + agentBotBackBtnHTML("agentBotFormMode = null", "Agent Bots") + `
                </div>
                <div class="agent-bot-form">
                  <section class="agent-bot-form-section">
                    <h3 class="agent-bot-form-section-title">Profile</h3>
                    <div class="agent-bot-form-row">
                      <div>
                        <label class="agent-bot-form-label">Name</label>
                        <input class="field-input agent-bot-form-input" type="text" x-model="agentBotDraft.name" placeholder="Quote Extractor" autofocus>
                      </div>
                      <div style="flex:0 0 auto;min-width:90px">
                        <label class="agent-bot-form-label">Enabled</label>
                        <div class="seg">
                          <button type="button" class="seg-btn" :class="{active: !agentBotDraft.enabled}" @click="agentBotDraft.enabled = false">Off</button>
                          <button type="button" class="seg-btn" :class="{active: agentBotDraft.enabled}" @click="agentBotDraft.enabled = true">On</button>
                        </div>
                      </div>
                    </div>
                    <div>
                      <label class="agent-bot-form-label">Description <span style="font-weight:400;text-transform:none;letter-spacing:0;">(optional)</span></label>
                      <input class="field-input agent-bot-form-input" type="text" x-model="agentBotDraft.description" placeholder="Brief description of what this agent bot does">
                    </div>
                  </section>

                  <section class="agent-bot-form-section">
                    <h3 class="agent-bot-form-section-title">Agent</h3>
                    <div class="agent-bot-form-row">
                      <div>
                        <label class="agent-bot-form-label">Agent</label>
` + cselectHTML(
		`(agents.find(function(a){return a.id===agentBotDraft.agentId&&a.available;}) || {name: 'Select agent'}).name`,
		`<template x-for="a in agents.filter(function(a){return a.available;})" :key="a.id"><button type="button" class="cselect-option" :class="agentBotDraft.agentId===a.id?'sel':''" @click="agentBotDraft.agentId=a.id; onAgentBotAgentChange(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span x-text="a.name"></span></button></template>`,
	) + `
                      </div>
                      <div>
                        <label class="agent-bot-form-label">Model</label>
` + cselectHTML(
		`agentBotDraft.model || 'Default'`,
		`<button type="button" class="cselect-option" :class="agentBotDraft.model===''?'sel':''" @click="agentBotDraft.model=''; csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Default</span></button>`+
			`<template x-for="m in agentBotModelsForAgent(agentBotDraft.agentId)" :key="m.id"><button type="button" class="cselect-option" :class="agentBotDraft.model===m.id?'sel':''" @click="agentBotDraft.model=m.id; csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span :title="(m.label && m.label !== m.id) ? m.label : ''" x-text="m.id"></span></button></template>`,
	) + `
                      </div>
                    </div>
                    <div>
                      <label class="agent-bot-form-label">System Prompt</label>
                      <textarea class="field-input agent-bot-form-textarea" x-model="agentBotDraft.systemPrompt" rows="1"
                                placeholder="Instructions prepended to every message…"></textarea>
                    </div>
                  </section>

                  <section class="agent-bot-form-section agent-bot-form-section-triggers">
                    <div class="agent-bot-trigger-section-top">
                      <h3 class="agent-bot-form-section-title">Triggers</h3>
                      <p class="agent-bot-section-desc">Automatically run this agent bot on vault events or on a schedule. Each trigger sends a prompt template to the agent.</p>
                    </div>
                    <div class="agent-bot-trigger-list">
                      <template x-for="(t, ti) in agentBotTriggers" :key="ti">
                        <div class="agent-bot-trigger-card" :class="{'is-open': t._open}">
                          <div class="agent-bot-trigger-hdr hairline" @click="t._open = !t._open">
                            <div class="agent-bot-trigger-hdr-main">
                              <span class="agent-bot-trigger-chev" :class="{open: t._open}">
                                <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24" aria-hidden="true">
                                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
                                </svg>
                              </span>
                              <span class="agent-bot-trigger-label">Trigger <span x-text="ti+1"></span></span>
                              <span class="agent-bot-trigger-summary" x-show="!t._open" x-text="agentBotTriggerSummary(t)"></span>
                            </div>
                            <div class="agent-bot-trigger-hdr-actions">
                              <button class="agent-bot-trigger-del" type="button" @click.stop="removeAgentBotTrigger(ti)">Remove</button>
                            </div>
                          </div>
                          <div class="agent-bot-trigger-body" x-show="t._open">
                            <div class="agent-bot-trigger-block agent-bot-trigger-events">
                              <div class="agent-bot-block-label">
                                <span class="agent-bot-block-title">Event</span>
                                <span class="agent-bot-block-hint" x-text="agentBotTriggerHint(t)"></span>
                              </div>
` + cselectHTML(
		`(agentBotEventDefs.find(function(d){return d.type===(t.eventTypes[0]||'');}) || {label: 'Select event…'}).label`,
		`<template x-for="def in agentBotEventDefs" :key="def.type"><button type="button" class="cselect-option" :class="(t.eventTypes[0]||'')===def.type ? 'sel' : ''" :title="def.description" @click="setAgentBotET(t, def.type); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span x-text="def.label"></span></button></template>`,
	) + `
                            </div>
                            <template x-if="eventOf(t) === 'scheduled'">
                              <div class="agent-bot-trigger-block agent-bot-trigger-schedule">
                                <div class="agent-bot-block-label">
                                  <span class="agent-bot-block-title">Schedule</span>
                                  <span class="agent-bot-block-hint">Choose how this trigger repeats, then pick a preset below.</span>
                                </div>
                                <div class="seg agent-bot-schedule-kind-seg">
                                  <button type="button" class="seg-btn" :class="{active: scheduleKindOf(t)==='every'}" @click="setScheduleKind(t, 'every')">Every</button>
                                  <button type="button" class="seg-btn" :class="{active: scheduleKindOf(t)==='daily'}" @click="setScheduleKind(t, 'daily')">Daily</button>
                                  <button type="button" class="seg-btn" :class="{active: scheduleKindOf(t)==='weekly'}" @click="setScheduleKind(t, 'weekly')">Weekly</button>
                                </div>

                                <template x-if="scheduleKindOf(t) !== 'weekly'">
                                  <div class="agent-bot-schedule-kind-body">
                                    <div class="agent-bot-schedule-presets">
                                      <template x-for="p in agentBotSchedulePresets[scheduleKindOf(t)]" :key="p.value">
                                        <button type="button" class="btn-outline btn--xs"
                                                :class="(t.schedule || '') === p.value ? 'active' : ''"
                                                @click="t.schedule = p.value"
                                                x-text="p.label"></button>
                                      </template>
                                    </div>
                                    <span class="agent-bot-block-hint" x-text="(scheduleKindOf(t) === 'every' ? 'Minimum interval: 15 minutes.' : 'Server local time.') + ' For a different value, edit it directly in Raw below.'"></span>
                                  </div>
                                </template>

                                <template x-if="scheduleKindOf(t) === 'weekly'">
                                  <div class="agent-bot-schedule-kind-body">
                                    <label class="agent-bot-schedule-custom-label">Days</label>
                                    <div class="agent-bot-schedule-presets agent-bot-weekday-toggles">
                                      <template x-for="d in agentBotWeekdayDefs" :key="d.abbr">
                                        <button type="button" class="btn-outline btn--xs"
                                                :class="weeklyDaysOf(t).indexOf(d.abbr) >= 0 ? 'active' : ''"
                                                @click="toggleWeeklyDay(t, d.abbr)"
                                                x-text="d.label"></button>
                                      </template>
                                    </div>
                                    <span class="agent-bot-block-hint">Server local time, defaults to 09:00. For a different time, edit it directly in Raw below.</span>
                                  </div>
                                </template>

                                <label class="agent-bot-schedule-custom-label">Raw</label>
                                <input class="field-input agent-bot-form-input" type="text" x-model="t.schedule"
                                       placeholder="every 1h · daily 09:00 · weekly mon,wed 09:00">
                              </div>
                            </template>
                            <template x-if="hasPathFilter(t)">
                              <div class="agent-bot-trigger-block agent-bot-trigger-paths">
                                <div class="agent-bot-block-label">
                                  <span class="agent-bot-block-title"><span x-text="agentBotFilterFor(t).title"></span> <span style="font-weight:400;opacity:0.6;">(optional)</span></span>
                                  <span class="agent-bot-block-hint" x-text="agentBotFilterFor(t).hint"></span>
                                </div>
                                <textarea class="field-input agent-bot-form-textarea"
                                          rows="2"
                                          :value="(t.pathPrefixes || []).join('\n')"
                                          @change="t.pathPrefixes = $event.target.value.split('\n').map(function(s){return s.trim();}).filter(Boolean)"
                                          :placeholder="agentBotFilterFor(t).placeholder"></textarea>
                              </div>
                            </template>
                            <div class="agent-bot-trigger-block agent-bot-trigger-prompt">
                              <div class="agent-bot-block-label">
                                <span class="agent-bot-block-title">Prompt template</span>
                                <span class="agent-bot-block-hint">Message sent to the agent when this trigger fires. Click a variable below to insert at cursor.</span>
                              </div>
                              <div class="agent-bot-var-panel">
                                <span class="agent-bot-var-panel-label">Variables</span>
                                <div class="agent-bot-var-chips">
                                  <template x-for="v in agentBotPromptVarsForTrigger(t)" :key="v.token">
                                    <button type="button" class="agent-bot-var-chip" :title="v.desc"
                                            @click="insertAgentBotVar(ti, v.token, $event)">
                                      <code x-text="v.token"></code>
                                    </button>
                                  </template>
                                </div>
                              </div>
                              <textarea class="field-input agent-bot-form-textarea agent-bot-prompt-textarea" x-model="t.prompt" rows="2"
                                        :placeholder="agentBotPromptPlaceholder(t)"></textarea>
                            </div>
                          </div>
                        </div>
                      </template>
                    </div>
                    <button class="agent-bot-trigger-add" type="button" @click="addAgentBotTrigger()" style="margin-top:0.25rem;">+ Add trigger</button>
                  </section>
                </div>
              </div>
            </template>

           </div><!-- .agent-bots-scroll -->

            <div class="agent-bot-form-footer hairline" x-show="agentBotFormMode">
              <button class="btn-solid" type="button"
                      :disabled="!agentBotDraft.name.trim() || agentBotSaving"
                      @click="saveAgentBot()"
                      x-text="agentBotSaving ? 'Saving…' : 'Save'"></button>
              <button class="btn-outline" type="button" @click="agentBotFormMode = null">Cancel</button>
              <span class="agent-bot-form-err" x-text="agentBotSaveError"></span>
            </div>

          </div><!-- .agent-bots-pane -->

`
}

const settingsAgentBotsJS = `
      agentBotsList: [],
      agentBotsLoading: false,
      agentBotsError: '',
      agentBotFormMode: null,
      agentBotTemplatePicker: false,
      agentBotEditId: '',
      // Full shape up front: the hidden form footer still evaluates name.trim().
      agentBotDraft: { name: '', description: '', agentId: '', model: '', cwd: '', systemPrompt: '', enabled: true },
      agentBotTriggers: [],
      agentBotSaving: false,
      agentBotSaveError: '',
      agentBotEventDefs: [],
      // One table instead of the same 4-way "which kind of trigger is this"
      // branch repeated across agentBotPromptVarsForTrigger/
      // agentBotPromptPlaceholder/the Event block's hint text — a new
      // trigger kind's vars/placeholder/hint get added once here, not in
      // three separate places that have to be kept in sync by hand.
      AGENT_BOT_EVENT_KINDS: {
        scheduled: {
          vars: [
            { token: '{Now}', desc: 'Trigger time (RFC3339)' },
            { token: '{Date}', desc: 'Date YYYY-MM-DD' },
            { token: '{Time}', desc: 'Time HH:MM' },
          ],
          placeholder: 'Review my vault and write a daily digest. Time: {Now}',
          hint: 'Scheduled triggers run on a timer — choose one schedule below',
        },
        wechat_message: {
          vars: [
            { token: '{Content}', desc: 'Incoming WeChat DM text' },
            { token: '{WechatUserID}', desc: 'Sender WeChat user ID' },
          ],
          placeholder: 'Reply to this WeChat message:\n\n{Content}',
          hint: 'WeChat DM trigger — use Content and WechatUserID in the prompt',
        },
        compile_requested: {
          vars: [
            { token: '{Path}', desc: 'Vault path of the note to compile' },
            { token: '{Name}', desc: 'Filename without extension' },
          ],
          placeholder: 'Compile {Path} into knowledge units.',
          hint: 'Fires when the user manually triggers compilation via the API — Path carries the note',
        },
        agent_run_completed: {
          vars: [
            { token: '{Name}', desc: 'Name of the agent bot whose run just succeeded' },
            { token: '{Content}', desc: 'Last assistant message from the completed run' },
            { token: '{Now}', desc: 'Trigger time (RFC3339)' },
          ],
          placeholder: '{Name} just finished — review its output and take action.',
          hint: "Fires when another agent bot's run succeeds — filter by source agent bot names below",
        },
      },
      // Fallback for every other (vault) event type — note_created/
      // note_updated/note_deleted/short_note_created all share this one
      // set, so it doesn't fit the per-kind table as a single key.
      agentBotPromptVarsVault: [
        { token: '{Path}', desc: 'Full vault path of the affected note' },
        { token: '{Name}', desc: 'Filename without extension' },
        { token: '{Content}', desc: 'Appended short-note text (short_note_created only)' },
      ],
      agentBotSchedulePresets: {
        every: [
          { label: 'Every hour', value: 'every 1h' },
          { label: 'Every 6 hours', value: 'every 6h' },
        ],
        daily: [
          { label: 'Daily 09:00', value: 'daily 09:00' },
          { label: 'Daily 21:00', value: 'daily 21:00' },
        ],
      },
      agentBotWeekdayDefs: [
        { abbr: 'mon', label: 'Mon' },
        { abbr: 'tue', label: 'Tue' },
        { abbr: 'wed', label: 'Wed' },
        { abbr: 'thu', label: 'Thu' },
        { abbr: 'fri', label: 'Fri' },
        { abbr: 'sat', label: 'Sat' },
        { abbr: 'sun', label: 'Sun' },
      ],
      // Trigger objects here are partial — chooseAgentBotTemplate() fills in
      // the id/mateId/pathPrefixes/enabled defaults addAgentBotTrigger() also sets.
      agentBotTemplates: [
        {
          id: 'compiler',
          name: 'Compiler',
          description: 'Compile notes into knowledge units — on demand, and automatically for new Web Clips.',
          systemPrompt: 'Always use the hylo-compile-note and hylo-index-knowledge skills to compile and index notes.',
          triggers: [
            { eventTypes: ['compile_requested'], prompt: '` + agentBotCompilerPrompt + `' },
            { eventTypes: ['note_created'], pathPrefixes: ['/Web Clips/'], prompt: '` + agentBotCompilerPrompt + `' },
          ],
        },
        {
          id: 'memory',
          name: 'Daily Memory',
          description: 'Extract personal memory from recent notes once a day.',
          triggers: [
            { eventTypes: ['scheduled'], schedule: 'daily 09:00',
              prompt: "Please update my personal memory. I'm [name], currently working on [project]." },
          ],
        },
        {
          id: 'recap',
          name: 'Recap',
          description: 'Cross-check new short notes against the knowledge base as you capture them, and review everything written each week.',
          triggers: [
            { eventTypes: ['short_note_created'], prompt: '` + agentBotRecapShortNotePrompt + `' },
            { eventTypes: ['scheduled'], schedule: 'weekly mon 09:00',
              prompt: 'Use the hylo-recap skill to review everything I wrote this past week.' },
          ],
        },
      ],
      async loadAgentBots() {
        this.agentBotsLoading = true;
        this.agentBotsError = '';
        try {
          const r = await fetch('/api/mates');
          if (!r.ok) throw new Error('HTTP ' + r.status);
          const d = await r.json();
          this.agentBotsList = d.mates || [];
        } catch(e) { this.agentBotsError = e.message; }
        finally { this.agentBotsLoading = false; }
      },

      async loadAgentBotEvents() {
        try {
          const r = await fetch('/api/mate-events');
          if (!r.ok) return;
          const d = await r.json();
          this.agentBotEventDefs = d.events || [];
        } catch(_) {}
      },

      agentBotColor(name) {
        return window.agentBotColorFor(name, this.agentBotsList);
      },

      newAgentBot() {
        this.agentBotEditId = '';
        this.agentBotSaveError = '';
        this.agentBotTemplatePicker = true;
      },

      chooseAgentBotTemplate(tpl) {
        const first = this.agents.find(function(a){ return a.available; });
        const firstModel = (first && first.models && first.models.length) ? first.models[0].id : '';
        this.agentBotDraft = { name: tpl.name, description: tpl.description, agentId: first ? first.id : '', model: firstModel, cwd: '', systemPrompt: tpl.systemPrompt || '', enabled: true };
        this.agentBotTriggers = (tpl.triggers || []).map(function(t) {
          return Object.assign({ id: '', mateId: '', schedule: '', pathPrefixes: [], enabled: true, _open: false }, t);
        });
        this.agentBotTemplatePicker = false;
        this.agentBotFormMode = 'create';
      },

      async openAgentBotEdit(id) {
        try {
          const [r] = await Promise.all([
            fetch('/api/mates/' + id),
            this.agentsLoaded ? Promise.resolve() : this.loadAgents(),
          ]);
          if (!r.ok) throw new Error('HTTP ' + r.status);
          const d = await r.json();
          const m = d.mate;
          const savedModel = m.model || '';
          const knownModels = this.agentBotModelsForAgent(m.agentId);
          const modelValid = !savedModel || savedModel === 'default' ||
            knownModels.length === 0 ||
            knownModels.some(function(x) { return x.id === savedModel; });
          this.agentBotDraft = { name: m.name, description: m.description || '', agentId: m.agentId, model: modelValid ? savedModel : '', cwd: m.cwd || '', systemPrompt: m.systemPrompt || '', enabled: m.enabled };
          this.agentBotTriggers = (m.triggers || []).map(function(t) {
            return Object.assign({}, t, {
              eventTypes: t.eventTypes || [],
              schedule: t.schedule || '',
              pathPrefixes: t.pathPrefixes || [],
              _open: false,
            });
          });
          this.agentBotEditId = m.id;
          this.agentBotSaveError = '';
          this.agentBotFormMode = 'edit';
        } catch(e) { window.showError('Load failed: ' + e.message, 'Load failed'); }
      },

      agentBotModelsForAgent(agentId) {
        const a = this.agents.find(function(x) { return x.id === agentId; });
        return (a && a.models) ? a.models : [];
      },

      onAgentBotAgentChange() {
        const models = this.agentBotModelsForAgent(this.agentBotDraft.agentId);
        this.agentBotDraft.model = models.length ? models[0].id : '';
      },

      async saveAgentBot() {
        if (this.agentBotSaving || !this.agentBotDraft.name.trim()) return;
        this.agentBotSaving = true;
        this.agentBotSaveError = '';
        try {
          const payload = Object.assign({}, this.agentBotDraft, { triggers: this.agentBotTriggers.map(function(t) {
            const copy = Object.assign({}, t);
            delete copy._open;
            return copy;
          }) });
          const url = this.agentBotFormMode === 'create' ? '/api/mates' : '/api/mates/' + this.agentBotEditId;
          const method = this.agentBotFormMode === 'create' ? 'POST' : 'PUT';
          const r = await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
          if (!r.ok) { this.agentBotSaveError = await r.text(); return; }
          const d = await r.json();
          const saved = Object.assign({}, d.mate, { triggerCount: (d.mate.triggers || []).length });
          if (this.agentBotFormMode === 'create') {
            this.agentBotsList.push(saved);
          } else {
            this.agentBotsList = this.agentBotsList.map(function(m) { return m.id === saved.id ? saved : m; });
          }
          this.agentBotFormMode = null;
        } catch(e) { this.agentBotSaveError = e.message; }
        finally { this.agentBotSaving = false; }
      },

      async deleteAgentBot(id) {
        const ok = await window.showConfirm({ title: 'Delete agent bot', message: 'This agent bot and all its data will be permanently deleted.', confirmLabel: 'Delete', danger: true });
        if (!ok) return;
        try {
          const r = await fetch('/api/mates/' + id, { method: 'DELETE' });
          if (!r.ok) { window.showError('Delete failed (server error)', 'Delete failed'); return; }
          this.agentBotsList = this.agentBotsList.filter(function(m) { return m.id !== id; });
        } catch(e) { window.showError('Delete failed: ' + e.message, 'Delete failed'); }
      },

      async moveAgentBot(idx, dir) {
        var newIdx = idx + dir;
        if (newIdx < 0 || newIdx >= this.agentBotsList.length) return;
        var tmp = this.agentBotsList[idx];
        this.agentBotsList[idx] = this.agentBotsList[newIdx];
        this.agentBotsList[newIdx] = tmp;
        this.agentBotsList = this.agentBotsList.slice();
        try {
          await fetch('/api/mates/reorder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids: this.agentBotsList.map(function(m) { return m.id; }) }),
          });
        } catch(_) {}
      },

      addAgentBotTrigger() {
        this.agentBotTriggers.push({ id: '', mateId: '', eventTypes: ['note_created'], schedule: '', prompt: '', pathPrefixes: [], enabled: true, _open: true });
      },

      removeAgentBotTrigger(idx) { this.agentBotTriggers.splice(idx, 1); },

      // Collapsed-state label: event + the one extra detail (schedule, or
      // path/source-bot filter) that distinguishes it from another trigger
      // on the same event, so a glance at the closed card is still useful.
      agentBotTriggerSummary(t) {
        const def = this.agentBotEventDefs.find(function(d){ return d.type === (t.eventTypes[0] || ''); });
        const label = def ? def.label : (t.eventTypes[0] || 'Select event…');
        if (this.eventOf(t) === 'scheduled') return t.schedule ? label + ' · ' + t.schedule : label;
        if ((t.pathPrefixes || []).length) return label + ' · ' + t.pathPrefixes.join(', ');
        return label;
      },

      // UI writes exactly one event type per trigger, so it keys the table.
      eventOf(t) { return (t.eventTypes || [])[0] || ''; },
      agentBotEventKind(t) { return this.AGENT_BOT_EVENT_KINDS[this.eventOf(t)] || null; },

      // Schedule and WeChat triggers carry no path; the rest filter by path prefix
      // (or, for agent_run_completed, by source bot name — same field).
      hasPathFilter(t) { return ['scheduled', 'wechat_message'].indexOf(this.eventOf(t)) < 0; },
      agentBotFilterFor(t) {
        if (this.eventOf(t) === 'agent_run_completed') {
          return {
            title: 'Source Agent Bots',
            hint: 'Only fire when the run was completed by one of these agent bots. Leave empty to fire on any agent bot. One agent bot name per line.',
            placeholder: 'Summarizer\nCompiler',
          };
        }
        return {
          title: 'Path Prefixes',
          hint: 'Only fire when the event path starts with one of these prefixes. Leave empty to match all paths. One prefix per line, e.g. /journal/',
          placeholder: '/journal/\n/projects/work/',
        };
      },

      agentBotPromptVarsForTrigger(t) {
        const kind = this.agentBotEventKind(t);
        return kind ? kind.vars : this.agentBotPromptVarsVault;
      },

      agentBotPromptPlaceholder(t) {
        const kind = this.agentBotEventKind(t);
        return kind ? kind.placeholder : 'Summarize the key points in {Path}';
      },

      agentBotTriggerHint(t) {
        const kind = this.agentBotEventKind(t);
        return kind ? kind.hint : 'Vault event that activates this trigger';
      },

      setAgentBotET(trigger, et) {
        if (et === 'scheduled') {
          trigger.eventTypes = ['scheduled'];
          if (!trigger.schedule) trigger.schedule = 'daily 09:00';
        } else {
          trigger.eventTypes = [et];
          trigger.schedule = '';
        }
      },

      // Backend owns the grammar ("every 1h" / "daily HH:MM" / "weekly d1,d2 HH:MM");
      // this is the one place the UI reads it back.
      parseAgentBotSchedule(t) {
        const parts = (t.schedule || '').trim().toLowerCase().split(/\s+/);
        const kind = parts[0] === 'every' || parts[0] === 'weekly' ? parts[0] : 'daily';
        if (kind !== 'weekly') return { kind: kind, days: [], time: '09:00' };
        const days = !parts[1] || parts[1] === 'none' ? [] : parts[1].split(',').filter(Boolean);
        return { kind: 'weekly', days: days, time: parts[2] || '09:00' };
      },
      scheduleKindOf(t) { return this.parseAgentBotSchedule(t).kind; },
      weeklyDaysOf(t) { return this.parseAgentBotSchedule(t).days; },

      setScheduleKind(t, kind) {
        if (this.scheduleKindOf(t) === kind) return;
        t.schedule = { every: 'every 1h', daily: 'daily 09:00', weekly: 'weekly mon 09:00' }[kind];
      },

      // dayField falls back to the "none" sentinel (instead of an empty string) when the
      // last selected day is removed, so the schedule string stays prefixed "weekly " and
      // scheduleKindOf() keeps the Weekly tab active rather than falling back to Daily.
      toggleWeeklyDay(t, abbr) {
        const order = this.agentBotWeekdayDefs.map(function(d) { return d.abbr; });
        let days = this.weeklyDaysOf(t);
        if (days.indexOf(abbr) >= 0) {
          days = days.filter(function(d) { return d !== abbr; });
        } else {
          days = days.concat([abbr]);
        }
        days.sort(function(a, b) { return order.indexOf(a) - order.indexOf(b); });
        t.schedule = 'weekly ' + (days.length ? days.join(',') : 'none') + ' ' + this.parseAgentBotSchedule(t).time;
      },

      insertAgentBotVar(ti, token, event) {
        const promptBlock = event.target.closest('.agent-bot-trigger-prompt');
        const ta = promptBlock && promptBlock.querySelector('textarea');
        if (ta && typeof ta.selectionStart === 'number') {
          const start = ta.selectionStart;
          const end = ta.selectionEnd;
          const val = ta.value || '';
          ta.value = val.slice(0, start) + token + val.slice(end);
          ta.dispatchEvent(new Event('input', { bubbles: true }));
          const pos = start + token.length;
          ta.focus();
          ta.setSelectionRange(pos, pos);
          return;
        }
        const t = this.agentBotTriggers[ti];
        if (!t) return;
        t.prompt = (t.prompt || '') + token;
      },

`
