import { type ServerSummary } from "../lib/api";

/** A server is shareable once it has any friend-facing address. */
function hasShareAddress(s: ServerSummary): boolean {
  return Boolean(s.tunnelAddress || s.externalAddress || s.relayAddress);
}

export function GetStartedChecklist({
  runtimeReady,
  servers,
  invitedFriend,
  onNewServer,
  onInvite,
}: {
  runtimeReady: boolean;
  servers: ServerSummary[] | null;
  invitedFriend: boolean;
  onNewServer: () => void;
  onInvite: () => void;
}) {
  const hasServer = (servers?.length ?? 0) > 0;
  const friendCanJoin = invitedFriend || (servers?.some(hasShareAddress) ?? false);

  const items = [
    {
      key: "docker",
      done: runtimeReady,
      label: "Set up Docker",
      hint: "Connect the container runtime that runs your game servers.",
      action: null as null | { cta: string; onClick: () => void },
    },
    {
      key: "server",
      done: hasServer,
      label: "Create your first server",
      hint: "Pick a game and spin one up in a couple of clicks.",
      action: runtimeReady && !hasServer ? { cta: "New server", onClick: onNewServer } : null,
    },
    {
      key: "friend",
      done: friendCanJoin,
      label: "Invite a friend",
      hint: "Start your server and share the address so a friend can join.",
      action: hasServer && !friendCanJoin ? { cta: "Show me", onClick: onInvite } : null,
    },
  ];

  if (items.every((i) => i.done)) return null;
  const completed = items.filter((i) => i.done).length;

  return (
    <section className="panel mx-6 mt-6 p-5">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="font-display text-base font-semibold text-zinc-100">Get started</h2>
        <span className="text-xs text-zinc-500">{completed} of {items.length} done</span>
      </div>
      <ul className="space-y-2.5">
        {items.map((it) => (
          <li key={it.key} className="flex items-center gap-3">
            <span
              className={`grid h-5 w-5 shrink-0 place-items-center rounded-full text-[11px] font-bold ${
                it.done ? "bg-emerald-500 text-zinc-950" : "border border-zinc-700 text-transparent"
              }`}
            >
              ✓
            </span>
            <div className="min-w-0 flex-1">
              <p className={`text-sm ${it.done ? "text-zinc-500 line-through" : "text-zinc-200"}`}>{it.label}</p>
              {!it.done && <p className="text-xs text-zinc-600">{it.hint}</p>}
            </div>
            {it.action && (
              <button
                onClick={it.action.onClick}
                className="shrink-0 rounded-lg bg-emerald-500 px-3 py-1.5 text-xs font-semibold text-zinc-950 transition hover:bg-emerald-400"
              >
                {it.action.cta}
              </button>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
