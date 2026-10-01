package view

// settingsModalCSS contains the modal overlay styles and all settings-specific CSS.
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
      /* --hairline, not --border — the shadow below already carries the
         floating edge; the line itself just needs to be a whisper. */
      border: var(--bd-w) solid var(--hairline);
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
    /* Sits outside .home-list-head in the DOM, so its drag region isn't
       carved out by that selector's own "button" no-drag rule (home.css) —
       needs its own, or the click never reaches it. */
    html.macos .settings-modal-close { -webkit-app-region: no-drag; }
    .settings-modal-inner {
      flex: 1; min-height: 0; display: flex; overflow: hidden;
      border-radius: 0;
    }

    /* ── Settings inner layout ───────────────────────────────── */
    .settings-body { flex: 1; display: flex; min-height: 0; overflow: hidden; }

    /* ── Primary sidebar ──────────────────────────────────────── */
    .settings-sidebar {
      flex-shrink: 0; width: 196px;
      /* --hairline, not --border — matches .home-side's sidebar/content
         seam (home.css) so both dividers read at the same weight. */
      border-right: var(--bd-w) solid var(--hairline);
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
    /* Row/hover/active/icon now come from the shared .side-nav-item /
       .side-nav-icon (base.css). */

    /* ── Content area ─────────────────────────────────────────── */
    .settings-content { flex: 1; min-width: 0; display: flex; flex-direction: column; overflow: hidden; border-radius: 0; }
    /* Reuses the shared .home-list-head (home.css) so this pane's top
       chrome matches every other section's header row instead of
       inventing its own; just a title here since the pane below already
       carries whatever controls it needs. */
    .settings-content-title { font-size: var(--text-base); font-weight: 600; color: var(--fg); }

    /* ── Pane ─────────────────────────────────────────────────── */
    .settings-pane { flex: 1; overflow-y: auto; padding: 1.75rem 1.5rem 3rem; }
    ::-webkit-scrollbar { display: none; }

    /* ── Appearance fields ────────────────────────────────────── */
    .settings-fields { max-width: 640px; display: flex; flex-direction: column; gap: 1.75rem; }
    .settings-field-label {
      display: block; font-size: var(--text-sm); font-weight: 600;
      color: var(--fg); margin-bottom: 0.5rem;
    }
    .settings-field-desc { font-size: var(--text-xs); color: var(--muted); margin-top: 0.4rem; line-height: 1.5; }
    .settings-field-row { display: flex; gap: 0.5rem; align-items: center; }
    /* Box/font come from the shared .field-input (base.css); this is
       inline next to a button in a flex row, not full-width. */
    .settings-input { flex: 1; min-width: 0; }
    /* Apply/save/toolbar/danger buttons all now use the shared
       .btn-outline / .btn-solid / .btn-solid--danger classes (base.css). */
    .settings-error { margin-top: 0.4rem; font-size: var(--text-xs); color: var(--s-err); }

    /* Theme/Enter-Effect/schedule-kind pickers use the shared .seg/.seg-btn
       component (base.css) instead of their own copy. */

    .accent-swatches { display: flex; flex-wrap: wrap; gap: 0.75rem; padding: 0.4rem; }
    /* Ring is a pseudo-element border, not outline: the page-wide
       "button:focus { outline: none }" (images.css/shorts.css) would wipe an
       outline the moment a click focuses the swatch. */
    .accent-swatch {
      position: relative; width: 1.5rem; height: 1.5rem; padding: 0; border: none;
      cursor: pointer; border-radius: var(--r-full); background: var(--sw);
    }
    .accent-swatch::after {
      content: ''; position: absolute; inset: -5px; border-radius: var(--r-full);
      border: 2px solid transparent; transition: border-color var(--motion-fast);
    }
    .accent-swatch:hover::after { border-color: rgba(var(--ink-rgb), 0.25); }
    .accent-swatch.active::after { border-color: var(--sw); }

    /* ── Server config ────────────────────────────────────────── */
    .cfg-content { flex: 1; min-width: 0; display: flex; flex-direction: column; overflow: hidden; }
    .cfg-action-bar {
      flex-shrink: 0; display: flex; align-items: center;
      justify-content: space-between; flex-wrap: wrap;
      gap: 0.75rem 1rem; margin-top: 2rem; padding-top: 1.25rem;
      border-top: var(--bd-w) solid var(--hairline); max-width: 640px;
    }
    .cfg-action-left { display: flex; align-items: center; gap: 0.75rem; flex: 1; min-width: 0; }
    .cfg-action-right { display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0; }
    .cfg-status-ok { font-size: var(--text-xs); color: var(--s-ok); font-weight: 500; }
    .cfg-status-err {
      font-size: var(--text-xs); color: var(--s-err); font-weight: 500;
      overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
      border-radius: 0;
    }
    .cfg-restart-note { font-size: var(--text-xs); color: var(--muted); }
    /* .cfg-dirty-badge now uses the shared .badge (base.css) exactly —
       its extra 1px outline had no functional reason to exist, so it's
       gone rather than kept as a one-off. */
    /* .cfg-discard-btn now uses the shared .btn-outline (base.css). */
    .cfg-pane-area { flex: 1; min-height: 0; position: relative; overflow: hidden; }
    .cfg-pane {
      position: absolute; inset: 0; overflow-y: auto;
      padding: 1.75rem 1.5rem 3rem;
    }
    .cfg-fields { max-width: 640px; display: flex; flex-direction: column; }
    /* No border — a card reads as a card from its background against the
       pane behind it (same idiom as home-note-row/img-card), not a line. */
    .cfg-section {
      max-width: 640px; margin-bottom: 0.45rem;
      border-radius: var(--r-lg);
      background: var(--surface-soft); overflow: hidden;
    }
    /* Neutral, not card-hov — is-open is a structural state, hover (below)
       is transient interaction feedback; keeping them on different tokens
       means glancing at a closed row mid-hover can't be mistaken for one
       that's actually expanded. */
    .cfg-section.is-open { background: var(--surface-2); }
    .cfg-section-head {
      display: flex; align-items: flex-start; justify-content: space-between;
      gap: 0.75rem; width: 100%; margin: 0; padding: 0.7rem 0.9rem;
      border: none; background: transparent; cursor: pointer;
      text-align: left; color: inherit;
    }
    .cfg-section-head:hover { background: var(--card-hov); }
    .cfg-section.is-open .cfg-section-head { border-bottom: var(--bd-w) solid var(--hairline); }
    .cfg-section-head-text { min-width: 0; flex: 1; }
    .cfg-section-title {
      font-size: var(--text-sm); font-weight: 600; letter-spacing: -0.01em;
      color: var(--fg); margin: 0; padding: 0; display: block;
    }
    .cfg-section-head-desc {
      display: block; font-size: var(--text-xs); color: var(--muted);
      margin: 0.28rem 0 0; line-height: 1.5;
    }
    .cfg-section-head-meta {
      display: flex; align-items: center; gap: 0.4rem; flex-shrink: 0; margin-top: 0.1rem;
    }
    /* .cfg-section-dirty-dot now uses the shared .dot / .dot--accent (base.css). */
    .cfg-section-chev {
      flex-shrink: 0; color: var(--muted); display: flex; align-items: center; margin-top: 0.15rem;
    }
    .cfg-section-chev svg { width: 14px; height: 14px; }
    .cfg-section-chev.open svg { transform: rotate(90deg); }
    .cfg-section-body { padding: 0; }
    .cfg-section.is-open .cfg-section-body { padding: 0 0.9rem 0.9rem; }
    .cfg-field {
      display: grid;
      grid-template-columns: 1fr 220px;
      grid-template-areas: "meta ctrl" "desc desc";
      column-gap: 1rem; padding: 0.88rem 0;
      border-bottom: var(--bd-w) solid var(--hairline); align-items: center;
    }
    .cfg-field.multiline {
      grid-template-columns: 1fr;
      grid-template-areas: "meta" "ctrl" "desc";
      align-items: start;
    }
    .cfg-field.multiline .cfg-field-ctrl { justify-content: stretch; margin-top: 0.5rem; }
    .cfg-section.is-open .cfg-field:last-of-type { border-bottom: none; }
    .cfg-field.dirty .cfg-field-label::before {
      content: ''; display: inline-block; width: 5px; height: 5px;
      border-radius: 50%; background: var(--accent);
      margin-right: 5px; vertical-align: middle; margin-bottom: 1px;
    }
    .cfg-field-meta { grid-area: meta; min-width: 0; }
    .cfg-field-ctrl { grid-area: ctrl; display: flex; align-items: center; justify-content: flex-end; }
    .cfg-field-label { font-size: var(--text-sm); font-weight: 500; color: var(--body); display: block; margin-bottom: 0.15rem; }
    .cfg-field-key {
      font-size: var(--text-xs); color: var(--muted); opacity: 0.65;
      font-family: var(--font-mono);
    }
    .cfg-field-desc { grid-area: desc; font-size: var(--text-xs); color: var(--muted); margin: 0.35rem 0 0; line-height: 1.5; }
    .cfg-wechat-auth { padding: 0.88rem 0 0; border-top: var(--bd-w) solid var(--hairline); }
    .cfg-wechat-auth-head {
      display: flex; align-items: center; justify-content: space-between;
      gap: 0.75rem; margin-bottom: 0.65rem;
    }
    .cfg-wechat-auth-title { font-size: var(--text-sm); font-weight: 500; color: var(--body); }
    /* .cfg-wechat-badge now uses the shared .badge / .badge--ok (base.css). */
    .cfg-wechat-meta { font-size: var(--text-xs); color: var(--muted); line-height: 1.55; margin: 0 0 0.75rem; }
    .cfg-wechat-meta code {
      font-size: var(--text-2xs); padding: 0.05rem 0.3rem; border-radius: var(--r-xs);
      background: var(--code-bg);
    }
    .cfg-wechat-actions { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; }
    .cfg-wechat-qr {
      margin-top: 0.85rem; padding: 0.75rem;
      background: var(--code-bg); text-align: center;
    }
    .cfg-wechat-qr img { width: 180px; height: 180px; object-fit: contain; background: #fff; }
    .cfg-wechat-status { font-size: var(--text-xs); color: var(--muted); margin-top: 0.5rem; }
    .cfg-wechat-err { font-size: var(--text-xs); color: var(--s-err); margin-top: 0.5rem; }
    .cfg-wechat-ok { font-size: var(--text-xs); color: var(--s-ok); margin-top: 0.5rem; }
    /* .cfg-input / .cfg-textarea now use the shared .field-input (base.css). */
    input[type="number"].field-input.cfg-input { width: 110px; }
    .cfg-reveal-wrap { display: flex; gap: 0.375rem; align-items: center; width: 100%; }
    /* .cfg-reveal-btn now uses the shared .icon-btn.icon-btn--lg (base.css). */
    /* Bool config fields now use the shared .seg/.seg-btn Off/On toggle
       (base.css) — same picker as Theme/Line Breaks above, not the old
       pill switch. */
    .cfg-loader { font-size: var(--text-xs); color: var(--muted); padding: 2rem 0; }
    .cfg-err-msg { font-size: var(--text-xs); color: var(--s-err); padding: 2rem 0; }

    /* ── Agents tab ───────────────────────────────────────────── */
    .agents-pane { flex: 1; overflow-y: auto; padding: 1.75rem 1.5rem 3rem; }
    .agents-toolbar { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; }
    /* Toolbar's refresh/new buttons now use the shared .btn-outline
       (base.css), which also defines the .spinning svg animation. */
    .agents-summary { font-size: var(--text-xs); color: var(--muted); }
    .agents-list { display: flex; flex-direction: column; gap: 0.45rem; }
    /* Box comes from the shared .list-card (base.css); this is a static
       display card, not a click target, so no interaction modifier. Drops
       the outline and sits on surface-soft instead — same idiom as
       home-note-row/img-card (home.css/images.css). */
    .agent-card {
      display: flex; flex-direction: column; gap: 0.625rem;
      min-width: 0; overflow: hidden;
      border-color: transparent; background: var(--surface-soft);
    }
    .agent-card:hover { background: var(--card-hov); }
    .agent-card.unavailable { opacity: 0.48; }
    .agent-card-top {
      display: flex; align-items: center; gap: 0.6rem; min-width: 0;
    }
    /* .agent-dot now uses the shared .dot / .dot--on / .dot--off (base.css). */
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

    /* ── Editor effects ───────────────────────────────────────── */
    .effect-card {
      display: flex; flex-direction: column; align-items: flex-start;
      padding: 0.38rem 0.8rem; border: none;
      background: var(--btn-bg); cursor: pointer; min-width: 86px;
      text-align: left;
      transition: background-color var(--motion-fast);
    }
    .effect-card:hover { background: var(--btn-bg-hov); }
    .effect-card:active:not(.active) { background: var(--btn-bg-on); }
    .effect-card.active { background: var(--btn-bg-on); color: var(--control-active-fg); }
    .effect-card-name { font-size: var(--text-sm); font-weight: 600; color: var(--fg); }
    .effect-card-desc { font-size: var(--text-xs); color: var(--muted); margin-top: 0.1rem; white-space: nowrap; }

    /* ── Shortcuts pane ───────────────────────────────────────── */
    .shortcuts-pane { flex: 1; overflow-y: auto; padding: 1.75rem 1.5rem 3rem; }
    .notifications-pane { flex: 1; overflow-y: auto; padding: 1.75rem 1.5rem 3rem; }
    .notif-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 0.35rem; }
    .notif-saved { font-size: var(--text-xs); color: var(--s-ok); margin: 0; font-weight: 500; }
    .notif-sound-row { display: flex; align-items: center; gap: 0.5rem; max-width: 280px; }
    /* .notif-play-btn now uses the shared .icon-btn.icon-btn--lg (base.css). */
    .shortcuts-fields { max-width: 640px; }
    .shortcuts-list { display: flex; flex-direction: column; }
    .shortcuts-row {
      display: flex; align-items: center; justify-content: space-between;
      padding: 0.55rem 0; border-bottom: var(--bd-w) solid var(--hairline); gap: 1rem;
    }
    .shortcuts-list .shortcuts-row:last-child { border-bottom: none; }
    .shortcuts-row-meta { min-width: 0; flex: 1; }
    .shortcuts-label { font-size: var(--text-sm); font-weight: 600; color: var(--fg); display: block; }
    .shortcuts-desc { font-size: var(--text-xs); color: var(--muted); margin-top: 0.1rem; }
    .shortcuts-keys { display: flex; gap: 0.3rem; align-items: center; flex-shrink: 0; }
    .kbd {
      display: inline-flex; align-items: center;
      padding: 0.18rem 0.42rem;
      border-radius: var(--r-xs);
      background: var(--bg); border: var(--bd-w) solid var(--border-strong);
      font-family: var(--font-mono);
      font-size: var(--text-xs); color: var(--fg); white-space: nowrap; line-height: 1.4;
    }
    /* .kbd-cmd sizing/alignment now comes from the shared .kbd-combo
       (base.css) — see that rule for why. */

    /* ── Agent Bots pane ───────────────────────────────────────────── */
    .agent-bots-pane { flex: 1; overflow-y: auto; padding: 1.75rem 1.5rem 3rem; }
    .agent-bots-toolbar { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; }
    .agent-bots-list { display: flex; flex-direction: column; gap: 0.45rem; }
    .agent-bots-empty { font-size: var(--text-sm); color: var(--muted); padding: 1.5rem 0; }
    /* Box + hover come from the shared .list-card.list-card--hover
       (base.css); the row itself isn't a click target (actions live in
       its own buttons), so no cursor/active. Same borderless/surface-soft
       override as .agent-card above. */
    .agent-bot-card {
      display: flex; align-items: flex-start; gap: 0.875rem;
      border-color: transparent; background: var(--surface-soft);
    }
    .agent-bot-card.disabled-card { opacity: 0.45; }
    /* .agent-bot-avatar now uses the shared .avatar.avatar--lg.avatar--neutral (base.css). */
    .agent-bot-card-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 0.3rem; }
    .agent-bot-card-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 0.5rem; min-width: 0; }
    .agent-bot-card-name { font-size: var(--text-sm); font-weight: 600; color: var(--fg); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; border-radius: 0; flex: 1; min-width: 0; padding-top: 0.1rem; }
    .agent-bot-card-desc { font-size: var(--text-xs); color: var(--muted); line-height: 1.5; }
    .agent-bot-card-actions { display: flex; gap: 0.3rem; flex-shrink: 0; }
    .agent-bot-card-tags { display: flex; flex-wrap: wrap; gap: 0.35rem; margin-top: 0.15rem; }
    /* Structure/color come from the shared .badge / .badge--ok (base.css). */
    .agent-bot-badge { max-width: 200px; }
    /* Row actions (move/edit/delete) now use the shared .btn-outline /
       .btn-solid.btn-solid--danger with the .btn--xs size modifier. */
    .agent-bot-form-wrap { max-width: 780px; }
    .agent-bot-form-header { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 2rem; }
    .agent-bot-back-btn {
      display: inline-flex; align-items: center; gap: 0.3rem;
      background: none; border: none; color: var(--muted); font-size: var(--text-sm);
      cursor: pointer; padding: 0;
    }
    .agent-bot-back-btn:hover { color: var(--fg); }
    .agent-bot-back-btn svg { width: 14px; height: 14px; }
    .agent-bot-form-title { font-size: var(--text-base); font-weight: 600; color: var(--fg); }
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
    /* Trailing rule after the title text — a real element (like every other
       divider in the app) rather than a ::after, so it's just .hairline
       like anything else instead of needing that class taught about
       pseudo-elements for this one caller. */
    .agent-bot-form-section-rule { flex: 1; height: 2px; background: var(--hairline); }
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
    .agent-bot-trigger-section-hdr {
      display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;
    }
    .agent-bot-trigger-section-hdr .agent-bot-form-section-title { flex: 1; }
    .agent-bot-trigger-add {
      font-size: var(--text-sm); font-weight: 600; color: var(--muted);
      background: none; border: none; cursor: pointer; padding: 0.15rem 0;
      display: block;
    }
    .agent-bot-trigger-add:hover { color: var(--fg); }
    .agent-bot-triggers-empty {
      font-size: var(--text-sm); color: var(--muted); line-height: 1.5;
      padding: 1.1rem 1rem; border-radius: var(--r-lg); background: var(--surface-soft);
      text-align: center; font-style: italic; opacity: 0.6;
    }
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
    .agent-bot-trigger-hdr {
      display: flex; align-items: center; justify-content: space-between;
      /* Header-to-body seam inside .agent-bot-trigger-card, which reads as
         a card from its own surface-soft background (above), not a line. */
      padding-bottom: 0.75rem; border-bottom: var(--bd-w) solid var(--hairline);
    }
    .agent-bot-trigger-label { font-size: var(--text-sm); font-weight: 600; color: var(--fg); }
    .agent-bot-trigger-hdr-actions { display: flex; align-items: center; gap: 0.65rem; }
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
    /* Preset/weekday chips now use the shared .btn-outline.btn--xs, with
       .active for "currently chosen" (base.css). */
    .agent-bot-schedule-custom-label { display: block; font-size: var(--text-xs); color: var(--muted); margin-bottom: 0.3rem; }
    .agent-bot-weekday-toggles { align-items: center; margin-bottom: 0.4rem; }
    .agent-bot-schedule-kind-seg { margin-bottom: 0.85rem; }
    .agent-bot-schedule-kind-body { margin-bottom: 0.85rem; }
    .agent-bot-form-footer {
      display: flex; align-items: center; gap: 0.5rem;
      margin-top: 0.25rem; padding-top: 1.1rem; border-top: var(--bd-w) solid var(--hairline);
    }
    /* Save/Cancel now use the shared .btn-solid / .btn-outline (base.css). */
    .agent-bot-form-err { flex: 1; font-size: var(--text-xs); color: var(--s-err); }

    /* .cselect* (custom select) moved to assets/cselect.css — it's an
       app-wide primitive shared with Shorts' month picker, not a
       settings-modal detail. */

    /* ── Skills pane ─────────────────────────────────────────── */
    .skills-pane { flex: 1; overflow-y: auto; padding: 1.75rem 1.5rem 3rem; }
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
    /* Box + hover come from the shared .list-card.list-card--hover
       (base.css); see .agent-bot-card above for why no cursor/active, and
       for the same borderless/surface-soft override. */
    .skill-card {
      display: flex; align-items: center; justify-content: space-between; gap: 1rem;
      border-color: transparent; background: var(--surface-soft);
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
       already, so this no longer needs its own copy. */`

// settingsModalHTML returns the settings modal DOM. Include once per page —
// opened via the sidebar's bottom-row Settings button (see home.html).
func settingsModalHTML() string {
	return `
  <div id="hylo-settings-modal"
       x-data="settingsCtrl()"
       x-show="$store.settingsModal.open"
       x-cloak
       class="settings-modal-overlay">
    <div class="settings-modal-panel" @mousedown.stop>
      <button class="icon-btn-ghost settings-modal-close" @click="$store.settingsModal.open = false" type="button">
        <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M18 6 6 18"/>
          <path stroke-linecap="round" d="m6 6 12 12"/>
        </svg>
      </button>
      <div class="settings-modal-inner">

        <!-- Primary sidebar -->
        <nav class="settings-sidebar hairline">
          <button class="side-nav-item settings-sidebar-item"
                  :class="{'is-active': tab === 'appearance'}"
                  @click="tab = 'appearance'">
            <svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="10"/>
              <path d="M12 18a6 6 0 0 0 0-12z" fill="currentColor" stroke="none"/>
            </svg>
            Appearance
          </button>
          <button class="side-nav-item settings-sidebar-item"
                  :class="{'is-active': tab === 'server'}"
                  @click="tab = 'server'">
            <svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
              <rect width="20" height="8" x="2" y="2" rx="2"/>
              <rect width="20" height="8" x="2" y="14" rx="2"/>
              <path stroke-linecap="round" d="M6 6h.01"/>
              <path stroke-linecap="round" d="M6 18h.01"/>
            </svg>
            Server
          </button>
          <button class="side-nav-item settings-sidebar-item"
                  :class="{'is-active': tab === 'agent-bots'}"
                  @click="tab = 'agent-bots'">
            <svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
              <path d="M12 6V2H8"/>
              <path d="M15 11v2"/>
              <path d="M2 12h2"/>
              <path d="M20 12h2"/>
              <path d="M20 16a2 2 0 0 1-2 2H8.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 4 20.286V8a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2z"/>
              <path d="M9 11v2"/>
            </svg>
            Agent Bots
          </button>
          <button class="side-nav-item settings-sidebar-item"
                  :class="{'is-active': tab === 'skills'}"
                  @click="tab = 'skills'">
            <svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M20 2v4"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M22 4h-4"/>
              <circle cx="4" cy="20" r="2"/>
            </svg>
            Skills
          </button>
          <button class="side-nav-item settings-sidebar-item"
                  :class="{'is-active': tab === 'agents'}"
                  @click="tab = 'agents'">
            <svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 20v2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 2v2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M17 20v2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M17 2v2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M2 12h2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M2 17h2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M2 7h2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M20 12h2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M20 17h2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M20 7h2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M7 20v2"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M7 2v2"/>
              <rect x="4" y="4" width="16" height="16" rx="2"/>
              <rect x="8" y="8" width="8" height="8" rx="1"/>
            </svg>
            Agent CLI
          </button>
          <button class="side-nav-item settings-sidebar-item"
                  x-show="isElectron"
                  :class="{'is-active': tab === 'notifications'}"
                  @click="tab = 'notifications'">
            <svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>
            </svg>
            Notifications
          </button>
          <button class="side-nav-item settings-sidebar-item"
                  :class="{'is-active': tab === 'shortcuts'}"
                  @click="tab = 'shortcuts'">
            <svg class="side-nav-icon" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M10 8h.01"/>
              <path stroke-linecap="round" d="M12 12h.01"/>
              <path stroke-linecap="round" d="M14 8h.01"/>
              <path stroke-linecap="round" d="M16 12h.01"/>
              <path stroke-linecap="round" d="M18 8h.01"/>
              <path stroke-linecap="round" d="M6 8h.01"/>
              <path stroke-linecap="round" stroke-linejoin="round" d="M7 16h10"/>
              <path stroke-linecap="round" d="M8 12h.01"/>
              <rect width="20" height="16" x="2" y="4" rx="2"/>
            </svg>
            Shortcuts
          </button>
        </nav>

        <div class="settings-content">

          <div class="home-list-head">
            <span class="settings-content-title"
                  x-text="({appearance:'Appearance',server:'Server','agent-bots':'Agent Bots',skills:'Skills',agents:'Agent CLI',notifications:'Notifications',shortcuts:'Shortcuts'})[tab] || ''"></span>
          </div>

          <!-- Appearance tab -->
          <div class="settings-pane" x-show="tab === 'appearance'">
            <div class="settings-fields">
              <div>
                <label class="settings-field-label">Theme</label>
                <div class="seg">
                  <button class="seg-btn" :class="{active: themePref==='light'}" @click="setTheme('light')">Light</button>
                  <button class="seg-btn" :class="{active: themePref==='dark'}" @click="setTheme('dark')">Dark</button>
                  <button class="seg-btn" :class="{active: themePref==='auto'}" @click="setTheme('auto')">Auto</button>
                </div>
                <p class="settings-field-desc">Light, dark, or match your system setting. Takes effect immediately.</p>
              </div>
              <div>
                <label class="settings-field-label">Accent Color</label>
                <div class="accent-swatches" role="radiogroup" aria-label="Accent color">
                  <template x-for="p in accentPresets" :key="p.id">
                    <button type="button" class="accent-swatch" role="radio"
                            :class="{active: accentPref===p.id}" :aria-checked="accentPref===p.id"
                            :style="'--sw:' + p.a" :title="p.name" :aria-label="p.name"
                            @click="setAccent(p.id)"></button>
                  </template>
                </div>
                <p class="settings-field-desc">Brand color for buttons, links and text selection. Takes effect immediately.</p>
              </div>
              <div>
                <label class="settings-field-label">Line Breaks</label>
                <div class="seg">
                  <button class="seg-btn" :class="{active: lineBreaksPref==='loose'}" @click="setLineBreaks('loose')">Loose</button>
                  <button class="seg-btn" :class="{active: lineBreaksPref==='strict'}" @click="setLineBreaks('strict')">Strict</button>
                </div>
                <p class="settings-field-desc">How a single line break inside a paragraph is interpreted when opening or pasting Markdown. Loose shows it as a visible break, matching Obsidian's default. Strict merges it into flowing text like plain CommonMark — use this if you paste text that was manually wrapped to a fixed width. Applies the next time that text is parsed (reopen the note, or switch out of source view).</p>
              </div>
            </div>
          </div>

          <!-- Server tab -->
          <div class="cfg-content" x-show="tab === 'server'">
            <div class="cfg-pane-area">
              <div class="cfg-pane">

                <div x-show="isElectron" style="max-width:640px; margin-bottom:1.75rem;">
                  <label class="settings-field-label">Connection</label>
                  <div class="settings-field-row">
                    <input type="url" class="field-input settings-input"
                           x-model="serverUrl"
                           @keydown.enter="applyServerUrl()"
                           placeholder="http://localhost:54321"
                           spellcheck="false">
                    <button class="btn-solid"
                            @click="applyServerUrl()"
                            :disabled="urlSaving"
                            x-text="urlSaving ? 'Applying…' : 'Apply'"></button>
                  </div>
                  <div class="settings-error" x-show="urlError" x-text="urlError"></div>
                  <p class="settings-field-desc">Hylo server address used by the desktop app. Changes reload immediately.</p>
                </div>

                <div x-show="isElectron && serverManaged" style="max-width:640px; margin-bottom:1.75rem;">
                  <label class="settings-field-label">Process</label>
                  <div>
                    <button class="btn-solid btn-solid--danger"
                            @click="stopServer()"
                            :disabled="!serverRunning || serverStopping"
                            x-text="serverStopping ? 'Stopping…' : 'Stop Server'"></button>
                  </div>
                  <div class="settings-error" x-show="serverStopError" x-text="serverStopError"></div>
                  <p class="settings-field-desc">Server process started and managed by this desktop app.</p>
                </div>

                <label class="settings-field-label" x-show="!cfgLoading && !cfgError" style="margin-bottom:0.75rem;">Config</label>
                <div x-show="cfgLoading" class="cfg-loader">Loading configuration…</div>
                <div x-show="cfgError && !cfgLoading" class="cfg-err-msg" x-text="'Error: ' + cfgError"></div>

                <template x-for="section in sectionTabs" :key="section">
                  <section class="cfg-section" :class="{ 'is-open': openSection === section }" x-show="!cfgLoading && !cfgError">
                    <button type="button" class="cfg-section-head hairline" @click="toggleSection(section)">
                      <div class="cfg-section-head-text">
                        <span class="cfg-section-title" x-text="sectionLabel(section)"></span>
                        <span class="cfg-section-head-desc"
                              x-show="sectionIntro(section)"
                              x-text="sectionIntro(section)"></span>
                      </div>
                      <div class="cfg-section-head-meta">
                        <span class="dot dot--accent" x-show="sectionHasDirty(section)" title="Unsaved changes in this section"></span>
                        <span class="cfg-section-chev" :class="{open: openSection === section}">
                          <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24" aria-hidden="true">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
                          </svg>
                        </span>
                      </div>
                    </button>
                    <div class="cfg-section-body" x-show="openSection === section">
                      <div class="cfg-fields">
                        <template x-for="field in fieldsForSection(section)" :key="field.key">
                          <div class="cfg-field hairline" :class="{dirty: isDirty(field.key), multiline: field.multiline}">
                            <div class="cfg-field-meta">
                              <span class="cfg-field-label" x-text="field.label"></span>
                              <code class="cfg-field-key" x-text="field.key"></code>
                            </div>
                            <div class="cfg-field-ctrl">
                              <template x-if="field.type === 'bool'">
                                <div class="seg">
                                  <button type="button" class="seg-btn" :class="{active: !getVal(field.key)}" @click="setVal(field.key, false)">Off</button>
                                  <button type="button" class="seg-btn" :class="{active: !!getVal(field.key)}" @click="setVal(field.key, true)">On</button>
                                </div>
                              </template>
                              <template x-if="field.type === 'string' && field.enum && field.enum.length">
                                <select class="field-input cfg-input" @change="setVal(field.key, $event.target.value)">
                                  <template x-for="opt in (field.enum || [])" :key="opt">
                                    <option :value="opt" :selected="getVal(field.key) === opt" x-text="opt"></option>
                                  </template>
                                </select>
                              </template>
                              <template x-if="field.type === 'int'">
                                <input type="number" class="field-input cfg-input"
                                       :value="getVal(field.key)"
                                       @change="setVal(field.key, Number($event.target.value))"
                                       :min="field.constraints ? field.constraints.min : undefined"
                                       :max="field.constraints ? field.constraints.max : undefined">
                              </template>
                              <template x-if="field.type === 'string_list'">
                                <textarea class="field-input cfg-textarea" rows="3" placeholder="One entry per line"
                                          :value="listToText(getVal(field.key))"
                                          @change="setVal(field.key, textToList($event.target.value))"></textarea>
                              </template>
                              <template x-if="field.sensitive && field.type !== 'bool' && field.type !== 'string_list'">
                                <div class="cfg-reveal-wrap">
                                  <input class="field-input cfg-input"
                                         :type="revealed[field.key] ? 'text' : 'password'"
                                         :value="getVal(field.key) || ''"
                                         :placeholder="(secrets[field.key] && !getVal(field.key)) ? '••• set •••' : (field.default != null ? String(field.default) : '')"
                                         @input="setVal(field.key, $event.target.value)">
                                  <button type="button" class="icon-btn icon-btn--lg"
                                          @click.stop="toggleReveal(field.key)"
                                          :title="revealed[field.key] ? 'Hide' : 'Reveal'">
                                    <svg x-show="!revealed[field.key]" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
                                      <path stroke-linecap="round" stroke-linejoin="round" d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/>
                                    </svg>
                                    <svg x-show="revealed[field.key]" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
                                      <path stroke-linecap="round" stroke-linejoin="round" d="M10.733 5.076a10.744 10.744 0 0 1 11.205 6.575 1 1 0 0 1 0 .696 10.747 10.747 0 0 1-1.444 2.49"/>
                                      <path stroke-linecap="round" stroke-linejoin="round" d="M14.084 14.158a3 3 0 0 1-4.242-4.242"/>
                                      <path stroke-linecap="round" stroke-linejoin="round" d="M17.479 17.499a10.75 10.75 0 0 1-15.417-5.151 1 1 0 0 1 0-.696 10.75 10.75 0 0 1 4.446-5.143"/>
                                      <path stroke-linecap="round" stroke-linejoin="round" d="m2 2 20 20"/>
                                    </svg>
                                  </button>
                                </div>
                              </template>
                              <template x-if="!field.sensitive && field.type === 'string' && field.multiline && !(field.enum && field.enum.length)">
                                <textarea class="field-input cfg-textarea" rows="6"
                                          :value="getVal(field.key) ?? ''"
                                          :placeholder="field.default != null ? String(field.default) : ''"
                                          @input="setVal(field.key, $event.target.value)"></textarea>
                              </template>
                              <template x-if="!field.sensitive && (field.type === 'string' || field.type === 'duration') && !field.multiline && !(field.enum && field.enum.length)">
                                <div style="display:flex;gap:0.375rem;align-items:center;width:100%;">
                                  <input type="text" class="field-input cfg-input" style="flex:1;min-width:0;"
                                         :value="getVal(field.key) ?? ''"
                                         :placeholder="field.default != null ? String(field.default) : ''"
                                         @input="setVal(field.key, $event.target.value)">
                                  <template x-if="field.key === 'vault.path' && isElectron">
                                    <button type="button" class="icon-btn icon-btn--lg"
                                            title="Browse for folder"
                                            style="font-size:13px;letter-spacing:0.05em;padding:0 6px;width:auto;"
                                            @click="pickFolder(field.key, getVal(field.key))">···</button>
                                  </template>
                                </div>
                              </template>
                            </div>
                            <p class="cfg-field-desc" x-text="field.description"></p>
                          </div>
                        </template>
                      </div>

                      <div class="cfg-wechat-auth hairline" x-show="section === 'plugins.wechat'">
                        <div class="cfg-wechat-auth-head">
                          <span class="cfg-wechat-auth-title">WeChat login</span>
                          <span class="badge"
                                :class="{ 'badge--ok': wechatStatus.connected }"
                                x-text="wechatStatus.connected ? 'Connected' : 'Not connected'"></span>
                        </div>
                        <p class="cfg-wechat-meta">
                          Scan with WeChat to obtain an iLink bot token. Credentials are saved to
                          <code>config.toml</code>. Create an agent bot with a <code>wechat_message</code> trigger to handle replies.
                        </p>
                        <template x-if="wechatStatus.connected">
                          <div>
                            <p class="cfg-wechat-meta" x-show="wechatStatus.account_id">
                              Bot ID: <code x-text="wechatStatus.account_id"></code>
                              <span x-show="wechatStatus.saved_at"> · saved <span x-text="wechatStatus.saved_at"></span></span>
                            </p>
                            <div class="cfg-wechat-actions">
                              <button type="button" class="btn-solid btn-solid--danger"
                                      @click="wechatLogout()"
                                      :disabled="wechatAuthBusy"
                                      x-text="wechatAuthBusy ? 'Disconnecting…' : 'Disconnect'"></button>
                            </div>
                          </div>
                        </template>
                        <template x-if="!wechatStatus.connected">
                          <div>
                            <div class="cfg-wechat-actions">
                              <button type="button" class="btn-solid"
                                      @click="startWechatLogin()"
                                      :disabled="wechatAuthBusy || wechatLoginBusy"
                                      x-text="wechatLoginBusy ? 'Waiting for scan…' : 'Scan QR to log in'"></button>
                              <button type="button" class="btn-solid"
                                      x-show="wechatLoginBusy"
                                      @click="cancelWechatLogin()">Cancel</button>
                            </div>
                            <div class="cfg-wechat-qr" x-show="wechatQrcodeImg">
                              <img :src="wechatQrcodeImg" alt="WeChat login QR code">
                              <p class="cfg-wechat-status" x-show="wechatLoginStatus" x-text="wechatLoginStatus"></p>
                            </div>
                          </div>
                        </template>
                        <p class="cfg-wechat-err" x-show="wechatAuthError" x-text="wechatAuthError"></p>
                        <p class="cfg-wechat-ok" x-show="wechatLoginOk">Connected — restart the server to start the bridge.</p>
                      </div>

                      <div class="cfg-wechat-auth hairline" x-show="section === 'plugins.discord'">
                        <div class="cfg-wechat-auth-head">
                          <span class="cfg-wechat-auth-title">Discord Bot status</span>
                          <span class="badge"
                                :class="{ 'badge--ok': !!getVal('plugins.discord.bot_token') }"
                                x-text="getVal('plugins.discord.bot_token') ? 'Token set' : 'Not configured'"></span>
                        </div>
                        <p class="cfg-wechat-meta">
                          Paste your Bot token above, then restart the server. Create an agent bot with a
                          <code>discord_message</code> trigger to handle replies. The bot must share a server
                          with you before it can send proactive DMs.
                        </p>
                        <template x-if="getVal('plugins.discord.user_id')">
                          <p class="cfg-wechat-meta">
                            Owner ID: <code x-text="getVal('plugins.discord.user_id')"></code>
                          </p>
                        </template>
                      </div>
                    </div>
                  </section>
                </template>

                <div class="cfg-action-bar hairline" x-show="!cfgLoading && !cfgError">
                  <div class="cfg-action-left">
                    <span class="cfg-status-ok" x-show="cfgSaveOk && !hasDirty">Saved — restart server to apply</span>
                    <span class="cfg-status-err" x-show="cfgSaveError" x-text="cfgSaveError"></span>
                    <span class="cfg-status-err" x-show="cfgRestartError" x-text="cfgRestartError"></span>
                    <span class="cfg-restart-note" x-show="cfgRestarting">Restarting server…</span>
                    <span class="badge" x-show="hasDirty"
                          x-text="Object.keys(patch).length + ' unsaved change' + (Object.keys(patch).length !== 1 ? 's' : '')"></span>
                    <span class="cfg-restart-note"
                          x-show="!hasDirty && !cfgSaveOk && !cfgSaveError && !cfgRestartError && !cfgRestarting">
                      Expand sections above to edit. Save once when done; restart server to apply.
                    </span>
                  </div>
                  <div class="cfg-action-right">
                    <button class="btn-outline" x-show="hasDirty" @click="discardAll()">Discard</button>
                    <button class="btn-solid"
                            @click="saveConfig()"
                            :disabled="!hasDirty || cfgSaving || cfgRestarting"
                            x-text="cfgRestarting ? 'Restarting…' : cfgSaving ? 'Saving…' : 'Save all'"></button>
                  </div>
                </div>
              </div>
            </div>
          </div><!-- .cfg-content server -->

          <!-- Shortcuts tab -->
          <div class="shortcuts-pane" x-show="tab === 'shortcuts'">
            <div class="shortcuts-fields">
              <div class="shortcuts-list">
                <template x-for="s in shortcutDefs" :key="s.id">
                  <div class="shortcuts-row hairline">
                    <div class="shortcuts-row-meta">
                      <span class="shortcuts-label" x-text="s.label"></span>
                      <span class="shortcuts-desc" x-text="s.desc"></span>
                    </div>
                    <div class="shortcuts-keys">
                      <template x-for="k in getEffectiveKeys(s)" :key="k">
                        <span class="kbd kbd-combo" x-html="hyloKbdHTML(k)"></span>
                      </template>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div><!-- .shortcuts-pane -->

          <!-- Notifications tab -->
          <div class="notifications-pane" x-show="tab === 'notifications'">
            <div class="cfg-loader" x-show="!isElectron">Available in the desktop app only.</div>
            <template x-if="isElectron">
              <div class="settings-fields">

                <div>
                  <div class="notif-row">
                    <label class="settings-field-label" style="margin:0">Text Notifications</label>
                    <div class="seg">
                      <button type="button" class="seg-btn" :class="{active: !notifySettings.textEnabled}" @click="notifySettings.textEnabled = false; saveNotifySettings()">Off</button>
                      <button type="button" class="seg-btn" :class="{active: notifySettings.textEnabled}" @click="notifySettings.textEnabled = true; saveNotifySettings()">On</button>
                    </div>
                  </div>
                  <p class="settings-field-desc">Show a system notification when a new inbox message arrives.</p>
                </div>

                <div>
                  <div class="notif-row">
                    <label class="settings-field-label" style="margin:0">Sound</label>
                    <div class="seg">
                      <button type="button" class="seg-btn" :class="{active: !notifySettings.soundEnabled}" @click="notifySettings.soundEnabled = false; saveNotifySettings()">Off</button>
                      <button type="button" class="seg-btn" :class="{active: notifySettings.soundEnabled}" @click="notifySettings.soundEnabled = true; saveNotifySettings()">On</button>
                    </div>
                  </div>
                  <p class="settings-field-desc">Play a sound when a new inbox message arrives.</p>
                </div>

                <template x-if="notifySettings.soundEnabled">
                  <div class="settings-fields" style="gap:1.25rem">

                    <div>
                      <label class="settings-field-label">Notification Sound</label>
                      <div class="notif-sound-row">
                        <div class="cselect" x-data="{ csOpen: false }" @click.outside="csOpen = false" style="flex:1;min-width:0">
                          <button type="button" class="cselect-btn" :class="{open: csOpen}" @click="csOpen = !csOpen" @keydown.escape="csOpen = false">
                            <span class="cselect-btn-text" x-text="notifySoundLabel(notifySettings.sound)"></span>
                            <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/></svg>
                          </button>
                          <div class="cselect-dropdown" x-show="csOpen">
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='beep'?'sel':''" @click="notifySettings.sound='beep'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>System beep</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='none'?'sel':''" @click="notifySettings.sound='none'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>None</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Glass'?'sel':''" @click="notifySettings.sound='Glass'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Glass</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Ping'?'sel':''" @click="notifySettings.sound='Ping'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Ping</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Pop'?'sel':''" @click="notifySettings.sound='Pop'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Pop</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Tink'?'sel':''" @click="notifySettings.sound='Tink'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Tink</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Hero'?'sel':''" @click="notifySettings.sound='Hero'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Hero</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Purr'?'sel':''" @click="notifySettings.sound='Purr'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Purr</span></button>
                            <button type="button" class="cselect-option" :class="notifySettings.sound==='Submarine'?'sel':''" @click="notifySettings.sound='Submarine'; saveNotifySettings(); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span>Submarine</span></button>
                          </div>
                        </div>
                        <button type="button" class="icon-btn icon-btn--lg" title="Preview sound"
                                :disabled="notifySettings.sound === 'none'"
                                @click="window.hyloDesktop?.inboxNotify?.previewSound(notifySettings.sound)">
                          <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 4v16l13-8z"/></svg>
                        </button>
                      </div>
                      <p class="settings-field-desc">Sound played when a new inbox message arrives.</p>
                    </div>

                  </div>
                </template>

                <p class="notif-saved" x-show="notifySaveOk">Saved</p>

              </div>
            </template>
          </div><!-- .notifications-pane -->

          <!-- Agent Bots tab -->
          <div class="agent-bots-pane" x-show="tab === 'agent-bots'">

            <template x-if="!agentBotFormMode && !agentBotsSubPage">
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
                    <div class="list-card list-card--hover agent-bot-card" :class="m.enabled ? '' : 'disabled-card'">
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


            <template x-if="agentBotFormMode">
              <div class="agent-bot-form-wrap">
                <div class="agent-bot-form-header">
` + agentBotBackBtnHTML("agentBotFormMode = null", "Agent Bots") + `
                </div>
                <div class="agent-bot-form">
                  <section class="agent-bot-form-section">
                    <h3 class="agent-bot-form-section-title">Profile<span class="agent-bot-form-section-rule hairline"></span></h3>
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
                    <h3 class="agent-bot-form-section-title">Agent<span class="agent-bot-form-section-rule hairline"></span></h3>
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
                    <div style="display:none">
                      <label class="agent-bot-form-label">Working Directory <span style="font-weight:400;text-transform:none;letter-spacing:0;">(vault root if blank)</span></label>
                      <input class="field-input agent-bot-form-input" type="text" x-model="agentBotDraft.cwd" placeholder="/absolute/path or leave blank">
                    </div>
                    <div>
                      <label class="agent-bot-form-label">System Prompt</label>
                      <textarea class="field-input agent-bot-form-textarea" x-model="agentBotDraft.systemPrompt" rows="1"
                                placeholder="Instructions prepended to every message…"></textarea>
                    </div>
                  </section>

                  <section class="agent-bot-form-section agent-bot-form-section-triggers">
                    <div class="agent-bot-trigger-section-top">
                      <div class="agent-bot-trigger-section-hdr">
                        <h3 class="agent-bot-form-section-title">Triggers<span class="agent-bot-form-section-rule hairline"></span></h3>
                      </div>
                      <p class="agent-bot-section-desc">Automatically run this agent bot on vault events or on a schedule. Each trigger sends a prompt template to the agent.</p>
                    </div>
                    <div class="agent-bot-trigger-list">
                      <template x-for="(t, ti) in agentBotTriggers" :key="ti">
                        <div class="agent-bot-trigger-card">
                          <div class="agent-bot-trigger-hdr hairline">
                            <span class="agent-bot-trigger-label">Trigger <span x-text="ti+1"></span></span>
                            <div class="agent-bot-trigger-hdr-actions">
                              <button class="agent-bot-trigger-del" type="button" @click="removeAgentBotTrigger(ti)">Remove</button>
                            </div>
                          </div>
                          <div class="agent-bot-trigger-body">
                            <div class="agent-bot-trigger-block agent-bot-trigger-events">
                              <div class="agent-bot-block-label">
                                <span class="agent-bot-block-title">Event</span>
                                <span class="agent-bot-block-hint" x-text="isScheduledTrigger(t) ? 'Scheduled triggers run on a timer — choose one schedule below' : (isWechatTrigger(t) ? 'WeChat DM trigger — use Content and WechatUserID in the prompt' : (isCompileTrigger(t) ? 'Fires when the user manually triggers compilation via the API — Path carries the note' : (isAgentRunCompletedTrigger(t) ? 'Fires when another agent bot\'s run succeeds — filter by source agent bot names below' : 'Vault event that activates this trigger')))"></span>
                              </div>
` + cselectHTML(
		`(agentBotEventDefs.find(function(d){return d.type===(t.eventTypes[0]||'');}) || {label: 'Select event…'}).label`,
		`<template x-for="def in agentBotEventDefs" :key="def.type"><button type="button" class="cselect-option" :class="(t.eventTypes[0]||'')===def.type ? 'sel' : ''" :title="def.description" @click="setAgentBotET(t, def.type); csOpen=false"><span class="dot dot--fg cselect-option-dot"></span><span x-text="def.label"></span></button></template>`,
	) + `
                            </div>
                            <template x-if="isScheduledTrigger(t)">
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

                                <template x-if="scheduleKindOf(t) === 'every'">
                                  <div class="agent-bot-schedule-kind-body">
                                    <div class="agent-bot-schedule-presets">
                                      <template x-for="p in agentBotIntervalPresets" :key="p.value">
                                        <button type="button" class="btn-outline btn--xs"
                                                :class="(t.schedule || '') === p.value ? 'active' : ''"
                                                @click="t.schedule = p.value"
                                                x-text="p.label"></button>
                                      </template>
                                    </div>
                                    <span class="agent-bot-block-hint">Minimum interval: 15 minutes. For a different interval, edit it directly in Raw below.</span>
                                  </div>
                                </template>

                                <template x-if="scheduleKindOf(t) === 'daily'">
                                  <div class="agent-bot-schedule-kind-body">
                                    <div class="agent-bot-schedule-presets">
                                      <template x-for="p in agentBotDailyPresets" :key="p.value">
                                        <button type="button" class="btn-outline btn--xs"
                                                :class="(t.schedule || '') === p.value ? 'active' : ''"
                                                @click="t.schedule = p.value"
                                                x-text="p.label"></button>
                                      </template>
                                    </div>
                                    <span class="agent-bot-block-hint">Server local time. For a different time, edit it directly in Raw below.</span>
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
                            <template x-if="!isScheduledTrigger(t) && !isWechatTrigger(t) && !isAgentRunCompletedTrigger(t)">
                              <div class="agent-bot-trigger-block agent-bot-trigger-paths">
                                <div class="agent-bot-block-label">
                                  <span class="agent-bot-block-title">Path Prefixes <span style="font-weight:400;opacity:0.6;">(optional)</span></span>
                                  <span class="agent-bot-block-hint">Only fire when the event path starts with one of these prefixes. Leave empty to match all paths. One prefix per line, e.g. <code style="font-size:var(--text-2xs);padding:0 3px;background:var(--code-bg);border-radius:var(--r-xs);">/journal/</code></span>
                                </div>
                                <textarea class="field-input agent-bot-form-textarea"
                                          rows="2"
                                          :value="(t.pathPrefixes || []).join('\n')"
                                          @change="t.pathPrefixes = $event.target.value.split('\n').map(function(s){return s.trim();}).filter(Boolean)"
                                          placeholder="/journal/&#10;/projects/work/"></textarea>
                              </div>
                            </template>
                            <template x-if="isAgentRunCompletedTrigger(t)">
                              <div class="agent-bot-trigger-block agent-bot-trigger-paths">
                                <div class="agent-bot-block-label">
                                  <span class="agent-bot-block-title">Source Agent Bots <span style="font-weight:400;opacity:0.6;">(optional)</span></span>
                                  <span class="agent-bot-block-hint">Only fire when the run was completed by one of these agent bots. Leave empty to fire on any agent bot. One agent bot name per line.</span>
                                </div>
                                <textarea class="field-input agent-bot-form-textarea"
                                          rows="2"
                                          :value="(t.pathPrefixes || []).join('\n')"
                                          @change="t.pathPrefixes = $event.target.value.split('\n').map(function(s){return s.trim();}).filter(Boolean)"
                                          placeholder="Summarizer&#10;Compiler"></textarea>
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
                                        @focus="agentBotActivePromptIdx = ti"
                                        :placeholder="agentBotPromptPlaceholder(t)"></textarea>
                            </div>
                          </div>
                        </div>
                      </template>
                    </div>
                    <button class="agent-bot-trigger-add" type="button" @click="addAgentBotTrigger()" style="margin-top:0.25rem;">+ Add trigger</button>
                  </section>

                  <div class="agent-bot-form-footer hairline">
                    <button class="btn-solid" type="button"
                            :disabled="!agentBotDraft.name.trim() || agentBotSaving"
                            @click="saveAgentBot()"
                            x-text="agentBotSaving ? 'Saving…' : 'Save'"></button>
                    <button class="btn-outline" type="button" @click="agentBotFormMode = null">Cancel</button>
                    <span class="agent-bot-form-err" x-text="agentBotSaveError"></span>
                  </div>
                </div>
              </div>
            </template>

          </div><!-- .agent-bots-pane -->

          <!-- Skills tab -->
          <div class="skills-pane" x-show="tab === 'skills'">
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
                  <div class="list-card list-card--hover skill-card" :class="s.installed ? '' : 'not-installed'">
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

          <!-- Agents tab -->
          <div class="agents-pane" x-show="tab === 'agents'">
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
                <div class="list-card agent-card" :class="{unavailable: !ag.available}">
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

        </div><!-- .settings-content -->
      </div><!-- .settings-modal-inner -->
    </div><!-- .settings-modal-panel -->
  </div><!-- #hylo-settings-modal -->`
}

