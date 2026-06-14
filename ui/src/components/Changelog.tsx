import { useEffect, type ReactNode } from "react";
import type { ChangelogEntry } from "../lib/changelog";

// Accent per change type, matching Keep-a-Changelog conventions.
const typeColor: Record<string, string> = {
  Added: "text-emerald-400",
  Changed: "text-sky-400",
  Fixed: "text-amber-400",
  Removed: "text-rose-400",
  Security: "text-violet-400",
  Deprecated: "text-zinc-400",
};

// Renders a changelog line's lightweight markdown: **bold** and `code`.
function renderInline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((p, i) => {
    if (p.startsWith("**") && p.endsWith("**")) {
      return (
        <strong key={i} className="font-semibold text-zinc-100">
          {p.slice(2, -2)}
        </strong>
      );
    }
    if (p.startsWith("`") && p.endsWith("`")) {
      return (
        <code key={i} className="rounded bg-zinc-800 px-1 py-0.5 text-[0.85em] text-zinc-300">
          {p.slice(1, -1)}
        </code>
      );
    }
    return p;
  });
}

// Changelog renders a list of version entries. It powers both the re-viewable
// "What's New" (all versions) and the post-update popup (scoped to new versions
// via the entries the caller passes + a tailored title/subtitle).
export function Changelog({
  title,
  subtitle,
  entries,
  onClose,
}: {
  title: string;
  subtitle?: string;
  entries: ChangelogEntry[];
  onClose: () => void;
}) {
  // Lock background scroll + close on Escape.
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
      <div
        className="flex max-h-[calc(100vh-3rem)] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/40"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3 border-b border-zinc-800 px-5 py-4">
          <div>
            <h2 className="text-lg font-semibold text-zinc-100">{title}</h2>
            {subtitle && <p className="mt-0.5 text-sm text-zinc-500">{subtitle}</p>}
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="rounded-lg px-2 py-1 text-sm text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
          >
            ✕
          </button>
        </div>

        <div className="flex-1 overflow-y-auto overscroll-contain px-5 py-4">
          {entries.length === 0 ? (
            <p className="py-8 text-center text-sm text-zinc-500">No changes to show.</p>
          ) : (
            <div className="space-y-6">
              {entries.map((e) => (
                <section key={e.version}>
                  <div className="flex items-baseline gap-2">
                    <h3 className="font-semibold text-zinc-100">v{e.version}</h3>
                    {e.date && <span className="text-xs text-zinc-500">{e.date}</span>}
                  </div>
                  <div className="mt-2 space-y-3">
                    {e.groups.map((g) => (
                      <div key={g.type}>
                        <p className={`text-[11px] font-semibold uppercase tracking-wide ${typeColor[g.type] ?? "text-zinc-400"}`}>
                          {g.type}
                        </p>
                        <ul className="mt-1 space-y-1">
                          {g.items.map((it, i) => (
                            <li key={i} className="flex gap-2 text-sm text-zinc-300">
                              <span className="mt-2 h-1 w-1 shrink-0 rounded-full bg-zinc-600" />
                              <span>{renderInline(it)}</span>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ))}
                  </div>
                </section>
              ))}
            </div>
          )}
        </div>

        <div className="border-t border-zinc-800 px-5 py-3">
          <button
            onClick={onClose}
            className="w-full rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
          >
            Got it
          </button>
        </div>
      </div>
    </div>
  );
}
