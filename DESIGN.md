---
version: 1.0
name: Hylo-design-system
description: "Hylo's own dark-first UI system: a deep graphite canvas (#18191e), soft-white ink, every neutral on one hue at low chroma, and a single lavender-blue accent (#5e6ad2) used scarcely — primary actions, focus rings, selected-state text, link emphasis. Depth comes from a surface ladder (bg → surface-soft → surface-2) and hairline borders, not shadows, except for a small set of floating layers (dialogs, popovers, the search overlay) that opt into a restrained lift-off shadow. Inter carries every weight of text, chrome-density type scale, tuned for a dense notes/agent app rather than a marketing page. A theme-invariant four-color identity palette marks people/entities; three semantic colors (ok/err/warn) mark status."

colors:
  accent: "#5e6ad2"
  accent-fg: "#ffffff"
  accent-hov-dark: "#7b86e8"
  accent-hov-light: "#4c56c8"
  accent-focus: "#5e6ad2"
  bg-dark: "#18191e"
  bg-light: "#fafafb"
  fg-dark: "#eff0f4"
  fg-light: "#17181b"
  muted-dark: "#91939d"
  muted-light: "#696b75"
  muted-soft-dark: "#696b74"
  muted-soft-light: "#9c9ea7"
  surface-soft-dark: "#212227"
  surface-soft-light: "#f2f2f4"
  surface-2-dark: "#292b2f"
  surface-2-light: "#e5e6ea"
  border: "rgba(ink, 0.09)"
  border-strong: "rgba(ink, 0.16)"
  canvas: "#ffffff"
  inverse-canvas: "#ffffff"
  inverse-ink: "#000000"
  fg-2-dark: "#b6b8c1"
  fg-2-light: "#4d4f58"
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
  title-list:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1.5
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
    backgroundColor: "glass (--glass-bg + --glass-filter)"
    border: 1px solid border-strong
    rounded: "{rounded.lg}"
    shadow: lg
    size: 720px wide, fixed 56px input + 400px list + 40px footer
  settings-modal:
    backgroundColor: "{colors.bg-dark}"
    border: 1px solid border-strong
    rounded: "{rounded.xl}"
    shadow: lg
  list-card:
    backgroundColor: "{colors.surface-soft-dark}"
    hoverBackgroundColor: "{colors.card-hov}"
    rounded: "{rounded.lg}"
    typography: "{typography.title-list}"
  note-badge:
    backgroundColor: transparent
    activeBackgroundColor: "{colors.fg-dark}"
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

Hylo's UI runs on a deep graphite canvas (`{colors.bg-dark}` #18191e) with soft-white ink (`{colors.fg-dark}` #eff0f4) and a single chromatic accent, lavender-blue (`{colors.accent}` #5e6ad2). The accent is scarce by design: primary actions, focus rings, list/tab selected-state text, and link emphasis — never a section background or card fill.

Depth is carried almost entirely by a **surface ladder** (`--bg` → `--surface-soft` → `--surface-2`) plus 1px hairline borders, not box-shadow. The one deliberate exception is a small set of floating layers — confirm/info dialogs, the settings modal, the search overlay, drawer popovers — which opt into a restrained lift-off shadow (`--shadow-{xs,sm,md,lg}` paired with `--shadow-color`) so a card reads as "off the surface" rather than a flat bordered rectangle. Everything else stays flat.

Inter carries every weight of text, from modal titles down to 10px meta labels — there is no separate display/body family split; the type scale is tuned for a dense, chrome-heavy product (notes list, agent chat, graph, image grid) rather than a marketing page, so it tops out around 22px rather than a hero-sized display cut. JetBrains Mono covers code blocks and inline code.

