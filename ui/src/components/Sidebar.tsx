import { type ServerSummary, type AccountStatus } from "../lib/api";
import { gameMetaFor } from "../lib/games";
import { Logo } from "./icons";

const GITHUB_URL = "https://github.com/NoName1312a/gamehost";
const DISCORD_URL = "https://discord.gg/gamenest";

function dotColor(s: ServerSummary): string {
  if (s.pulling) return "bg-sky-400";
  if (s.running) return "bg-emerald-400";
  if (s.status === "exited" || s.status === "created") return "bg-amber-400";
  return "bg-zinc-600";
}

export function Sidebar({
  servers, activeServerId, runtimeReady, appVersion, engineVersion, account,
  onDashboard, onSelectServer, onNewServer, onOpenSettings, onOpenAccount, onWhatsNew,
}: {
  servers: ServerSummary[] | null;
  activeServerId: string | null;
  runtimeReady: boolean;
  appVersion: string | null;
  engineVersion?: string;
  account?: AccountStatus;
  onDashboard: () => void;
  onSelectServer: (id: string) => void;
  onNewServer: () => void;
  onOpenSettings: () => void;
  onOpenAccount: () => void;
  onWhatsNew: () => void;
}) {
  const dashboardActive = activeServerId === null;
  return (
    <aside className="flex h-full w-64 shrink-0 flex-col border-r border-zinc-800/80 bg-zinc-950/60 backdrop-blur">
      {/* Brand */}
      <button
        onClick={onDashboard}
        className="flex items-center gap-2.5 px-4 py-4 text-left"
      >
        <Logo className="h-8 w-8 text-emerald-400" />
        <span className="font-display text-base font-semibold text-zinc-100">GameNest</span>
      </button>

      {/* Primary nav */}
      <nav className="px-2">
        <NavItem active={dashboardActive} onClick={onDashboard} label="Dashboard" icon="&#x25A6;" />
      </nav>

      {/* Servers */}
      <div className="mt-4 flex min-h-0 flex-1 flex-col px-2">
        <div className="flex items-center justify-between px-2 pb-1">
          <span className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">Servers</span>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {servers && servers.length > 0 ? (
            servers.map((s) => {
              const meta = gameMetaFor(s.game, s.name);
              const active = s.id === activeServerId;
              return (
                <button
                  key={s.id}
                  onClick={() => onSelectServer(s.id)}
                  className={`group flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-sm transition ${
                    active ? "bg-emerald-500/10 text-emerald-200 ring-1 ring-inset ring-emerald-500/30" : "text-zinc-300 hover:bg-zinc-800/60"
                  }`}
                >
                  <span className={`grid h-6 w-6 shrink-0 place-items-center rounded-md bg-gradient-to-br ${meta.gradient} text-xs`}>{meta.glyph}</span>
                  <span className="min-w-0 flex-1 truncate">{s.name}</span>
                  <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${dotColor(s)}`} />
                </button>
              );
            })
          ) : (
            <p className="px-2 py-1 text-xs text-zinc-600">No servers yet.</p>
          )}
        </div>
        <button
          onClick={onNewServer}
          disabled={!runtimeReady}
          title={runtimeReady ? "" : "Finish Docker setup first"}
          className="mt-2 inline-flex items-center justify-center gap-1.5 rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span className="text-base leading-none">+</span> New server
        </button>
      </div>

      {/* Footer */}
      <div className="mt-2 border-t border-zinc-800/80 px-2 py-2">
        <NavItem onClick={onOpenSettings} label="Settings" icon="&#x2699;" />
        <NavItem onClick={onOpenAccount} label={account?.linked ? "Account · Plus" : "Account"} icon="&#x25CD;" />
        <NavItem onClick={onWhatsNew} label="What's New" icon="&#x2726;" />
        <a href={GITHUB_URL} target="_blank" rel="noreferrer" className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-zinc-400 transition hover:bg-zinc-800/60 hover:text-zinc-200">GitHub &#x2197;</a>
        <a href={DISCORD_URL} target="_blank" rel="noreferrer" className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-zinc-400 transition hover:bg-zinc-800/60 hover:text-zinc-200">Discord &#x2197;</a>
        <p className="px-2 pt-1 text-[11px] text-zinc-600">
          {appVersion ? `v${appVersion}` : ""}{appVersion && engineVersion ? " · " : ""}{engineVersion ? `engine v${engineVersion}` : ""}
        </p>
      </div>
    </aside>
  );
}

function NavItem({ label, icon, active, onClick }: { label: string; icon: string; active?: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-sm transition ${
        active ? "bg-emerald-500/10 text-emerald-200 ring-1 ring-inset ring-emerald-500/30" : "text-zinc-300 hover:bg-zinc-800/60"
      }`}
    >
      <span className="w-4 text-center text-zinc-500">{icon}</span>
      <span>{label}</span>
    </button>
  );
}
