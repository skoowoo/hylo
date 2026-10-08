package view

// Token architecture (DESIGN.md):
//   1. Primitives — raw tuples / theme-invariant literals
//   2. Semantic — what components consume (--bg/--fg/…); dark on :root,
//      light via data-theme or prefers-color-scheme; ink/accent RGB re-tints rgba()
//   3. Visual-regime — flat by default; floating layers opt into --shadow-*
//   4. Component-scoped — search, lightbox, etc.
// New state classes: is-* (legacy .active/.open/.sel stay as-is).
//
// This file owns UI chrome color (bg/fg/borders/status/etc). The markdown
// content palette (headings, prose, links, code, tables, syntax highlight —
// shared by the note reader, the CM6 editor and every other .prose surface)
// lives separately in shared_content_tokens.go so the two can be themed
// independently; appTokensCSS below just concatenates both into the same
// :root blocks.
const appTokensShared = `
      /* ═══ Layer 1: primitives ═══ */
      --accent-rgb:94,106,210;
      --inverse-canvas:#ffffff; --inverse-ink:#000000;
      /* Brand accent — CTA / focus / links only. */
      --accent:#5e6ad2; --accent-fg:#ffffff;
      /* Selected surface: surface-2 + --fg, not accent fill. */
      --control-active-bg:var(--surface-2); --control-active-fg:var(--fg);
      /* Modal scrim — shared by dialogs/settings/search/content pane. Lightbox has its own. */
      --overlay-bg:rgba(0,0,0,0.25);
      --card-hov:rgba(var(--accent-rgb),0.10);
      --icon-hov:rgba(var(--accent-rgb),0.10);
      --tint-soft:rgba(var(--accent-rgb),0.06);
      --tint-md:rgba(var(--accent-rgb),0.10);
      --tint-strong:rgba(var(--accent-rgb),0.14);
      /* Borderless neutral buttons: rest / hover+press / selected. Ink-based so one ladder serves both themes. */
      --btn-bg:rgba(var(--ink-rgb),0.06);
      --btn-bg-hov:rgba(var(--ink-rgb),0.10);
      --btn-bg-on:rgba(var(--ink-rgb),0.15);
      --input-focus-ring:rgba(var(--accent-rgb),0.16);
      --selection-bg:rgba(var(--accent-rgb),0.30);
      /* --line-hair: structural seam between regions (sidebar/content, list rows).
         --line-edge: outer edge of a floating surface or input; always paired
         with a shadow. Prefer a bg step or whitespace over either. */
      --line-hair:rgba(var(--ink-rgb),0.09);
      --line-edge:rgba(var(--ink-rgb),0.16);
      --nav-fg:rgba(var(--ink-rgb),0.82);
      --cm-active-line:rgba(var(--ink-rgb),0.04);
      --cnt-bg:rgba(var(--ink-rgb),0.07);
      --scrollbar-thumb:rgba(var(--ink-rgb),0.12); --scrollbar-thumb-hov:rgba(var(--ink-rgb),0.26);
      --nav-act:rgba(var(--ink-rgb),0.1);
      /* Content-identity chips — theme-invariant. */
      --p0:#22d3ee; --p1:#f472b6; --p2:#a78bfa; --p3:#34d399;
      --entity-type-bg:var(--p2);
      /* Agent-bot avatar palette — theme-invariant, own token set (not --accent,
         not --p0..--p3) so retuning it never moves other surfaces. home.js
         hashes a bot's name into --bot0..--bot7. Each is the matching
         accentPresets hue (shared_accent.go: indigo/blue/teal/green/amber/
         coral/rose/violet, in that order) lifted to a fixed 82% sat / 78%
         light so a bot's initial (rendered in --inverse-ink, i.e. always
         black) stays readable — Accent itself is too dark for that. */
      --bot0:#99a2f5; --bot1:#99c1f5; --bot2:#99f5f1; --bot3:#99f5c6;
      --bot4:#f5d099; --bot5:#f5a999; --bot6:#f599bc; --bot7:#c199f5;

      /* ═══ Layer 3: visual-regime ═══ */
      --shadow-xs:0 1px 2px; --shadow-sm:0 2px 5px; --shadow-md:0 5px 14px; --shadow-lg:0 14px 34px;
      --accent-focus:var(--accent);
      --r-xs:4px; --r-sm:6px; --r-md:8px; --r-lg:12px; --r-xl:16px; --r-full:999px;
      /* fast=hover, base=small toggle, slow=panel/scrim, view=a bigger
         spatial change (view switch, shared-element motion) — reserved,
         nothing uses it yet. */
      --motion-fast:100ms; --motion-base:160ms; --motion-slow:220ms; --motion-view:320ms;
      /* Named curves, paired with --motion-* above. --ease-standard just
         gives a name to the plain "ease" this app already defaults to
         everywhere; --ease-out/--ease-in match the curves Tailwind's
         ease-out/ease-in utility classes already use for the menu/dialog
         x-transitions (content_pane.html), so a hand-rolled @keyframes/
         transition and an Alpine x-transition read as the same motion.
         --ease-spring is the one deliberate exception to that restrained
         default — an overshoot, reserved for a receipt/confirmation instant
         (e.g. a drop landing), never for ambient or enter/exit motion. */
      --ease-standard:ease;
      --ease-out:cubic-bezier(0,0,.2,1);
      --ease-in:cubic-bezier(.4,0,1,1);
      --ease-spring:cubic-bezier(.34,1.56,.64,1);
      --bd-w:1px; /* whole px — fractional breaks AA at radius arcs */
      --glass-scrim-filter:blur(3px);

      /* ═══ Layer 4: component-scoped ═══ */
      --srch-ic:var(--muted); --srch-ph:var(--muted-soft); --srch-av:var(--control-active-bg); --srch-backdrop:var(--overlay-bg);
      --sr-dir:var(--cover-dir); --sr-tm:var(--muted); --sr-ic:var(--muted); --sr-em:var(--muted);
      --srch-kbd-fg:var(--fg);
      --srch-panel-bd:var(--line-edge); --srch-row-bd:var(--line-hair);

      /* Lightbox/images — fixed veil over arbitrary photos. */
      --lightbox-overlay-bg:rgba(0,0,0,0.4); --lightbox-viewer-bg:#0c0c0c;
      --img-chip-bg:rgba(17,17,17,0.72);
      --img-select-border:rgba(255,255,255,0.72); --img-select-bg:rgba(0,0,0,0.28);

      --font-ui:"Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
      --font-sans:"Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
      --font-mono:"JetBrains Mono",ui-monospace,monospace;
      --text-xs:0.75rem; --text-sm:0.8125rem; --text-base:0.875rem; --text-body:1rem;
      --text-title-sm:1rem; --text-title-lg:1.375rem;
      --text-2xs:0.6875rem; --text-3xs:0.625rem;
      --fw-regular:400; --fw-medium:500; --fw-semibold:600;
      --lh-tight:1.2; --lh-snug:1.3; --lh-normal:1.4; --lh-relaxed:1.5;
      --ls-cap:0.08em;
      --space-xxs:4px; --space-xs:8px; --space-sm:12px; --space-md:16px; --space-lg:24px;
      --space-xl:32px; --space-xxl:48px;
      --topbar-h:40px; --toolbar-h:52px; --action-btn-sz:28px;
      /* Sidebar width — content pane/inbox dock flush to this edge. */
      --home-side-w:248px;
      /* --btn-h primary; --btn-h-xs compact secondary; --btn-h-sm chrome-bar. */
      --btn-h:32px; --btn-h-sm:36px; --btn-h-xs:28px;
      --card-note-h:96px; --card-note-w-max:210px;`

