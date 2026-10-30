# Compatibility Notes

Special-case rules added to handle non-standard or site-specific HTML.
Each entry lists: the site/scope, the problem, and where the fix lives.

---

## Pre-Readability DOM fixes

### [Zhihu] Invalid nested lists — `fixInvalidNestedLists`

**Problem:** Zhihu emits `<ul><li>Parent</li><ul><li>Child</li></ul></ul>`.
The inner `<ul>` is a direct child of the outer `<ul>`, not inside a `<li>` — invalid HTML.
Browsers render it with CSS indentation; Readability sees a flat list and loses the hierarchy.

**Fix:** Before Readability runs, move every `ul > ul` / `ul > ol` into the preceding `<li>`.
Also removes stray `<br>` elements that Zhihu places directly inside `<ul>` as row separators.

---

### [All sites] `<object type="image/…">` → `<img>` — `convertObjectsToImages`

**Problem:** Readability strips `<object>` tags entirely, losing embedded SVG/image objects.

**Fix:** Convert matching `<object data="…">` to `<img src="…">` before Readability runs.
Relative `data` URLs are resolved to absolute using the page base URL.

---

### [All sites] Relative `<img>` src → absolute — `resolveImageUrls`

**Problem:** Readability may drop or mangle relative image URLs.

**Fix:** Walk all `<img>` elements and resolve relative `src` to absolute before Readability.

---

### [WeChat / lazy-load] `data-src` → `src` — `promoteLazyImageSources`

**Problem:** Pages such as `mp.weixin.qq.com` set `src` to a tiny `data:image/svg+xml`
placeholder and put the real CDN URL in `data-src` (or `data-original`, etc.). The
clipper kept the data URI, so Markdown images did not show real pictures.

**Fix:** Before Readability, if a lazy attribute resolves to `http(s):` and `src` is
empty, a known data-URI placeholder, or `data:image/svg+xml` while a real URL exists,
set `src` to that URL. Runs before `resolveImageUrls`.

---

## Turndown rules (HTML → Markdown)

### [All sites] Table cell inline conversion — `*InTableCell` rules

**Problem:** GFM table rows must fit on one line. Sites that put `<ul>`, `<p>`, or `<br>`
inside `<td>`/`<th>` produce newlines that break the row.

**Fix:** Rules override block-level elements inside table cells:
- `<br>` → literal `<br>` HTML
- `<p>` → `content<br>`
- `<section>` / `<div>` (non-GitHub-highlight) → `content<br>` — 微信公众号等用 `<td><section><span>…</span></section></td>`，默认会把表格行拆断
- `<li>` → `- item<br>`
- `<ul>`/`<ol>` → strip container, keep inlined items

---

### [All sites] Page hashtags — `escapeTags`

**Problem:** Hylo reads Obsidian-style `#tag` in a note body as a vault tag, so a clipped
page's own hashtags (tweets, blog tag lines) would flood the tag list.

**Fix:** Turndown's `escape` (text outside code only) and the `# Title` line write `\#tag`,
which renders as `#tag` but isn't one. `C#`, `#42` and code are left alone.

---

### [All sites] Link text normalisation — `links`

**Problem:** Some sites (e.g. GitHub) wrap link text across multiple DOM nodes with newlines,
producing broken Markdown like `[Create\nan\nAPI\nKey](url)`.

**Fix:** Collapse all whitespace/newlines in link text to a single space.

---

### [GitHub] Code block fixes — `githubHighlightDivPassthrough`, `preWithCode`, `preWithoutCode`

**Problem:** GitHub wraps `<pre>` in `<div class="highlight-source-*">`. The gfm plugin's
`highlightedCodeBlock` rule assumes `div.firstChild === pre`, which fails here. Also, some
`<pre>` blocks contain only `<span>` nodes (no `<code>`), which the default rule misses.

**Fix:** Three rules cover all GitHub code shapes:
1. Pass-through the wrapper `<div>`, letting the `<pre>` rule handle content.
2. `<pre><code class="language-…">` → fenced block with language tag.
3. `<pre>` without `<code>` → fenced block, language from parent `<div>` class.
