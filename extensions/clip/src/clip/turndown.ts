/**
 * Stage 2 — HTML → Markdown conversion via Turndown.
 *
 * Exports a pre-configured TurndownService singleton with custom rules for:
 *   - Table cell inline elements (GFM rows must stay on one line)
 *   - Link text normalisation
 *   - Image preservation
 *   - GitHub code block detection
 *   - Escaping page text that Hylo would read as a #tag
 */

import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";

// ── Internal helpers ─────────────────────────────────────────────────────────

/** GitHub: highlight-source-bash, highlight-source-shell, … */
const GITHUB_HIGHLIGHT_CLASS = /highlight-(?:text|source)-([a-z0-9]+)/i;

function fencedBlock(language: string, code: string, fenceChar: string): string {
  let fenceLen = 3;
  const fenceInCode = new RegExp(`^${fenceChar}{3,}`, "gm");
  let m: RegExpExecArray | null;
  while ((m = fenceInCode.exec(code))) {
    if (m[0].length >= fenceLen) fenceLen = m[0].length + 1;
  }
  const fence = fenceChar.repeat(fenceLen);
  return `\n\n${fence}${language}\n${code.replace(/\n$/, "")}\n${fence}\n\n`;
}

function languageFromGithubWrapper(pre: HTMLElement): string {
  const parent = pre.parentElement;
  const pcls = parent?.getAttribute("class") || "";
  const hm = pcls.match(GITHUB_HIGHLIGHT_CLASS);
  return hm ? hm[1] : "";
}

// Same rules as Hylo's util.InlineTags (internal/util/testdata/tags.json).
const TAG_CANDIDATE = /(^|\s)#([\p{L}\p{N}\p{M}_\-/]+)/gu;

function isTagRun(run: string): boolean {
  if (run.startsWith("/")) return false;
  const cut = run.indexOf("//");
  const tag = (cut >= 0 ? run.slice(0, cut) : run).replace(/\/+$/, "");
  return tag !== "" && !/^\p{Nd}+$/u.test(tag);
}

/**
 * A page's own hashtags would land in the vault's tag list. A text node may
 * follow inline markup ending in a space, so its start counts as a boundary;
 * an extra `\#` there is harmless.
 */
export function escapeTags(text: string): string {
  return text.replace(TAG_CANDIDATE, (m, lead: string, run: string) =>
    isTagRun(run) ? `${lead}\\#${run}` : m,
  );
}

/** Returns true if the node is a direct or nested descendant of a <td>/<th>. */
function isInsideTableCell(node: Node): boolean {
  let p = node.parentNode;
  while (p) {
    const name = (p as Element).nodeName;
    if (name === "TD" || name === "TH") return true;
    if (name === "TABLE") return false;
    p = p.parentNode;
  }
  return false;
}

// ── Builder ───────────────────────────────────────────────────────────────────

