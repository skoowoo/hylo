---
version: 1.0
name: Vaultr-design-system
description: "Vaultr's own dark-first UI system: a near-black canvas (#08080b), soft-white ink, and a single lavender-blue accent (#5e6ad2) used scarcely — primary actions, focus rings, selected-state text, link emphasis. Depth comes from a surface ladder (bg → surface-soft → surface-2) and hairline borders, not shadows, except for a small set of floating layers (dialogs, popovers, the search overlay) that opt into a restrained lift-off shadow. Inter carries every weight of text, chrome-density type scale, tuned for a dense notes/agent app rather than a marketing page. A theme-invariant four-color identity palette marks people/entities; three semantic colors (ok/err/warn) mark status."

colors:
  accent: "#5e6ad2"
  accent-fg: "#ffffff"
  accent-hov-dark: "#7b86e8"
  accent-hov-light: "#4c56c8"
  accent-focus: "#5e6ad2"
  bg-dark: "#08080b"
  bg-light: "#fcfcfc"
  fg-dark: "#eef0f4"
  fg-light: "#18181b"
  muted-dark: "#8d92a0"
  muted-light: "#6d7080"
  muted-soft-dark: "#656a78"
  muted-soft-light: "#9b9eac"
  surface-soft-dark: "#15161a"
  surface-soft-light: "#f6f6f7"
  surface-2-dark: "#1b1c21"
  surface-2-light: "#ececf0"
  border-dark: "#262730"
  border-light: "#e7e7ea"
  border-strong-dark: "#363742"
  border-strong-light: "#d4d4d8"
  canvas: "#ffffff"
  inverse-canvas: "#ffffff"
  inverse-ink: "#000000"
  body-text-dark: "#d0d6e0"
  body-text-light: "#3f4147"
  identity-cyan: "#22d3ee"
  identity-pink: "#f472b6"
  identity-violet: "#a78bfa"
  identity-green: "#34d399"
  success-dark: "#34c481"
  success-light: "#059669"
  error-dark: "#f87171"
  error-light: "#dc2626"
  warning-dark: "#eeae3a"
  warning-light: "#d97706"

typography:
  title-lg:
    fontFamily: Inter
    fontSize: 22px
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: -0.014em
  title-sm:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: 0
  body:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: 0
  small:
    fontFamily: Inter
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: 0
  caption:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: 0
  micro:
    fontFamily: Inter
    fontSize: 11px
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: 0.03em
  nano:
    fontFamily: Inter
    fontSize: 10px
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: 0
  button:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 600
    lineHeight: 1.0
    letterSpacing: 0
  code:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: 0
  prose-body:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: 400
    lineHeight: 1.75
    letterSpacing: 0

rounded:
  xs: 4px
  sm: 6px
  md: 8px
  lg: 12px
  xl: 16px
  full: 999px

spacing:
  xxs: 4px
  xs: 8px
  sm: 12px
  md: 16px
  lg: 24px
  xl: 32px
  xxl: 48px

