// Thin wrapper over the Tauri updater. Everything is guarded by isTauri() and
// uses dynamic import() for the plugin modules, so this file is safe to bundle
// in the plain browser/headless build (the calls just no-op there).
import { isTauri } from "@tauri-apps/api/core";
import type { Update, DownloadEvent } from "@tauri-apps/plugin-updater";

export const isDesktop = () => isTauri();

export async function appVersion(): Promise<string | null> {
  if (!isTauri()) return null;
  const { getVersion } = await import("@tauri-apps/api/app");
  return getVersion();
}

export interface UpdateInfo {
  version: string;
  notes?: string;
}

// The Update handle from the last check, kept so the UI can install it.
let pending: Update | null = null;

export async function checkForUpdate(): Promise<UpdateInfo | null> {
  if (!isTauri()) return null;
  const { check } = await import("@tauri-apps/plugin-updater");
  const u = await check();
  pending = u;
  return u ? { version: u.version, notes: u.body ?? undefined } : null;
}

// applyUpdate downloads + installs the pending update, reporting fractional
// progress (0..1, or null if the total size is unknown), then relaunches.
export async function applyUpdate(onProgress?: (fraction: number | null) => void): Promise<void> {
  if (!pending) throw new Error("no update available to apply");
  let total = 0;
  let got = 0;
  await pending.downloadAndInstall((e: DownloadEvent) => {
    if (e.event === "Started") {
      total = e.data.contentLength ?? 0;
    } else if (e.event === "Progress") {
      got += e.data.chunkLength;
      onProgress?.(total ? got / total : null);
    }
  });
  const { relaunch } = await import("@tauri-apps/plugin-process");
  await relaunch();
}
