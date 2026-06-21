import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { friendlyError } from "../lib/errors";
import {
  api,
  type AccountStatus,
  type BackupInfo,
  type Connectivity,
  type Reachable,
  type Relay,
  type ServerSummary,
  type Stats,
  type Template,
  type TunnelStatus,
} from "../lib/api";
import { gameMetaFor } from "../lib/games";
import { ServerConsole } from "./ServerConsole";
import { FileManager } from "./FileManager";

// Button styles shared within this page.
const primaryBtn =
  "rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50";
const ghostBtn =
  "rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50";

function statusStyle(status: string): string {
  if (status === "running") return "text-emerald-400 bg-emerald-400/10 ring-emerald-400/20";
  if (status === "exited" || status === "created") return "text-amber-400 bg-amber-400/10 ring-amber-400/20";
  if (status === "not created") return "text-zinc-400 bg-zinc-400/10 ring-zinc-400/20";
  return "text-sky-400 bg-sky-400/10 ring-sky-400/20";
}

// ---- sharing (playit.gg relay) ---------------------------------------------

function CopyRow({ label, addr, pill }: { label: string; addr: string; pill: ReactNode }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(addr);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable */
    }
  }
  return (
    <div className="flex items-center justify-between gap-2">
      <div className="min-w-0">
        <p className="text-[11px] uppercase tracking-wide text-zinc-500">{label}</p>
        <code className="text-sm text-zinc-200">{addr}</code>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {pill}
        <button
          onClick={copy}
          className="rounded border border-zinc-700 px-2 py-0.5 text-[11px] text-zinc-300 hover:bg-zinc-800"
        >
          {copied ? "copied" : "copy"}
        </button>
      </div>
    </div>
  );
}

const forwardedPill = (
  <span className="rounded-full bg-emerald-400/10 px-2 py-0.5 text-[11px] text-emerald-300 ring-1 ring-inset ring-emerald-400/20">
    auto-forwarded
  </span>
);
const relayPill = (
  <span className="rounded-full bg-sky-400/10 px-2 py-0.5 text-[11px] text-sky-300 ring-1 ring-inset ring-sky-400/20">
    via relay
  </span>
);
const reachablePill = (
  <span className="rounded-full bg-emerald-400/10 px-2 py-0.5 text-[11px] text-emerald-300 ring-1 ring-inset ring-emerald-400/20">
    reachable ✓
  </span>
);
const tunnelPill = (
  <span className="rounded-full bg-violet-400/10 px-2 py-0.5 text-[11px] text-violet-300 ring-1 ring-inset ring-violet-400/20">
    via GameNest
  </span>
);