components:
  icon-btn-ghost:
    backgroundColor: transparent
    textColor: "{colors.muted-dark}"
    size: 28px
    rounded: none
  icon-btn:
    backgroundColor: "rgba(ink, 0.06)"
    textColor: "{colors.muted-dark}"
    size: 28px
    rounded: "{rounded.sm}"
  icon-btn-active:
    backgroundColor: "rgba(ink, 0.15)"
    textColor: "{colors.fg-dark}"
    rounded: "{rounded.sm}"
  seg-toggle:
    backgroundColor: "rgba(ink, 0.06)"
    padding: 3px
    rounded: "{rounded.md}"
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.accent-fg}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    height: 32px
  button-secondary:
    backgroundColor: "rgba(ink, 0.06)"
    textColor: "{colors.fg-dark}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    height: 32px
  button-danger:
    backgroundColor: "{colors.error-dark}"
    textColor: "{colors.inverse-ink}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
  dialog-card:
    backgroundColor: "{colors.bg-dark}"
    border: 1px solid border-strong
    rounded: "{rounded.xl}"
    padding: 24px 24px 20px
    shadow: lg
  drawer-panel:
    backgroundColor: "{colors.bg-dark}"
    border: 1px solid border (left edge only)
    rounded: none
    width: 80vw (expandable to 100vw)
  search-panel:
    backgroundColor: "{colors.bg-dark}"
    border: 1px solid border-strong
    rounded: "{rounded.xl}"
    shadow: md
  settings-modal:
    backgroundColor: "{colors.bg-dark}"
    border: 1px solid border-strong
    rounded: "{rounded.xl}"
    shadow: lg
  list-card:
    backgroundColor: "{colors.surface-soft-dark}"
    hoverBackgroundColor: "{colors.card-hov}"
    rounded: "{rounded.lg}"
  note-badge:
    backgroundColor: transparent
    activeBackgroundColor: "{colors.accent}"
    rounded: "{rounded.sm}"
  status-pill-ok:
    backgroundColor: "rgba(success, 0.16)"
    textColor: "{colors.success-dark}"
    border: 1px solid "rgba(success, 0.4)"
    rounded: "{rounded.xs}"
  status-pill-err:
    backgroundColor: "rgba(error, 0.14)"
    textColor: "{colors.error-dark}"
    border: 1px solid "rgba(error, 0.35)"
    rounded: "{rounded.xs}"
  status-pill-warn:
    backgroundColor: "rgba(warning, 0.14)"
    textColor: "{colors.warning-dark}"
    border: 1px solid "rgba(warning, 0.36)"
    rounded: "{rounded.xs}"
  entity-chip:
    backgroundColor: "{colors.identity-violet}"
    rounded: "{rounded.full}"
  image-tile:
    backgroundColor: "{colors.surface-soft-dark}"
    rounded: "{rounded.xl}"
  avatar:
    rounded: "{rounded.full}"
---

## Overview

Vaultr's UI runs on a near-black canvas (`{colors.bg-dark}` #08080b — lifted just off true black to avoid a flat, harsh void) with soft-white ink (`{colors.fg-dark}` #eef0f4) and a single chromatic accent, lavender-blue (`{colors.accent}` #5e6ad2). The accent is scarce by design: primary actions, focus rings, list/tab selected-state text, and link emphasis — never a section background or card fill.

Depth is carried almost entirely by a **surface ladder** (`--bg` → `--surface-soft` → `--surface-2`) plus 1px hairline borders, not box-shadow. The one deliberate exception is a small set of floating layers — confirm/info dialogs, the settings modal, the search overlay, drawer popovers — which opt into a restrained lift-off shadow (`--shadow-{xs,sm,md,lg}` paired with `--shadow-color`) so a card reads as "off the surface" rather than a flat bordered rectangle. Everything else stays flat.

Inter carries every weight of text, from modal titles down to 10px meta labels — there is no separate display/body family split; the type scale is tuned for a dense, chrome-heavy product (notes list, agent chat, graph, image grid) rather than a marketing page, so it tops out around 22px rather than a hero-sized display cut. JetBrains Mono covers code blocks and inline code.

Dark is the shipped default (bare `:root`). Light is Vaultr's own from-scratch adaptation of the same hue/radius/spacing system — cool near-neutral grays, not a warm cream/pure-black pairing — kept alive because a notes app (unlike a marketing page) needs a comfortable reading surface in bright rooms. The theme toggle (light/dark/auto) lives in Settings → Editor and is applied via `data-theme` on `<html>`, with an `auto` mode that follows the OS `prefers-color-scheme` live.

