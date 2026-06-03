import { useCallback, useEffect, useState, type ReactNode } from "react";
import {
  api,
  type Health,
  type Runtime,
  type Template,
  type ServerSummary,
} from "./lib/api";
import { CreateServerModal } from "./components/CreateServerModal";
import { ServerConsole } from "./components/ServerConsole";

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
        alive && setState({ status: "error", error: e instanceof Error ? e.message : String(e) }),
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

const categoryAccent: Record<string, string> = {
  Sandbox: "text-emerald-400 bg-emerald-400/10 ring-emerald-400/20",
  Survival: "text-amber-400 bg-amber-400/10 ring-amber-400/20",
  Shooter: "text-rose-400 bg-rose-400/10 ring-rose-400/20",
};
const accentFor = (c: string) =>
  categoryAccent[c] ?? "text-zinc-400 bg-zinc-400/10 ring-zinc-400/20";

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

function Header({ version }: { version?: string }) {
  return (
    <header className="flex items-center justify-between border-b border-zinc-800 px-6 py-4">
      <div className="flex items-center gap-3">
        <div className="grid h-9 w-9 place-items-center rounded-lg bg-gradient-to-br from-emerald-400 to-cyan-500 text-lg font-black text-zinc-950">
          G
        </div>
        <div>
          <h1 className="text-lg font-semibold leading-none text-zinc-100">GameHost</h1>
          <p className="text-xs text-zinc-500">Self-host game servers, simply</p>
        </div>
      </div>
      {version && <Badge className="bg-zinc-800 text-zinc-400 ring-zinc-700">engine {version}</Badge>}
    </header>
  );
}