function buildTurndown(): TurndownService {
  const td = new TurndownService({
    headingStyle: "atx",
    codeBlockStyle: "fenced",
    bulletListMarker: "-",
    fence: "```",
    preformattedCode: true,
    emDelimiter: "_",
  });

  td.use(gfm);

  // Only text nodes outside code reach escape(), so code keeps its '#'.
  const escapeMarkdown = td.escape.bind(td);
  td.escape = (text: string) => escapeTags(escapeMarkdown(text));

  // ── Table cell inline conversion ─────────────────────────────────────────
  // GFM table rows must fit on a single line. Block-level elements inside
  // <td>/<th> are converted to inline equivalents separated by HTML <br> tags.

  td.addRule("brInTableCell", {
    filter: (node: any) => node.nodeName === "BR" && isInsideTableCell(node),
    replacement: () => "<br>",
  });

  td.addRule("paragraphInTableCell", {
    filter: (node: any) => node.nodeName === "P" && isInsideTableCell(node),
    replacement: (content: any) => {
      const trimmed = content.trim();
      return trimmed ? trimmed + "<br>" : "";
    },
  });

  td.addRule("listItemInTableCell", {
    filter: (node: any) => node.nodeName === "LI" && isInsideTableCell(node),
    replacement: (content: any) => {
      const trimmed = content.replace(/\n+/g, " ").trim();
      return trimmed ? `- ${trimmed}<br>` : "";
    },
  });

  td.addRule("listInTableCell", {
    filter: (node: any) =>
      (node.nodeName === "UL" || node.nodeName === "OL") && isInsideTableCell(node),
    replacement: (content: any) => content.replace(/\n+/g, "").trimEnd(),
  });

  // WeChat 公众号: <td><section><span>…</span></section></td>
  td.addRule("sectionInTableCell", {
    filter: (node: any) => node.nodeName === "SECTION" && isInsideTableCell(node),
    replacement: (content: any) => {
      const trimmed = content.trim();
      return trimmed ? trimmed + "<br>" : "";
    },
  });

  td.addRule("divInTableCell", {
    filter: (node: any) => {
      if (node.nodeName !== "DIV" || !isInsideTableCell(node)) return false;
      const el = node as HTMLElement;
      if (GITHUB_HIGHLIGHT_CLASS.test(el.className || "") && el.querySelector("pre")) return false;
      return true;
    },
    replacement: (content: any) => {
      const trimmed = content.trim();
      return trimmed ? trimmed + "<br>" : "";
    },
  });

  // ── Link text normalisation ───────────────────────────────────────────────
  // GitHub <a> inner text may span multiple DOM nodes separated by newlines,
  // producing broken links like [Create\nan\nAPI\nKey](url).
  td.addRule("links", {
    filter: "a",
    replacement: (content: any, node: any) => {
      const el = node as HTMLElement;
      const href = el.getAttribute("href") || "";
      if (!href) return content;
      const cleanContent = content.replace(/\s+/g, " ").trim();
      const titlePart = el.getAttribute("title") ? ` "${el.getAttribute("title")}"` : "";
      return `[${cleanContent}](${href}${titlePart})`;
    },
  });

  // ── Image preservation ────────────────────────────────────────────────────
  // Ensures <img> tags always convert using the (already-resolved) src.
  td.addRule("images", {
    filter: "img",
    replacement: (_content: any, node: any) => {
      const el = node as HTMLElement;
      const src = el.getAttribute("src") || "";
      if (!src) return "";
      const alt = el.getAttribute("alt") || "";
      const titlePart = el.getAttribute("title") ? ` "${el.getAttribute("title")}"` : "";
      return `![${alt}](${src}${titlePart})`;
    },
  });

  // ── Video preservation ────────────────────────────────────────────────────
  // Markdown has no native video syntax. Render as a poster thumbnail that
  // links to the video URL, or a plain link when there is no poster.
  // When <video> has no src, fall back to a <source> child: prefer one without
  // a media attribute (universal fallback), otherwise use the last one.
  td.addRule("video", {
    filter: "video",
    replacement: (_content: any, node: any) => {
      const el = node as HTMLElement;
      let src = el.getAttribute("src") || "";
      if (!src) {
        const sources = Array.from(el.querySelectorAll("source[src]")) as HTMLElement[];
        const universal = sources.find((s) => !s.getAttribute("media"));
        const chosen = universal ?? sources[sources.length - 1];
        src = chosen?.getAttribute("src") || "";
      }
      if (!src) return "";
      const poster = el.getAttribute("poster") || "";
      if (poster) {
        return `\n\n[![Video](${poster})](${src})\n\n`;
      }
      return `\n\n[Video](${src})\n\n`;
    },
  });

  // ── GitHub code block fixes ───────────────────────────────────────────────
  // GitHub wraps highlighted code in <div class="highlight-source-*"><pre>…</pre></div>.

  // Pass-through the wrapper <div>; let the inner <pre> rule handle content.
  td.addRule("githubHighlightDivPassthrough", {
    filter: (node: any) => {
      if (node.nodeName !== "DIV") return false;
      const el = node as HTMLElement;
      return GITHUB_HIGHLIGHT_CLASS.test(el.className || "") && !!el.querySelector("pre");
    },
    replacement: (content: any) => content,
  });

  // <pre><code class="language-…"> — standard fenced block with language tag.
  td.addRule("preWithCode", {
    filter: (node: any) =>
      node.nodeName === "PRE" && !!(node as HTMLElement).querySelector("code"),
    replacement: (_content: any, node: any, options: any) => {
      const pre = node as HTMLElement;
      const codeEl = pre.querySelector("code")!;
      const codeClass = codeEl.getAttribute("class") || "";
      const langMatch = codeClass.match(/language-(\S+)/);
      const language = langMatch ? langMatch[1] : languageFromGithubWrapper(pre);
      return fencedBlock(language, codeEl.textContent ?? "", options.fence.charAt(0));
    },
  });

  // <pre> without <code> (GitHub syntax-highlighted spans only).
  td.addRule("preWithoutCode", {
    filter: (node: any) =>
      node.nodeName === "PRE" && !(node as HTMLElement).querySelector("code"),
    replacement: (_content: any, node: any, options: any) => {
      const pre = node as HTMLElement;
      return fencedBlock(
        languageFromGithubWrapper(pre),
        pre.textContent ?? "",
        options.fence.charAt(0),
      );
    },
  });

  return td;
}

// ── Singleton export ──────────────────────────────────────────────────────────

export const turndown = buildTurndown();
