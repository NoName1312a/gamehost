import { useState } from "react";
import { type Setup } from "../lib/api";
import { Logo } from "./icons";
import { SetupWizard } from "./SetupWizard";

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
}: {
  setup: Async<Setup>;
  runtimeReady: boolean;
  onRecheck: () => void;
  onFinish: () => void;
  onSkip: () => void;
}) {
  const [step, setStep] = useState<OnboardingStep>("welcome");

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
                onClick={onFinish}
                disabled={!runtimeReady}
                title={runtimeReady ? "" : "Finish Docker setup to continue"}
                className="rounded-lg bg-emerald-500 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Continue
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
