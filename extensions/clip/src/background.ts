import { loadSettings, buildVaultWriteUrl } from "./shared/settings.js";
import { resolveNotePath } from "./shared/path-template.js";
import type {
  ClipAndSaveResult,
  ClipResponseMessage,
  ClipExtractResult,
  SaveMarkdownResult,
} from "./shared/messages.js";

const CONTEXT_MENU_ID = "clip-page";

async function ensureOriginPermission(urlStr: string): Promise<void> {
  let origin: string;
  try {
    origin = new URL(urlStr).origin;
  } catch {
    throw new Error("Invalid API base URL.");
  }

  const localhost =
    origin.startsWith("http://127.0.0.1") ||
    origin.startsWith("http://localhost") ||
    origin.startsWith("http://[::1]");

  if (localhost) {
    return;
  }

  const perm = { origins: [`${origin}/*`] } as chrome.permissions.Permissions;
  const has = await chrome.permissions.contains(perm);
  if (has) {
    return;
  }
  const granted = await chrome.permissions.request(perm);
  if (!granted) {
    throw new Error("Permission was denied for this API origin. Allow it in the prompt to continue.");
  }
}

async function saveMarkdownToVault(
  markdown: string,
  pageTitle: string,
): Promise<{ vaultPath: string }> {
  const settings = await loadSettings();
  const vaultPath = resolveNotePath(pageTitle, settings.saveDir);
  const writeUrl = buildVaultWriteUrl(settings.baseUrl);
  await ensureOriginPermission(writeUrl);

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Accept: "application/json",
  };
  if (settings.apiKey.trim()) {
    headers["X-Hylo-API-Key"] = settings.apiKey.trim();
  }

  const res = await fetch(writeUrl, {
    method: "POST",
    headers,
    body: JSON.stringify({ path: "/" + vaultPath, content: markdown }),
    credentials: "omit",
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(
      `Server returned ${res.status}${text ? `: ${text.slice(0, 200)}` : ""}`,
    );
  }
  return { vaultPath };
}

async function performClipAndSave(tabId: number): Promise<ClipAndSaveResult> {
  let tab: chrome.tabs.Tab;
  try {
    tab = await chrome.tabs.get(tabId);
  } catch {
    return { ok: false, error: "Tab not found." };
  }

  const u = tab.url ?? "";
  if (!u.startsWith("http://") && !u.startsWith("https://")) {
    return { ok: false, error: "Only http and https pages can be clipped." };
  }

  try {
    const raw = await chrome.tabs.sendMessage(tabId, { type: "CLIP_REQUEST" });
    const clip = raw as ClipResponseMessage;
    if (!clip || !("ok" in clip)) {
      return { ok: false, error: "Content script did not respond. Reload the page." };
    }
    if (!clip.ok) {
      return { ok: false, error: clip.error || "Extraction failed." };
    }
    const { vaultPath } = await saveMarkdownToVault(clip.markdown, clip.title);
    return { ok: true, vaultPath };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    if (msg.includes("Could not establish connection")) {
      return { ok: false, error: "Reload the page, then try again." };
    }
    return { ok: false, error: msg };
  }
}

function showClipBadge(ok: boolean, detail?: string) {
  if (ok) {
    chrome.action.setBadgeBackgroundColor({ color: "#0a0a0a" });
    chrome.action.setBadgeText({ text: "OK" });
  } else {
    chrome.action.setBadgeBackgroundColor({ color: "#737373" });
    chrome.action.setBadgeText({ text: "!" });
  }
  globalThis.setTimeout(() => {
    chrome.action.setBadgeText({ text: "" });
  }, 3200);
  if (!ok && detail) {
    console.warn("Clip:", detail);
  }
}

function installContextMenu() {
  chrome.contextMenus.removeAll(() => {
    chrome.contextMenus.create({
      id: CONTEXT_MENU_ID,
      title: "Clip page",
      contexts: ["page", "frame", "link"],
      documentUrlPatterns: ["http://*/*", "https://*/*"],
    });
  });
}

installContextMenu();
chrome.runtime.onInstalled.addListener(installContextMenu);

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId !== CONTEXT_MENU_ID || tab?.id == null) {
    return;
  }
  void performClipAndSave(tab.id).then((result) => {
    if (result.ok) {
      console.info("Clip saved:", result.vaultPath);
      showClipBadge(true);
    } else {
      showClipBadge(false, result.error);
    }
  });
});

async function performClipExtract(tabId: number): Promise<ClipExtractResult> {
  let tab: chrome.tabs.Tab;
  try {
    tab = await chrome.tabs.get(tabId);
  } catch {
    return { ok: false, error: "Tab not found." };
  }

  const u = tab.url ?? "";
  if (!u.startsWith("http://") && !u.startsWith("https://")) {
    return { ok: false, error: "Only http and https pages can be clipped." };
  }

  try {
    const raw = await chrome.tabs.sendMessage(tabId, { type: "CLIP_REQUEST" });
    const clip = raw as ClipResponseMessage;
    if (!clip || !("ok" in clip)) {
      return { ok: false, error: "Content script did not respond. Reload the page." };
    }
    if (!clip.ok) {
      return { ok: false, error: clip.error || "Extraction failed." };
    }
    return { ok: true, title: clip.title, url: clip.url, markdown: clip.markdown };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    if (msg.includes("Could not establish connection")) {
      return { ok: false, error: "Reload the page, then try again." };
    }
    return { ok: false, error: msg };
  }
}

async function performSaveMarkdown(
  markdown: string,
  title: string,
): Promise<SaveMarkdownResult> {
  try {
    const { vaultPath } = await saveMarkdownToVault(markdown, title);
    return { ok: true, vaultPath };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return { ok: false, error: msg };
  }
}

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg?.type === "CLIP_AND_SAVE") {
    const tabId = (msg as { tabId: number }).tabId;
    performClipAndSave(tabId).then(sendResponse);
    return true;
  }
  if (msg?.type === "CLIP_EXTRACT") {
    const tabId = (msg as { tabId: number }).tabId;
    performClipExtract(tabId).then(sendResponse);
    return true;
  }
  if (msg?.type === "SAVE_MARKDOWN") {
    const { markdown, title } = msg as { markdown: string; title: string };
    performSaveMarkdown(markdown, title).then(sendResponse);
    return true;
  }
  return false;
});
