import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { friendlyError } from "../lib/errors";
import {
  api,
  type AccountStatus,
  type BackupInfo,
  type Connectivity,
  type Reachable,
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

type Tab = "overview" | "console" | "files" | "settings" | "backups" | "mods";

function statusStyle(status: string): string {
  if (status === "running") return "text-emerald-400 bg-emerald-400/10 ring-emerald-400/20";
  if (status === "exited" || status === "created") return "text-amber-400 bg-amber-400/10 ring-amber-400/20";
  if (status === "not created") return "text-zinc-400 bg-zinc-400/10 ring-zinc-400/20";
  return "text-sky-400 bg-sky-400/10 ring-sky-400/20";
}

// ---- sharing (direct-first, GameNest tunnel fallback) ----------------------

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
        <code className="font-mono text-sm text-zinc-200">{addr}</code>
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

// Plus-only: a reserved vanity address. Dormant until GameNest Plus (Stage C);
// shown only when an account is configured + linked.
function VanityControl({ s, account, onChanged }: { s: ServerSummary; account?: AccountStatus; onChanged: () => void }) {
  const [vanityName, setVanityName] = useState("");
  const [vanityBusy, setVanityBusy] = useState(false);
  const [vanityError, setVanityError] = useState<string | null>(null);
  const plusLinked = account?.configured && account?.linked;
  if (!plusLinked) return null;
  async function applyVanity() {
    setVanityBusy(true);
    setVanityError(null);
    try {
      await api.setVanity(s.id, vanityName.trim());
      onChanged();
    } catch (e) {
      setVanityError(friendlyError(e));
    } finally {
      setVanityBusy(false);
    }
  }
  return (
    <div className="mt-2">
      <label className="text-[11px] uppercase tracking-wide text-zinc-500">Reserved address (Plus)</label>
      <div className="mt-1 flex items-center gap-2">
        <input
          value={vanityName}
          onChange={(e) => setVanityName(e.target.value)}
          placeholder={s.tunnelSlug ?? "your-name"}
          className="min-w-0 flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 font-mono text-xs text-zinc-100 outline-none focus:border-emerald-500"
        />
        <button onClick={applyVanity} disabled={vanityBusy || !vanityName.trim()} className="shrink-0 rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50">
          {vanityBusy ? "…" : "Set"}
        </button>
      </div>
      {vanityError && <p className="mt-1 text-xs text-rose-400">{vanityError}</p>}
    </div>
  );
}

// ConnectionPanel implements "direct-first": friends connect directly when the
// port is auto-forwarded (nothing extra running). When direct hosting fails
// (no UPnP forward, or forwarded-but-unreachable / CGNAT), it AUTOMATICALLY
// enables the built-in GameNest tunnel — no toggle, no extra app — and shows a
// single best "Friends connect to" address.
function ConnectionPanel({
  s, tunnel, account, onChanged,
}: {
  s: ServerSummary;
  tunnel?: TunnelStatus;
  account?: AccountStatus;
  onChanged: () => void;
}) {
  const [conn, setConn] = useState<Connectivity | null>(null);
  const [testing, setTesting] = useState(false);
  const [test, setTest] = useState<Reachable | null>(null);
  const autoTested = useRef(false);
  const autoTunneled = useRef(false);

  // Load connectivity while running; reset auto-guards when it stops.
  useEffect(() => {
    if (!s.running) {
      setConn(null); setTest(null);
      autoTested.current = false; autoTunneled.current = false;
      return;
    }
    let alive = true;
    api.connectivity(s.id).then((c) => alive && setConn(c)).catch(() => {});
    return () => { alive = false; };
  }, [s.id, s.running]);

  async function runTest() {
    setTesting(true);
    try {
      const r = await api.testConnectivity(s.id);
      setTest(r);
    } catch (e) {
      setTest({ open: false, checked: false, detail: friendlyError(e) });
    } finally {
      setTesting(false);
    }
  }

  // Auto-run the reachability test once we have a forwarded TCP external address.
  useEffect(() => {
    if (!conn || !conn.running || autoTested.current) return;
    if (conn.forwarded && /tcp/i.test(conn.protocol) && conn.externalAddress) {
      autoTested.current = true;
      runTest();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conn]);

  const directAddr = conn?.externalAddress ?? s.externalAddress;
  const forwarded = conn?.forwarded ?? s.shared;
  const reachableConfirmed = !!(test && test.checked && test.open);
  const testedNotOpen = !!(test && test.checked && !test.open);
  // Direct works: we have an address AND (it's UPnP-forwarded and not proven-closed, or a test confirmed it).
  const directOK = !!directAddr && (reachableConfirmed || (forwarded && !testedNotOpen));
  // Direct has been ruled out: connectivity loaded, not direct-OK, and either no UPnP forward or a test came back closed.
  const directFailed = !!conn && !directOK && (testedNotOpen || !forwarded);

  // Auto-fallback: when direct fails and the tunnel is available, turn it on (once).
  useEffect(() => {
    if (autoTunneled.current) return;
    if (!s.running || !tunnel?.configured || s.useTunnel) return;
    if (!directFailed) return;
    autoTunneled.current = true;
    api.setUseTunnel(s.id, true).then(onChanged).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [s.running, s.id, s.useTunnel, tunnel?.configured, directFailed]);

  if (!s.running) {
    return <p className="text-sm text-zinc-500">Start the server to get a connection address.</p>;
  }

  return (
    <div className="space-y-3">
      {directOK ? (
        <CopyRow label="Friends connect to" addr={directAddr!} pill={reachableConfirmed ? reachablePill : forwardedPill} />
      ) : s.tunnelAddress ? (
        <CopyRow label="Friends connect to" addr={s.tunnelAddress} pill={tunnelPill} />
      ) : tunnel?.configured ? (
        <p className="text-sm text-zinc-400">Setting up a public address through GameNest — your friends will be able to join in a moment…</p>
      ) : directAddr ? (
        <CopyRow label="Friends connect to (unverified)" addr={directAddr} pill={forwardedPill} />
      ) : (
        <p className="text-sm text-zinc-400">Working out how friends can reach you…</p>
      )}

      {/* Verify-direct affordance (kept for the forwarded-but-untested case) */}
      {directAddr && !reachableConfirmed && (
        <button onClick={runTest} disabled={testing} className="rounded-lg border border-zinc-700 px-3 py-1 text-xs text-zinc-300 hover:bg-zinc-800 disabled:opacity-50">
          {testing ? "Checking…" : "Test connection"}
        </button>
      )}
      {test && test.checked && !test.open && (
        <p className="text-xs text-amber-400/80">Direct connection isn't reachable — sharing through GameNest instead.</p>
      )}

      {/* Plus vanity, dormant until Stage C */}
      {s.useTunnel && <VanityControl s={s} account={account} onChanged={onChanged} />}
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
  tunnel,
  account,
  busy,
  onChanged,
  onStart,
  onStop,
  onDelete,
}: {
  server: ServerSummary;
  template?: Template;
  tunnel?: TunnelStatus;
  account?: AccountStatus;
  busy?: string;
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
  const [tab, setTab] = useState<Tab>("overview");
  const showMods = template?.runtime === "java";
  const tabs: { id: Tab; label: string }[] = [
    { id: "overview", label: "Overview" },
    { id: "console", label: "Console" },
    { id: "files", label: "Files" },
    { id: "settings", label: "Settings" },
    { id: "backups", label: "Backups" },
    ...(showMods ? [{ id: "mods" as Tab, label: "Mods" }] : []),
  ];

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
    <div className="flex h-full flex-col">
      {/* Sticky header: icon · name · status · Start/Stop */}
      <header className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-zinc-800/80 bg-zinc-950/70 px-6 py-3 backdrop-blur">
        <div className="flex min-w-0 items-center gap-3">
          <div className={`grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-gradient-to-br ${meta.gradient} text-base`}>
            {meta.glyph}
          </div>
          <div className="min-w-0">
            <h2 className="truncate font-display text-lg font-semibold text-zinc-100">{server.name}</h2>
            <p className="text-xs text-zinc-600">{server.game}</p>
          </div>
          <span
            className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${statusStyle(server.status)}`}
          >
            {status}
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {server.running ? (
            <button onClick={onStop} disabled={!!busy} className={ghostBtn}>Stop</button>
          ) : (
            <button onClick={onStart} disabled={!!busy} className={primaryBtn}>Start</button>
          )}
        </div>
      </header>

      {/* Tab bar */}
      <nav className="flex shrink-0 items-center gap-1 border-b border-zinc-800/80 px-4">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`relative px-3 py-2.5 text-sm font-medium transition ${
              tab === t.id ? "text-zinc-100" : "text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {t.label}
            {tab === t.id && <span className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-emerald-400" />}
          </button>
        ))}
      </nav>

      {/* Tab content */}
      <div className="min-h-0 flex-1">
        {tab === "overview" && (
          <div className="h-full overflow-y-auto px-6 py-6">
            <div className="mx-auto max-w-3xl space-y-5">
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
              <section className="panel p-5">
                <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Connection &amp; sharing</h3>
                <ConnectionPanel s={server} tunnel={tunnel} account={account} onChanged={onChanged} />
              </section>
              <section className="panel p-5">
                <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Resources</h3>
                <ResourcesPanel s={server} />
              </section>
            </div>
          </div>
        )}

        {tab === "console" && <ServerConsole server={server} />}
        {tab === "files" && <FileManager server={server} />}

        {tab === "settings" && (
          <div className="h-full overflow-y-auto px-6 py-6">
            <div className="mx-auto max-w-3xl space-y-5">
              <section className="panel p-5">
                <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Server settings</h3>
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
              <section className="panel p-5">
                <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-rose-300">Danger zone</h3>
                <p className="mb-3 text-sm text-zinc-400">Permanently delete this server and all of its data. This can't be undone.</p>
                <button
                  onClick={onDelete}
                  disabled={!!busy}
                  className="rounded-lg border border-rose-500/30 px-3 py-1.5 text-sm text-rose-300 hover:bg-rose-500/10 disabled:opacity-50"
                >
                  Delete server
                </button>
              </section>
            </div>
          </div>
        )}

        {tab === "backups" && (
          <div className="h-full overflow-y-auto px-6 py-6">
            <div className="mx-auto max-w-3xl space-y-5">
              <section className="panel p-5">
                <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Backups</h3>
                <BackupsPanel s={server} />
              </section>
              <section className="panel p-5">
                <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Schedules</h3>
                <SchedulesPanel s={server} onChanged={onChanged} />
              </section>
            </div>
          </div>
        )}

        {tab === "mods" && showMods && (
          <div className="h-full overflow-y-auto px-6 py-6">
            <div className="mx-auto max-w-3xl space-y-5">
              <section className="panel p-5">
                <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Mods &amp; plugins</h3>
                <ModsPanel s={server} onChanged={onChanged} />
              </section>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
