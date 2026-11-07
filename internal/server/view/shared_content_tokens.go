package view

// Markdown content color palette — everything that colors rendered markdown
// text, kept apart from shared_tokens.go's UI chrome palette so the two can
// be customized/extended independently later. Backs:
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
      --code-bg:oklch(var(--ink) / 0.055);
      --th-bg:oklch(var(--ink) / 0.04); --tbl-bd:oklch(var(--ink) / 0.12);
      --th-tx:var(--fg-2);`

// Default palette (bare :root). Only tokens that differ from light.
const contentTokensDark = `
      --link:var(--fg);
      --link-ul:var(--accent); --link-ul-hov:var(--accent-hov);
      /* Headings step down by size, never below the body's ink; only the tiny h6 goes muted. */
      --h1:var(--fg); --h2:var(--fg); --h3:var(--fg); --h4:var(--fg); --h5:var(--prose-body); --h6:var(--muted);
      --prose-body:oklch(0.8643 0.004 var(--accent-h));
      --prose-strong:var(--fg); --prose-em:var(--prose-body);
      --pre-tx:oklch(0.9347 0.004 var(--accent-h));
      --code-tx:var(--prose-body);
      --bq-tx:oklch(0.8643 0.004 var(--accent-h) / 0.85);
      --td-tx:var(--prose-body);
      --cm-md-muted:var(--muted);
      /* Code-fence syntax highlighting — Tokyo Night. */
      --syn-keyword:#bb9af7; --syn-const:#ff9e64; --syn-string:#9ece6a;
      --syn-escape:#b4f9f8; --syn-def:#7aa2f7; --syn-param:#e0af68;
      --syn-type:#2ac3de; --syn-class:#73daca; --syn-builtin:#f7768e;
      --syn-property:#73daca; --syn-comment:#565f89; --syn-invalid:#db4b4b;`

// Light adaptation.
const contentTokensLight = `
      --link:var(--fg);
      --link-ul:var(--accent); --link-ul-hov:var(--accent-hov);
      --h1:var(--fg); --h2:var(--fg); --h3:var(--fg); --h4:var(--fg); --h5:var(--prose-body); --h6:var(--muted);
      --prose-body:oklch(0.3694 0.008 var(--accent-h));
      --prose-strong:var(--fg); --prose-em:var(--prose-body);
      --pre-tx:var(--fg);
      --code-tx:var(--prose-body);
      --bq-tx:oklch(0.3694 0.008 var(--accent-h) / 0.88);
      --td-tx:var(--prose-body);
      --cm-md-muted:var(--muted-soft);
      /* Code-fence syntax highlighting — Tokyo Night Light. */
      --syn-keyword:#5a4a78; --syn-const:#965027; --syn-string:#385f0d;
      --syn-escape:#0f4b6e; --syn-def:#34548a; --syn-param:#8f5e15;
      --syn-type:#166775; --syn-class:#33635c; --syn-builtin:#8c4351;
      --syn-property:#33635c; --syn-comment:#848cb1; --syn-invalid:#c53b53;`