function RuntimeBanner({ runtime }: { runtime: Async<Runtime> }) {
  if (runtime.status === "loading") return null;
  const connected = runtime.status === "ok" && runtime.data.connected;

  if (connected) {
    const d = (runtime as { data: Runtime }).data;
    return (
      <div className="mx-6 mt-6 flex items-center gap-3 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-4 py-3">
        <span className="h-2 w-2 rounded-full bg-emerald-400" />
        <p className="text-sm text-emerald-200">
          Docker connected{d.serverVersion ? ` — engine v${d.serverVersion}` : ""}. You're ready to host.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-6 mt-6 rounded-lg border border-amber-500/20 bg-amber-500/5 p-5">
      <div className="flex items-center gap-3">
        <span className="h-2 w-2 rounded-full bg-amber-400" />
        <h2 className="text-sm font-semibold text-amber-100">Set up Docker to start hosting</h2>
      </div>
      <p className="mt-2 text-sm text-amber-200/80">
        GameHost runs each game server in its own container. Docker isn't reachable yet — a one-time
        setup. On Windows, in an <span className="font-semibold">Administrator</span> terminal:
      </p>
      <pre className="mt-3 overflow-x-auto rounded-md bg-zinc-950/70 p-3 text-xs leading-relaxed text-zinc-300 ring-1 ring-zinc-800">
{`wsl --install                                 # reboot if prompted
winget install -e --id Docker.DockerDesktop   # then launch it once`}
      </pre>
    </div>
  );
}

function ServerCard({
  s,
  busy,
  onStart,
  onStop,
  onDelete,
  onConsole,
}: {
  s: ServerSummary;
  busy?: string;
  onStart: () => void;
  onStop: () => void;
  onDelete: () => void;
  onConsole: () => void;
}) {
  const port = s.ports?.[0]?.host;
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-4">
      <div className="flex items-start justify-between gap-2">
        <div>
          <h3 className="font-semibold text-zinc-100">{s.name}</h3>
          <p className="text-xs text-zinc-500">
            {s.game}
            {port ? ` · port ${port}` : ""} · {s.memoryMB} MB
          </p>
        </div>
        <Badge className={statusStyle(s.status)}>{busy ?? s.status}</Badge>
      </div>

      <div className="mt-4 flex flex-wrap gap-2">
        {s.running ? (
          <button
            onClick={onStop}
            disabled={!!busy}
            className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50"
          >
            Stop
          </button>
        ) : (
          <button
            onClick={onStart}
            disabled={!!busy}
            className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
          >
            Start
          </button>
        )}
        <button
          onClick={onConsole}
          className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800"
        >
          Console
        </button>
        <button
          onClick={onDelete}
          disabled={!!busy}
          className="ml-auto rounded-lg border border-rose-500/30 px-3 py-1.5 text-sm text-rose-300 hover:bg-rose-500/10 disabled:opacity-50"
        >
          Delete
        </button>
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
  const templates = useAsync<Template[]>(api.templates, nonce);
  const { servers, refresh } = useServers(health.status === "ok");

  const [modalTemplate, setModalTemplate] = useState<Template | null>(null);
  const [consoleServer, setConsoleServer] = useState<ServerSummary | null>(null);
  const [busy, setBusy] = useState<Record<string, string>>({});
  const [toast, setToast] = useState<string | null>(null);

  async function action(id: string, label: string, fn: () => Promise<unknown>) {
    setBusy((b) => ({ ...b, [id]: label }));
    setToast(null);
    try {
      await fn();
    } catch (e) {
      setToast(e instanceof Error ? e.message : String(e));
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

  const version = health.status === "ok" ? health.data.version : undefined;
  const runtimeReady = runtime.status === "ok" && runtime.data.connected;

  return (
    <div className="mx-auto min-h-screen max-w-6xl">
      <Header version={version} />
      <RuntimeBanner runtime={runtime} />

      {/* Servers */}
      <section className="px-6 pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">
          Your servers
        </h2>
        {servers && servers.length > 0 ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {servers.map((s) => (
              <ServerCard
                key={s.id}
                s={s}
                busy={busy[s.id]}
                onStart={() => action(s.id, "starting…", () => api.startServer(s.id))}
                onStop={() => action(s.id, "stopping…", () => api.stopServer(s.id))}
                onDelete={() => {
                  if (confirm(`Delete "${s.name}" and its data? This can't be undone.`))
                    action(s.id, "deleting…", () => api.deleteServer(s.id));
                }}
                onConsole={() => setConsoleServer(s)}
              />
            ))}
          </div>
        ) : (
          <div className="grid place-items-center rounded-xl border border-dashed border-zinc-800 py-12 text-center">
            <p className="text-zinc-400">No servers yet.</p>
            <p className="mt-1 text-sm text-zinc-600">
              {runtimeReady
                ? "Pick a game from the library below to create your first server."
                : "Finish Docker setup above, then create a server from the library below."}
            </p>
          </div>
        )}
      </section>

      {/* Game library */}
      <section className="px-6 py-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-500">
          Game library
        </h2>
        {templates.status === "loading" && <p className="text-sm text-zinc-500">Loading games…</p>}
        {templates.status === "error" && (
          <p className="text-sm text-rose-400">Couldn't load templates: {templates.error}</p>
        )}
        {templates.status === "ok" && (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {templates.data.map((t) => (
              <div key={t.id} className="flex flex-col rounded-xl border border-zinc-800 bg-zinc-900/40 p-4">
                <div className="flex items-start justify-between gap-2">
                  <h3 className="font-semibold text-zinc-100">{t.name}</h3>
                  <Badge className={accentFor(t.category)}>{t.category}</Badge>
                </div>
                <p className="mt-2 flex-1 text-sm text-zinc-400">{t.description}</p>
                <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                  <Badge className="bg-zinc-800 text-zinc-300 ring-zinc-700">{t.runtime}</Badge>
                  <span>·</span>
                  <span>{t.recMemoryMB} MB rec.</span>
                </div>
                <button
                  onClick={() => setModalTemplate(t)}
                  disabled={!runtimeReady}
                  title={runtimeReady ? "" : "Set up Docker first"}
                  className="mt-4 rounded-lg bg-emerald-500 px-3 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:bg-zinc-800/40 disabled:text-zinc-500"
                >
                  + Add server
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Overlays */}
      {modalTemplate && (
        <CreateServerModal
          template={modalTemplate}
          onClose={() => setModalTemplate(null)}
          onCreated={() => {
            setModalTemplate(null);
            refresh();
          }}
        />
      )}
      {consoleServer && (
        <ServerConsole server={consoleServer} onClose={() => setConsoleServer(null)} />
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
  );
}

function EngineOffline({ error, onRetry }: { error: string; onRetry: () => void }) {
  return (
    <div className="grid min-h-screen place-items-center p-6">
      <div className="max-w-md rounded-xl border border-zinc-800 bg-zinc-900/50 p-6 text-center">
        <div className="mx-auto mb-4 grid h-12 w-12 place-items-center rounded-lg bg-rose-500/10 text-2xl">
          ⚠️
        </div>
        <h1 className="text-lg font-semibold text-zinc-100">Engine not running</h1>
        <p className="mt-2 text-sm text-zinc-400">
          The control panel can't reach the GameHost engine at{" "}
          <code className="rounded bg-zinc-800 px-1 py-0.5 text-zinc-300">{api.base}</code>.
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
