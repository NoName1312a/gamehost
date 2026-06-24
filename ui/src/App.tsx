import { useCallback, useEffect, useState, type FormEvent } from "react";
import {
  api,
  type AccountStatus,
  type Health,
  type Relay,
  type Runtime,
  type Setup,
  type Template,
  type ServerSummary,
  type TunnelStatus,
} from "./lib/api";
import { ConfigureServerModal } from "./components/ConfigureServerModal";
import { GamePicker } from "./components/GamePicker";
import { groupGames, type GameGroup } from "./lib/games";
import { ServerDetail } from "./components/ServerDetail";
import { SetupWizard } from "./components/SetupWizard";
import { Settings } from "./components/Settings";
import { Sidebar } from "./components/Sidebar";
import { Changelog } from "./components/Changelog";
import { changelog as changelogEntries, entriesSince, type ChangelogEntry } from "./lib/changelog";
import { appVersion, checkForUpdate, type UpdateInfo } from "./lib/updater";
import { friendlyError } from "./lib/errors";
import { Logo } from "./components/icons";
import { Dashboard } from "./components/Dashboard";
import { Account } from "./components/Account";

// ---- tiny async helper -----------------------------------------------------

type Async<T> =
  | { status: "loading" }
  | { status: "ok"; data: T }
  | { status: "error"; error: string };

function useAsync<T>(fn: () => Promise<T>, nonce = 0): Async<T> {
  const [state, setState] = useState<Async<T>>({ status: "loading" });
  useEffect(() => {
    let alive = true;
    // Note: we don't reset to "loading" on re-runs, so polling keeps the last
    // known value on screen instead of flickering.
    fn()
      .then((data) => alive && setState({ status: "ok", data }))
      .catch((e: unknown) =>
        alive && setState({ status: "error", error: friendlyError(e) }),
      );
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nonce]);
  return state;
}

function useServers(enabled: boolean) {
  const [servers, setServers] = useState<ServerSummary[] | null>(null);
  const load = useCallback(async () => {
    try {
      setServers(await api.servers());
    } catch {
      /* keep last known list on transient errors */
    }
  }, []);
  useEffect(() => {
    if (!enabled) return;
    load();
    const t = setInterval(load, 3000);
    return () => clearInterval(t);
  }, [enabled, load]);
  return { servers, refresh: load };
}

// ReadyBanner is shown once Docker is reachable; the guided SetupWizard handles
// the not-yet-ready case.
function ReadyBanner({ runtime }: { runtime: Async<Runtime> }) {
  if (runtime.status !== "ok" || !runtime.data.connected) return null;
  const { serverVersion } = runtime.data;
  return (
    <div className="panel mx-6 mt-6 flex items-center gap-3 px-4 py-3 ring-1 ring-emerald-500/40">
      <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400" />
      <p className="text-sm text-emerald-200">
        Docker connected{serverVersion ? ` — engine v${serverVersion}` : ""}. You're ready to host.
      </p>
    </div>
  );
}

// ---- view/route model ------------------------------------------------------

type View =
  | { kind: "dashboard" }
  | { kind: "server"; id: string }
  | { kind: "settings" }
  | { kind: "account" };

// ---- app -------------------------------------------------------------------

