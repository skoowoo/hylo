# Review

Looks back. Two kinds:

| Kind              | The user asks                                                         | Section below    |
| ----------------- | --------------------------------------------------------------------- | ---------------- |
| **Periodic**      | "weekly review", "回顾一下这周/这个月": everything from a past stretch | Periodic review  |
| **Random look back** | "随便翻翻", "random look back", "anything I've forgotten": history at random | Random look back |

**Run all steps to completion without stopping. Only speak at the end.** Say it in the conversation; never write a file.

---

## Periodic review

Everything created or updated in a recent window — knowledge units, regular notes, short notes — as a digest: what got added, which themes are emerging, and where the new conflicts with the established. Default window is the past 7 days; "this month" is 30. Use the user's range if they give one (`start` / `end`, or a different `latest`). Narrow to a folder or domain if they ask.

### Step 1 — Collect activity in the window

Run in parallel:

```
knowledge_list {"kind": "knowledge", "latest": <window>}
short_list {"latest": <window>, "limit": 100}
list {"latest": <window>}
```

Scope any of these to a directory if the user narrowed the request. If everything comes back empty, say nothing changed in the window and stop.

### Step 2 — Read what changed

- **Knowledge units**: `extract_tag` and `extract_segment` with `head: 20` for what each is about and its frontmatter: `perspective`, and whether `created_at` falls inside the window (new) or only `last_compiled_at` does (grew this week).
- **Short notes**: `short_list` returns `content` inline.
- **Regular notes**: `extract_outline` or `extract_segment` with `head: 20`; a full `read` only for a note central to the week.

### Step 3 — Find themes and tensions

- **Group** units and notes into 2–5 themes by shared tags, domain or subject. Unrelated single items don't need a theme.
- **Tensions within the window**: two things from the same window taking conflicting positions, or a short note contradicting a unit updated the same week.
- **Tensions with older knowledge**: only those that surface from what you already read; don't launch a fresh vault-wide search.
- **Growth**: units touched repeatedly this window are threads the user is actively developing.
- Keep the user's own writing apart from what they read: "you wrote" vs "you read".

If the window is thin or has no clear theme, say so. A short honest review beats a padded one.

### Step 4 — Answer

Omit any section with nothing in it.

```markdown
# Review: <date range>

## 📈 Activity
- N knowledge units created, M updated · K short notes · J regular notes

## 🧵 Themes
- **<Theme>** — units/notes involved, one line on the throughline

## ⚡ Tensions
- **[[Unit A]]** vs **[[Unit B]]** — what conflicts and why

## 🔭 Worth revisiting
- <fast-growing threads, open questions, loose ends>
```

---

## Random look back

Surface a few things from the past they've likely forgotten, chosen because they are interesting now, not just old.

### Step 1 — Draw candidates

About 20 notes and units not touched for 90 days or more, at random:

```bash
find . -name '*.md' -not -path './.*' -not -path './_knowledge/_indexes/*' -not -path './_memory/*' -not -path './_bots/*' -not -path './_setup/*' -mtime +90 | sort -R | head -20
```

Without a shell: pick two random months from the past few years and `list {"start": "<YYYY-MM-01>", "end": "<next month>", "limit": 15}` for each; try another month when one comes back empty.

### Step 2 — Keep what's worth it

Skim each candidate (`extract_segment` with `head: 20`, or the short note itself) and keep the 3–5 most interesting: a sharp idea, a strong claim, a question still open, a plan never followed up, something that ties to what they care about now (`/_memory/_goals.md`, `_state.md`, their latest short notes). Drop empty notes, logs and todo lists.

### Step 3 — Answer

```markdown
# From your past

## <a short name for it>
[[note]] · YYYY-MM-DD
<what it says, in their words where possible, 1–2 sentences>
<why it's worth a look now, one line>
```

One block per find. End with `choices`, such as: draw again, dig into one of these (name it), find everything related to it.