// Default palette (bare :root). Only tokens that differ from light.
const appTokensDark = `
      --ink-rgb:238,240,244;
      --bg:#18191e; --fg:#eef0f4;
      --muted:#8d92a0; --surface-soft:#212229; --surface-2:#292a33;
      /* Extra lift when container already sits on --surface-soft. */
      --control-active-bg-on-soft:#3d3e49;
      --seg-thumb:var(--control-active-bg-on-soft);
      /* macOS vibrancy wash — pull native material toward --surface-soft. */
      --sidebar-glass-bg:rgba(33,34,41,0.6); --sidebar-glass-border:rgba(255,255,255,0.1);
      --sidebar-glass-active:rgba(255,255,255,0.08);
      --canvas:#ffffff; --muted-soft:#656a78; --body:#d0d6e0;
            --accent-hov:#7b86e8;
      /* Accent as text on the canvas (links, active labels): lifted for dark; fills keep --accent. */
      --accent-text:#707ad7;
      --cover-dir:#8d92a0;
      --cnt-tx:#9a9fa8;
      --s-ok:#34c481; --s-ok-bg:rgba(39,166,68,0.16); --s-ok-bd:rgba(39,166,68,0.4);
      --s-err:#f87171; --s-err-bg:rgba(248,113,113,0.14); --s-err-bd:rgba(248,113,113,0.35);
      --s-warn:#eeae3a; --s-warn-bg:rgba(251,191,36,0.14); --s-warn-bd:rgba(251,191,36,0.36);
      /* Solid status fills need dark ink (these hues are too light for white text). */
      --s-ok-fg:var(--inverse-ink); --s-err-fg:var(--inverse-ink); --s-warn-fg:var(--inverse-ink);
      --shadow-color:rgba(0,0,0,0.55);`