export default function App() {
  const [nonce, setNonce] = useState(0);
  const [tick, setTick] = useState(0);
  const retry = () => setNonce((n) => n + 1);

  // Poll engine + Docker status so the UI auto-updates when Docker comes online.
  useEffect(() => {
    const t = setInterval(() => setTick((n) => n + 1), 4000);
    return () => clearInterval(t);
  }, []);

  const health = useAsync<Health>(api.health, nonce + tick);
  const runtime = useAsync<Runtime>(api.runtime, nonce + tick);
  const setup = useAsync<Setup>(api.setup, nonce + tick);
  const relay = useAsync<Relay>(api.relay, nonce + tick);
  const tunnel = useAsync<TunnelStatus>(api.tunnel, nonce + tick);
  const account = useAsync<AccountStatus>(api.account, nonce + tick);
  const templates = useAsync<Template[]>(api.templates, nonce);
  const { servers, refresh } = useServers(health.status === "ok");

  const [configureGroup, setConfigureGroup] = useState<GameGroup | null>(null);
  const [view, setView] = useState<View>({ kind: "dashboard" });
  const [busy, setBusy] = useState<Record<string, string>>({});
  const [toast, setToast] = useState<string | null>(null);
  const [showPicker, setShowPicker] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [appVer, setAppVer] = useState<string | null>(null);
  const [whatsNew, setWhatsNew] = useState<{ title: string; subtitle?: string; entries: ChangelogEntry[] } | null>(null);

  // Auth gate: loopback (desktop) is always authenticated, so this only ever
  // shows a login for remote browsers. null = unknown (don't gate yet).
  const [authed, setAuthed] = useState<boolean | null>(null);
  const refreshAuth = useCallback(() => {
    api
      .authStatus()
      .then((s) => setAuthed(s.authenticated))
      .catch(() => setAuthed(true)); // status unreachable -> let normal offline handling take over
  }, []);
  useEffect(() => {
    refreshAuth();
  }, [refreshAuth]);

  // Check for a newer desktop-app version once on launch (no-op in a browser).
  useEffect(() => {
    checkForUpdate()
      .then(setUpdateInfo)
      .catch(() => {});
  }, []);

  // After an update, pop "What's New" scoped to what changed since the version
  // the user last ran. First run (no stored version) and unchanged launches stay
  // silent. No-op in a browser (appVersion() is null there).
  useEffect(() => {
    appVersion()
      .then((v) => {
        setAppVer(v);
        if (!v) return;
        const KEY = "gamenest.lastSeenVersion";
        const last = localStorage.getItem(KEY);
        if (last && last !== v) {
          const since = entriesSince(last);
          if (since.length > 0) {
            setWhatsNew({
              title: `Updated to v${v} 🎉`,
              subtitle: `Here's what's new since v${last}.`,
              entries: since,
            });
          }
        }
        localStorage.setItem(KEY, v);
      })
      .catch(() => {});
  }, []);

  async function action(id: string, label: string, fn: () => Promise<unknown>) {
    setBusy((b) => ({ ...b, [id]: label }));
    setToast(null);
    try {
      await fn();
    } catch (e) {
      setToast(friendlyError(e));
    } finally {
      setBusy((b) => {
        const next = { ...b };
        delete next[id];
        return next;
      });
      refresh();
    }
  }

  if (health.status === "error") {
    return <EngineOffline error={health.error} onRetry={retry} />;
  }

  if (authed === false) {
    return (
      <Login
        onLoggedIn={() => {
          setAuthed(true);
          retry();
        }}
      />
    );
  }

  const version = health.status === "ok" ? health.data.version : undefined;
  const runtimeReady = runtime.status === "ok" && runtime.data.connected;

  // The open detail page tracks the live server record from the polled list, so
  // its status/share panels update without re-opening. Closes if it's deleted.
  const activeServerId = view.kind === "server" ? view.id : null;
  const detailServer = activeServerId ? servers?.find((s) => s.id === activeServerId) ?? null : null;

  return (
    <>
      <div className="bg-glow" aria-hidden />
      <div className="grain" aria-hidden />
      <div className="relative z-10 flex h-screen overflow-hidden">
        <Sidebar
          servers={servers}
          activeView={view.kind}
          activeServerId={activeServerId}
          runtimeReady={runtimeReady}
          appVersion={appVer}
          engineVersion={version}
          account={account.status === "ok" ? account.data : undefined}
          onDashboard={() => setView({ kind: "dashboard" })}
          onSelectServer={(id) => setView({ kind: "server", id })}
          onNewServer={() => setShowPicker(true)}
          onOpenSettings={() => setView({ kind: "settings" })}
          onOpenAccount={() => setView({ kind: "account" })}
          onWhatsNew={() => setWhatsNew({ title: "What's New", entries: changelogEntries })}
        />
        <main className="flex-1 overflow-y-auto">
          <div className="mx-auto max-w-5xl">
            {updateInfo && (
              <div className="mx-6 mt-6 flex items-center justify-between gap-3 rounded-2xl border border-sky-500/20 bg-sky-500/5 px-4 py-3">
                <p className="text-sm text-sky-200">
                  GameNest <span className="font-semibold">v{updateInfo.version}</span> is available.
                </p>
                <button onClick={() => setView({ kind: "settings" })} className="rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-sky-400">Update</button>
              </div>
            )}
            {runtime.status !== "loading" &&
              (runtimeReady ? <ReadyBanner runtime={runtime} /> : <SetupWizard setup={setup} onRecheck={retry} />)}
            <Dashboard
              servers={servers}
              runtimeReady={runtimeReady}
              busy={busy}
              onNewServer={() => setShowPicker(true)}
              onOpenServer={(id) => setView({ kind: "server", id })}
              onStart={(id) => action(id, "starting…", () => api.startServer(id))}
              onStop={(id) => action(id, "stopping…", () => api.stopServer(id))}
            />
          </div>
        </main>
      </div>

      {/* Overlays — unchanged, still launched via the state flags */}
      {showPicker && templates.status === "ok" && (
        <GamePicker
          groups={groupGames(templates.data)}
          onPick={(g) => {
            setShowPicker(false);
            setConfigureGroup(g);
          }}
          onClose={() => setShowPicker(false)}
        />
      )}
      {configureGroup && (
        <ConfigureServerModal
          group={configureGroup}
          onClose={() => setConfigureGroup(null)}
          onCreated={() => {
            setConfigureGroup(null);
            refresh();
          }}
        />
      )}
      {view.kind === "server" && detailServer && (
        <ServerDetail
          key={detailServer.id}
          server={detailServer}
          template={templates.status === "ok" ? templates.data.find((t) => t.id === detailServer.templateId) : undefined}
          relay={relay.status === "ok" ? relay.data : undefined}
          tunnel={tunnel.status === "ok" ? tunnel.data : undefined}
          account={account.status === "ok" ? account.data : undefined}
          busy={busy[detailServer.id]}
          onClose={() => setView({ kind: "dashboard" })}
          onChanged={() => {
            refresh();
            retry();
          }}
          onStart={() => action(detailServer.id, "starting…", () => api.startServer(detailServer.id))}
          onStop={() => action(detailServer.id, "stopping…", () => api.stopServer(detailServer.id))}
          onDelete={() => {
            if (confirm(`Delete "${detailServer.name}" and its data? This can't be undone.`)) {
              action(detailServer.id, "deleting…", () => api.deleteServer(detailServer.id));
              setView({ kind: "dashboard" });
            }
          }}
        />
      )}
      {view.kind === "settings" && (
        <Settings
          engineVersion={version}
          initialUpdate={updateInfo}
          onClose={() => setView({ kind: "dashboard" })}
        />
      )}
      {view.kind === "account" && <Account onClose={() => setView({ kind: "dashboard" })} />}
      {whatsNew && (
        <Changelog
          title={whatsNew.title}
          subtitle={whatsNew.subtitle}
          entries={whatsNew.entries}
          onClose={() => setWhatsNew(null)}
        />
      )}

      {toast && (
        <div className="fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-lg border border-rose-500/30 bg-rose-500/15 px-4 py-2 text-sm text-rose-200 shadow-lg">
          {toast}
          <button onClick={() => setToast(null)} className="ml-3 text-rose-400 hover:text-rose-200">
            ✕
          </button>
        </div>
      )}
    </>
  );
}

