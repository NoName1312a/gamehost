import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import {
  api,
  type Connectivity,
  type Reachable,
  type Relay,
  type ServerSummary,
  type Template,
} from "../lib/api";
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

// ConnectionPanel implements "direct-first": it shows whether the port was
// auto-forwarded (friends connect directly — nothing extra running), guides a
// one-time manual forward when the router blocks UPnP, lets the user verify
// reachability from the internet, and offers the playit relay as the
// no-router-access fallback.
function ConnectionPanel({ s, relay, onChanged }: { s: ServerSummary; relay?: Relay; onChanged: () => void }) {
  const [conn, setConn] = useState<Connectivity | null>(null);
  const [testing, setTesting] = useState(false);
  const [test, setTest] = useState<Reachable | null>(null);

  useEffect(() => {
    if (!s.running) {
      setConn(null);
      setTest(null);
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
      setTest({ open: false, checked: false, detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setTesting(false);
    }
  }
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
  // at the router (manual forward) even when GameHost's own UPnP mapping failed.
  const reachableConfirmed = !!(test && test.checked && test.open);
  const directOK = (forwarded || reachableConfirmed) && !!addr;

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

      <div className="border-t border-zinc-800 pt-3">
        {s.relayAddress ? (
          <div className="space-y-1">
            <CopyRow label="Or via relay" addr={s.relayAddress} pill={relayPill} />
            <button
              onClick={stopSharing}
              className="text-[11px] text-zinc-500 underline-offset-2 hover:text-zinc-300 hover:underline"
            >
              change / stop relay
            </button>
          </div>
        ) : (
          <details>
            <summary className="cursor-pointer text-xs text-zinc-400 hover:text-zinc-200">
              No router access? Share via a relay instead →
            </summary>
            <div className="mt-2">
              <RelaySetup s={s} relay={relay} port={port} proto={proto} onChanged={onChanged} />
            </div>
          </details>
        )}
      </div>
    </div>
  );
}

// ---- page ------------------------------------------------------------------

export function ServerDetail({
  server,
  template,
  relay,
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
        <div className="mx-auto max-w-3xl space-y-8 px-6 py-6">
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
          <section>
            <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide text-zinc-500">
              Connection &amp; sharing
            </h3>
            <ConnectionPanel s={server} relay={relay} onChanged={onChanged} />
          </section>

          {/* Settings */}
          <section>
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
        </div>
      </div>

      {showConsole && <ServerConsole server={server} onClose={() => setShowConsole(false)} />}
      {showFiles && <FileManager server={server} onClose={() => setShowFiles(false)} />}
    </div>
  );
}