// Light adaptation — cool near-neutral grays.
const appTokensLight = `
      --ink-rgb:24,24,27;
      --bg:#f9f9fb; --fg:#18181b;
      --muted:#6d7080; --surface-soft:#f5f5f7; --surface-2:#e3e3e9;
      --control-active-bg-on-soft:var(--surface-2);
      --seg-thumb:var(--canvas);
      /* Brighter canvas needs a more visible thumb than the shared (dark-tuned) value. */
      --scrollbar-thumb:rgba(var(--ink-rgb),0.16); --scrollbar-thumb-hov:rgba(var(--ink-rgb),0.30);
      /* Vibrancy reads dull alone — lean lighter while staying translucent. */
      --sidebar-glass-bg:rgba(250,250,251,0.72); --sidebar-glass-border:rgba(0,0,0,0.08);
      --sidebar-glass-active:rgba(0,0,0,0.06);
      --canvas:#ffffff; --muted-soft:#9b9eac; --body:#3f4147;
            --accent-hov:#4c56c8;
      --accent-text:var(--accent);
      --cover-dir:#6b7280;
      --cnt-tx:#6b7280;
      --s-ok:#059669; --s-ok-bg:rgba(16,185,129,0.08); --s-ok-bd:rgba(16,185,129,0.35);
      --s-err:#dc2626; --s-err-bg:rgba(239,68,68,0.06); --s-err-bd:rgba(239,68,68,0.25);
      --s-warn:#d97706; --s-warn-bg:rgba(217,119,6,0.07); --s-warn-bd:rgba(217,119,6,0.26);
      --s-ok-fg:var(--canvas); --s-err-fg:var(--canvas); --s-warn-fg:var(--canvas);
      --shadow-color:rgba(15,15,25,0.10);`

// Drop into a <style> tag. Dark on :root; light via data-theme or OS preference.
const appTokensCSS = `    :root {` + appTokensShared + appTokensDark + contentTokensShared + contentTokensDark + `
    }
    :root[data-theme="light"] {` + appTokensLight + contentTokensLight + `
    }
    @media (prefers-color-scheme: light) {
      :root:not([data-theme="dark"]) {` + appTokensLight + contentTokensLight + `
      }
    }
    *:focus-visible {
      outline: 2px solid color-mix(in srgb, var(--accent-focus) 50%, transparent);
      outline-offset: 1px;
      box-shadow: none;
    }
    /* Floor — component :active still wins by specificity. */
    button:not(:disabled):active {
      opacity: 0.85;
    }
    /* OS-level "reduce motion" (System Settings/Control Panel accessibility,
       not an in-app toggle) — collapses every --motion-* duration app-wide in
       one place, since virtually every transition/animation in the app is
       already written against these tokens rather than a literal duration.
       Not 0: a literal 0ms transition/animation doesn't fire transitionend/
       animationend in some engines, and code elsewhere (e.g. home.js's
       drag-to-move cleanup) depends on that event actually firing — 0.01ms
       is imperceptible but still real enough to fire correctly. This mutes
       the *motion*; it never removes the feedback itself (e.g. a failed
       move still turns the card red, just without the animated shake). */
    @media (prefers-reduced-motion: reduce) {
      :root {
        --motion-fast:0.01ms; --motion-base:0.01ms; --motion-slow:0.01ms; --motion-view:0.01ms;
      }
    }`
