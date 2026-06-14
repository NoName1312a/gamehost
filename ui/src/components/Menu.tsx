import { useEffect, useState, type ReactNode } from "react";

const GITHUB_URL = "https://github.com/NoName1312a/gamehost";
const DISCORD_URL = "https://discord.gg/gamenest";

function GitHubIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className="h-4 w-4" aria-hidden>
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.6 7.6 0 014 0c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0016 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

function DiscordIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className="h-4 w-4" aria-hidden>
      <path d="M20.317 4.369A19.79 19.79 0 0016.558 3c-.2.36-.43.84-.59 1.23a18.27 18.27 0 00-3.94 0A12.6 12.6 0 0011.43 3a19.74 19.74 0 00-3.76 1.37C3.96 7.92 3.24 11.36 3.5 14.75a19.9 19.9 0 006.07 3.08c.49-.67.93-1.38 1.3-2.13-.71-.27-1.39-.6-2.03-.99.17-.13.34-.26.5-.4a14.2 14.2 0 0012.12 0c.16.14.33.27.5.4-.64.39-1.32.72-2.03.99.37.75.81 1.46 1.3 2.13a19.85 19.85 0 006.07-3.08c.3-3.9-.48-7.3-2.66-10.38zM9.68 12.65c-.97 0-1.77-.89-1.77-1.98 0-1.09.78-1.98 1.77-1.98.99 0 1.79.9 1.77 1.98 0 1.09-.78 1.98-1.77 1.98zm4.64 0c-.97 0-1.77-.89-1.77-1.98 0-1.09.78-1.98 1.77-1.98.99 0 1.79.9 1.77 1.98 0 1.09-.78 1.98-1.77 1.98z" />
    </svg>
  );
}

function Item({ icon, label, onClick }: { icon: ReactNode; label: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm text-zinc-200 transition hover:bg-zinc-800/70"
    >
      <span className="grid h-5 w-5 place-items-center text-zinc-400">{icon}</span>
      {label}
    </button>
  );
}

function LinkItem({ icon, label, href }: { icon: ReactNode; label: string; href: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm text-zinc-200 transition hover:bg-zinc-800/70"
    >
      <span className="grid h-5 w-5 place-items-center text-zinc-400">{icon}</span>
      <span className="flex-1">{label}</span>
      <span className="text-xs text-zinc-600">↗</span>
    </a>
  );
}

// Menu is the slide-out navigation drawer (from the left) holding Settings,
// What's New, and community links — opened from the header burger button.
export function Menu({
  appVersion,
  engineVersion,
  onSettings,
  onWhatsNew,
  onClose,
}: {
  appVersion?: string | null;
  engineVersion?: string;
  onSettings: () => void;
  onWhatsNew: () => void;
  onClose: () => void;
}) {
  const [shown, setShown] = useState(false);

  useEffect(() => {
    // Trigger the slide-in on the next frame after mount.
    const id = requestAnimationFrame(() => setShown(true));
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => {
      cancelAnimationFrame(id);
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-40 bg-black/60" onClick={onClose}>
      <aside
        onClick={(e) => e.stopPropagation()}
        className={`absolute inset-y-0 left-0 flex w-72 max-w-[80vw] flex-col border-r border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/40 transition-transform duration-200 ease-out ${
          shown ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {/* Brand */}
        <div className="flex items-center gap-3 border-b border-zinc-800 px-4 py-4">
          <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-emerald-400 to-cyan-500 text-lg font-black text-zinc-950 shadow-lg shadow-emerald-500/20">
            G
          </div>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-zinc-100">GameNest</p>
            <p className="truncate text-xs text-zinc-500">{appVersion ? `v${appVersion}` : "Self-host, simply"}</p>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto p-2">
          <Item icon="⚙" label="Settings" onClick={onSettings} />
          <Item icon="✨" label="What's New" onClick={onWhatsNew} />
          <div className="my-2 border-t border-zinc-800/80" />
          <LinkItem icon={<GitHubIcon />} label="GitHub" href={GITHUB_URL} />
          <LinkItem icon={<DiscordIcon />} label="Discord" href={DISCORD_URL} />
        </nav>

        {/* Footer */}
        <div className="border-t border-zinc-800 px-4 py-3 text-[11px] leading-relaxed text-zinc-600">
          {engineVersion && <p>engine {engineVersion}</p>}
          <p>Free &amp; open source · AGPL-3.0</p>
        </div>
      </aside>
    </div>
  );
}
