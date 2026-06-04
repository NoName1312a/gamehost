import { useEffect, useState } from "react";
import { appVersion, applyUpdate, checkForUpdate, isDesktop, type UpdateInfo } from "../lib/updater";

export function Settings({
  engineVersion,
  initialUpdate,
  onClose,
}: {
  engineVersion?: string;
  initialUpdate?: UpdateInfo | null;
  onClose: () => void;
}) {
  const [appVer, setAppVer] = useState<string | null>(null);
  const [update, setUpdate] = useState<UpdateInfo | null>(initialUpdate ?? null);
  const [checking, setChecking] = useState(false);
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<string | null>(null);

  useEffect(() => {
    appVersion().then(setAppVer).catch(() => setAppVer(null));
  }, []);

  async function check() {
    setChecking(true);
    setStatus(null);
    try {
      const u = await checkForUpdate();
      setUpdate(u);
      setStatus(u ? null : "You're on the latest version.");
    } catch (e) {
      setStatus("Couldn't check for updates: " + (e instanceof Error ? e.message : String(e)));
    } finally {
      setChecking(false);
    }
  }

  async function apply() {
    setBusy(true);
    setStatus("Downloading update…");
    try {
      await applyUpdate((f) => setStatus(f != null ? `Downloading update… ${Math.round(f * 100)}%` : "Downloading update…"));
      setStatus("Installing and restarting…");
    } catch (e) {
      setStatus("Update failed: " + (e instanceof Error ? e.message : String(e)));
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
      <div
        className="w-full max-w-md rounded-xl border border-zinc-800 bg-zinc-900 p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-zinc-100">Settings</h2>
          <button onClick={onClose} className="rounded-lg px-2 py-1 text-sm text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200">
            ✕
          </button>
        </div>

        <dl className="mt-4 space-y-1 text-sm">
          <div className="flex justify-between">
            <dt className="text-zinc-500">App version</dt>
            <dd className="text-zinc-200">{appVer ?? (isDesktop() ? "…" : "browser")}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-zinc-500">Engine version</dt>
            <dd className="text-zinc-200">{engineVersion ?? "—"}</dd>
          </div>
        </dl>

        <div className="mt-5 border-t border-zinc-800 pt-4">
          <h3 className="text-sm font-semibold text-zinc-200">Updates</h3>
          {isDesktop() ? (
            <div className="mt-2 space-y-2">
              {update ? (
                <button
                  onClick={apply}
                  disabled={busy}
                  className="w-full rounded-lg bg-emerald-500 px-3 py-2 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
                >
                  {busy ? "Updating…" : `Update to v${update.version} & restart`}
                </button>
              ) : (
                <button
                  onClick={check}
                  disabled={checking}
                  className="w-full rounded-lg border border-zinc-700 px-3 py-2 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50"
                >
                  {checking ? "Checking…" : "Check for updates"}
                </button>
              )}
              {status && <p className="text-xs text-zinc-400">{status}</p>}
            </div>
          ) : (
            <p className="mt-2 text-xs text-zinc-500">Updates are managed by the desktop app.</p>
          )}
        </div>
      </div>
    </div>
  );
}
