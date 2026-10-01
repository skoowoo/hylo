import type { ClipExtractResult, SaveMarkdownResult } from "./shared/messages.js";

// ── State ────────────────────────────────────────────────
let currentMarkdown = "";
let currentTitle = "";

// ── DOM helpers ──────────────────────────────────────────
function $<T extends HTMLElement>(id: string): T {
  return document.getElementById(id) as T;
}

function showView(id: "view-initial" | "view-preview") {
  $("view-initial").classList.toggle("is-hidden", id !== "view-initial");
  $("view-preview").classList.toggle("is-hidden", id !== "view-preview");
}

// ── Clip error (view 1) ───────────────────────────────────
function showClipError(title: string, detail: string) {
  const box = $("clipError");
  box.classList.remove("is-hidden", "feedback--success", "feedback--error");
  box.classList.add("feedback--error");
  $("clipErrorLabel").textContent = title;
  $("clipErrorDetail").textContent = detail;
}

function hideClipError() {
  $("clipError").classList.add("is-hidden");
}

// ── Save feedback (view 2) ────────────────────────────────
function showSaveFeedback(ok: boolean, label: string, detail: string, isPath = false) {
  const box = $("saveFeedback");
  box.classList.remove("is-hidden", "feedback--success", "feedback--error");
  box.classList.add(ok ? "feedback--success" : "feedback--error");
  $("saveFeedbackLabel").textContent = label;
  const d = $("saveFeedbackDetail");
  d.className = "feedback__detail" + (isPath ? " feedback__detail--path" : "");
  d.textContent = detail;
}

function hideSaveFeedback() {
  $("saveFeedback").classList.add("is-hidden");
}

// ── Copy button ───────────────────────────────────────────
function setCopied(yes: boolean) {
  const btn = $<HTMLButtonElement>("copyMd");
  $("copyIcon").classList.toggle("is-hidden", yes);
  $("checkIcon").classList.toggle("is-hidden", !yes);
  btn.classList.toggle("copied", yes);
}

// ── Clip action ───────────────────────────────────────────
async function clipCurrentTab(): Promise<void> {
  hideClipError();

  const btn = $<HTMLButtonElement>("clip");
  btn.disabled = true;
  btn.textContent = "Clipping…";

  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab?.id) {
      showClipError("Could not clip", "No active tab was found.");
      return;
    }

    const u = tab.url ?? "";
    if (!u.startsWith("http://") && !u.startsWith("https://")) {
      showClipError(
        "Could not clip",
        "Only http and https pages are supported.",
      );
      return;
    }

    const result = (await chrome.runtime.sendMessage({
      type: "CLIP_EXTRACT",
      tabId: tab.id,
    })) as ClipExtractResult;

    if (!result.ok) {
      showClipError("Could not clip", result.error);
      return;
    }

    currentMarkdown = result.markdown;
    currentTitle = result.title;

    // Populate preview
    $("mdContent").textContent = currentMarkdown;
    hideSaveFeedback();
    $<HTMLButtonElement>("save").disabled = false;

    showView("view-preview");
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    showClipError("Something went wrong", msg);
  } finally {
    btn.disabled = false;
    btn.textContent = "Clip";
  }
}

// ── Save action ───────────────────────────────────────────
async function saveToVault(): Promise<void> {
  if (!currentMarkdown) return;

  const btn = $<HTMLButtonElement>("save");
  btn.disabled = true;
  btn.textContent = "Saving…";
  hideSaveFeedback();

  try {
    const result = (await chrome.runtime.sendMessage({
      type: "SAVE_MARKDOWN",
      markdown: currentMarkdown,
      title: currentTitle,
    })) as SaveMarkdownResult;

    if (result.ok) {
      showSaveFeedback(true, "Saved", result.vaultPath, true);
      btn.textContent = "Saved";
    } else {
      showSaveFeedback(false, "Could not save", result.error);
      btn.disabled = false;
      btn.textContent = "Add to Hylo";
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    showSaveFeedback(false, "Something went wrong", msg);
    btn.disabled = false;
    btn.textContent = "Add to Hylo";
  }
}

// ── Copy markdown ─────────────────────────────────────────
async function copyMarkdown(): Promise<void> {
  if (!currentMarkdown) return;
  try {
    await navigator.clipboard.writeText(currentMarkdown);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  } catch {
    // Fallback: select text in the pre element
    const pre = $("mdContent");
    const sel = window.getSelection();
    if (sel) {
      const range = document.createRange();
      range.selectNodeContents(pre);
      sel.removeAllRanges();
      sel.addRange(range);
    }
  }
}

// ── Options ───────────────────────────────────────────────
function openOptionsPage(e: Event) {
  e.preventDefault();
  chrome.tabs.create({ url: chrome.runtime.getURL("options.html") });
}

// ── Init ──────────────────────────────────────────────────
document.addEventListener("DOMContentLoaded", () => {
  $("clip").addEventListener("click", () => void clipCurrentTab());
  $("openOptions").addEventListener("click", openOptionsPage);

  $("back").addEventListener("click", () => {
    showView("view-initial");
    // Reset preview state
    currentMarkdown = "";
    currentTitle = "";
    $("mdContent").textContent = "";
    setCopied(false);
    hideSaveFeedback();
    $<HTMLButtonElement>("save").disabled = false;
    $<HTMLButtonElement>("save").textContent = "Add to Hylo";
  });

  $("copyMd").addEventListener("click", () => void copyMarkdown());
  $("save").addEventListener("click", () => void saveToVault());
});
