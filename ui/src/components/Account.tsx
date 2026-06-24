import { useEffect, useState } from "react";
import { api, type AccountStatus } from "../lib/api";
import { friendlyError } from "../lib/errors";
import { Logo } from "./icons";

// The Account screen — GameNest Plus sign-in / status. Distinct from Settings.
// When the hosted platform isn't configured yet (the common case today), it
// shows an honest "coming soon" state instead of a dead sign-in form.
export function Account() {
  const [account, setAccount] = useState<AccountStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [linkCode, setLinkCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .account()
      .then(setAccount)
      .catch(() => setAccount(null))
      .finally(() => setLoading(false));
  }, []);

  async function link() {
    if (!linkCode.trim()) return;
    setBusy(true);
    setError(null);
    try {
      setAccount(await api.linkAccount(linkCode.trim()));
      setLinkCode("");
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setBusy(false);
    }
  }

  async function unlink() {
    setBusy(true);
    setError(null);
    try {
      setAccount(await api.unlinkAccount());
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-md px-6 py-8">
        <div className="flex items-center gap-2">
          <h2 className="font-display text-2xl font-semibold text-zinc-100">Account</h2>
          {account?.linked && (
            <span className="rounded-full bg-emerald-400/10 px-2 py-0.5 text-[11px] font-medium text-emerald-300 ring-1 ring-inset ring-emerald-400/20">
              Plus
            </span>
          )}
        </div>

        <div className="panel p-5 mt-5">
          {loading ? (
            <p className="text-xs text-zinc-500">…</p>
          ) : account?.configured ? (
            account.linked ? (
              <div className="space-y-3">
                <p className="text-sm text-zinc-300">
                  ✓ Signed in to <span className="font-medium text-emerald-300">GameNest Plus</span> — your vanity
                  tunnel names and higher capacity are active on this machine.
                </p>
                <button
                  onClick={unlink}
                  disabled={busy}
                  className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800 disabled:opacity-50"
                >
                  {busy ? "…" : "Sign out"}
                </button>
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-zinc-400">
                  Sign in to <span className="font-medium text-zinc-200">GameNest Plus</span> to unlock vanity tunnel
                  names (e.g. <span className="font-mono text-emerald-300">you.gn.coderaum.com</span>) and higher
                  capacity.
                </p>
                <p className="text-xs text-zinc-500">
                  Get your link code from the{" "}
                  <a href="https://gamenest.cc/dashboard" target="_blank" rel="noreferrer" className="text-emerald-400 hover:underline">
                    dashboard at gamenest.cc
                  </a>
                  .
                </p>
                <input
                  value={linkCode}
                  onChange={(e) => setLinkCode(e.target.value)}
                  placeholder="Paste your link code"
                  className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 font-mono text-xs text-zinc-100 outline-none focus:border-emerald-500"
                />
                <button
                  onClick={link}
                  disabled={busy || !linkCode.trim()}
                  className="w-full rounded-lg bg-emerald-500 px-3 py-2 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
                >
                  {busy ? "Signing in…" : "Sign in"}
                </button>
              </div>
            )
          ) : (
            <div className="space-y-3 text-center">
              <div className="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-zinc-950 ring-1 ring-inset ring-zinc-800">
                <Logo className="h-8 w-8 text-emerald-400" />
              </div>
              <p className="text-sm font-medium text-zinc-200">GameNest Plus is on the way</p>
              <p className="text-xs text-zinc-500">
                Accounts will unlock a memorable address for your servers (like{" "}
                <span className="font-mono text-emerald-300">you.gn.coderaum.com</span>) and higher capacity. Sign-in
                goes live when the hosted service launches — for now, everything in GameNest is free and unlimited.
              </p>
              <a
                href="https://gamenest.cc"
                target="_blank"
                rel="noreferrer"
                className="inline-block rounded-lg border border-zinc-700 px-3 py-1.5 text-xs text-zinc-300 hover:bg-zinc-800"
              >
                Learn more at gamenest.cc
              </a>
            </div>
          )}
          {error && <p className="mt-2 text-xs text-rose-400">{error}</p>}
        </div>
      </div>
    </div>
  );
}
