package view

import (
	"fmt"
	"strings"
)

// settingsModalCSS/settingsModalHTML/settingsCtrlJS assemble the settings
// modal's shared shell (overlay, panel, sidebar nav, pane chrome) with each
// tab's own CSS/HTML/JS slice, split into its own shared_settings_<tab>.go
// file so a single tab can be read/reviewed without the other six. Order
// matters for settingsModalHTML (it's visible DOM/tab order) but not for
// the CSS/JS consts (pure concatenation, no ordering dependency).
const settingsModalCSS = `
    /* ── Settings modal overlay ─────────────────────────────── */
    [x-cloak] { display: none !important; }
    .settings-modal-overlay {
      position: fixed; inset: 0; z-index: 1000;
      background: var(--overlay-bg);
      backdrop-filter: var(--glass-scrim-filter);
      -webkit-backdrop-filter: var(--glass-scrim-filter);
      border-radius: 0; /* full-viewport scrim — never rounds */
      display: flex; align-items: center; justify-content: center;
    }
    .settings-modal-panel {
      width: 1040px; max-width: calc(100vw - 2rem);
      height: 720px; max-height: calc(100vh - 2rem);
      background: var(--bg);
      /* Shadow carries the floating edge; the line is only a whisper. */
      border: var(--bd-w) solid var(--line-hair);
      border-radius: var(--r-xl);
      box-shadow: var(--shadow-lg) var(--shadow-color);
      display: flex; flex-direction: column; overflow: hidden;
      /* Anchors .settings-modal-close, now that there's no title bar for it
         to sit in. */
      position: relative;
    }

    /* Positioned against the whole panel rather than just .settings-content,
       so it also floats above .settings-sidebar (which has no header of its
       own) — same 12px corner offset as the app's other floating chrome
       (graph's zoom controls/node panel). Lands inside the content header's
       row (below) on the right. */
    .settings-modal-close {
      position: absolute; top: 12px; right: 12px; z-index: 5;
    }
    /* The desktop webview builds the window drag mask from -webkit-app-region rects in
       DOM order (drag = union, no-drag = subtract); z-index and selector
       specificity don't matter. The panel's no-drag subtracts the page's
       own drag strips underneath it; the reused .home-list-head re-adds
       drag (home.css), which covers this button's corner — so the button
       must come AFTER .settings-modal-inner in the DOM for its no-drag to
       win. Don't move it back to the top of the panel. */
    html.macos .settings-modal-panel,
    html.macos .settings-modal-close { -webkit-app-region: no-drag; }
    .settings-modal-inner {
      flex: 1; min-height: 0; display: flex; overflow: hidden;
      border-radius: 0;
    }

    /* ── Settings inner layout ───────────────────────────────── */

    /* ── Primary sidebar ──────────────────────────────────────── */
    .settings-sidebar {
      flex-shrink: 0; width: 196px;
      border-right: var(--bd-w) solid var(--line-hair);
      background: var(--surface-soft);
      padding: 1rem 0.5rem; display: flex; flex-direction: column;
      gap: 2px; user-select: none;
      /* .side-nav-item.is-active (base.css) just consumes the generic
         --control-active-bg — but since this container's own background is
         --surface-soft rather than --bg, the token is locally repointed at
         --control-active-bg-on-soft (shared_tokens.go) so the selected item
         still reads as a clear lift. Same fix as .home-side (home.css). */
      --control-active-bg: var(--control-active-bg-on-soft);
    }

    /* ── Content area ─────────────────────────────────────────── */
    .settings-content { flex: 1; min-width: 0; display: flex; flex-direction: column; overflow: hidden; border-radius: 0; }
    /* Reuses the shared .home-list-head (home.css) so this pane's top
       chrome matches every other section's header row instead of
       inventing its own; just a title here since the pane below already
       carries whatever controls it needs. */
    .settings-content-title { font-size: var(--text-base); font-weight: 600; color: var(--fg); }

    /* Shared "back button (+ optional title)" row at the top of a detail
       view — Server's config-section detail and Agent Bots' template
       picker/form all push into one via the same back-button pattern
       (agentBotBackBtnHTML). Only the margin below it varies per caller
       (.cfg-detail-header / .agent-bot-form-header). */
    .settings-detail-header { display: flex; align-items: center; gap: 0.75rem; }

    /* ── Pane ─────────────────────────────────────────────────── */
    /* Shared by every tab that's just "a scrollable column of fields"
       (Appearance/Shortcuts/Notifications/Skills/Agent CLI) — Server and
       Agent Bots have their own multi-part layouts (.cfg-content,
       .agent-bots-pane) and don't use this. */
    .settings-scroll-pane { flex: 1; overflow-y: auto; padding: 1.75rem 1.5rem 3rem; }
    ::-webkit-scrollbar { display: none; }

` + settingsAppearanceCSS + settingsServerCSS + settingsWechatCSS + settingsAgentCLICSS + settingsNotificationsCSS + settingsShortcutsCSS + settingsAgentBotsCSS + settingsSkillsCSS

