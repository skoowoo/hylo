# Periodic Review Recap

Reviews everything created or updated in a recent window — knowledge units, regular notes, short notes — and produces a digest: what got added, what themes are emerging, and any tensions between what's new and what's already established. Default window is the past week; the user may specify a different range.

**Run all steps to completion without stopping. Only speak at the end (Step 5), presenting the recap.**

---

## Inputs

1. **Window** — default `7` days ("past week"). Use a custom range if the user specifies one (`--start` / `--end`, or a different `--latest N`).
2. **Scope** — default: knowledge units + short notes + regular notes, vault-wide. Narrow to a specific directory or domain if the user asks (e.g. "just my journal this week", "just AI knowledge units").

---

## Step 1 — Collect activity in the window

Run in parallel:

```bash
hylo knowledge list --kind knowledge --latest <window>
hylo short list --latest <window> --limit 100
hylo list --latest <window>
```

Scope any of these to a specific directory if the user narrowed the request (e.g. `hylo list /journal --latest <window>`).

If everything comes back empty, report that nothing changed in the window and stop.

---

## Step 2 — Read what changed

**Knowledge units**: for each, `hylo extract tag <unit>` and `hylo extract segment <unit> --head 20` for a quick read of what it's about, without a full load. Note whether `created_at` falls inside the window (new unit) or only `last_compiled_at` does (existing unit that grew this week) — `hylo extract segment` on the frontmatter block or `hylo knowledge read <unit>` if the distinction isn't clear from the preview.

**Short notes**: `hylo short list` returns `content` inline — no extra read needed.

**Regular notes**: skim with `hylo extract outline <note>` or `hylo extract segment <note> --head 20`; full `hylo read` only if a note looks central to the week's theme.

---

## Step 3 — Find themes and tensions

- **Group** knowledge units and notes into 2–5 theme clusters by overlapping tags, domain, or subject matter. Single unrelated items don't need a cluster — list them individually.
- **Flag within-week tensions**: if two units created/updated in the same window take conflicting positions, or a new short note contradicts a unit updated the same week, call it out.
- **Flag tension against established knowledge**: if a new unit's stance conflicts with an older, unrelated-in-time unit and the connection is obvious from what you already read (don't launch a fresh full-vault search for this — only surface what surfaces naturally from Step 2's reads).
- **Note growth signals**: units with a high `compile_count` jump or repeated touches this week — these are threads the author is actively developing.

---

## Step 4 — Nothing to force

If the week's activity is sparse or has no clear theme, say so — don't manufacture clusters or tensions from thin material. A short, honest recap beats a padded one.

---

## Step 5 — Present the recap

Output directly in the response. Never write it to a file. Format:

```markdown
# Weekly Recap: <date range>

## 📈 Activity
- N knowledge units created, M updated · K short notes · J regular notes

## 🧵 Themes
- **<Theme>** — units/notes involved, one line on the throughline

## ⚡ Tensions
- **[[Unit A]]** vs **[[Unit B]]** — what conflicts and why

## 🔭 Worth revisiting
- <threads with fast growth, open questions, or loose ends from this week>
```

Omit any section with nothing in it.