Dark is the shipped default (bare `:root`). Light is Hylo's own from-scratch adaptation of the same hue/radius/spacing system — cool near-neutral grays, not a warm cream/pure-black pairing — kept alive because a notes app (unlike a marketing page) needs a comfortable reading surface in bright rooms. The theme toggle (light/dark/auto) lives in Settings → Editor and is applied via `data-theme` on `<html>`, with an `auto` mode that follows the OS `prefers-color-scheme` live.

**Key Characteristics:**
- **Dark-first graphite canvas** (`{colors.bg-dark}` #18191e) with a from-scratch light counterpart, not an afterthought palette swap.
- **One neutral hue.** Every grey — surfaces, text, borders, in both themes — sits on OKLCH h≈275 at low, even chroma (surfaces ≈0.009, text ≤0.016), so steps differ by lightness only. Mixing grey families (slate text on zinc chrome, pure #111 headings) or letting chroma climb with lightness is what makes greys look dirty.
- **One chromatic accent** (`{colors.accent}` #5e6ad2) — used scarcely: primary CTA, focus ring, selected-state text, link emphasis.
- Three-step surface ladder (bg → surface-soft → surface-2) carries hierarchy without shadow; shadow is reserved for floating layers only.
- One continuous type family (Inter) across every size; no display/body split.
- A theme-invariant four-color identity palette (`{colors.identity-cyan}`, `{colors.identity-pink}`, `{colors.identity-violet}`, `{colors.identity-green}`) marks "which person/entity", independent of "which mode".
- Three semantic colors (success/error/warning) — tuned bright for legibility as running text on dark, paired with `--{ok,err,warn}-fg` (inverse ink) when used as a solid fill instead.
- Corner radius scales from `{rounded.xs}` (4px, chips/badges) to `{rounded.xl}` (16px, modal/panel surfaces) — never a hard 0° corner by default (base.css rounds every element to `{rounded.md}` unless overridden), except edge-docked, full-viewport chrome (drawer panel, scrims, topbars) which explicitly opts back to 0.

## Colors

### Brand & Accent
- **Accent** (`{colors.accent}`): The one chromatic color in the system — primary buttons, focus rings, selected-tab/row text, link emphasis. List markers (bullet, number, task checkbox) use the body ink (`--prose-body`), not accent, in every surface; a checked box recedes to a `--muted-soft` fill so done items sit behind open ones.
- **Accent Hover**: Lightens on dark (`{colors.accent-hov-dark}` #7b86e8), darkens on light (`{colors.accent-hov-light}` #4c56c8) — same hue, opposite direction per theme so it always reads as "brighter than resting state" against its own canvas.
- **Customizable**: `{colors.accent}` is the default (indigo), not a constant. Settings → Appearance offers 8 luminance-matched presets (`internal/server/view/shared_accent.go`), applied as inline `--accent`/`--accent-rgb`/`--accent-hov` on `<html>` before first paint. Components must consume only these tokens (or `rgba(var(--accent-rgb), …)`) — never a literal accent hex — or the preset won't reach them. A new preset needs white text ≥4.5:1 on its accent.
- **Accent Text** (`--accent-text`): Accent used as *text or icon color* on the canvas (links, active tab/row labels, badges). Equals `--accent` on light; on dark it is lifted to ≈4.6:1 (presets carry a `TextDark`), because the resting accent is tuned for white-on-fill (≥4.5:1) and reads dim as small text. Use `--accent` for fills, borders and rings, `--accent-text` for `color:`.
- **Accent Focus** (`{colors.accent-focus}`, always `var(--accent)`): The literal ring color for `:focus-visible` — 2px outline at 50% opacity, offset 1px. The one place the system uses anything resembling a glow.
- **Selected surface** (`--control-active-bg` / `-fg`): Not an accent fill — steps up to `--surface-2` with full `--fg` text. This is the single treatment for "this tab/segment/row/toggle is currently selected" everywhere in the app (drawer tabs, segmented controls, search results, icon-toggle buttons). Primary-action buttons (compose, publish, save, send) use accent directly instead — never this pair.
- **Nav row wash** (`--nav-hov` / `--nav-act`, ink at 3% / 6%): the one exception — sidebar nav rows (`.side-nav-item`: home sidebar, settings tab list) and the reading-pane toggle. Translucent, so the same rows read alike on `--surface-soft` and on macOS sidebar glass; the selected row also carries full `--fg` text and a full-opacity icon.

### Surface
- **Bg** (`{colors.bg-dark}` / `{colors.bg-light}`): The page canvas.
- **Surface Soft**: One step up from bg — sidebars, dropdowns, floating toolbars, note list cards.
- **Surface 2**: Two steps up — selected tabs, featured/active states.
- **Border** / **Border Strong**: 1px hairlines for cards, dividers, and input outlines; "strong" is used where the border itself needs to read as structure (icon-btn outline, drawer tab-bar-start divider) rather than a quiet separator.
- **Canvas** (`{colors.canvas}` #ffffff, fixed in both themes): A theme-invariant white used only where an element must stay white regardless of mode (e.g. the drawer's expand-button icon against a filled accent-adjacent chrome bar).

### Text
Three roles, split by what the text *is*, not how important it feels: the user's own writing never drops to meta grey.
- **Fg**: Primary text and headings.
- **Fg-2** (`{colors.fg-2-dark}` / `{colors.fg-2-light}`, ~8:1): Secondary *content* — note-card excerpts, table heads, lightbox note previews.
- **Muted** (~5:1): Meta — timestamps, counts, folders, sidebar section headings, icon-button resting color. Nav item labels use `--nav-fg` (ink at 82%) instead.
- **Muted Soft** (~3:1): Placeholders and disabled labels only — never text someone has to read.
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
| `{typography.title-sm}`   | 16px | 600    | 1.4         | Sub-headings                                                                                  |
| `{typography.title-list}` | 14px | 500    | 1.5         | Note card and list row titles — a screen holds a dozen, so they stay at 500                   |
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
- **No caps labels in chrome.** Section headers, menu headers and meta labels are sentence case at 0 letter-spacing; weight and color carry the hierarchy, in the same soft voice as the rounded line icons. Prose H6 keeps its uppercase as content typography.
- **Prose headings step by size, not ink.** h1–h4 carry `--fg`, h5 the body ink, and only the tiny h6 goes `--muted`; a heading is never dimmer than the paragraph under it.
- **Mono only in code contexts** — code fences, inline code spans, and note-editor code blocks.
- **One timestamp format.** Lists show `14:32` today, `Oct 31` this year, `Oct 11, 2025` before that (`formatRelativeTime`, search.go); chat messages add the clock (`Oct 31, 16:57`) and keep `5m ago` / `19h ago` inside their last day; a note card inside chat is a note, so it keeps the list format (`formatDate`, chat_reduce.js). Always 24-hour, always `tabular-nums`. Month labels are sentence case (`Nov 2026`), never caps.

## Layout

### Spacing System
- **Base unit**: 4px.
- **Tokens**: `{spacing.xxs}` 4px · `{spacing.xs}` 8px · `{spacing.sm}` 12px · `{spacing.md}` 16px · `{spacing.lg}` 24px · `{spacing.xl}` 32px · `{spacing.xxl}` 48px.
- Dialog interior padding: 24px top/sides, 20px bottom (`.confirm-card`, `.info-card`).
- Standard button height: 32px (`--btn-h`); compact/icon-only actions: 28px (`--btn-h-xs` / `--action-btn-sz`); chrome-bar height: 36px (`--btn-h-sm`).
- Topbar height: 40px (`--topbar-h`).
- Sidebar nav rows (`.side-nav-item`, top level and children alike): 32px tall, 2px apart — one 34px pitch down the whole sidebar.
- List pane header (`.home-list-head`): the same 52px `--toolbar-h`, so its controls line up with the editor tool bar across the pane divider. No title, no bottom divider and no glass: it stays solid page, and the section body starts below it; a section that has a number to show shows its count at the left as quiet `--muted` text with its noun ("133 notes"), not a pill.
- Editor tool bar: 52px (`--toolbar-h`), fixed and solid `--bg` — it no longer tracks the list pane's header. It has no bottom divider; instead the editor's top 20px eases into `--bg` (a smoothstep ramp, so neither end reads as a gradient edge) and text scrolling under the bar dissolves rather than being cut off.

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
| 1 (soft lift)            | `--surface-soft` background                                | Sidebars, dropdowns, floating toolbars, note list cards                                                 |
| 2 (surface-2 lift)       | `--surface-2` background                                   | Selected tabs/rows, active states                                                                       |
| 3 (hairline)             | 1px `--border` / `--border-strong`                         | Card outlines, dividers, input outlines                                                                 |
| 4 (focus ring)           | 2px `--accent-focus` outline at 50% opacity, 1px offset    | Focused input, focused button — the only glow-like effect in the system                                 |
| 5 (float shadow, opt-in) | `box-shadow: var(--shadow-{sm,md,lg}) var(--shadow-color)` | Confirm/info dialogs (`lg`), settings modal (`lg`), search overlay panel (`lg`), drawer popovers (`sm`) |

Everything that doesn't explicitly opt into level 5 stays flat, matching the surface-ladder-only depth model. `--shadow-color` is the one actual shadow color per theme (`rgba(0,0,0,0.55)` dark, `rgba(15,15,25,0.10)` light); `--shadow-xs/sm/md/lg` only ever supply offset/blur shape.

### Decorative Depth
- No atmospheric gradients, no spotlight cards, no product-screenshot hero treatment — this is an app UI, not a marketing canvas.
- **Glass** marks what floats over moving content, nothing else: menus, dropdowns, completion lists, tooltips, the find panel and the whole search panel with its input (`--glass-bg`; the search preview is an ink step on it, not a solid card), the run toast (status tint over `--glass-bg`), the chat composer (the input card and its context chips; it has no backing band, so messages scroll on under it) with its jump-to-latest button and card-strip arrows (hover is an `--icon-hov` step over the glass), the graph's node panel, and the lightbox counter and steps (`--img-glass-bg`). All share `--glass-filter` (`blur(20px) saturate(1.6)`); without backdrop-filter support they fall back to solid. Chrome bars stay solid: the list head and the editor tool bar are `--bg`. Static surfaces (cards, list rows, chat bubbles, dialogs, settings, the fence-card PiP window and card buttons over a still diagram) never get glass.
- The macOS desktop sidebar is real glass: the window's native `Sidebar` vibrancy shows through a `--sidebar-glass-bg` wash (45% dark, 50% light). The wash is what keeps a bright or saturated wallpaper from tinting the sidebar off the neutral hue; under ~30% it leaks colour, past ~65% the glass turns into a flat grey panel.
- Modal/dialog scrims apply a faint backdrop blur (`--glass-scrim-filter`, 3px) — just enough to soften the background, well short of a frosted-glass look.
- Scrollbars are one shape everywhere a pane shows one (note lists, Shorts, chat, images, the search preview and the editor): a 10px track with a 6px pill 2px off the edge, `--scrollbar-thumb` (10% ink dark, 12% light) and `--scrollbar-thumb-hov` on hover, transparent track. Every other scroller hides its bar. Only the editor's bar fades out at rest, since it is drawn by the editor itself; WebKit doesn't repaint a native bar when its style changes, so native bars stay visible.
- Image lightbox and grid chrome (selection borders, chip backgrounds) use fixed black/white overlays independent of theme, since they draw on top of arbitrary photo content rather than app surfaces.

## Shapes

### Border Radius Scale

| Token            | Value | Use                                                                                         |
| ---------------- | ----- | ------------------------------------------------------------------------------------------- |
| `{rounded.xs}`   | 4px   | Chips, status badges, small icon buttons                                                    |
| `{rounded.sm}`   | 6px   | Inline tags, inline code, prose images                                                      |
| `{rounded.md}`   | 8px   | Default for buttons and form inputs — and the baseline every element gets unless overridden |
| `{rounded.lg}`   | 12px  | List cards, image grid tiles, dropdown menus, the search panel                              |
| `{rounded.xl}`   | 16px  | Dialogs, modals, image lightbox tiles                                                        |
| `{rounded.full}` | 999px | Pills, avatars, unread dots, agent-bot-color swatches                                       |

A surface that holds rounded rows close to its edge takes the row radius plus the gap between them: a menu with `{rounded.sm}` rows, 4px padding and a 1px border is `{rounded.lg}`, so the inner and outer corners run parallel.

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

**`icon-btn-ghost`** — Icon-only action with no fill at rest, 28px square (24px `--sm` modifier for tighter contexts). Fills with `--icon-hov` (ink at 8%) on hover. Used for dismiss/close actions and any icon action that should sit quietly on its surface.

**`btn-ghost`** — The labeled sibling of `icon-btn-ghost`: no fill at rest, `--muted` text, `--icon-hov` and `--fg` on hover. For text actions in a section head beside ghost icon buttons and dropdowns (Images' Select).

**`icon-btn`** — Tonal icon-only action (`--btn-bg`, `{rounded.sm}`) with an `.active` toggle state (`--btn-bg-on` + `--control-active-fg`) for controls that need to show "this is currently on" (pin, compile, source toggle), not just hover feedback. `.icon-btn--lg` (32px) uses `{rounded.md}` to sit flush with inputs.

**`seg`** / **`seg-btn`** — Segmented "pick one of N" control: a `--btn-bg` track with 3px padding and flat buttons inside (inner radius `{rounded.sm}`); `.active` is a raised thumb, `--seg-thumb` (`--control-active-bg-on-soft` on dark, `--canvas` on light) with `--control-active-fg` text. One shared definition backs every picker in the app (graph color mode, settings pickers) so they can't drift apart in size.

**Chips and choice cards** (agent-bot chip, variable chip, effect card, copy button, load-more) follow the same ladder; they are not a separate component family.

### Dialogs & Overlays

**`dialog-card`** (confirm/info dialogs) — Centered floating card over a blurred, dimmed scrim.
- Background `--bg`, 1px `--border-strong` border, rounded `{rounded.xl}`, padding 24px/24px/20px, `--shadow-lg` lift. Title at `{typography.body}` weight 600; message at `{typography.small}`, `--muted`.

**`drawer-panel`** — Edge-docked slide-in panel (note editor, agent chat side panel).
- Background `--bg`, left border only, no radius (flush to 3 viewport edges), 80vw wide (100vw when expanded). Slides in via transform + opacity, not a fade alone, so it reads as spatial motion.

**`search-panel`** — The command/search overlay.
- A launcher window of one fixed size: a 56px input row (`--text-lg`), a 400px list area, a 40px footer, hairlines between them. Empty, no-result and preview states swap inside the list area; the frame never resizes.
- Glass: `--glass-bg` with `--glass-filter`, 1px `--border-strong`, rounded `{rounded.lg}`, `--shadow-lg`. No scrim: the app stays visible and undimmed; a click outside closes. Opens and closes with an 80ms opacity fade only, on the panel itself.
- Rows are 40px: a 24px icon tile (`--btn-bg`, `--bg` when active), the name, the folder in `--sr-dir`, and on the right the type and list timestamp in `--muted`. The active row is an `--srch-av` ink wash. A small label heads each source: an empty query lists `Open tabs` then `Bots`; a query lists up to three name-matched `Bots` above `Notes · N`. Bot rows carry the sidebar avatar in the tile, the description, and unread count in `--accent-text`; ↵ opens that bot's chat, and the preview is its card. Bots appear only in the All scope. Beside the preview, rows show the name only.
- The scope is a filter button at the input's right (accent tint when narrowed); its menu floats over the list rather than replacing it, and `/` opens it. The footer names the scope on the left and what ↵ and tab do on the right.
- Search runs on every keystroke with no debounce; each request aborts the last.

**`settings-modal`** — Same floating-card family as dialogs: `--bg`, `--border-strong`, rounded `{rounded.xl}`, `--shadow-lg`.

### Sections

**`sec`** — Every view in the home list pane (note lists, Tags, Images, Shorts, Graph, Chat) is one `.sec`: a `.home-list-head` (`--toolbar-h`, count + right-aligned controls), one `.sec-body` that scrolls (`20px 24px 40px` padding; `.sec-body--flush` for the graph canvas), an optional floating footer (the chat composer), and its `.empty-state--page` as a direct child. A section overrides only what is truly its own: Images' tighter padding, Shorts' centered 600px column in list view (its composer sits on `--surface-soft` like the entries below it, with a hairline only while focused); in grid view Shorts fills the pane like every note card grid, the same 24px edges and 240px minimum column, with only the composer kept to 600px, Chat's reading column (its head shows the bot's name where other sections show their count). Section DOM caching saves and restores scroll on `.sec-body` (except Chat, which sticks to the newest message), so a new section gets it for free.

**`empty-state`** — A section with nothing yet: a neutral `--surface-soft` tile (hairline border, `--muted` sidebar icon), a `--fg` title, at most one `--muted` sentence, and at most one action, which alone carries the accent. No rings, glow or entry animation. On a page (`.empty-state--page`) the tile always sits at 40% of the window height, whatever head, composer or button surrounds it. Server-rendered sections use the `empty-state` template (home_section_rows.html).

### Lists & Cards

**`list-card`** — Notes list rows. Note rows (Favorites, Folders, and the other lists that share that card) sit on `--card-rest` (ink at 2.5%, half the hover step) with no outline. Title in `{typography.title-list}`, excerpt and time in `--muted` at `{typography.small}`.

**`note-badge`** — Small status marker on a note card. Settled states are plain `--muted` glyphs: the favorite star (hidden in Favorites, where every row is one) and the compiled check. Checklist progress is an accent ring after the title that turns `--muted-soft` once the list is finished.

**`tag-cloud`** — The Tags wall: plain words packed by d3-cloud, no pills. Use shows twice — size (13–32px, 400 below 23px, 500 above) and ink (`--muted` up to `--fg`); hover turns a word `--accent-text`. The box is cropped to the placed words and centered in the section, scrolling once it outgrows it.

**Fence cards** (`editor/src/cm-live/fence-card/`) — receipt, proposal, embed and mindmap share one shape: `--surface-soft`, `{rounded.lg}`, title at 600. The mindmap root node sits on `--surface-2` with `--fg` ink; nothing in a card is a solid `--fg` block.

**`choices`** — Reply suggestions in chat. Short options that fit one line stay `--btn-bg` chips (13px, 400). Anything longer becomes a list card: `--surface-soft`, 4px inset, full-width rows in body type (14px, 400, `--prose-body`) split by hairlines, each led by a muted `reply` arrow (a box for multi-select). Hover is the ink wash; once answered, the pick keeps a `--btn-bg` row and an accent check while the rest drop to `--muted-soft`. Never heavier than the answer above it, and never shaped like the user's own bubble.

**`status-pill`** — ok/err/warn tinted pill: low-opacity tinted background + matching-opacity border + bright text, `{rounded.xs}`, `{typography.caption}`.

### Identity & Media

**`entity-chip`** — Colored dot drawing from the four-color identity palette (cyan, pink, violet, green), theme-invariant. The graph's entity-type nodes default to the violet slot.

**`image-tile`** — Grid tile in the image gallery: `--surface-soft` background, rounded `{rounded.xl}`. Selection state uses a fixed white border + black scrim overlay (not theme tokens), since tiles sit on top of arbitrary photo content.

**`avatar`** — Agent bot identity: a cheerful face on a disc in the bot's `--bot0…7` color. Each color slot has its own face, so a bot keeps one color and one face everywhere. A face is two eyes only, no mouth or accents: each eye a solid circle (three sizes) or ellipse (tall or wide) in `--inverse-ink`, paired same or mixed. Faces sit on one of five spots (the four corners at one shared offset, or centered), sometimes with a slight tilt. No triangles, lines, or sad faces. Each face has a one-shot signature move (squint, hop, look around, wink, curious tilt, wobble, giggle, wave) played on hover of a bot row and when a new reply starts; the only loop is an occasional blink, on the sidebar and the latest reply. Sizes 20 / 28 / 36px.

## Do's and Don'ts

### Do
- Reserve `{colors.accent}` for primary actions, focus rings, selected-state text, and link emphasis only.
- Make every neutral hover an ink wash (`--card-hov` 5%, `--icon-hov` 8%, `--nav-hov`, `--btn-bg-hov`); an accent tint on a grey surface reads as lavender (or orange, under another accent preset).
- Use the three-step surface ladder (bg → surface-soft → surface-2) for hierarchy; don't skip a level.
- Let full-viewport/edge-docked chrome reset to 0 radius; let everything else default to `{rounded.md}` unless a card/panel calls for `{rounded.lg}`/`{rounded.xl}`.
- Keep the identity palette (cyan/pink/violet/green) theme-invariant — it marks "who/what", not "which mode".
- Pair a semantic color's bright running-text tone with its own `-fg` token when using it as a solid fill, never with plain white/black.
- Apply the shared `.icon-btn`/`.icon-btn-ghost`/`.seg`/`.seg-btn` components instead of a new per-page near-copy.
- Build any new button from the `--btn-bg` / `--btn-bg-hov` / `--btn-bg-on` ladder; never add a border to one.
- Secondary detail for a floating menu (e.g. the editor's note Info) swaps into the same panel behind a `‹` back row rather than opening a second popover; label/value rows use `--muted-soft` / `--muted` with mono values.

### Don't
- Don't use the accent color as a section background or card fill.
- Don't add box-shadow to a surface that isn't one of the documented floating layers (dialogs, settings modal, search overlay, drawer popovers).
- Don't introduce a second chromatic brand accent.
- Don't reach for a display-scale font size — this system tops out at `{typography.title-lg}` (22px); there's no marketing-hero tier.
- Don't hardcode a theme-specific hex in component CSS — reference the semantic token (`--bg`, `--fg`, `--accent`, etc.) so it re-themes automatically.
- Don't let a new "selected" state invent its own fill — reuse `--control-active-bg`/`-fg` (or `--control-active-bg-on-soft` inside a `--surface-soft` container, where the plain surface-2 jump is too faint to read); a sidebar nav row uses `--nav-act`.

## Motion

Restrained by default: an animation exists to confirm a user-triggered action, explain a spatial relationship, or give transient feedback — never ambient, never looping, never present while the user is reading or typing. If it's only there to look nice, it doesn't ship.

Four durations, chosen by what's moving, not per-component ad hoc values:

| Token           | Duration | Use                                                                                                        |
| --------------- | -------- | ---------------------------------------------------------------------------------------------------------- |
| `--motion-fast` | 100ms    | Hover/press feedback — background, color, opacity tweaks. No easing curve; too short to perceive one.      |
| `--motion-base` | 160ms    | Small reveal/toggle transforms — chevron rotate, disclosure arrows, drag-state feedback.                   |
| `--motion-slow` | 220ms    | Panel/overlay-scrim fades (drawer, lightbox) — gives a spatial change room to read as motion. |
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
