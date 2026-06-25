import { useState } from "react";
import { type ServerSummary, type Setup } from "../lib/api";
import { type GameGroup } from "../lib/games";
import { Logo } from "./icons";
import { SetupWizard } from "./SetupWizard";
import { GamePicker } from "./GamePicker";
import { ConfigureServerModal } from "./ConfigureServerModal";

// Mirrors the tiny Async helper in App.tsx (kept local to avoid a shared export).
type Async<T> =
  | { status: "loading" }
  | { status: "ok"; data: T }
  | { status: "error"; error: string };

export type OnboardingStep = "welcome" | "setup" | "pick" | "live";

export function Onboarding({
  setup,
  runtimeReady,
  onRecheck,
  onFinish,
  onSkip,
  groups,
  servers,
  onStartServer,
  onOpenServer,
  onMarkInvited,
}: {
  setup: Async<Setup>;
  runtimeReady: boolean;
  onRecheck: () => void;
  onFinish: () => void;
  onSkip: () => void;
  groups: GameGroup[];
  servers: ServerSummary[] | null;
  onStartServer: (id: string) => void;
  onOpenServer: (id: string) => void;
  onMarkInvited: () => void;
}) {
  const [step, setStep] = useState<OnboardingStep>("welcome");
  const [showPicker, setShowPicker] = useState(false);
  const [pickerGroup, setPickerGroup] = useState<GameGroup | null>(null);
  const [createdId, setCreatedId] = useState<string | null>(null);
  const liveServer = createdId ? servers?.find((s) => s.id === createdId) ?? null : null;

  return (
    <div className="relative grid min-h-screen place-items-center p-6">
      <div className="bg-glow" aria-hidden />
      <div className="grain" aria-hidden />
      <div className="panel relative z-10 w-full max-w-xl p-8">
        {step === "welcome" && (
          <div className="text-center">
            <Logo className="mx-auto h-14 w-14 text-emerald-400" />
            <h1 className="mt-5 font-display text-2xl font-semibold text-zinc-100">Welcome to GameNest</h1>
            <p className="mx-auto mt-3 max-w-md text-sm text-zinc-400">
              Host a game server your friends can actually join — one click, no port-forwarding.
            </p>
            <div className="mt-7 flex items-center justify-center gap-3">
              <button
                onClick={() => setStep("setup")}
                className="rounded-lg bg-emerald-500 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
              >
                Get started
              </button>
              <button onClick={onSkip} className="text-sm text-zinc-500 transition hover:text-zinc-300">
                Skip for now
              </button>
            </div>
          </div>
        )}

        {step === "setup" && (
          <div>
            <div className="mb-1 text-center">
              <h1 className="font-display text-xl font-semibold text-zinc-100">Quick setup</h1>
              <p className="mt-2 text-sm text-zinc-400">
                GameNest runs your servers in Docker. Let's make sure it's ready.
              </p>
            </div>
            <div className="-mx-3 mt-4">
              <SetupWizard setup={setup} onRecheck={onRecheck} />
            </div>
            <div className="mt-6 flex items-center justify-between gap-3">
              <button onClick={onSkip} className="text-sm text-zinc-500 transition hover:text-zinc-300">
                Skip for now
              </button>
              <button
                onClick={() => setStep("pick")}
                disabled={!runtimeReady}
                title={runtimeReady ? "" : "Finish Docker setup to continue"}
                className="rounded-lg bg-emerald-500 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Continue
              </button>
            </div>
          </div>
        )}

        {step === "pick" && (
          <div className="text-center">
            <h1 className="font-display text-xl font-semibold text-zinc-100">Make your first server</h1>
            <p className="mx-auto mt-2 max-w-md text-sm text-zinc-400">Pick a game — you can tweak everything later.</p>
            <div className="mt-6 flex items-center justify-center gap-3">
              <button
                onClick={() => setShowPicker(true)}
                className="rounded-lg bg-emerald-500 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
              >
                Choose a game
              </button>
              <button onClick={onFinish} className="text-sm text-zinc-500 transition hover:text-zinc-300">
                I'll do this later
              </button>
            </div>
          </div>
        )}

        {step === "live" && (
          <div className="text-center">
            <Logo className="mx-auto h-12 w-12 text-emerald-400" />
            <h1 className="mt-4 font-display text-2xl font-semibold text-zinc-100">You're live!</h1>
            {liveServer && (liveServer.tunnelAddress || liveServer.externalAddress || liveServer.relayAddress) ? (
              <>
                <p className="mt-2 text-sm text-zinc-400">Send this address to a friend so they can join:</p>
                <div className="mx-auto mt-4 flex max-w-sm items-center gap-2">
                  <code className="min-w-0 flex-1 truncate rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-left font-mono text-sm text-emerald-300">
                    {liveServer.tunnelAddress || liveServer.externalAddress || liveServer.relayAddress}
                  </code>
                  <button
                    onClick={() => {
                      const addr = liveServer.tunnelAddress || liveServer.externalAddress || liveServer.relayAddress || "";
                      navigator.clipboard?.writeText(addr);
                      onMarkInvited();
                    }}
                    className="shrink-0 rounded-lg bg-emerald-500 px-3 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
                  >
                    Copy
                  </button>
                </div>
              </>
            ) : (
              <p className="mx-auto mt-2 max-w-md text-sm text-zinc-400">
                {liveServer?.pulling
                  ? `Setting up your server — downloading game files… ${liveServer.pullPercent ?? 0}%`
                  : "Your server is starting. You can grab the share link any time from the server's Overview tab."}
              </p>
            )}
            <div className="mt-7">
              <button
                onClick={() => { if (createdId) onOpenServer(createdId); onFinish(); }}
                className="rounded-lg bg-emerald-500 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
              >
                Open my server
              </button>
            </div>
          </div>
        )}
      </div>

      {showPicker && (
        <GamePicker
          groups={groups}
          onPick={(g) => { setShowPicker(false); setPickerGroup(g); }}
          onClose={() => setShowPicker(false)}
        />
      )}
      {pickerGroup && (
        <ConfigureServerModal
          group={pickerGroup}
          onClose={() => setPickerGroup(null)}
          onCreated={(server) => {
            setPickerGroup(null);
            setCreatedId(server.id);
            onStartServer(server.id);
            setStep("live");
          }}
        />
      )}
    </div>
  );
}
