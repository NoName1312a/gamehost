import { useEffect, useState } from "react";
import { api, type LicenseInfo, type RemoteAccess } from "../lib/api";
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

  const [remote, setRemote] = useState<RemoteAccess | null>(null);
  const [newPw, setNewPw] = useState("");
  const [raBusy, setRaBusy] = useState(false);
  const [raError, setRaError] = useState<string | null>(null);

  const [license, setLicense] = useState<LicenseInfo | null>(null);
  const [licKey, setLicKey] = useState("");
  const [licBusy, setLicBusy] = useState(false);
  const [licError, setLicError] = useState<string | null>(null);

  useEffect(() => {
    appVersion().then(setAppVer).catch(() => setAppVer(null));
    api.remoteAccess().then(setRemote).catch(() => setRemote(null));
    api.license().then(setLicense).catch(() => setLicense(null));
  }, []);

  async function activateLicense() {
    setLicBusy(true);
    setLicError(null);
    try {
      setLicense(await api.setLicense(licKey.trim()));
      setLicKey("");
    } catch (e) {
      setLicError(e instanceof Error ? e.message : String(e));
    } finally {
      setLicBusy(false);
    }
  }

  async function removeLicense() {
    setLicBusy(true);
    setLicError(null);
    try {
      setLicense(await api.clearLicense());
    } catch (e) {
      setLicError(e instanceof Error ? e.message : String(e));
    } finally {
      setLicBusy(false);
    }
  }

  async function savePassword() {
    if (newPw.length < 8) {
      setRaError("Password must be at least 8 characters.");
      return;
    }
    setRaBusy(true);
    setRaError(null);
    try {
      await api.setPassword(newPw);
      setNewPw("");
      setRemote(await api.remoteAccess());
    } catch (e) {
      setRaError(e instanceof Error ? e.message : String(e));
    } finally {
      setRaBusy(false);
    }
  }

  async function toggleRemote() {
    if (!remote) return;
    setRaBusy(true);
    setRaError(null);
    try {
      setRemote(await api.setRemoteAccess(!remote.enabled));
    } catch (e) {
      setRaError(e instanceof Error ? e.message : String(e));
    } finally {
      setRaBusy(false);
    }
  }

  // Best public address to reach the panel remotely (LAN IP fallback handled by the engine).
  const remoteURL = remote?.addr
    ? `https://${remote.externalIP || remote.addr.replace(/^0\.0\.0\.0/, "")}`
    : remote?.externalIP
      ? `https://${remote.externalIP}:${remote.port}`
      : `https://<your-ip>:${remote?.port ?? 8788}`;

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

        <div className="mt-5 border-t border-zinc-800 pt-4">
          <h3 className="text-sm font-semibold text-zinc-200">Remote access</h3>
          <p className="mt-1 text-xs text-zinc-500">
            Manage your servers from another device over HTTPS on your network. Set a password first.
          </p>
          {remote === null ? (
            <p className="mt-2 text-xs text-zinc-500">…</p>
          ) : !remote.hasPassword ? (
            <div className="mt-3 space-y-2">
              <input
                type="password"
                value={newPw}
                onChange={(e) => setNewPw(e.target.value)}
                placeholder="Set a password (min 8 characters)"
                className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
              />
              <button
                onClick={savePassword}
                disabled={raBusy}
                className="w-full rounded-lg bg-emerald-500 px-3 py-2 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
              >
                {raBusy ? "Saving…" : "Set password"}
              </button>
            </div>
          ) : (
            <div className="mt-3 space-y-2">
              <button
                onClick={toggleRemote}
                disabled={raBusy}
                className={`w-full rounded-lg px-3 py-2 text-sm font-semibold disabled:opacity-50 ${
                  remote.enabled
                    ? "border border-zinc-700 text-zinc-200 hover:bg-zinc-800"
                    : "bg-emerald-500 text-zinc-950 hover:bg-emerald-400"
                }`}
              >
                {raBusy ? "…" : remote.enabled ? "Turn off remote access" : "Turn on remote access"}
              </button>
              {remote.enabled && (
                <div className="rounded-lg border border-zinc-800 bg-zinc-950 p-3 text-xs">
                  <p className="text-zinc-400">Reach the panel at:</p>
                  <p className="mt-1 break-all font-mono text-emerald-300">{remoteURL}</p>
                  <p className="mt-2 text-zinc-500">
                    Uses a self-signed certificate — your browser warns once; choose “proceed”/trust it.
                  </p>
                </div>
              )}
            </div>
          )}
          {raError && <p className="mt-2 text-xs text-rose-400">{raError}</p>}
        </div>

        <div className="mt-5 border-t border-zinc-800 pt-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-zinc-200">Plan</h3>
            {license && (
              <span
                className={`rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-inset ${
                  license.pro
                    ? "text-emerald-300 bg-emerald-400/10 ring-emerald-400/20"
                    : "text-zinc-400 bg-zinc-400/10 ring-zinc-400/20"
                }`}
              >
                {license.pro ? "Pro" : "Free"}
              </span>
            )}
          </div>
          {license?.pro ? (
            <div className="mt-2 space-y-2">
              <p className="text-xs text-zinc-500">
                Pro is active{license.email ? ` — ${license.email}` : ""}. Thanks for supporting GameHost!
              </p>
              <button
                onClick={removeLicense}
                disabled={licBusy}
                className="rounded-lg border border-zinc-700 px-3 py-1.5 text-xs text-zinc-300 hover:bg-zinc-800 disabled:opacity-50"
              >
                {licBusy ? "…" : "Remove license"}
              </button>
            </div>
          ) : (
            <div className="mt-2 space-y-2">
              <p className="text-xs text-zinc-500">
                Free runs up to 2 servers at once. Pro unlocks unlimited servers, scheduled backups &amp; restarts, and
                off-site backups.
              </p>
              <input
                value={licKey}
                onChange={(e) => setLicKey(e.target.value)}
                placeholder="Paste your license key"
                className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 font-mono text-xs text-zinc-100 outline-none focus:border-emerald-500"
              />
              <button
                onClick={activateLicense}
                disabled={licBusy || !licKey.trim()}
                className="w-full rounded-lg bg-emerald-500 px-3 py-2 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
              >
                {licBusy ? "Activating…" : "Activate Pro"}
              </button>
            </div>
          )}
          {licError && <p className="mt-2 text-xs text-rose-400">{licError}</p>}
        </div>
      </div>
    </div>
  );
}
