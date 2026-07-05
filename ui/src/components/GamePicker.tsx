import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import type { GameGroup } from "../lib/games";

// Category accent colors, matching the badges used elsewhere in the app.
const categoryAccent: Record<string, string> = {
  Sandbox: "text-emerald-400 bg-emerald-400/10 ring-emerald-400/20",
  Survival: "text-amber-400 bg-amber-400/10 ring-amber-400/20",
  Shooter: "text-rose-400 bg-rose-400/10 ring-rose-400/20",
  Simulation: "text-sky-400 bg-sky-400/10 ring-sky-400/20",
  Strategy: "text-violet-400 bg-violet-400/10 ring-violet-400/20",
  Modded: "text-violet-400 bg-violet-400/10 ring-violet-400/20",
};
const accentFor = (c: string) => categoryAccent[c] ?? "text-zinc-400 bg-zinc-400/10 ring-zinc-400/20";

// A compact cover thumbnail; falls back to the gradient + glyph tile if the
// game has no cover art or the image fails to load.
function Thumb({ group }: { group: GameGroup }) {
  const [err, setErr] = useState(false);
  if (group.cover && !err) {
    return (
      <img
        src={group.cover}
        alt=""
        loading="lazy"
        onError={() => setErr(true)}
        className="h-10 w-14 shrink-0 rounded-md object-cover"
      />
    );
  }
  return (
    <div className={`grid h-10 w-14 shrink-0 place-items-center rounded-md bg-gradient-to-br ${group.gradient} text-lg`}>
      {group.glyph}
    </div>
  );
}

// GamePicker is a searchable command-palette for choosing a game to create a
// server from. It replaces the old full-page cover-art wall: type to filter,
// arrow keys to move, Enter to pick.
export function GamePicker({
  groups,
  onPick,
  onClose,
}: {
  groups: GameGroup[];
  onPick: (g: GameGroup) => void;
  onClose: () => void;
}) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  // Lock background scrolling while the picker is open.
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, []);

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return groups;
    return groups.filter(
      (g) =>
        g.name.toLowerCase().includes(q) ||
        g.category.toLowerCase().includes(q) ||
        g.blurb.toLowerCase().includes(q),
    );
  }, [groups, query]);

  // Reset the highlight to the top result whenever the query changes.
  useEffect(() => setActive(0), [query]);

  // Keep the highlighted row scrolled into view.
  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-idx="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [active]);

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      onClose();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((i) => Math.min(i + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const g = results[active];
      if (g) onPick(g);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/60 p-6 pt-[12vh] backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="flex max-h-[70vh] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-zinc-800 bg-zinc-900/90 shadow-2xl shadow-black/40 backdrop-blur"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        {/* Search */}
        <div className="flex items-center gap-3 border-b border-zinc-800 px-4">
          <svg className="h-4 w-4 shrink-0 text-zinc-500" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.6">
            <circle cx="9" cy="9" r="6" />
            <path d="m14 14 3 3" strokeLinecap="round" />
          </svg>
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search games…"
            className="w-full bg-transparent py-3.5 text-sm text-zinc-100 outline-none placeholder:text-zinc-600"
          />
          <kbd className="hidden rounded border border-zinc-700 px-1.5 py-0.5 text-[10px] text-zinc-500 sm:block">esc</kbd>
        </div>

        {/* Results */}
        <div ref={listRef} className="flex-1 overflow-y-auto overscroll-contain p-2">
          {results.length === 0 ? (
            <p className="px-3 py-10 text-center text-sm text-zinc-500">
              {query.trim()
                ? <>No games match “{query.trim()}”.</>
                : "The game library is empty — GameNest couldn't load its game templates. Reinstalling usually fixes this."}
            </p>
          ) : (
            results.map((g, i) => (
              <button
                key={g.game}
                data-idx={i}
                onClick={() => onPick(g)}
                onMouseMove={() => setActive(i)}
                className={`flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left transition ${
                  i === active ? "bg-zinc-800/70 ring-1 ring-inset ring-zinc-700" : "hover:bg-zinc-800/40"
                }`}
              >
                <Thumb group={g} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-medium text-zinc-100">{g.name}</span>
                    {g.templates.length > 1 && (
                      <span className="shrink-0 text-[11px] text-zinc-500">{g.templates.length} editions</span>
                    )}
                  </div>
                  <p className="truncate text-xs text-zinc-500">{g.blurb}</p>
                </div>
                <span className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-inset ${accentFor(g.category)}`}>
                  {g.category}
                </span>
              </button>
            ))
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between border-t border-zinc-800 px-4 py-2 text-[11px] text-zinc-600">
          <span>
            {results.length} game{results.length === 1 ? "" : "s"}
          </span>
          <span className="hidden sm:block">↑↓ navigate · ↵ select · esc close</span>
        </div>
      </div>
    </div>
  );
}