function Login({ onLoggedIn }: { onLoggedIn: () => void }) {
  const [pw, setPw] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api.login(pw);
      onLoggedIn();
    } catch (e) {
      setErr(friendlyError(e));
      setBusy(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center p-6">
      <form onSubmit={submit} className="w-full max-w-sm rounded-xl border border-zinc-800 bg-zinc-900/50 p-6">
        <div className="mb-4 flex items-center gap-3">
          <Logo className="h-9 w-9 text-emerald-400" />
          <h1 className="font-display text-base font-semibold text-zinc-100">Sign in to GameNest</h1>
        </div>
        <label className="mb-1 block text-xs font-medium text-zinc-400">Password</label>
        <input
          type="password"
          value={pw}
          autoFocus
          onChange={(e) => setPw(e.target.value)}
          className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
        />
        {err && <p className="mt-2 text-xs text-rose-400">{err}</p>}
        <button
          type="submit"
          disabled={busy}
          className="mt-4 w-full rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-50"
        >
          {busy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  );
}

function EngineOffline({ error, onRetry }: { error: string; onRetry: () => void }) {
  return (
    <div className="grid min-h-screen place-items-center p-6">
      <div className="max-w-md rounded-xl border border-zinc-800 bg-zinc-900/50 p-6 text-center">
        <div className="mx-auto mb-4 grid h-12 w-12 place-items-center rounded-lg bg-rose-500/10 text-2xl">
          ⚠️
        </div>
        <h1 className="font-display text-lg font-semibold text-zinc-100">Engine not running</h1>
        <p className="mt-2 text-sm text-zinc-400">
          The control panel can't reach the GameNest engine at{" "}
          <code className="font-mono rounded bg-zinc-800 px-1 py-0.5 text-zinc-300">{api.base}</code>.
        </p>
        <pre className="mt-4 overflow-x-auto rounded-md bg-zinc-950/70 p-3 text-left text-xs text-zinc-300 ring-1 ring-zinc-800">
{`cd engine
go run ./cmd/engine`}
        </pre>
        <p className="mt-2 text-xs text-zinc-600">{error}</p>
        <button
          onClick={onRetry}
          className="mt-4 rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
        >
          Retry
        </button>
      </div>
    </div>
  );
}
