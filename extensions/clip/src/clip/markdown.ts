/**
 * Clip a web page to Markdown.
 *
 * Pipeline:
 *   1. Pre-Readability  — DOM fixes (pre-readability.ts)
 *   2. Readability      — extract article body
 *   3. Turndown         — HTML → Markdown (turndown.ts)
 *   4. Post-Markdown    — fix remaining raw HTML tables (post-markdown.ts)
 */

import { Readability } from "@mozilla/readability";
import { captureCanvasDataUrls, prepareDocument } from "./pre-readability";
import { escapeTags, turndown } from "./turndown";
import { convertRemainingHtmlTables } from "./post-markdown";
import { appendOgFrontMatter, extractOgPreview } from "./og";

// ── Utilities ─────────────────────────────────────────────────────────────────

function isGitHubPage(href: string): boolean {
  try {
    const h = new URL(href).hostname;
    return h === "github.com" || h.endsWith(".github.com");
  } catch {
    return href.includes("github.com");
  }
}

/** Escape a string for YAML double-quoted scalars. */
function yamlDoubleQuoted(s: string): string {
  return (
    '"' +
    s
      .replace(/\\/g, "\\\\")
      .replace(/"/g, '\\"')
      .replace(/\n/g, "\\n")
      .replace(/\r/g, "\\r")
      .replace(/\t/g, "\\t") +
    '"'
  );
}

// ── Public API ────────────────────────────────────────────────────────────────

export interface ClipResult {
  title: string;
  url: string;
  markdown: string;
}

export function clipPageToMarkdown(doc: Document, locationHref: string): ClipResult {
  // Stage 1a — capture canvas data BEFORE cloning (buffers are not copied).
  const canvasUrls = captureCanvasDataUrls(doc);

  const clone = doc.cloneNode(true) as Document;
  const base = doc.baseURI || locationHref;

  // Stage 1b — apply all pre-Readability DOM fixes to the clone.
  prepareDocument(clone, base, canvasUrls, locationHref);

  // Stage 2 — Readability extracts the article body.
  const readabilityOptions: ConstructorParameters<typeof Readability>[1] = {
    charThreshold: 20,
  };
  if (isGitHubPage(locationHref)) {
    readabilityOptions.keepClasses = true; // needed for code-block language detection
  }

  const article = new Readability(clone, readabilityOptions).parse();

  const title =
    article?.title?.trim() ||
    doc.title?.trim() ||
    "Untitled page";

  const html =
    article?.content ||
    doc.body?.innerHTML ||
    "<p>(No article body could be extracted.)</p>";

  // Stage 3 — Turndown converts HTML to Markdown.
  const rawBody = turndown.turndown(html);

  // Stage 4 — Convert any <table> HTML Turndown left unconverted.
  const body = convertRemainingHtmlTables(rawBody, doc);

  // Assemble YAML front matter + body.
  const clipped = new Date().toISOString();
  const og = extractOgPreview(doc, base);
  const fm: string[] = [
    "---",
    `title: ${yamlDoubleQuoted(title)}`,
    `source: ${yamlDoubleQuoted(locationHref)}`,
    `clipped: ${yamlDoubleQuoted(clipped)}`,
  ];
  if (article?.byline?.trim()) {
    fm.push(`author: ${yamlDoubleQuoted(article.byline.trim())}`);
  }
  appendOgFrontMatter(fm, og, yamlDoubleQuoted);
  fm.push("---", "", `# ${escapeTags(title)}`, "", body);
  const markdown = fm.join("\n");

  return { title, url: base, markdown };
}