// settingsModalHTML returns the settings modal DOM. Include once per page —
// opened via the sidebar's bottom-row Settings button (see home.html).
func settingsModalHTML() string {
	return `
  <div id="hylo-settings-modal"
       x-data="settingsCtrl()"
       x-show="$store.settingsModal.open"
       x-effect="__hyloTrapReturn($el, $store.settingsModal.open)"
       x-trap.inert.noreturn="$store.settingsModal.open"
       x-cloak
       class="settings-modal-overlay">
    <div class="settings-modal-panel" @mousedown.stop>
      <div class="settings-modal-inner">

        <!-- Primary sidebar -->
` + settingsNavHTML() + `

        <div class="settings-content">

          <div class="home-list-head">
            <span class="settings-content-title" x-text="` + settingsTabTitlesJS() + `[tab] || ''"></span>
          </div>

` +
		settingsAppearanceTabHTML() +
		settingsServerTabHTML() +
		settingsShortcutsTabHTML() +
		settingsNotificationsTabHTML() +
		settingsAgentBotsTabHTML() +
		settingsSkillsTabHTML() +
		settingsAgentCLITabHTML() +
		`
        </div><!-- .settings-content -->
      </div><!-- .settings-modal-inner -->
      <!-- Must stay after .settings-modal-inner: see the -webkit-app-region note in the CSS above. -->
      <button class="icon-btn-ghost settings-modal-close" @click="requestClose()" type="button">
        <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M18 6 6 18"/>
          <path stroke-linecap="round" d="m6 6 12 12"/>
        </svg>
      </button>
    </div><!-- .settings-modal-panel -->
  </div><!-- #hylo-settings-modal -->
`
}

// settingsCtrlJS is the Alpine.js controller for the settings modal. State
// and methods shared across ≥2 tabs (the agents/loadAgents() list used by
// both Agent Bots' picker and the Agent CLI tab; init()'s tab-switch
// orchestration) stay here; everything tab-specific lives in that tab's own
// shared_settings_<tab>.go file as a *JS const (settingsXJS), concatenated
// in below. Property/method order doesn't matter to Alpine.
const settingsCtrlJS = `
  function settingsCtrl() {
    return {
` +
	`      isDesktop: !!window.hyloDesktop,
      tab: 'appearance',
      agents: [],
      agentsLoading: false,
      agentsError: '',
      agentsLoaded: false,
      agentsFromCache: false,
      agentsCachedAt: 0,
      async init() {
        // Another same-origin view can change the accent; keep the picker's ring in step.
        window.addEventListener('hylo:accent', () => {
          var id = localStorage.getItem('hylo-accent');
          this.accentPref = this.accentPresets.some(p => p.id === id) ? id : 'indigo';
        });
        window.addEventListener('hylo:switch-server', (e) => {
          const id = (e.detail && e.detail.id) || '';
          if (id) this.requestSwitch(id);
        });
        window.__hyloHotkeys.register('open-settings', ',', function() {
          Alpine.store('settingsModal').open = true;
        });
        this.$watch('tab', (val) => { if (Alpine.store('settingsModal').open) this.loadTab(val); });
        this.$watch('$store.settingsModal.open', (open) => {
          if (!open) {
            if (window.__hyloEscPop) window.__hyloEscPop('settings');
            this.cancelWechatLogin();
            return;
          }
          if (window.__hyloEscPush) window.__hyloEscPush('settings', () => this.requestClose());
          this.loadTab(this.tab);
        });
      },

      async requestClose() {
        if (this.hasDirty) {
          const ok = await window.showConfirm({ title: 'Discard changes?', message: 'You have unsaved server settings.', confirmLabel: 'Discard', danger: true });
          if (!ok) return;
          this.discardAll();
        }
        Alpine.store('settingsModal').open = false;
      },

      loadTab(tab) {
        if (tab === 'server') { this.loadServerStatus(); if (!this.hasDirty) this.loadConfig(); }
        else if (tab === 'agent-bots') {
          this.loadAgentBots();
          if (!this.agentsLoaded) this.loadAgents();
          if (!this.agentBotEventDefs.length) this.loadAgentBotEvents();
        }
        else if (tab === 'skills') this.loadSkills();
        else if (tab === 'agents') { if (!this.agentsLoaded) this.loadAgents(); }
        else if (tab === 'notifications') this.loadNotifySettings();
      },

      async loadAgents(force = false) {
        // Reuse the in-flight promise so a concurrent openAgentBotEdit call doesn't
        // issue a second request while the first is still pending.
        if (!force && this._agentsLoadPromise) return this._agentsLoadPromise;
        this.agentsLoading = true;
        this.agentsError = '';
        const p = (async () => {
          try {
            const url = force ? '/api/agents?force=true' : '/api/agents';
            const r = await fetch(url);
            if (!r.ok) throw new Error('HTTP ' + r.status);
            const d = await r.json();
            this.agents = d.agents || [];
            this.agentsFromCache = d.fromCache || false;
            this.agentsCachedAt = d.fetchedAt || 0;
            this.agentsLoaded = true;
          } catch (e) { this.agentsError = e.message; }
          finally { this.agentsLoading = false; this._agentsLoadPromise = null; }
        })();
        this._agentsLoadPromise = p;
        return p;
      },

      get sortedAgents() {
        return [...this.agents].sort((a, b) => (b.available ? 1 : 0) - (a.available ? 1 : 0));
      },

` +
	settingsAppearanceJS + settingsServerJS + settingsWechatJS + settingsAgentCLIJS + settingsNotificationsJS + settingsShortcutsJS + settingsAgentBotsJS + settingsSkillsJS +
	`    };
  }
`