**Key Characteristics:**
- **Dark-first, near-black canvas** (`{colors.bg-dark}` #08080b) with a from-scratch light counterpart, not an afterthought palette swap.
- **One chromatic accent** (`{colors.accent}` #5e6ad2) — used scarcely: primary CTA, focus ring, selected-state text, link emphasis.
- Three-step surface ladder (bg → surface-soft → surface-2) carries hierarchy without shadow; shadow is reserved for floating layers only.
- One continuous type family (Inter) across every size; no display/body split.
- A theme-invariant four-color identity palette (`{colors.identity-cyan}`, `{colors.identity-pink}`, `{colors.identity-violet}`, `{colors.identity-green}`) marks "which person/entity", independent of "which mode".
- Three semantic colors (success/error/warning) — tuned bright for legibility as running text on dark, paired with `--{ok,err,warn}-fg` (inverse ink) when used as a solid fill instead.
- Corner radius scales from `{rounded.xs}` (4px, chips/badges) to `{rounded.xl}` (16px, modal/panel surfaces) — never a hard 0° corner by default (base.css rounds every element to `{rounded.md}` unless overridden), except edge-docked, full-viewport chrome (drawer panel, scrims, topbars) which explicitly opts back to 0.

## Colors

### Brand & Accent
- **Accent** (`{colors.accent}`): The one chromatic color in the system — primary buttons, focus rings, selected-tab/row text, link emphasis. List markers (bullet, number, task checkbox) use the body ink (`--prose-body`), not accent, in every surface.
- **Accent Hover**: Lightens on dark (`{colors.accent-hov-dark}` #7b86e8), darkens on light (`{colors.accent-hov-light}` #4c56c8) — same hue, opposite direction per theme so it always reads as "brighter than resting state" against its own canvas.
- **Customizable**: `{colors.accent}` is the default (indigo), not a constant. Settings → Appearance offers 8 luminance-matched presets (`internal/server/view/shared_accent.go`), applied as inline `--accent`/`--accent-rgb`/`--accent-hov` on `<html>` before first paint. Components must consume only these tokens (or `rgba(var(--accent-rgb), …)`) — never a literal accent hex — or the preset won't reach them. A new preset needs white text ≥4.5:1 on its accent.
- **Accent Text** (`--accent-text`): Accent used as *text or icon color* on the canvas (links, active tab/row labels, badges). Equals `--accent` on light; on dark it is lifted to ≈4.6:1 (presets carry a `TextDark`), because the resting accent is tuned for white-on-fill (≥4.5:1) and reads dim as small text. Use `--accent` for fills, borders and rings, `--accent-text` for `color:`.
- **Accent Focus** (`{colors.accent-focus}`, always `var(--accent)`): The literal ring color for `:focus-visible` — 2px outline at 50% opacity, offset 1px. The one place the system uses anything resembling a glow.
- **Selected surface** (`--control-active-bg` / `-fg`): Not an accent fill — steps up to `--surface-2` with full `--fg` text. This is the single treatment for "this tab/segment/row/toggle is currently selected" everywhere in the app (sidebar items, drawer tabs, segmented controls, search results, icon-toggle buttons). Primary-action buttons (compose, publish, save, send) use accent directly instead — never this pair.

### Surface
- **Bg** (`{colors.bg-dark}` / `{colors.bg-light}`): The page canvas.
- **Surface Soft**: One step up from bg — sidebars, dropdowns, floating toolbars, unread-cleared inbox cards.
- **Surface 2**: Two steps up — selected rows/tabs, featured/active states, settings sidebar hover.
- **Border** / **Border Strong**: 1px hairlines for cards, dividers, and input outlines; "strong" is used where the border itself needs to read as structure (icon-btn outline, drawer tab-bar-start divider) rather than a quiet separator.
- **Canvas** (`{colors.canvas}` #ffffff, fixed in both themes): A theme-invariant white used only where an element must stay white regardless of mode (e.g. the drawer's expand-button icon against a filled accent-adjacent chrome bar).

### Text
- **Fg**: Primary text and headings.
- **Muted**: Secondary text — icon-button resting color, nav item labels (dimmed further via `--nav-fg` at 82% opacity), timestamps.
- **Muted Soft**: Tertiary text — placeholders, deselected/disabled labels.
- **Body**: A distinct reading tone (#d0d6e0 dark / #3f4147 light) used for meta/lead text that needs to sit between `--fg` and `--muted`.
- **Cover Dir**: The directory-label tone used on note cover previews and search result directory breadcrumbs.

### Semantic
- **Success / Error / Warning**: Each ships as a triple — a bright running-text color, a low-opacity tinted background, and a matching-opacity border — for status pills, save-state indicators, and validation messages. When one of these needs to be a *solid fill* instead of running text (a handful of solid status buttons/pills), pair it with the matching `-fg` token (inverse ink), since the bright running-text tones are too light for white text on top.
- **Identity Palette** (`{colors.identity-cyan}`, `{colors.identity-pink}`, `{colors.identity-violet}`, `{colors.identity-green}`): Theme-invariant — these mark "which thing" (an agent bot's assigned color, an entity-type chip in the graph), not "which mode", so they never re-tint between light and dark.
- **Overlay / Scrim**: Modal and drawer backdrops are always a fixed black veil (25% for dialogs, 20% for the drawer, 40% for the image lightbox) independent of theme — darkening a backdrop reads the same whether the surface above it is paper-white or near-black.

## Typography

### Font Family
- **Inter** — the single UI voice, covering everything from modal titles down to 10px micro-labels. No display/body split: the family change other systems use to mark "hero" vs. "body" is absent here because nothing in this dense, chrome-heavy app operates at marketing-hero scale.
- **JetBrains Mono** — code blocks, inline code, and note-editor code fences.

### Hierarchy

| Token                     | Size | Weight | Line Height | Use                                                                                           |
| ------------------------- | ---- | ------ | ----------- | --------------------------------------------------------------------------------------------- |
| `{typography.title-lg}`   | 22px | 600    | 1.3         | Search overlay title, modal titles                                                            |
| `{typography.title-sm}`   | 16px | 600    | 1.4         | List item titles, card titles                                                                 |
| `{typography.body}`       | 14px | 400    | 1.5         | Default UI text, buttons at 600                                                               |
| `{typography.small}`      | 13px | 400    | 1.5         | Secondary UI text, dialog messages                                                            |
| `{typography.caption}`    | 12px | 400    | 1.4         | Captions, badge labels, section labels — practical minimum for most UI                        |
| `{typography.micro}`      | 11px | 400    | 1.4         | Search kbd hints, dense meta rows                                                             |
| `{typography.nano}`       | 10px | 400    | 1.4         | Rare micro-labels only                                                                        |
| `{typography.button}`     | 14px | 600    | 1.0         | All button labels                                                                             |
| `{typography.code}`       | 14px | 400    | 1.5         | JetBrains Mono, code blocks and inline code                                                   |
| `{typography.prose-body}` | 16px | 400    | 1.75        | Note editor / reading-mode running prose — the one place line-height opens up for readability |

### Principles
- **One family, every weight.** Inter at 400/500/600 covers the whole app; there's no separate display cut.
- **Chrome type stays tight** (line-height 1.0–1.5); **prose opens up** (1.75) — the editor and reading surfaces are the one place optimized for sustained reading rather than scanning.
- **Uppercase/caps labels** (section headers, badge text) get a small positive tracking (`--ls-cap`, 0.08em) to stay legible at small sizes; everything else holds 0 letter-spacing — negative tracking on display type isn't part of this system since there's no display tier.
- **Mono only in code contexts** — code fences, inline code spans, and note-editor code blocks.

## Layout

### Spacing System
- **Base unit**: 4px.
- **Tokens**: `{spacing.xxs}` 4px · `{spacing.xs}` 8px · `{spacing.sm}` 12px · `{spacing.md}` 16px · `{spacing.lg}` 24px · `{spacing.xl}` 32px · `{spacing.xxl}` 48px.
- Dialog interior padding: 24px top/sides, 20px bottom (`.confirm-card`, `.info-card`).
- Standard button height: 32px (`--btn-h`); compact/icon-only actions: 28px (`--btn-h-xs` / `--action-btn-sz`); chrome-bar height: 36px (`--btn-h-sm`).
- Topbar height: 40px (`--topbar-h`).
- List pane header (`.home-list-head`): the same 52px `--toolbar-h`, so its controls line up with the editor tool bar across the pane divider. No title, no bottom divider and no scroll fade (the list just scrolls to a plain edge); a section that has a number to show gets a low-key count pill (`--cnt-bg` / `--cnt-tx`, the same recipe as the graph index counts) at the left, with the tooltip naming what it counts.
- Editor tool bar: 52px (`--toolbar-h`), fixed — it no longer tracks the list pane's header. It has no bottom divider; instead the editor's top 32px eases into `--bg` (a smoothstep ramp, so neither end reads as a gradient edge) and text scrolling under the bar dissolves rather than being cut off.

### Grid & Container
- The app is chrome-first, not a marketing grid: a persistent list/sidebar column plus a main content pane (notes list + reading pane, drawer over content, graph canvas with an index column).
- The image grid and note-card grids reflow responsively by tile width rather than a fixed N-up breakpoint table.
- The drawer panel docks to the right edge at 80vw, expandable to 100vw — it's an overlay, not a page section.

### Whitespace Philosophy
The dark canvas IS the whitespace — sections separate by lifting onto `--surface-soft`/`--surface-2`, not by gaps in white. Full-viewport chrome (drawer, scrims, topbars) is flush to the edge and never rounds. Floating dialogs and popovers get generous interior padding (20–24px) since they're small, focused surfaces rather than dense list rows.

## Elevation & Depth

| Level                    | Treatment                                                  | Use                                                                                                     |
| ------------------------ | ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| 0 (flat)                 | No shadow, no border                                       | Default for body text, list rows, chrome strips                                                         |
| 1 (soft lift)            | `--surface-soft` background                                | Sidebars, dropdowns, floating toolbars, cleared-inbox cards                                             |
| 2 (surface-2 lift)       | `--surface-2` background                                   | Selected tabs/rows, active states                                                                       |
| 3 (hairline)             | 1px `--border` / `--border-strong`                         | Card outlines, dividers, input outlines                                                                 |
| 4 (focus ring)           | 2px `--accent-focus` outline at 50% opacity, 1px offset    | Focused input, focused button — the only glow-like effect in the system                                 |
| 5 (float shadow, opt-in) | `box-shadow: var(--shadow-{sm,md,lg}) var(--shadow-color)` | Confirm/info dialogs (`lg`), settings modal (`lg`), search overlay panel (`md`), drawer popovers (`sm`) |

Everything that doesn't explicitly opt into level 5 stays flat, matching the surface-ladder-only depth model. `--shadow-color` is the one actual shadow color per theme (`rgba(0,0,0,0.55)` dark, `rgba(15,15,25,0.10)` light); `--shadow-xs/sm/md/lg` only ever supply offset/blur shape.

### Decorative Depth
- No atmospheric gradients, no spotlight cards, no product-screenshot hero treatment — this is an app UI, not a marketing canvas.
- Modal/dialog/search-overlay scrims apply a faint backdrop blur (`--glass-scrim-filter`, 3px) — just enough to soften the background, well short of a frosted-glass look.
- Image lightbox and grid chrome (selection borders, chip backgrounds) use fixed black/white overlays independent of theme, since they draw on top of arbitrary photo content rather than app surfaces.

## Shapes

### Border Radius Scale

| Token            | Value | Use                                                                                         |
| ---------------- | ----- | ------------------------------------------------------------------------------------------- |
| `{rounded.xs}`   | 4px   | Chips, status badges, small icon buttons                                                    |
| `{rounded.sm}`   | 6px   | Inline tags, inline code, prose images                                                      |
| `{rounded.md}`   | 8px   | Default for buttons and form inputs — and the baseline every element gets unless overridden |
| `{rounded.lg}`   | 12px  | List cards, image grid tiles                                                                |
| `{rounded.xl}`   | 16px  | Dialogs, modals, the search overlay panel, image lightbox tiles                             |
| `{rounded.full}` | 999px | Pills, avatars, unread dots, agent-bot-color swatches                                       |

Full-viewport or edge-docked chrome (drawer panel and its tab bar, scrims, `<html>`/`<body>`, SVG icons, and replaced elements like the graph's canvas) explicitly resets to 0 — these are structural surfaces flush to real viewport edges, never a "card" shape.

## Components

### Buttons & Icon Actions

**`button-primary`** — Accent-filled. The default primary action (confirm, save, send).
- Background `{colors.accent}`, text `{colors.accent-fg}`, height 32px, rounded `{rounded.md}`. Hover shifts to accent-hover; active press dims to 85% opacity app-wide as a baseline (any component-specific `:active` rule wins over this floor).

**No button carries a border.** Hierarchy comes from fill alone, using a three-step neutral ladder built on the theme's ink color (`--ink-rgb`, so one set of values serves both themes): `--btn-bg` (6%, rest) → `--btn-bg-hov` (10%, hover) → `--btn-bg-on` (15%, pressed / selected). No shadows or inner highlights; keyboard focus is the 2px accent ring from the elevation table (borderless controls have no other focus cue). Hover/press color changes use `--motion-fast`.

**`button-secondary`** (`.btn-outline`, name kept from before the redesign) — Neutral tonal fill. Cancel/dismiss/toolbar actions.
- `--btn-bg` background, `--fg` text, height 32px. Hover `--btn-bg-hov`, press `--btn-bg-on`. `.active` (chosen-among-choices) holds `--btn-bg-on`.
- `.btn-outline--danger` stays neutral at rest and reveals the `--s-err-bg` tint plus `--s-err` text on hover/focus.

**`button-danger`** — Reserved for destructive confirmations (delete).
- Background `--s-err`, text `--s-err-fg` (inverse ink for contrast against the bright error tone).

**`icon-btn-ghost`** — Icon-only action with no fill at rest, 28px square (24px `--sm` modifier for tighter contexts). Fills with `--icon-hov` (an accent tint) on hover. Used for dismiss/close actions and any icon action that should sit quietly on its surface.

**`icon-btn`** — Tonal icon-only action (`--btn-bg`, `{rounded.sm}`) with an `.active` toggle state (`--btn-bg-on` + `--control-active-fg`) for controls that need to show "this is currently on" (pin, compile, source toggle), not just hover feedback. `.icon-btn--lg` (32px) uses `{rounded.md}` to sit flush with inputs.

**`seg`** / **`seg-btn`** — Segmented "pick one of N" control: a `--btn-bg` track with 3px padding and flat buttons inside (inner radius `{rounded.sm}`); `.active` is a raised thumb, `--seg-thumb` (`--control-active-bg-on-soft` on dark, `--canvas` on light) with `--control-active-fg` text. One shared definition backs every picker in the app (Inbox filter, chat mode toggle, settings pickers) so they can't drift apart in size.

**Chips and choice cards** (agent-bot chip, variable chip, effect card, copy button, load-more) follow the same ladder; they are not a separate component family.

### Dialogs & Overlays

**`dialog-card`** (confirm/info dialogs) — Centered floating card over a blurred, dimmed scrim.
- Background `--bg`, 1px `--border-strong` border, rounded `{rounded.xl}`, padding 24px/24px/20px, `--shadow-lg` lift. Title at `{typography.body}` weight 600; message at `{typography.small}`, `--muted`.

**`drawer-panel`** — Edge-docked slide-in panel (note editor, agent chat side panel).
- Background `--bg`, left border only, no radius (flush to 3 viewport edges), 80vw wide (100vw when expanded). Slides in via transform + opacity, not a fade alone, so it reads as spatial motion.

**`search-panel`** — The command/search overlay.
- Background `--bg`, 1px `--border-strong`, rounded `{rounded.xl}`, `--shadow-md` lift, backdrop blur scrim. Row dividers use `--border`; keyboard-hint chips use `{rounded.xs}` with a hairline border.

**`settings-modal`** — Same floating-card family as dialogs: `--bg`, `--border-strong`, rounded `{rounded.xl}`, `--shadow-lg`.

### Lists & Cards

**`list-card`** — Notes list rows, inbox cards. Note rows (Pinned, Folders, and the other lists that share that card) and a read/cleared inbox card sit on `--surface-soft` with no outline; the read card also dims its title/timestamp to `--muted` and drops its unread dot. An unread inbox card stays flat on `--bg` with a `--border-strong` outline.

**`note-badge`** — Small status marker on a note card. The pin variant is a plain accent-colored glyph with no fill; the "done" variant is a solid accent fill with `--accent-fg` text.

**`status-pill`** — ok/err/warn tinted pill: low-opacity tinted background + matching-opacity border + bright text, `{rounded.xs}`, `{typography.caption}`.

### Identity & Media

**`entity-chip`** / **`agent-bot-avatar`** — Colored dot or avatar fallback drawing from the four-color identity palette (cyan, pink, violet, green), theme-invariant. The graph's entity-type nodes default to the violet slot.

**`image-tile`** — Grid tile in the image gallery: `--surface-soft` background, rounded `{rounded.xl}`. Selection state uses a fixed white border + black scrim overlay (not theme tokens), since tiles sit on top of arbitrary photo content.

**`avatar`** — `{rounded.full}` circle, sized per context (agent bot chips, testimonial-style contexts if introduced later).

## Do's and Don'ts

### Do
- Reserve `{colors.accent}` for primary actions, focus rings, selected-state text, and link emphasis only.
- Use the three-step surface ladder (bg → surface-soft → surface-2) for hierarchy; don't skip a level.
- Let full-viewport/edge-docked chrome reset to 0 radius; let everything else default to `{rounded.md}` unless a card/panel calls for `{rounded.lg}`/`{rounded.xl}`.
- Keep the identity palette (cyan/pink/violet/green) theme-invariant — it marks "who/what", not "which mode".
- Pair a semantic color's bright running-text tone with its own `-fg` token when using it as a solid fill, never with plain white/black.
- Apply the shared `.icon-btn`/`.icon-btn-ghost`/`.seg`/`.seg-btn` components instead of a new per-page near-copy.
- Build any new button from the `--btn-bg` / `--btn-bg-hov` / `--btn-bg-on` ladder; never add a border to one.

### Don't
- Don't use the accent color as a section background or card fill.
- Don't add box-shadow to a surface that isn't one of the documented floating layers (dialogs, settings modal, search overlay, drawer popovers).
- Don't introduce a second chromatic brand accent.
- Don't reach for a display-scale font size — this system tops out at `{typography.title-lg}` (22px); there's no marketing-hero tier.
- Don't hardcode a theme-specific hex in component CSS — reference the semantic token (`--bg`, `--fg`, `--accent`, etc.) so it re-themes automatically.
- Don't let a new "selected" state invent its own fill — reuse `--control-active-bg`/`-fg` (or `--control-active-bg-on-soft` inside a `--surface-soft` container, where the plain surface-2 jump is too faint to read).

## Motion

Restrained by default: an animation exists to confirm a user-triggered action, explain a spatial relationship, or give transient feedback — never ambient, never looping, never present while the user is reading or typing. If it's only there to look nice, it doesn't ship.

Four durations, chosen by what's moving, not per-component ad hoc values:

| Token           | Duration | Use                                                                                                        |
| --------------- | -------- | ---------------------------------------------------------------------------------------------------------- |
| `--motion-fast` | 100ms    | Hover/press feedback — background, color, opacity tweaks. No easing curve; too short to perceive one.      |
| `--motion-base` | 160ms    | Small reveal/toggle transforms — chevron rotate, disclosure arrows, drag-state feedback.                   |
| `--motion-slow` | 220ms    | Panel/overlay-scrim fades (drawer, inbox sheet, lightbox) — gives a spatial change room to read as motion. |
| `--motion-view` | 320ms    | Reserved for a bigger spatial change — a view switch or shared-element transition. Nothing uses it yet.    |

Four named curves pair with the durations above:

| Token             | Curve                          | Use                                                                                                                                                                                                    |
| ----------------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `--ease-standard` | `ease`                         | The default for everything above — this app's plain, un-opinionated fallback.                                                                                                                          |
| `--ease-out`      | `cubic-bezier(0,0,.2,1)`       | Entering/arriving/expanding — the same curve Tailwind's `ease-out` gives the menu/dialog `x-transition`s, so a hand-rolled `transition`/`@keyframes` and an Alpine transition read as the same motion. |
| `--ease-in`       | `cubic-bezier(.4,0,1,1)`       | Leaving/collapsing/dismissing — the `ease-in` counterpart to the one above.                                                                                                                            |
| `--ease-spring`   | `cubic-bezier(.34,1.56,.64,1)` | The one deliberate overshoot in the system. Reserved for a receipt/confirmation instant (a drop landing, a toggle settling) — never for ambient motion, an ordinary enter/exit, or a loop.             |

A bespoke `@keyframes` animation (a shake, a fly-to-target) should still borrow its *duration* from `--motion-*` as a multiple (e.g. `calc(var(--motion-base) * 2)`) even when its own keyframe curve is too specific to generalize into `--ease-*` — durations drift into inconsistency far more easily than one-off curves do, since every new feature reinvents "how long," but rarely needs a genuinely new easing shape. Older bespoke multi-property choreography that predates this rule (the drawer's slide-in transform+opacity, zen-mode fades) still keeps its own hand-tuned durations outside the scale; it hasn't been retrofitted, not exempted.

`prefers-reduced-motion: reduce` collapses all four duration tokens to ~0 globally (`shared_tokens.go`) — since virtually every transition in the app already reads its duration from these tokens rather than a literal value, this one override mutes motion app-wide without a per-component reduced-motion rule. It mutes the *motion* only; the underlying feedback (a color change, a state flip) still happens, just without the animated journey there.

## Theming

- Dark values live on bare `:root` — the shipped default look.
- Light values apply via an explicit `data-theme="light"` attribute on `<html>`, or automatically when the OS prefers light and no explicit choice has been made.
- The user-facing toggle (Settings → Editor → Theme) stores `light` / `dark` / `auto` in `localStorage` and applies immediately pre-paint to avoid a flash of the wrong theme; `auto` keeps a live `prefers-color-scheme` listener so the page follows an OS change without a reload.
- Component CSS should never hardcode a per-theme value — reference the Layer 2 semantic token and it re-themes automatically. Most `rgba(var(--ink-rgb), α)` / `rgba(var(--accent-rgb), α)` tokens don't need restating per theme either: overriding the base RGB triple alone re-tints every derived token.

## Known Gaps

- There is no marketing/landing surface in the current app — this document describes the dense product UI only. If a future marketing page is added, it would need its own display-scale type tier layered on top of this system, not a replacement for it.
- Form-field error/validation styling beyond the shared status-pill triple is not yet standardized per input type.
- The identity/entity-type color mapping currently exposes four slots (cyan/pink/violet/green); a fifth would need a deliberate hue choice to stay distinguishable from the accent and semantic colors.
