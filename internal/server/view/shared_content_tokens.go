package view

// Markdown content color palette — everything that colors rendered markdown
// text, kept apart from shared_tokens.go's UI chrome palette so the two can
// be customized/extended independently later (values below are unchanged
// from before the split). Backs:
//   - the note reader (.prose, note_shared_prose.css)
//   - the CM6 live-preview editor (editor/src/cm-live/theme.js —
//     same var() names, read at runtime via EditorView.theme())
//   - every other surface that reuses .prose: agent chat (.msg-body),
//     shorts (.shorts-entry-prose)
// Same three-layer shape as the UI tokens (shared primitives, dark default,
// light override); appTokensCSS (shared_tokens.go) concatenates both
// palettes into the same :root blocks.
const contentTokensShared = `
      --bq-bd:rgba(var(--accent-rgb),0.55);
      --ul-mk:var(--prose-body); --ol-mk:var(--prose-body);
      --code-bg:rgba(var(--ink-rgb),0.055);
      --th-bg:rgba(var(--ink-rgb),0.04); --tbl-bd:rgba(var(--ink-rgb),0.12);
      --th-tx:var(--fg-2);`

// Default palette (bare :root). Only tokens that differ from light.
const contentTokensDark = `
      --link:#eff0f4;
      --link-ul:var(--accent); --link-ul-hov:var(--accent-hov);
      /* Headings step down by size, never below the body's ink; only the tiny h6 goes muted. */
      --h1:#eff0f4; --h2:#eff0f4; --h3:#eff0f4; --h4:#eff0f4; --h5:var(--prose-body); --h6:#91939d;
      --prose-body:#d0d2d9;
      --prose-strong:#eff0f4; --prose-em:#d0d2d9;
      --pre-tx:#e8e9ee;
      --code-tx:#d0d2d9;
      --bq-tx:rgba(208,210,217,0.85);
      --td-tx:#d0d2d9;
      --cm-md-muted:#91939d;
      /* Code-fence syntax highlighting — Tokyo Night. */
      --syn-keyword:#bb9af7; --syn-const:#ff9e64; --syn-string:#9ece6a;
      --syn-escape:#b4f9f8; --syn-def:#7aa2f7; --syn-param:#e0af68;
      --syn-type:#2ac3de; --syn-class:#73daca; --syn-builtin:#f7768e;
      --syn-property:#73daca; --syn-comment:#565f89; --syn-invalid:#db4b4b;`

// Light adaptation — cool near-neutral grays.
const contentTokensLight = `
      --link:#17181b;
      --link-ul:var(--accent); --link-ul-hov:var(--accent-hov);
      --h1:#17181b; --h2:#17181b; --h3:#17181b; --h4:#17181b; --h5:var(--prose-body); --h6:#696b75;
      --prose-body:#3e3f46;
      --prose-strong:#17181b; --prose-em:#3e3f46;
      --pre-tx:#17181b;
      --code-tx:#3e3f46;
      --bq-tx:rgba(62,63,70,0.88);
      --td-tx:#3e3f46;
      --cm-md-muted:#9c9ea7;
      /* Code-fence syntax highlighting — Tokyo Night Light. */
      --syn-keyword:#5a4a78; --syn-const:#965027; --syn-string:#385f0d;
      --syn-escape:#0f4b6e; --syn-def:#34548a; --syn-param:#8f5e15;
      --syn-type:#166775; --syn-class:#33635c; --syn-builtin:#8c4351;
      --syn-property:#33635c; --syn-comment:#848cb1; --syn-invalid:#c53b53;`
