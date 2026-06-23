import { type ReactNode } from "react";
import { type ServerSummary } from "../lib/api";
import { gameMetaFor } from "../lib/games";

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

export function Dashboard({
  servers, runtimeReady, busy, onNewServer, onOpenServer, onStart, onStop,
}: {
  servers: ServerSummary[] | null;
  runtimeReady: boolean;
  busy: Record<string, string>;
  onNewServer: () => void;
  onOpenServer: (id: string) => void;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
}) {
  return (
    <section className="px-6 py-8">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="font-display text-lg font-semibold text-zinc-100">Your servers</h2>
        <button
          onClick={onNewServer}
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
              onOpen={() => onOpenServer(s.id)}
              onStart={() => onStart(s.id)}
              onStop={() => onStop(s.id)}
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
              onClick={onNewServer}
              className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
            >
              <span className="text-base leading-none">+</span> New server
            </button>
          )}
        </div>
      )}
    </section>
  );
}
