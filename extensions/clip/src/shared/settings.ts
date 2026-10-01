export const STORAGE_KEY = "clipSettings";

export interface ClipSettings {
  /** e.g. http://127.0.0.1:54321 — no trailing slash */
  baseUrl: string;
  /** Optional API Key for authentication (sent as X-Hylo-API-Key header) */
  apiKey: string;
  /** Vault-relative directory to save clipped pages into. Default: "Web Clips" */
  saveDir: string;
}

export const defaultSettings: ClipSettings = {
  baseUrl: "http://127.0.0.1:54321",
  apiKey: "",
  saveDir: "Web Clips",
};

export async function loadSettings(): Promise<ClipSettings> {
  const raw = await chrome.storage.sync.get(STORAGE_KEY);
  const v = raw[STORAGE_KEY] as Partial<ClipSettings> | undefined;
  return { ...defaultSettings, ...v };
}

export async function saveSettings(s: ClipSettings): Promise<void> {
  await chrome.storage.sync.set({ [STORAGE_KEY]: s });
}

/**
 * Normalize and validate a save directory path.
 * Strips leading/trailing slashes; rejects empty, ".", ".." segments.
 */
export function normalizeSaveDir(input: string): string {
  const trimmed = input.trim().replace(/^\/+|\/+$/g, "");
  if (!trimmed) {
    throw new Error("Save directory cannot be empty.");
  }
  for (const part of trimmed.split("/")) {
    if (part === "" || part === "." || part === "..") {
      throw new Error("Save directory contains invalid path segment.");
    }
  }
  return trimmed;
}

/** Validate and normalize base URL; only http(s). */
export function normalizeBaseUrl(input: string): string {
  const t = input.trim().replace(/\/+$/, "");
  let u: URL;
  try {
    u = new URL(t);
  } catch {
    throw new Error("Invalid URL.");
  }
  if (u.protocol !== "http:" && u.protocol !== "https:") {
    throw new Error("Only http and https are supported.");
  }
  if (u.username || u.password) {
    throw new Error("Do not embed credentials in the URL. Use the API Key field instead.");
  }
  return `${u.protocol}//${u.host}`;
}

export function buildVaultWriteUrl(baseUrl: string): string {
  const base = normalizeBaseUrl(baseUrl);
  return `${base}/api/vault/write`;
}