type settingsTab struct {
	id, label, icon string
	desktopOnly     bool
}

var settingsTabs = []settingsTab{
	{id: "appearance", label: "Appearance", desktopOnly: false, icon: `<circle cx="12" cy="12" r="10"/><path d="M12 18a6 6 0 0 0 0-12z" fill="currentColor" stroke="none"/>`},
	{id: "server", label: "Server", desktopOnly: false, icon: `<rect width="20" height="8" x="2" y="2" rx="2"/><rect width="20" height="8" x="2" y="14" rx="2"/><path stroke-linecap="round" d="M6 6h.01"/><path stroke-linecap="round" d="M6 18h.01"/>`},
	{id: "agent-bots", label: "Agent Bots", desktopOnly: false, icon: `<path d="M12 6V2H8"/><path d="M15 11v2"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="M20 16a2 2 0 0 1-2 2H8.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 4 20.286V8a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2z"/><path d="M9 11v2"/>`},
	{id: "skills", label: "Skills", desktopOnly: false, icon: `<path stroke-linecap="round" stroke-linejoin="round" d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 2v4"/><path stroke-linecap="round" stroke-linejoin="round" d="M22 4h-4"/><circle cx="4" cy="20" r="2"/>`},
	{id: "agents", label: "Agent CLI", desktopOnly: false, icon: `<path stroke-linecap="round" stroke-linejoin="round" d="M12 20v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 2v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M17 20v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M17 2v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M2 12h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M2 17h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M2 7h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 12h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 17h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 7h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 20v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 2v2"/><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="8" y="8" width="8" height="8" rx="1"/>`},
	{id: "notifications", label: "Notifications", desktopOnly: true, icon: `<path stroke-linecap="round" stroke-linejoin="round" d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path stroke-linecap="round" stroke-linejoin="round" d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>`},
	{id: "shortcuts", label: "Shortcuts", desktopOnly: false, icon: `<path stroke-linecap="round" d="M10 8h.01"/><path stroke-linecap="round" d="M12 12h.01"/><path stroke-linecap="round" d="M14 8h.01"/><path stroke-linecap="round" d="M16 12h.01"/><path stroke-linecap="round" d="M18 8h.01"/><path stroke-linecap="round" d="M6 8h.01"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 16h10"/><path stroke-linecap="round" d="M8 12h.01"/><rect width="20" height="16" x="2" y="4" rx="2"/>`},
}

func settingsNavHTML() string {
	var b strings.Builder
	b.WriteString(`<nav class="settings-sidebar hairline">`)
	for _, t := range settingsTabs {
		show := ""
		if t.desktopOnly {
			show = ` x-show="isDesktop"`
		}
		fmt.Fprintf(&b, `<button class="side-nav-item settings-sidebar-item"%s :class="{'is-active': tab === '%s'}" x-press="tab = '%s'">`+
			`<svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">%s</svg>%s</button>`,
			show, t.id, t.id, t.icon, t.label)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

func settingsTabTitlesJS() string {
	parts := make([]string, len(settingsTabs))
	for i, t := range settingsTabs {
		parts[i] = fmt.Sprintf("'%s':'%s'", t.id, t.label)
	}
	return "({" + strings.Join(parts, ",") + "})"
}

func cselectHTML(labelExpr, body string) string {
	return `<div class="cselect" x-data="{ csOpen: false }" @click.outside="csOpen = false">` +
		`<button type="button" class="cselect-btn" :class="{open: csOpen}" @click="csOpen = !csOpen" @keydown.escape="csOpen = false">` +
		`<span class="cselect-btn-text" x-text="` + labelExpr + `"></span>` +
		`<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/></svg>` +
		`</button><div class="cselect-dropdown" x-show="csOpen">` + body + `</div></div>`
}

func agentBotBackBtnHTML(onclick, label string) string {
	return `<button class="agent-bot-back-btn" type="button" @click="` + onclick + `">` +
		`<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/></svg>` +
		label + `</button>`
}

func toolbarRefreshBtnHTML(loadingExpr, onclick, busyLabel string) string {
	return `<button class="btn-outline" :class="{spinning: ` + loadingExpr + `}" @click="` + onclick + `" :disabled="` + loadingExpr + `">` +
		`<svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path stroke-linecap="round" stroke-linejoin="round" d="M3 3v5h5"/><path stroke-linecap="round" stroke-linejoin="round" d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path stroke-linecap="round" stroke-linejoin="round" d="M16 16h5v5"/></svg>` +
		`<span x-text="` + loadingExpr + ` ? '` + busyLabel + `' : 'Refresh'"></span></button>`
}
