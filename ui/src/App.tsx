import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
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
import { groupGames, gameMetaFor, type GameGroup } from "./lib/games";
import { ServerDetail } from "./components/ServerDetail";
import { SetupWizard } from "./components/SetupWizard";
import { Settings } from "./components/Settings";
import { Menu } from "./components/Menu";
import { Changelog } from "./components/Changelog";
import { changelog as changelogEntries, entriesSince, type ChangelogEntry } from "./lib/changelog";
import { appVersion, checkForUpdate, type UpdateInfo } from "./lib/updater";
import { friendlyError } from "./lib/errors";
import { Logo } from "./components/icons";

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

// ---- presentational helpers ------------------------------------------------

function Badge({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${className}`}
    >
      {children}
    </span>
  );
}

function statusStyle(status: string): string {
  if (status === "running") return "text-emerald-400 bg-emerald-400/10 ring-emerald-400/20";
  if (status === "exited" || status === "created") return "text-amber-400 bg-amber-400/10 ring-amber-400/20";
  if (status === "not created") return "text-zinc-400 bg-zinc-400/10 ring-zinc-400/20";
  return "text-sky-400 bg-sky-400/10 ring-sky-400/20";
}

// ---- sections --------------------------------------------------------------

function Header({ onMenu }: { onMenu: () => void }) {
  return (
    <header className="sticky top-0 z-30 flex items-center gap-3 border-b border-zinc-800/80 bg-zinc-950/80 px-4 py-3.5 backdrop-blur sm:px-6">
      <button
        onClick={onMenu}
        title="Menu"
        aria-label="Open menu"
        className="grid h-9 w-9 place-items-center rounded-xl text-zinc-300 ring-1 ring-inset ring-zinc-800 transition hover:bg-zinc-800/60 hover:text-zinc-100"
      >
        <svg className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
          <path d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>
      <Logo className="h-9 w-9 text-emerald-400" />
      <div>
        <h1 className="font-display text-base font-semibold leading-none text-zinc-100">GameNest</h1>
        <p className="mt-0.5 text-xs text-zinc-500">Self-host game servers, simply</p>
      </div>
    </header>
  );
}

// ReadyBanner is shown once Docker is reachable; the guided SetupWizard handles
// the not-yet-ready case.
function ReadyBanner({ runtime }: { runtime: Async<Runtime> }) {
  if (runtime.status !== "ok" || !runtime.data.connected) return null;
  const { serverVersion } = runtime.data;
  return (
    <div className="panel mx-6 mt-6 flex items-center gap-3 px-4 py-3">
      <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400" />
      <p className="text-sm text-emerald-200">
        Docker connected{serverVersion ? ` — engine v${serverVersion}` : ""}. You're ready to host.
      </p>
    </div>
  );
}

// A server card is a clickable summary; clicking it opens the full server
// detail page (settings, sharing, console). Start/Stop stays on the card as a
// quick action and stops click-through to the card.
function ServerCard({
  s,
  busy,
  onOpen,
  onStart,
  onStop,
}: {
  s: ServerSummary;
  busy?: string;
  onOpen: () => void;
  onStart: () => void;
  onStop: () => void;
}) {
  const port = s.ports?.[0]?.host;
  const meta = gameMetaFor(s.game, s.name);
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
      className="panel group cursor-pointer p-4 transition hover:-translate-y-0.5 hover:border-zinc-600 hover:bg-zinc-900/70 focus:outline-none focus:ring-2 focus:ring-emerald-500/40"
    >
      <div className="flex items-start gap-3">
        <div className={`grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-gradient-to-br ${meta.gradient} text-lg`}>
          {meta.glyph}
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="truncate font-semibold text-zinc-100">{s.name}</h3>
          <p className="text-xs text-zinc-500">
            {port ? `port ${port} · ` : ""}
            {s.memoryMB} MB
          </p>
        </div>
        <Badge className={statusStyle(s.pulling ? "downloading" : s.status)}>
          {busy && !s.pulling ? busy : s.pulling ? `downloading ${s.pullPercent ?? 0}%` : s.status}
        </Badge>
      </div>

      <div className="mt-4 flex items-center gap-2">
        {s.pulling ? (
          <div className="w-full">
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-zinc-800">
              <div
                className="h-full rounded-full bg-emerald-500 transition-all"
                style={{ width: `${s.pullPercent ?? 0}%` }}
              />
            </div>
            <p className="mt-1 text-[11px] text-zinc-400">{s.pullStatus ?? "Downloading game files…"}</p>
          </div>
        ) : (
          <>
            {s.running ? (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onStop();
                }}
                disabled={!!busy}
                className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50"
              >
                Stop
              </button>
            ) : (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onStart();
                }}
                disabled={!!busy}
                className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
              >
                Start
              </button>
            )}
            <span className="ml-auto text-sm text-zinc-500 group-hover:text-zinc-300">Manage →</span>
          </>
        )}
      </div>
    </div>
  );
}

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
  const [detailId, setDetailId] = useState<string | null>(null);
  const [busy, setBusy] = useState<Record<string, string>>({});
  const [toast, setToast] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [showPicker, setShowPicker] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
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
  const detailServer = detailId ? servers?.find((s) => s.id === detailId) ?? null : null;

  return (
    <>
      <div className="bg-glow" aria-hidden />
      <div className="grain" aria-hidden />
      <div className="relative z-10 mx-auto min-h-screen max-w-6xl">
      <Header onMenu={() => setMenuOpen(true)} />
      {updateInfo && (
        <div className="mx-6 mt-6 flex items-center justify-between gap-3 rounded-2xl border border-sky-500/20 bg-sky-500/5 px-4 py-3">
          <p className="text-sm text-sky-200">
            GameNest <span className="font-semibold">v{updateInfo.version}</span> is available.
          </p>
          <button
            onClick={() => setShowSettings(true)}
            className="rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-sky-400"
          >
            Update
          </button>
        </div>
      )}
      {runtime.status !== "loading" &&
        (runtimeReady ? (
          <ReadyBanner runtime={runtime} />
        ) : (
          <SetupWizard setup={setup} onRecheck={retry} />
        ))}

      {/* Servers */}
      <section className="px-6 py-8">
        <div className="mb-4 flex items-center justify-between gap-3">
          <h2 className="font-display text-lg font-semibold text-zinc-100">Your servers</h2>
          <button
            onClick={() => setShowPicker(true)}
            disabled={!runtimeReady}
            title={runtimeReady ? "" : "Finish Docker setup first"}
            className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <span className="text-base leading-none">+</span> New server
          </button>
        </div>
        {servers && servers.length > 0 ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {servers.map((s) => (
              <ServerCard
                key={s.id}
                s={s}
                busy={busy[s.id]}
                onOpen={() => setDetailId(s.id)}
                onStart={() => action(s.id, "starting…", () => api.startServer(s.id))}
                onStop={() => action(s.id, "stopping…", () => api.stopServer(s.id))}
              />
            ))}
          </div>
        ) : (
          <div className="panel grid place-items-center py-14 text-center">
            <div className="mb-3 grid h-12 w-12 place-items-center rounded-2xl bg-zinc-900 text-2xl ring-1 ring-inset ring-zinc-800">
              🎮
            </div>
            <p className="text-zinc-300">No servers yet.</p>
            <p className="mt-1 text-sm text-zinc-600">
              {runtimeReady
                ? "Click “+ New server” to create your first one."
                : "Finish Docker setup above, then add a server."}
            </p>
            {runtimeReady && (
              <button
                onClick={() => setShowPicker(true)}
                className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
              >
                <span className="text-base leading-none">+</span> New server
              </button>
            )}
          </div>
        )}
      </section>

      {/* Overlays */}
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
      {detailServer && (
        <ServerDetail
          key={detailServer.id}
          server={detailServer}
          template={templates.status === "ok" ? templates.data.find((t) => t.id === detailServer.templateId) : undefined}
          relay={relay.status === "ok" ? relay.data : undefined}
          tunnel={tunnel.status === "ok" ? tunnel.data : undefined}
          account={account.status === "ok" ? account.data : undefined}
          busy={busy[detailServer.id]}
          onClose={() => setDetailId(null)}
          onChanged={() => {
            refresh();
            retry();
          }}
          onStart={() => action(detailServer.id, "starting…", () => api.startServer(detailServer.id))}
          onStop={() => action(detailServer.id, "stopping…", () => api.stopServer(detailServer.id))}
          onDelete={() => {
            if (confirm(`Delete "${detailServer.name}" and its data? This can't be undone.`)) {
              action(detailServer.id, "deleting…", () => api.deleteServer(detailServer.id));
              setDetailId(null);
            }
          }}
        />
      )}
      {showSettings && (
        <Settings
          engineVersion={version}
          initialUpdate={updateInfo}
          onClose={() => setShowSettings(false)}
        />
      )}
      {menuOpen && (
        <Menu
          appVersion={appVer}
          engineVersion={version}
          onSettings={() => {
            setMenuOpen(false);
            setShowSettings(true);
          }}
          onWhatsNew={() => {
            setMenuOpen(false);
            setWhatsNew({ title: "What's New", entries: changelogEntries });
          }}
          onClose={() => setMenuOpen(false)}
        />
      )}
      {whatsNew && (
        <Changelog
          title={whatsNew.title}
          subtitle={whatsNew.subtitle}
          entries={whatsNew.entries}
          onClose={() => setWhatsNew(null)}
        />
      )}

      {toast && (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 rounded-lg border border-rose-500/30 bg-rose-500/15 px-4 py-2 text-sm text-rose-200 shadow-lg">
          {toast}
          <button onClick={() => setToast(null)} className="ml-3 text-rose-400 hover:text-rose-200">
            ✕
          </button>
        </div>
      )}
    </div>
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
