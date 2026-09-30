package view

// Markdown content color palette — everything that colors rendered markdown
// text, kept apart from shared_tokens.go's UI chrome palette so the two can
// be customized/extended independently later (values below are unchanged
// from before the split). Backs:
//   - the note reader (.prose, note_shared_prose.css)
//   - the CM6 live-preview editor (desktop-app/editor/src/cm-live/theme.js —
//     same var() names, read at runtime via EditorView.theme())
//   - every other surface that reuses .prose: agent chat (.msg-body),
//     inbox (.inbox-sheet-body), shorts (.short-entry-prose)
// Same three-layer shape as the UI tokens (shared primitives, dark default,
// light override); appTokensCSS (shared_tokens.go) concatenates both
// palettes into the same :root blocks.
const contentTokensShared = `
      --bq-bd:rgba(var(--accent-rgb),0.55);
      --ul-mk:var(--prose-body); --ol-mk:var(--prose-body);
      --code-bg:rgba(var(--ink-rgb),0.055);
      --th-bg:rgba(var(--ink-rgb),0.04); --tbl-bd:rgba(var(--ink-rgb),0.12);
      /* h6 rides the same muted-soft tier both themes already use for
         lowest-emphasis text — a heading that size doesn't need its own hue,
         just its own size (see theme.js's h1-h6 comment). */
      --h6:var(--muted-soft);`

// Default palette (bare :root). Only tokens that differ from light.
const contentTokensDark = `
      --link:#eef0f4;
      --link-ul:var(--accent); --link-ul-hov:var(--accent-hov);
      --h1:#eef0f4; --h2:#d0d6e0; --h3:#adb2bc; --h4:#8d92a0; --h5:#797e8c;
      --prose-body:#ccd1db;
      --prose-strong:#eef0f4; --prose-em:#ccd1db;
      --pre-tx:#e9ebf0;
      --code-tx:#ccd1db;
      --bq-tx:rgba(208,214,224,0.85);
      --th-tx:#9a9fa8; --td-tx:#ccd1db; --tbl-bd:rgba(var(--ink-rgb),0.12);
      --cm-md-muted:#8d92a0;
      /* Code-fence syntax highlighting — Tokyo Night. */
      --syn-keyword:#bb9af7; --syn-const:#ff9e64; --syn-string:#9ece6a;
      --syn-escape:#b4f9f8; --syn-def:#7aa2f7; --syn-param:#e0af68;
      --syn-type:#2ac3de; --syn-class:#73daca; --syn-builtin:#f7768e;
      --syn-property:#73daca; --syn-comment:#565f89; --syn-invalid:#db4b4b;`

// Light adaptation — cool near-neutral grays.
const contentTokensLight = `
      --link:#111111;
      --link-ul:var(--accent); --link-ul-hov:var(--accent-hov);
      --h1:#111111; --h2:#1f2937; --h3:#374151; --h4:#6b7280; --h5:#838896;
      --prose-body:#374151;
      --prose-strong:#111111; --prose-em:#374151;
      --pre-tx:#111111;
      --code-tx:#374151;
      --bq-tx:rgba(55,65,81,0.88);
      --th-tx:#9ca3af; --td-tx:#374151; --tbl-bd:rgba(var(--ink-rgb),0.12);
      --cm-md-muted:#9ca3af;
      /* Code-fence syntax highlighting — Tokyo Night Light. */
      --syn-keyword:#5a4a78; --syn-const:#965027; --syn-string:#385f0d;
      --syn-escape:#0f4b6e; --syn-def:#34548a; --syn-param:#8f5e15;
      --syn-type:#166775; --syn-class:#33635c; --syn-builtin:#8c4351;
      --syn-property:#33635c; --syn-comment:#848cb1; --syn-invalid:#c53b53;`