// settingsCtrlJS is the Alpine.js controller for the settings modal.
// It uses lazy init — data is only loaded when the modal is first opened.
const settingsCtrlJS = `
  function settingsCtrl() {
    return {
      _inited: false,
      isElectron: !!window.hyloDesktop,
      tab: 'appearance',
      serverUrl: '',
      urlSaving: false,
      urlError: '',
      serverManaged: false,
      serverRunning: false,
      serverStopping: false,
      serverStopError: '',

      schema: [],
      values: {},
      secrets: {},
      cfgLoading: false,
      cfgError: '',
      cfgSaving: false,
      cfgSaveError: '',
      cfgSaveOk: false,
      cfgRestarting: false,
      cfgRestartError: '',
      patch: {},
      revealed: {},
      openSection: '',

      wechatStatus: { connected: false, account_id: '', saved_at: '', enabled: false },
      wechatAuthBusy: false,
      wechatAuthError: '',
      wechatLoginBusy: false,
      wechatLoginStatus: '',
      wechatQrcode: '',
      wechatQrcodeImg: '',
      wechatPollTimer: null,
      wechatLoginOk: false,

      agents: [],
      agentsLoading: false,
      agentsError: '',
      agentsLoaded: false,
      agentsFromCache: false,
      agentsCachedAt: 0,
      agentCopied: '',

      agentBotsList: [],
      agentBotsLoading: false,
      agentBotsError: '',
      agentBotFormMode: null,
      agentBotsSubPage: null,
      agentBotEditId: '',
      agentBotDraft: {},
      agentBotTriggers: [],
      agentBotActivePromptIdx: -1,
      agentBotSaving: false,
      agentBotSaveError: '',
      agentBotEventDefs: [],
      agentBotPromptVarsVault: [
        { token: '{Path}', desc: 'Full vault path of the affected note' },
        { token: '{Name}', desc: 'Filename without extension' },
        { token: '{Content}', desc: 'Appended short-note text (short_note_created only)' },
      ],
      agentBotPromptVarsCompile: [
        { token: '{Path}', desc: 'Vault path of the note to compile' },
        { token: '{Name}', desc: 'Filename without extension' },
      ],
      agentBotPromptVarsWechat: [
        { token: '{Content}', desc: 'Incoming WeChat DM text' },
        { token: '{WechatUserID}', desc: 'Sender WeChat user ID' },
      ],
      agentBotPromptVarsScheduled: [
        { token: '{Now}', desc: 'Trigger time (RFC3339)' },
        { token: '{Date}', desc: 'Date YYYY-MM-DD' },
        { token: '{Time}', desc: 'Time HH:MM' },
      ],
      agentBotPromptVarsAgentRunCompleted: [
        { token: '{Name}', desc: 'Name of the agent bot whose run just succeeded' },
        { token: '{Content}', desc: 'Last assistant message from the completed run' },
        { token: '{Now}', desc: 'Trigger time (RFC3339)' },
      ],
      agentBotIntervalPresets: [
        { label: 'Every hour', value: 'every 1h' },
        { label: 'Every 6 hours', value: 'every 6h' },
      ],
      agentBotDailyPresets: [
        { label: 'Daily 09:00', value: 'daily 09:00' },
        { label: 'Daily 21:00', value: 'daily 21:00' },
      ],
      agentBotWeekdayDefs: [
        { abbr: 'mon', label: 'Mon' },
        { abbr: 'tue', label: 'Tue' },
        { abbr: 'wed', label: 'Wed' },
        { abbr: 'thu', label: 'Thu' },
        { abbr: 'fri', label: 'Fri' },
        { abbr: 'sat', label: 'Sat' },
        { abbr: 'sun', label: 'Sun' },
      ],
      skillsList: [],
      skillsLoading: false,
      skillsError: '',
      skillsInstalling: {},
      skillsUninstalling: {},

      notifySettings: { textEnabled: true, soundEnabled: true, sound: 'beep' },
      notifySaveOk: false,

      get sortedSkillsList() {
        return [...this.skillsList].sort(function(a, b) {
          if (a.default !== b.default) return a.default ? -1 : 1;
          return a.name.localeCompare(b.name);
        });
      },

      // Read live by the editor's remark pipeline on every parse (breaks.js),
      // not threaded through Editor.make() config — so this takes effect on
      // the next parse without needing the editor to be recreated.
      lineBreaksPref: localStorage.getItem('hylo-line-breaks') || 'loose',
      setLineBreaks(key) { this.lineBreaksPref = key; localStorage.setItem('hylo-line-breaks', key); },

      // themePref mirrors what themeBootstrapScript already resolved at
      // first paint (light/dark/auto) — switching here just persists the
      // new choice and re-runs that same resolution logic immediately via
      // window.__hyloApplyTheme (defined once in <head>, shared by both).
      themePref: localStorage.getItem('hylo-theme') || 'auto',
      setTheme(key) {
        this.themePref = key;
        localStorage.setItem('hylo-theme', key);
        if (window.__hyloApplyTheme) window.__hyloApplyTheme(key);
      },

      // Presets come from accentBootstrapScript (shared_accent.go), which also
      // owns applying them; this just persists the pick and re-runs it.
      accentPresets: window.__hyloAccentPresets || [],
      accentPref: (function() {
        var id = localStorage.getItem('hylo-accent');
        return (window.__hyloAccentPresets || []).some(function(p) { return p.id === id; }) ? id : 'indigo';
      })(),
      setAccent(id) {
        this.accentPref = id;
        localStorage.setItem('hylo-accent', id);
        if (window.__hyloApplyAccent) window.__hyloApplyAccent(id);
      },

      isMac: /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent),
      customKeys: JSON.parse(localStorage.getItem('hylo-custom-keys') || '{}'),
      shortcutDefs: [
        { id: 'dismiss',        label: 'Dismiss',                desc: 'Close any open overlay or dialog',          mac: 'Esc',  win: 'Esc' },
        { id: 'toggle-search',  label: 'Search',                 desc: 'Open the quick search overlay',             mac: '⌘K',   win: 'Ctrl+K' },
        { id: 'new-note',       label: 'New Note',               desc: 'Open a new blank note in the editor',       mac: '⌘N',   win: 'Ctrl+N' },
        { id: 'toggle-editor',  label: 'Toggle Editor',          desc: 'Open or close the editor panel',            mac: '⌘O',   win: 'Ctrl+O' },
        { id: 'close-tab',      label: 'Close Editor Tab',       desc: 'Close the active tab in the editor',        mac: '⌘W',   win: 'Ctrl+W' },
        { id: 'expand-editor',  label: 'Expand / Shrink Editor', desc: 'Toggle editor between 80% and 100% width',  mac: '⌘\\',  win: 'Ctrl+\\' },
        { id: 'reading-mode',   label: 'Reading Mode',           desc: 'Toggle read-only view for saved notes',     mac: '⌘L',   win: 'Ctrl+L' },
        { id: 'refresh',        label: 'Refresh',               desc: 'Reload the current page',                   mac: '⌘R',   win: 'Ctrl+R' },
        { id: 'open-settings',  label: 'Settings',               desc: 'Open the settings dialog',                  mac: '⌘,',   win: 'Ctrl+,' },
      ],
      getEffectiveKeys(s) {
        const custom = this.customKeys[s.id];
        const key = this.isMac ? (custom?.mac ?? s.mac) : (custom?.win ?? s.win);
        return key.split('/').map(k => k.trim());
      },

      get sectionTabs() {
        const excluded = new Set(['log', 'server']);
        const seen = new Set(), tabs = [];
        for (const f of this.schema) {
          if (!seen.has(f.section) && !excluded.has(f.section)) {
            seen.add(f.section);
            tabs.push(f.section);
          }
        }
        return tabs;
      },

      sectionLabel(s) {
        const m = {
          'server': 'Server', 'vault': 'Vault', 'agent': 'Agent',
          'plugins.search': 'Search', 'plugins.git_sync': 'Git Sync',
          'plugins.compile': 'Compile',
          'plugins.wechat': 'WeChat',
          'plugins.discord': 'Discord',
        };
        return m[s] || s;
      },

      sectionIntro(s) {
        const m = {
          'vault': 'Paths, layout, and core vault behavior.',
          'agent': 'Agent CLI integration and global prompt settings.',
          'plugins.search': 'Indexing and search quality.',
          'plugins.git_sync': 'Automatic git push/pull for the vault.',
          'plugins.compile': 'AI knowledge compilation and related options.',
          'plugins.wechat': 'WeChat iLink bridge — poll DMs and emit wechat_message agent bot events.',
          'plugins.discord': 'Discord Bot bridge — receive DMs and emit discord_message agent bot events.',
          'server.listen': 'HTTP listen address and port.',
          'server': 'HTTP listen address and port.',
        };
        return m[s] || '';
      },

      fieldsForSection(s) { return this.schema.filter(f => f.section === s); },

      toggleSection(s) {
        const opening = this.openSection !== s;
        this.openSection = opening ? s : '';
        if (opening && s === 'plugins.wechat') this.loadWechatStatus();
      },

      sectionHasDirty(s) { return this.fieldsForSection(s).some(f => this.isDirty(f.key)); },

      getVal(key) {
        if (key in this.patch) return this.patch[key];
        const parts = key.split('.');
        let v = this.values;
        for (const p of parts) {
          if (v == null || typeof v !== 'object') return null;
          v = v[p];
        }
        return v ?? null;
      },

      setVal(key, val) {
        this.patch = { ...this.patch, [key]: val };
        this.cfgSaveOk = false;
        this.cfgSaveError = '';
        this.cfgRestartError = '';
      },

      isDirty(key) { return key in this.patch; },
      get hasDirty() { return Object.keys(this.patch).length > 0; },

      discardAll() {
        this.patch = {};
        this.cfgSaveOk = false;
        this.cfgSaveError = '';
        this.cfgRestartError = '';
      },

      toggleReveal(key) { this.revealed = { ...this.revealed, [key]: !this.revealed[key] }; },
      listToText(v) { if (!Array.isArray(v)) return ''; return v.join('\n'); },
      textToList(s) { return s.split('\n').map(x => x.trim()).filter(Boolean); },

      buildNested(flat) {
        const out = {};
        for (const [key, val] of Object.entries(flat)) {
          const parts = key.split('.');
          let o = out;
          for (let i = 0; i < parts.length - 1; i++) {
            if (!(parts[i] in o)) o[parts[i]] = {};
            o = o[parts[i]];
          }
          o[parts[parts.length - 1]] = val;
        }
        return out;
      },

      async init() {
        window.__hyloSettingsShell = this;
        // Another same-origin view can change the accent; keep the picker's ring in step.
        window.addEventListener('hylo:accent', () => {
          var id = localStorage.getItem('hylo-accent');
          this.accentPref = this.accentPresets.some(p => p.id === id) ? id : 'indigo';
        });
        window.__hyloHotkeys.register('open-settings', ',', function() {
          Alpine.store('settingsModal').open = true;
        });
        this.$watch('$store.settingsModal.open', async (open) => {
          if (!open) {
            if (window.__hyloEscPop) window.__hyloEscPop('settings');
            this.cancelWechatLogin();
            return;
          }
          if (window.__hyloEscPush) window.__hyloEscPush('settings', () => { Alpine.store('settingsModal').open = false; });
          if (!this._inited) {
            this._inited = true;
            if (window.hyloDesktop) {
              this.serverUrl = await window.hyloDesktop.getServerUrl();
            }
            this.$watch('tab', val => {
              if (val === 'server') { this.loadServerStatus(); this.loadConfig(); }
              if (val === 'agent-bots') {
                this.agentBotsSubPage = null;
                this.loadAgentBots();
                if (!this.agentsLoaded) this.loadAgents();
                if (!this.agentBotEventDefs.length) this.loadAgentBotEvents();
              }
              if (val === 'skills') this.loadSkills();
              if (val === 'agents') { if (!this.agentsLoaded) this.loadAgents(); }
              if (val === 'notifications') this.loadNotifySettings();
            });
          }
          await Promise.all([this.loadConfig(), this.loadServerStatus()]);
          if (this.tab === 'agent-bots') {
            this.loadAgentBots();
            if (!this.agentsLoaded) this.loadAgents();
            if (!this.agentBotEventDefs.length) this.loadAgentBotEvents();
          }
          if (this.tab === 'skills') this.loadSkills();
          if (this.tab === 'agents') { if (!this.agentsLoaded) this.loadAgents(); }
        });
      },

      async loadServerStatus() {
        if (!window.hyloDesktop?.getServerProcessStatus) return;
        try {
          const s = await window.hyloDesktop.getServerProcessStatus();
          this.serverManaged = s.managed;
          this.serverRunning = s.alive;
        } catch { /* noop */ }
      },

      async stopServer() {
        this.serverStopping = true;
        this.serverStopError = '';
        try {
          const r = await window.hyloDesktop.stopServer();
          if (!r.ok) { this.serverStopError = r.error || 'Stop failed'; this.serverStopping = false; }
        } catch (e) { this.serverStopError = e.message; this.serverStopping = false; }
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

      agentDisplayModels(ag) { return (ag.models || []).slice(0, 4); },

      copyCliExample(ag) {
        if (!ag.cliExample) return;
        navigator.clipboard.writeText(ag.cliExample).catch(function() {});
        this.agentCopied = ag.id;
        setTimeout(() => { if (this.agentCopied === ag.id) this.agentCopied = ''; }, 1500);
      },

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
        const first = this.agents.find(function(a){ return a.available; });
        const firstModel = (first && first.models && first.models.length) ? first.models[0].id : '';
        this.agentBotDraft = { name: '', description: '', agentId: first ? first.id : '', model: firstModel, cwd: '', systemPrompt: '', enabled: true };
        this.agentBotTriggers = [];
        this.agentBotEditId = '';
        this.agentBotSaveError = '';
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
              eventTypes: (t.eventTypes || []).map(function(et) {
                return et === 'weixin_message' ? 'wechat_message' : et;
              }),
              schedule: t.schedule || '',
              pathPrefixes: t.pathPrefixes || [],
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
          const payload = Object.assign({}, this.agentBotDraft, { triggers: this.agentBotTriggers });
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
        const ok = (typeof window.showConfirm === 'function')
          ? await window.showConfirm({ title: 'Delete agent bot', message: 'This agent bot and all its data will be permanently deleted.', confirmLabel: 'Delete', danger: true })
          : window.confirm('Delete this agent bot? This cannot be undone.');
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
        this.agentBotTriggers.push({ id: '', mateId: '', eventTypes: ['note_created'], schedule: '', prompt: '', pathPrefixes: [], enabled: true });
      },

      removeAgentBotTrigger(idx) { this.agentBotTriggers.splice(idx, 1); },

      isScheduledTrigger(t) { return (t.eventTypes || []).indexOf('scheduled') >= 0; },
      isWechatTrigger(t) { return (t.eventTypes || []).indexOf('wechat_message') >= 0; },
      isCompileTrigger(t) { return (t.eventTypes || []).indexOf('compile_requested') >= 0; },
      isAgentRunCompletedTrigger(t) { return (t.eventTypes || []).indexOf('agent_run_completed') >= 0; },

      agentBotPromptVarsForTrigger(t) {
        if (this.isScheduledTrigger(t)) return this.agentBotPromptVarsScheduled;
        if (this.isWechatTrigger(t)) return this.agentBotPromptVarsWechat;
        if (this.isCompileTrigger(t)) return this.agentBotPromptVarsCompile;
        if (this.isAgentRunCompletedTrigger(t)) return this.agentBotPromptVarsAgentRunCompleted;
        return this.agentBotPromptVarsVault;
      },

      agentBotPromptPlaceholder(t) {
        if (this.isScheduledTrigger(t)) return 'Review my vault and write a daily digest. Time: {Now}';
        if (this.isWechatTrigger(t)) return 'Reply to this WeChat message:\n\n{Content}';
        if (this.isCompileTrigger(t)) return 'Compile {Path} into knowledge units.';
        if (this.isAgentRunCompletedTrigger(t)) return '{Name} just finished — review its output and take action.';
        return 'Summarize the key points in {Path}';
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

      scheduleKindOf(t) {
        const s = (t.schedule || '').trim().toLowerCase();
        if (s.startsWith('every ')) return 'every';
        if (s.startsWith('weekly ')) return 'weekly';
        return 'daily';
      },

      setScheduleKind(t, kind) {
        if (this.scheduleKindOf(t) === kind) return;
        if (kind === 'every') t.schedule = 'every 1h';
        else if (kind === 'daily') t.schedule = 'daily 09:00';
        else if (kind === 'weekly') t.schedule = 'weekly mon 09:00';
      },

      weeklyDaysOf(t) {
        const s = (t.schedule || '').trim().toLowerCase();
        if (!s.startsWith('weekly ')) return [];
        const parts = s.split(/\s+/);
        if (parts.length < 2 || parts[1] === 'none') return [];
        return parts[1].split(',').map(function(d) { return d.trim(); }).filter(Boolean);
      },

      weeklyTimeOf(t) {
        const s = (t.schedule || '').trim().toLowerCase();
        if (!s.startsWith('weekly ')) return '09:00';
        const parts = s.split(/\s+/);
        return parts.length >= 3 ? parts[2] : '09:00';
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
        const time = this.weeklyTimeOf(t);
        t.schedule = 'weekly ' + (days.length ? days.join(',') : 'none') + ' ' + time;
      },

      insertAgentBotVar(ti, token, event) {
        this.agentBotActivePromptIdx = ti;
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

      async loadConfig() {
        this.cfgLoading = true;
        this.cfgError = '';
        try {
          const [sr, vr] = await Promise.all([
            fetch('/api/config/schema'),
            fetch('/api/config'),
          ]);
          this.schema = (await sr.json()).fields || [];
          const vd = await vr.json();
          this.values = vd.values || {};
          this.secrets = vd.secrets || {};
          this.openSection = '';
          await this.loadWechatStatus();
        } catch (e) { this.cfgError = e.message; }
        finally { this.cfgLoading = false; }
      },

      async loadWechatStatus() {
        try {
          const r = await fetch('/api/wechat/status');
          if (!r.ok) return;
          const d = await r.json();
          this.wechatStatus = {
            connected: !!d.connected,
            account_id: d.account_id || '',
            saved_at: d.saved_at || '',
            enabled: !!d.enabled,
          };
        } catch { /* noop */ }
      },

      cancelWechatLogin() {
        if (this.wechatPollTimer) { clearInterval(this.wechatPollTimer); this.wechatPollTimer = null; }
        this.wechatLoginBusy = false;
        this.wechatLoginStatus = '';
        this.wechatQrcode = '';
        this.wechatQrcodeImg = '';
      },

      wechatStatusLabel(status) {
        const m = { wait: 'Open WeChat and scan the QR code', scaned: 'QR scanned — confirm login in WeChat', expired: 'QR expired — fetching a new code…', confirmed: 'Login successful' };
        return m[status] || status;
      },

      async startWechatLogin() {
        this.wechatAuthError = '';
        this.wechatLoginOk = false;
        this.wechatAuthBusy = true;
        try {
          const r = await fetch('/api/wechat/login/start', { method: 'POST' });
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Failed to start login');
          this.wechatQrcode = d.qrcode || '';
          this.wechatQrcodeImg = d.qrcode_image || '';
          this.wechatLoginBusy = true;
          this.wechatLoginStatus = this.wechatStatusLabel('wait');
          if (this.wechatPollTimer) clearInterval(this.wechatPollTimer);
          this.wechatPollTimer = setInterval(() => this.pollWechatLogin(), 1500);
          await this.pollWechatLogin();
        } catch (e) { this.wechatAuthError = e.message; this.cancelWechatLogin(); }
        finally { this.wechatAuthBusy = false; }
      },

      async pollWechatLogin() {
        if (!this.wechatQrcode) return;
        try {
          const r = await fetch('/api/wechat/login/status?qrcode=' + encodeURIComponent(this.wechatQrcode));
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Login poll failed');
          if (d.qrcode && d.qrcode !== this.wechatQrcode) this.wechatQrcode = d.qrcode;
          if (d.qrcode_image) this.wechatQrcodeImg = d.qrcode_image;
          if (d.status) this.wechatLoginStatus = this.wechatStatusLabel(d.status);
          if (d.status === 'confirmed') {
            this.cancelWechatLogin();
            this.wechatLoginOk = true;
            await this.loadWechatStatus();
            await this.loadConfig();
          }
        } catch (e) { this.wechatAuthError = e.message; this.cancelWechatLogin(); }
      },

      async wechatLogout() {
        this.wechatAuthError = '';
        this.wechatLoginOk = false;
        this.wechatAuthBusy = true;
        try {
          const r = await fetch('/api/wechat/logout', { method: 'POST' });
          const d = await r.json();
          if (!r.ok) throw new Error(d.error || 'Logout failed');
          this.wechatStatus = { connected: false, account_id: '', saved_at: '', enabled: false };
          await this.loadConfig();
        } catch (e) { this.wechatAuthError = e.message; }
        finally { this.wechatAuthBusy = false; }
      },

      async saveConfig() {
        if (!this.hasDirty || this.cfgSaving || this.cfgRestarting) return;
        this.cfgSaving = true;
        this.cfgSaveError = '';
        this.cfgSaveOk = false;
        this.cfgRestartError = '';
        let saved = false;
        try {
          const res = await fetch('/api/config', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ patch: this.buildNested(this.patch) }),
          });
          const data = await res.json();
          if (!res.ok) {
            const detail = Array.isArray(data.errors) ? data.errors.join('; ') : (data.error || 'Save failed');
            this.cfgSaveError = detail;
          } else {
            this.patch = {};
            const prevSection = this.openSection;
            await this.loadConfig();
            this.openSection = prevSection;
            saved = true;
          }
        } catch (e) { this.cfgSaveError = e.message; }
        finally { this.cfgSaving = false; }

        if (saved && window.hyloDesktop?.restartServer) {
          this.cfgRestarting = true;
          try {
            const r = await window.hyloDesktop.restartServer();
            if (r.ok) return;
            if (r.reason === 'no_pid') { this.cfgSaveOk = true; }
            else { this.cfgRestartError = r.error || 'Restart failed'; }
          } catch (e) { this.cfgRestartError = e.message; }
          finally { this.cfgRestarting = false; }
        } else if (saved) {
          this.cfgSaveOk = true;
        }
      },

      async applyServerUrl() {
        this.urlError = '';
        const raw = this.serverUrl.trim().replace(/\/$/, '');
        try {
          const p = new URL(raw);
          if (!['http:', 'https:'].includes(p.protocol)) throw new Error('Must use http:// or https://');
          this.urlSaving = true;
          await window.hyloDesktop.setServerUrl(raw);
        } catch (e) { this.urlError = e.message; this.urlSaving = false; }
      },

      async pickFolder(key, currentVal) {
        if (!window.hyloDesktop?.pickFolder) return;
        const result = await window.hyloDesktop.pickFolder({
          title: 'Select Vault Root Folder',
          defaultPath: currentVal || undefined,
        });
        if (!result.canceled && result.path) this.setVal(key, result.path);
      },

      notifySoundLabel(v) {
        const m = { beep: 'System beep', none: 'None', Glass: 'Glass', Ping: 'Ping', Pop: 'Pop', Tink: 'Tink', Hero: 'Hero', Purr: 'Purr', Submarine: 'Submarine' };
        return m[v] || v;
      },

      async loadNotifySettings() {
        if (!window.hyloDesktop?.inboxNotify) return;
        try {
          this.notifySettings = await window.hyloDesktop.inboxNotify.getSettings();
        } catch(_) {}
      },

      async saveNotifySettings() {
        if (!window.hyloDesktop?.inboxNotify) return;
        try {
          // Spread to a plain object so Electron's contextBridge Structured Clone
          // doesn't silently drop the Alpine.js reactive Proxy wrapper.
          const snap = {
            textEnabled:  this.notifySettings.textEnabled,
            soundEnabled: this.notifySettings.soundEnabled,
            sound:        this.notifySettings.sound,
          };
          this.notifySettings = await window.hyloDesktop.inboxNotify.setSettings(snap);
          this.notifySaveOk = true;
          setTimeout(() => { this.notifySaveOk = false; }, 1500);
        } catch(e) { console.error('[notify] saveNotifySettings error:', e); }
      },
    };
  }
`

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
