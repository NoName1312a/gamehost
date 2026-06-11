import { useEffect, useState } from "react";
import { api, type LicenseInfo, type RemoteAccess, type Telemetry, type UserInfo } from "../lib/api";
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

  const [offsiteDir, setOffsiteDir] = useState("");
  const [offBusy, setOffBusy] = useState(false);
  const [offError, setOffError] = useState<string | null>(null);
  const [offSaved, setOffSaved] = useState(false);

  const [telemetry, setTelemetry] = useState<Telemetry | null>(null);
  const [telBusy, setTelBusy] = useState(false);
  const [telError, setTelError] = useState<string | null>(null);

  const [purgeBusy, setPurgeBusy] = useState(false);
  const [purgeMsg, setPurgeMsg] = useState<string | null>(null);

  const [isOwner, setIsOwner] = useState(false);
  const [users, setUsers] = useState<UserInfo[] | null>(null);
  const [nu, setNu] = useState({ username: "", password: "", role: "operator" });
  const [usrBusy, setUsrBusy] = useState(false);
  const [usrError, setUsrError] = useState<string | null>(null);

  function loadUsers() {
    api
      .users()
      .then((r) => setUsers(r.users))
      .catch(() => setUsers(null));
  }

  async function addUser() {
    setUsrBusy(true);
    setUsrError(null);
    try {
      await api.addUser(nu.username.trim(), nu.password, nu.role);
      setNu({ username: "", password: "", role: "operator" });
      loadUsers();
    } catch (e) {
      setUsrError(e instanceof Error ? e.message : String(e));
    } finally {
      setUsrBusy(false);
    }
  }

  async function removeUser(username: string) {
    setUsrError(null);
    try {
      await api.deleteUser(username);
      loadUsers();
    } catch (e) {
      setUsrError(e instanceof Error ? e.message : String(e));
    }
  }

  // Lock background scrolling while the modal is open, so the wheel scrolls the
  // dialog instead of the page behind it.
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, []);

  useEffect(() => {
    appVersion().then(setAppVer).catch(() => setAppVer(null));
    api.remoteAccess().then(setRemote).catch(() => setRemote(null));
    api.license().then(setLicense).catch(() => setLicense(null));
    api
      .offsite()
      .then((o) => setOffsiteDir(o.dir))
      .catch(() => {});
    api.telemetry().then(setTelemetry).catch(() => setTelemetry(null));
    api
      .authStatus()
      .then((s) => {
        if (s.role === "owner") {
          setIsOwner(true);
          loadUsers();
        }
      })
      .catch(() => setIsOwner(false));
  }, []);

  async function saveOffsite() {
    setOffBusy(true);
    setOffError(null);
    setOffSaved(false);
    try {
      const o = await api.setOffsite(offsiteDir.trim());
      setOffsiteDir(o.dir);
      setOffSaved(true);
    } catch (e) {
      setOffError(e instanceof Error ? e.message : String(e));
    } finally {
      setOffBusy(false);
    }
  }

  async function toggleTelemetry() {
    if (!telemetry) return;
    setTelBusy(true);
    setTelError(null);
    try {
      setTelemetry(await api.setTelemetry(!telemetry.enabled));
    } catch (e) {
      setTelError(e instanceof Error ? e.message : String(e));
    } finally {
      setTelBusy(false);
    }
  }

  async function removeAllServers() {
    if (!window.confirm("Remove ALL servers, their saved worlds, and Docker volumes? This cannot be undone.")) {
      return;
    }
    setPurgeBusy(true);
    setPurgeMsg(null);
    try {
      const r = await api.purge();
      setPurgeMsg(`Removed ${r.removed} server${r.removed === 1 ? "" : "s"}. Close settings to refresh.`);
    } catch (e) {
      setPurgeMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setPurgeBusy(false);
    }
  }

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
        className="max-h-[calc(100vh-3rem)] w-full max-w-md overflow-y-auto overscroll-contain rounded-xl border border-zinc-800 bg-zinc-900 p-6"
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
            <h3 className="text-sm font-semibold text-zinc-200">Supporter</h3>
            {license?.pro && (
              <span className="rounded-full px-2 py-0.5 text-[11px] font-medium text-emerald-300 bg-emerald-400/10 ring-1 ring-inset ring-emerald-400/20">
                Supporter
              </span>
            )}
          </div>
          {license?.pro ? (
            <div className="mt-2 space-y-2">
              <p className="text-xs text-zinc-500">
                Thanks for supporting GameNest{license.email ? ` — ${license.email}` : ""}! ❤
              </p>
              <button
                onClick={removeLicense}
                disabled={licBusy}
                className="rounded-lg border border-zinc-700 px-3 py-1.5 text-xs text-zinc-300 hover:bg-zinc-800 disabled:opacity-50"
              >
                {licBusy ? "…" : "Remove key"}
              </button>
            </div>
          ) : (
            <div className="mt-2 space-y-2">
              <p className="text-xs text-zinc-500">
                GameNest is free and open source — every feature is unlocked. A hosted version is coming. Have a
                supporter or hosted key? Redeem it here.
              </p>
              <input
                value={licKey}
                onChange={(e) => setLicKey(e.target.value)}
                placeholder="Paste your key"
                className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 font-mono text-xs text-zinc-100 outline-none focus:border-emerald-500"
              />
              <button
                onClick={activateLicense}
                disabled={licBusy || !licKey.trim()}
                className="w-full rounded-lg bg-emerald-500 px-3 py-2 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
              >
                {licBusy ? "Redeeming…" : "Redeem key"}
              </button>
            </div>
          )}
          {licError && <p className="mt-2 text-xs text-rose-400">{licError}</p>}
        </div>

        <div className="mt-5 border-t border-zinc-800 pt-4">
          <h3 className="text-sm font-semibold text-zinc-200">Off-site backups</h3>
          <p className="mt-1 text-xs text-zinc-500">
            Also copy each backup to a folder — a NAS, external drive, or a synced cloud folder (OneDrive/Dropbox).
          </p>
          <div className="mt-2 flex gap-2">
            <input
              value={offsiteDir}
              onChange={(e) => {
                setOffsiteDir(e.target.value);
                setOffSaved(false);
              }}
              placeholder="e.g. D:\Backups or \\nas\games"
              className="min-w-0 flex-1 rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 font-mono text-xs text-zinc-100 outline-none focus:border-emerald-500"
            />
            <button
              onClick={saveOffsite}
              disabled={offBusy}
              className="shrink-0 rounded-lg border border-zinc-700 px-3 py-2 text-xs text-zinc-200 hover:bg-zinc-800 disabled:opacity-50"
            >
              {offBusy ? "…" : "Save"}
            </button>
          </div>
          {offSaved && <p className="mt-2 text-xs text-emerald-400">Saved.</p>}
          {offError && <p className="mt-2 text-xs text-rose-400">{offError}</p>}
        </div>

        <div className="mt-5 border-t border-zinc-800 pt-4">
          <h3 className="text-sm font-semibold text-zinc-200">Diagnostics</h3>
          <p className="mt-1 text-xs text-zinc-500">
            Help improve GameNest by sharing anonymous crash reports and basic usage (app version, OS).
            Off by default, never includes personal data, and you can turn it off anytime.
          </p>
          {telemetry === null ? (
            <p className="mt-2 text-xs text-zinc-500">…</p>
          ) : (
            <button
              onClick={toggleTelemetry}
              disabled={telBusy}
              className={`mt-3 w-full rounded-lg px-3 py-2 text-sm font-semibold disabled:opacity-50 ${
                telemetry.enabled
                  ? "border border-zinc-700 text-zinc-200 hover:bg-zinc-800"
                  : "bg-emerald-500 text-zinc-950 hover:bg-emerald-400"
              }`}
            >
              {telBusy ? "…" : telemetry.enabled ? "Turn off diagnostics" : "Turn on diagnostics"}
            </button>
          )}
          {telError && <p className="mt-2 text-xs text-rose-400">{telError}</p>}
        </div>

        <div className="mt-5 border-t border-zinc-800 pt-4">
          <h3 className="text-sm font-semibold text-rose-300">Danger zone</h3>
          <p className="mt-1 text-xs text-zinc-500">
            Permanently remove every server, its saved world, and its Docker volume. Your settings and
            license stay. This cannot be undone.
          </p>
          <button
            onClick={removeAllServers}
            disabled={purgeBusy}
            className="mt-3 w-full rounded-lg border border-rose-500/40 px-3 py-2 text-sm font-semibold text-rose-300 hover:bg-rose-500/10 disabled:opacity-50"
          >
            {purgeBusy ? "Removing…" : "Remove all servers"}
          </button>
          {purgeMsg && <p className="mt-2 text-xs text-zinc-400">{purgeMsg}</p>}
        </div>

        {isOwner && (
          <div className="mt-5 border-t border-zinc-800 pt-4">
            <h3 className="text-sm font-semibold text-zinc-200">Users</h3>
            <p className="mt-1 text-xs text-zinc-500">
              Extra accounts that can manage servers over remote access. Only you (owner) can manage them.
            </p>
            <div className="mt-2 space-y-1">
              {(users ?? []).map((u) => (
                <div key={u.username} className="flex items-center justify-between rounded-lg bg-zinc-950 px-3 py-1.5 text-xs">
                  <span className="text-zinc-200">
                    {u.username} <span className="text-zinc-500">· {u.role}</span>
                  </span>
                  {u.role !== "owner" && (
                    <button onClick={() => removeUser(u.username)} className="text-rose-400 hover:text-rose-300">
                      Remove
                    </button>
                  )}
                </div>
              ))}
            </div>
            <div className="mt-2 grid grid-cols-[1fr_1fr_auto_auto] gap-2">
              <input
                value={nu.username}
                onChange={(e) => setNu({ ...nu, username: e.target.value })}
                placeholder="username"
                className="min-w-0 rounded-lg border border-zinc-700 bg-zinc-950 px-2 py-1.5 text-xs text-zinc-100 outline-none focus:border-emerald-500"
              />
              <input
                type="password"
                value={nu.password}
                onChange={(e) => setNu({ ...nu, password: e.target.value })}
                placeholder="password"
                className="min-w-0 rounded-lg border border-zinc-700 bg-zinc-950 px-2 py-1.5 text-xs text-zinc-100 outline-none focus:border-emerald-500"
              />
              <select
                value={nu.role}
                onChange={(e) => setNu({ ...nu, role: e.target.value })}
                className="rounded-lg border border-zinc-700 bg-zinc-950 px-2 py-1.5 text-xs text-zinc-100"
              >
                <option value="operator">operator</option>
                <option value="admin">admin</option>
              </select>
              <button
                onClick={addUser}
                disabled={usrBusy || !nu.username.trim() || nu.password.length < 8}
                className="rounded-lg bg-emerald-500 px-3 py-1.5 text-xs font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
              >
                Add
              </button>
            </div>
            {usrError && <p className="mt-2 text-xs text-rose-400">{usrError}</p>}
          </div>
        )}
      </div>
    </div>
  );
}