// RelaySetup guides the playit relay fallback: install -> link account -> open
// dashboard to create a tunnel -> paste the address.
function RelaySetup({
  s,
  relay,
  port,
  proto,
  onChanged,
}: {
  s: ServerSummary;
  relay?: Relay;
  port?: number;
  proto: string;
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [addr, setAddr] = useState("");
  const [secret, setSecret] = useState("");

  async function act(action: string) {
    setBusy(true);
    try {
      await api.relayAction(action);
    } catch {
      /* surfaced via status re-poll */
    } finally {
      setBusy(false);
      onChanged();
    }
  }
  async function link() {
    if (!secret.trim()) return;
    setBusy(true);
    try {
      await api.relayLink(secret.trim());
    } catch {
      /* surfaced via status re-poll */
    } finally {
      setBusy(false);
      setSecret("");
      onChanged();
    }
  }
  async function save() {
    if (!addr.trim()) return;
    setBusy(true);
    try {
      await api.setRelayAddress(s.id, addr.trim());
    } finally {
      setBusy(false);
      onChanged();
    }
  }

  return (
    <div className="space-y-2">
      <p className="text-xs text-zinc-400">
        This server isn't auto-forwarded. Use a free playit.gg relay so friends can connect without router setup
        {port ? ` — or forward port ${port} (${proto}) in your router.` : "."}
      </p>

      {!relay?.installed && (
        <button disabled={busy} onClick={() => act("install")} className={primaryBtn}>
          {busy ? "Installing…" : "Install playit relay"}
        </button>
      )}

      {relay?.installed && !relay.linked && (
        <div className="space-y-2">
          <button disabled={busy} onClick={() => act("open-setup")} className={ghostBtn}>
            1. Get a secret key from playit.gg →
          </button>
          <div className="flex gap-2">
            <input
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              placeholder="2. paste your playit secret key"
              className="flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-sm text-zinc-100 outline-none focus:border-emerald-500"
            />
            <button disabled={busy || !secret.trim()} onClick={link} className={primaryBtn}>
              {busy ? "Linking…" : "Link"}
            </button>
          </div>
        </div>
      )}

      {relay?.installed && relay.linked && (
        <div className="space-y-2">
          <button disabled={busy} onClick={() => act("open-dashboard")} className={ghostBtn}>
            Open playit dashboard → create a tunnel
          </button>
          <div className="flex gap-2">
            <input
              value={addr}
              onChange={(e) => setAddr(e.target.value)}
              placeholder="paste your playit address, e.g. name.playit.gg:35211"
              className="flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-sm text-zinc-100 outline-none focus:border-emerald-500"
            />
            <button disabled={busy || !addr.trim()} onClick={save} className={primaryBtn}>
              Save
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// TunnelShare is the built-in GameNest tunnel: one click provisions a public
// <slug>.gn.coderaum.com address with no router setup, no account, and no extra
// app — the recommended sharing path when the tunnel feature is configured. It
// supersedes the playit RelaySetup (kept as an advanced fallback).
function TunnelShare({ s, account, onChanged }: { s: ServerSummary; account?: AccountStatus; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  const [vanityName, setVanityName] = useState("");
  const [vanityBusy, setVanityBusy] = useState(false);
  const [vanityError, setVanityError] = useState<string | null>(null);

  async function toggle(on: boolean) {
    setBusy(true);
    try {
      await api.setUseTunnel(s.id, on);
    } finally {
      setBusy(false);
      onChanged();
    }
  }

  async function applyVanity() {
    if (!vanityName.trim()) return;
    setVanityBusy(true);
    setVanityError(null);
    try {
      await api.setVanity(s.id, vanityName.trim());
      setVanityName("");
      onChanged();
    } catch (e) {
      setVanityError(friendlyError(e));
    } finally {
      setVanityBusy(false);
    }
  }

  const plusLinked = account?.configured && account?.linked;

  if (!s.useTunnel) {
    return (
      <div className="space-y-1">
        <button disabled={busy} onClick={() => toggle(true)} className={primaryBtn}>
          {busy ? "Enabling…" : "Share with friends (no setup)"}
        </button>
        <p className="text-xs text-zinc-500">
          Get a public address through GameNest's relay — no port-forwarding, no account, no extra app.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-1">
      {s.tunnelAddress ? (
        <CopyRow label="Friends connect to" addr={s.tunnelAddress} pill={tunnelPill} />
      ) : (
        <p className="text-xs text-zinc-400">
          {s.running ? "Setting up your public address…" : "Starts sharing once the server is running."}
        </p>
      )}
      {/* Vanity name control — only when Plus account is linked */}
      {plusLinked && (
        <div className="mt-2 space-y-1 border-t border-zinc-800 pt-2">
          <p className="text-[11px] text-zinc-500">Use my GameNest name</p>
          <div className="flex gap-2">
            <input
              value={vanityName}
              onChange={(e) => setVanityName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); applyVanity(); } }}
              placeholder={s.tunnelSlug ?? "your-name"}
              className="min-w-0 flex-1 rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-1.5 font-mono text-xs text-zinc-100 outline-none focus:border-emerald-500"
            />
            <button
              onClick={applyVanity}
              disabled={vanityBusy || !vanityName.trim()}
              className="shrink-0 rounded-lg border border-zinc-700 px-3 py-1.5 text-xs text-zinc-200 hover:bg-zinc-800 disabled:opacity-50"
            >
              {vanityBusy ? "…" : "Set"}
            </button>
          </div>
          {vanityError && <p className="text-[11px] text-rose-400">{vanityError}</p>}
        </div>
      )}
      <button
        onClick={() => toggle(false)}
        disabled={busy}
        className="text-[11px] text-zinc-500 underline-offset-2 hover:text-zinc-300 hover:underline disabled:opacity-50"
      >
        stop sharing
      </button>
    </div>
  );
}

// ConnectionPanel implements "direct-first": it shows whether the port was
// auto-forwarded (friends connect directly — nothing extra running), guides a
// one-time manual forward when the router blocks UPnP, lets the user verify
// reachability from the internet, and offers the playit relay as the
// no-router-access fallback.
function ConnectionPanel({ s, relay, tunnel, account, onChanged }: { s: ServerSummary; relay?: Relay; tunnel?: TunnelStatus; account?: AccountStatus; onChanged: () => void }) {
  const [conn, setConn] = useState<Connectivity | null>(null);
  const [testing, setTesting] = useState(false);
  const [test, setTest] = useState<Reachable | null>(null);
  const autoTested = useRef(false);

  useEffect(() => {
    if (!s.running) {
      setConn(null);
      setTest(null);
      autoTested.current = false;
      return;
    }
    let alive = true;
    api.connectivity(s.id).then((c) => alive && setConn(c)).catch(() => {});
    return () => {
      alive = false;
    };
  }, [s.id, s.running]);

  async function runTest() {
    setTesting(true);
    setTest(null);
    try {
      setTest(await api.testConnectivity(s.id));
    } catch (e) {
      setTest({ open: false, checked: false, detail: friendlyError(e) });
    } finally {
      setTesting(false);
    }
  }

  // Auto-confirm reachability once the port looks forwarded, so users don't have
  // to click "Test connection" — and so a "forwarded but unreachable" state
  // (CGNAT/firewall) surfaces on its own and routes them to the relay.
  useEffect(() => {
    if (!conn || !conn.running || autoTested.current) return;
    if (conn.forwarded && /tcp/i.test(conn.protocol) && conn.externalAddress) {
      autoTested.current = true;
      runTest();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conn]);

  async function stopSharing() {
    try {
      await api.setRelayAddress(s.id, "");
    } finally {
      onChanged();
    }
  }

  if (!s.running) {
    return <p className="text-sm text-zinc-500">Start the server to get a connect address you can share with friends.</p>;
  }

  const port = conn?.port ?? s.ports?.[0]?.host;
  const proto = (conn?.protocol ?? s.ports?.[0]?.protocol ?? "tcp").toUpperCase();
  const addr = conn?.externalAddress ?? s.externalAddress;
  const forwarded = conn?.forwarded ?? s.shared;
  // A successful reachability test is the source of truth: the port can be open
  // at the router (manual forward) even when GameNest's own UPnP mapping failed.
  const reachableConfirmed = !!(test && test.checked && test.open);
  const directOK = (forwarded || reachableConfirmed) && !!addr;
  // Nudge the relay when direct hosting isn't working: either the router never
  // forwarded the port, or it did but an external test couldn't reach it.
  const testedNotOpen = !!(test && test.checked && !test.open);
  const suggestRelay = !s.relayAddress && !reachableConfirmed && (testedNotOpen || !forwarded);

  const testLine = test ? (
    <p
      className={`text-xs ${
        test.checked ? (test.open ? "text-emerald-400" : "text-amber-400") : "text-zinc-500"
      }`}
    >
      {test.checked
        ? test.open
          ? "✓ Reachable from the internet — friends can connect."
          : "✗ Not reachable yet — the port isn't open from outside."
        : test.detail}
    </p>
  ) : null;
  const testBtn = (
    <button onClick={runTest} disabled={testing} className={ghostBtn}>
      {testing ? "Testing…" : "Test connection"}
    </button>
  );

  return (
    <div className="space-y-3 rounded-lg border border-zinc-800 bg-zinc-950/40 p-3">
      {directOK ? (
        <div className="space-y-2">
          <CopyRow label="Friends connect to" addr={addr!} pill={reachableConfirmed ? reachablePill : forwardedPill} />
          <div className="flex flex-wrap items-center gap-3">
            {testBtn}
            {testLine}
          </div>
        </div>
      ) : (
        <div className="space-y-2">
          {addr && (
            <CopyRow
              label="Your address"
              addr={addr}
              pill={<span className="text-[11px] text-amber-300">unverified</span>}
            />
          )}
          <p className="text-xs text-zinc-400">
            Click <span className="text-zinc-200">Test connection</span> to check whether friends can already reach you. If
            not, let them connect <span className="text-zinc-200">directly</span> (nothing extra running) by signing in to
            your router and forwarding{" "}
            <span className="text-zinc-200">
              port {port} ({proto})
            </span>{" "}
            to this PC
            {conn?.localIP ? (
              <>
                {" "}
                at <code className="text-zinc-200">{conn.localIP}</code>
              </>
            ) : null}
            — or use a relay below (no router setup).
          </p>
          <div className="flex flex-wrap items-center gap-3">
            {testBtn}
            {testLine}
          </div>
        </div>
      )}

      <div className="space-y-3 border-t border-zinc-800 pt-3">
        {/* Built-in GameNest tunnel — the recommended no-setup sharing path, when configured. */}
        {tunnel?.configured && <TunnelShare s={s} account={account} onChanged={onChanged} />}

        {/* playit.gg relay: the primary fallback when the built-in tunnel isn't
            configured; otherwise demoted to an advanced option. */}
        {s.relayAddress ? (
          <div className="space-y-1">
            <CopyRow label="Or via playit relay" addr={s.relayAddress} pill={relayPill} />
            <button
              onClick={stopSharing}
              className="text-[11px] text-zinc-500 underline-offset-2 hover:text-zinc-300 hover:underline"
            >
              change / stop relay
            </button>
          </div>
        ) : tunnel?.configured ? (
          <details>
            <summary className="cursor-pointer text-xs text-zinc-400 hover:text-zinc-200">
              Advanced: use a playit.gg relay instead →
            </summary>
            <div className="mt-2">
              <RelaySetup s={s} relay={relay} port={port} proto={proto} onChanged={onChanged} />
            </div>
          </details>
        ) : (
          <>
            {suggestRelay && (
              <p className="mb-2 text-xs text-amber-300">
                {testedNotOpen
                  ? "Friends couldn't reach the port. If the server has finished starting, your ISP may be blocking it (CGNAT) — share via a relay below, no router setup needed."
                  : "Your router didn't open the port automatically. Forward it manually above, or share via a relay below — no router setup needed."}
              </p>
            )}
            <details open={suggestRelay}>
              <summary className="cursor-pointer text-xs text-zinc-400 hover:text-zinc-200">
                No router access? Share via a relay instead →
              </summary>
              <div className="mt-2">
                <RelaySetup s={s} relay={relay} port={port} proto={proto} onChanged={onChanged} />
              </div>
            </details>
          </>
        )}
      </div>
    </div>
  );
}

// ---- backups & schedules ---------------------------------------------------

const fieldCls =
  "w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500";
const labelCls = "mb-1 block text-xs font-medium text-zinc-400";

function fmtBytes(n: number): string {
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + " MB";
  return (n / 1024 / 1024 / 1024).toFixed(2) + " GB";
}
// Backup filenames are engine timestamps like 2026-06-04_11-05-30.tar.gz.
function fmtBackupName(name: string): string {
  const base = name.replace(/\.tar\.gz$/, "");
  const [date, time] = base.split("_");
  return date && time ? `${date} ${time.replace(/-/g, ":")} UTC` : name;
}

function BackupsPanel({ s }: { s: ServerSummary }) {
  const [backups, setBackups] = useState<BackupInfo[] | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const r = await api.listBackups(s.id);
      setBackups(r.backups);
    } catch (e) {
      setError(friendlyError(e));
    }
  }, [s.id]);
  useEffect(() => {
    load();
  }, [load]);

  const fail = (e: unknown) => setError(friendlyError(e));

  async function backup() {
    setBusy("Backing up… (this can take a while)");
    setError(null);
    try {
      await api.createBackup(s.id);
      await load();
    } catch (e) {
      fail(e);
    } finally {
      setBusy(null);
    }
  }
  async function restore(name: string) {
    if (
      !confirm(
        `Restore "${fmtBackupName(name)}"? This replaces the current world/data` +
          (s.running ? " and restarts the server" : "") +
          ". Anything not in this backup is lost.",
      )
    )
      return;
    setBusy("Restoring…");
    setError(null);
    try {
      await api.restoreBackup(s.id, name);
    } catch (e) {
      fail(e);
    } finally {
      setBusy(null);
    }
  }
  async function del(name: string) {
    if (!confirm(`Delete backup "${fmtBackupName(name)}"?`)) return;
    try {
      await api.deleteBackup(s.id, name);
      await load();
    } catch (e) {
      fail(e);
    }
  }

  const sorted = backups ? [...backups].sort((a, b) => b.name.localeCompare(a.name)) : [];

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-3">
        <button onClick={backup} disabled={!!busy} className={primaryBtn}>
          {busy === "Restoring…" ? "Back up now" : busy ?? "Back up now"}
        </button>
        {busy === "Restoring…" && <span className="text-xs text-amber-400">Restoring…</span>}
      </div>
      {error && <p className="text-xs text-rose-400">{error}</p>}
      {backups && sorted.length === 0 && <p className="text-sm text-zinc-500">No backups yet.</p>}
      {sorted.map((b) => (
        <div
          key={b.name}
          className="flex items-center justify-between gap-2 rounded-lg border border-zinc-800 bg-zinc-950/40 px-3 py-2"
        >
          <div className="min-w-0">
            <p className="truncate text-sm text-zinc-200">{fmtBackupName(b.name)}</p>
            <p className="text-[11px] text-zinc-500">{fmtBytes(b.size)}</p>
          </div>
          <div className="flex shrink-0 gap-2">
            <button onClick={() => restore(b.name)} disabled={!!busy} className={ghostBtn}>
              Restore
            </button>
            <button
              onClick={() => del(b.name)}
              disabled={!!busy}
              className="rounded-lg border border-rose-500/30 px-3 py-1.5 text-sm text-rose-300 hover:bg-rose-500/10 disabled:opacity-50"
            >
              Delete
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}

function SchedulesPanel({ s, onChanged }: { s: ServerSummary; onChanged: () => void }) {
  const [restartAt, setRestartAt] = useState(s.restartAt ?? "");
  const [backupAt, setBackupAt] = useState(s.backupAt ?? "");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setSaved(false);
    setError(null);
    try {
      await api.setSchedule(s.id, restartAt, backupAt);
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
      onChanged();
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-zinc-500">Daily, in this PC's local time. Leave a field blank to turn it off.</p>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className={labelCls}>Auto-restart at</label>
          <input type="time" className={fieldCls} value={restartAt} onChange={(e) => setRestartAt(e.target.value)} />
        </div>
        <div>
          <label className={labelCls}>Auto-backup at</label>
          <input type="time" className={fieldCls} value={backupAt} onChange={(e) => setBackupAt(e.target.value)} />
        </div>
      </div>
      {error && <p className="text-xs text-rose-400">{error}</p>}
      <div className="flex items-center gap-3">
        <button onClick={save} disabled={saving} className={primaryBtn}>
          {saving ? "Saving…" : "Save schedule"}
        </button>
        {saved && <span className="text-sm text-emerald-400">Saved ✓</span>}
      </div>
    </div>
  );
}

// ---- mods ------------------------------------------------------------------

function ModsPanel({ s, onChanged }: { s: ServerSummary; onChanged: () => void }) {
  const [text, setText] = useState((s.mods ?? []).join("\n"));
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setSaved(false);
    setError(null);
    try {
      const mods = text
        .split(/[\n,]/)
        .map((m) => m.trim())
        .filter(Boolean);
      await api.setMods(s.id, mods);
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
      onChanged();
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-zinc-500">
        One{" "}
        <a href="https://modrinth.com" target="_blank" rel="noreferrer" className="text-emerald-400 hover:underline">
          Modrinth
        </a>{" "}
        project slug per line (e.g. <span className="font-mono text-zinc-300">sodium</span>,{" "}
        <span className="font-mono text-zinc-300">fabric-api</span>). They're installed automatically on the next start.
      </p>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={4}
        placeholder={"sodium\nlithium\nfabric-api"}
        className={`${fieldCls} font-mono`}
      />
      {error && <p className="text-xs text-rose-400">{error}</p>}
      <div className="flex items-center gap-3">
        <button onClick={save} disabled={saving} className={primaryBtn}>
          {saving ? "Saving…" : "Save mods"}
        </button>
        {saved && <span className="text-sm text-emerald-400">Saved ✓ — restart to apply</span>}
      </div>
    </div>
  );
}

// ---- resources -------------------------------------------------------------

function Sparkline({ data, max, color }: { data: number[]; max: number; color: string }) {
  const w = 240;
  const h = 40;
  const m = Math.max(max, 1);
  const len = data.length;
  const pts = data.map((v, i) => {
    const x = len <= 1 ? 0 : (i / (len - 1)) * w;
    const y = h - (Math.min(Math.max(v, 0), m) / m) * h;
    return [x, y] as const;
  });
  const line = pts.map((p) => `${p[0].toFixed(1)},${p[1].toFixed(1)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-10 w-full" preserveAspectRatio="none">
      {len >= 2 && <polygon points={`0,${h} ${line} ${w},${h}`} fill={color} opacity={0.12} />}
      {len >= 2 && <polyline points={line} fill="none" stroke={color} strokeWidth={1.5} vectorEffect="non-scaling-stroke" />}
    </svg>
  );
}

function ResourcesPanel({ s }: { s: ServerSummary }) {
  const [hist, setHist] = useState<Stats[]>([]);

  useEffect(() => {
    if (!s.running) {
      setHist([]);
      return;
    }
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const tick = async () => {
      try {
        const st = await api.stats(s.id);
        if (alive) setHist((h) => [...h, st].slice(-60));
      } catch {
        /* transient; keep the last samples */
      }
      if (alive) timer = setTimeout(tick, 2500);
    };
    tick();
    return () => {
      alive = false;
      if (timer) clearTimeout(timer);
    };
  }, [s.id, s.running]);

  if (!s.running) {
    return <p className="text-sm text-zinc-500">Start the server to see live CPU and memory usage.</p>;
  }
  const latest = hist[hist.length - 1];
  const cpuMax = Math.max(100, ...hist.map((x) => x.cpuPercent));
  return (
    <div className="space-y-4">
      {hist.length === 0 && <p className="text-xs text-zinc-500">Sampling…</p>}
      <div>
        <div className="mb-1 flex justify-between text-xs">
          <span className="text-zinc-400">CPU</span>
          <span className="text-zinc-200">{latest ? latest.cpuPercent.toFixed(0) : "—"}%</span>
        </div>
        <Sparkline data={hist.map((x) => x.cpuPercent)} max={cpuMax} color="#34d399" />
      </div>
      <div>
        <div className="mb-1 flex justify-between text-xs">
          <span className="text-zinc-400">Memory</span>
          <span className="text-zinc-200">
            {latest ? `${latest.memUsedMB.toFixed(0)} / ${latest.memLimitMB.toFixed(0)} MB (${latest.memPercent.toFixed(0)}%)` : "—"}
          </span>
        </div>
        <Sparkline data={hist.map((x) => x.memPercent)} max={100} color="#38bdf8" />
      </div>
    </div>
  );
}

// ---- page ------------------------------------------------------------------

export function ServerDetail({
  server,
  template,
  relay,
  tunnel,
  account,
  busy,
  onClose,
  onChanged,
  onStart,
  onStop,
  onDelete,
}: {
  server: ServerSummary;
  template?: Template;
  relay?: Relay;
  tunnel?: TunnelStatus;
  account?: AccountStatus;
  busy?: string;
  onClose: () => void;
  onChanged: () => void;
  onStart: () => void;
  onStop: () => void;
  onDelete: () => void;
}) {
  const variables = template?.variables ?? [];

  // Form state is initialised once at mount (App keys this component by server
  // id, so switching servers remounts and re-seeds). Polling updates flow into
  // the `server` prop for the status/share panels without clobbering edits.
  const [name, setName] = useState(server.name);
  const [port, setPort] = useState<number>(server.ports?.[0]?.host ?? 0);
  const [memory, setMemory] = useState<number>(server.memoryMB);
  const [vars, setVars] = useState<Record<string, string>>(() => {
    const v: Record<string, string> = {};
    for (const variable of variables) v[variable.key] = server.env?.[variable.key] ?? variable.default ?? "";
    return v;
  });

  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [showConsole, setShowConsole] = useState(false);
  const [showFiles, setShowFiles] = useState(false);

  async function save(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSaveError(null);
    setSaved(false);
    try {
      await api.updateServer(server.id, { name: name.trim() || server.name, memoryMB: memory, port, variables: vars });
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
      onChanged();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  const field =
    "w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500";
  const label = "mb-1 block text-xs font-medium text-zinc-400";
  const status = busy ?? server.status;
  const meta = gameMetaFor(server.game, server.name);

  return (
    <div className="fixed inset-0 z-40 flex flex-col bg-zinc-950">
      <header className="flex items-center justify-between border-b border-zinc-800 px-6 py-3">
        <div className="flex items-center gap-3">
          <button
            onClick={onClose}
            className="rounded-lg px-2 py-1 text-sm text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
          >
            ← Back
          </button>
          <div className={`grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-gradient-to-br ${meta.gradient} text-base`}>
            {meta.glyph}
          </div>
          <h2 className="font-semibold text-zinc-100">{server.name}</h2>
          <span
            className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${statusStyle(server.status)}`}
          >
            {status}
          </span>
        </div>
        <span className="text-xs text-zinc-600">{server.game}</span>
      </header>

      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-3xl space-y-5 px-6 py-6">
          {server.pulling && (
            <div className="rounded-2xl border border-emerald-500/20 bg-emerald-500/5 px-4 py-3">
              <p className="text-sm text-emerald-200">First start — downloading game files… {server.pullPercent ?? 0}%</p>
              <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-zinc-800">
                <div
                  className="h-full rounded-full bg-emerald-500 transition-all"
                  style={{ width: `${server.pullPercent ?? 0}%` }}
                />
              </div>
              {server.pullStatus && <p className="mt-1 text-xs text-emerald-300/70">{server.pullStatus}</p>}
            </div>
          )}

          {/* Actions */}
          <section className="flex flex-wrap items-center gap-2">
            {server.running ? (
              <button onClick={onStop} disabled={!!busy} className={ghostBtn}>
                Stop
              </button>
            ) : (
              <button onClick={onStart} disabled={!!busy} className={primaryBtn}>
                Start
              </button>
            )}
            <button onClick={() => setShowConsole(true)} className={ghostBtn}>
              Open console
            </button>
            <button onClick={() => setShowFiles(true)} className={ghostBtn}>
              Files
            </button>
            <button
              onClick={onDelete}
              disabled={!!busy}
              className="ml-auto rounded-lg border border-rose-500/30 px-3 py-1.5 text-sm text-rose-300 hover:bg-rose-500/10 disabled:opacity-50"
            >
              Delete server
            </button>
          </section>

          {/* Connection / share online */}
          <section className="rounded-2xl border border-zinc-800 bg-zinc-900/30 p-5">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">
              Connection &amp; sharing
            </h3>
            <ConnectionPanel s={server} relay={relay} tunnel={tunnel} account={account} onChanged={onChanged} />
          </section>

          {/* Resources */}
          <section className="rounded-2xl border border-zinc-800 bg-zinc-900/30 p-5">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">Resources</h3>
            <ResourcesPanel s={server} />
          </section>

          {/* Settings */}
          <section className="rounded-2xl border border-zinc-800 bg-zinc-900/30 p-5">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">Settings</h3>
            <form onSubmit={save} className="space-y-4">
              <div>
                <label htmlFor="sd-name" className={label}>Server name</label>
                <input id="sd-name" className={field} value={name} onChange={(e) => setName(e.target.value)} />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label htmlFor="sd-port" className={label}>Host port</label>
                  <input
                    id="sd-port"
                    className={field}
                    type="number"
                    value={port}
                    onChange={(e) => setPort(Number(e.target.value))}
                  />
                </div>
                <div>
                  <label htmlFor="sd-memory" className={label}>Memory (MB)</label>
                  <input
                    id="sd-memory"
                    className={field}
                    type="number"
                    value={memory}
                    onChange={(e) => setMemory(Number(e.target.value))}
                    min={template?.minMemoryMB}
                  />
                </div>
              </div>

              {variables.map((v) => (
                <div key={v.key}>
                  <label htmlFor={`sd-${v.key}`} className={label}>
                    {v.label}
                    {v.required && <span className="text-rose-400"> *</span>}
                  </label>
                  {v.type === "enum" && v.options ? (
                    <select
                      id={`sd-${v.key}`}
                      className={field}
                      value={vars[v.key] ?? ""}
                      onChange={(e) => setVars((s) => ({ ...s, [v.key]: e.target.value }))}
                    >
                      {v.options.map((o) => (
                        <option key={o} value={o}>
                          {o}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <input
                      id={`sd-${v.key}`}
                      className={field}
                      value={vars[v.key] ?? ""}
                      onChange={(e) => setVars((s) => ({ ...s, [v.key]: e.target.value }))}
                    />
                  )}
                  {v.description && <p className="mt-1 text-xs text-zinc-600">{v.description}</p>}
                </div>
              ))}

              {!template && (
                <p className="text-xs text-amber-400/80">
                  This server's game template isn't loaded, so game-specific options can't be shown — name, port,
                  and memory are still editable.
                </p>
              )}

              {server.running && (
                <p className="text-xs text-amber-400/80">
                  Saving will restart the server to apply changes. Your saved world/config data is kept.
                </p>
              )}

              {saveError && (
                <p className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
                  {saveError}
                </p>
              )}

              <div className="flex items-center gap-3">
                <button type="submit" disabled={saving} className={primaryBtn}>
                  {saving ? "Saving…" : "Save changes"}
                </button>
                {saved && <span className="text-sm text-emerald-400">Saved ✓</span>}
              </div>
            </form>
          </section>

          {/* Backups */}
          <section className="rounded-2xl border border-zinc-800 bg-zinc-900/30 p-5">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">Backups</h3>
            <BackupsPanel s={server} />
          </section>

          {/* Schedules */}
          <section className="rounded-2xl border border-zinc-800 bg-zinc-900/30 p-5">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">Schedules</h3>
            <SchedulesPanel s={server} onChanged={onChanged} />
          </section>

          {/* Mods (Minecraft) */}
          {template?.runtime === "java" && (
            <section className="rounded-2xl border border-zinc-800 bg-zinc-900/30 p-5">
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">Mods &amp; plugins</h3>
              <ModsPanel s={server} onChanged={onChanged} />
            </section>
          )}
        </div>
      </div>

      {showConsole && <ServerConsole server={server} onClose={() => setShowConsole(false)} />}
      {showFiles && <FileManager server={server} onClose={() => setShowFiles(false)} />}
    </div>
  );
}
