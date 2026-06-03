import { useState, type ReactNode } from "react";
import { api, type Setup, type SetupStep } from "../lib/api";

// Mirrors the tiny Async helper in App.tsx (kept local to avoid a shared export).
type Async<T> =
  | { status: "loading" }
  | { status: "ok"; data: T }
  | { status: "error"; error: string };

function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="mx-6 mt-6 rounded-lg border border-amber-500/20 bg-amber-500/5 p-5">
      <div className="flex items-center gap-3">
        <span className="h-2 w-2 rounded-full bg-amber-400" />
        <h2 className="text-sm font-semibold text-amber-100">Set up Docker to start hosting</h2>
      </div>
      <p className="mt-2 text-sm text-amber-200/80">
        GameHost runs each game server in its own container. A few one-time steps and you're ready —
        we'll do the work, just approve the Windows prompts.
      </p>
      {children}
    </div>
  );
}

export function SetupWizard({
  setup,
  onRecheck,
}: {
  setup: Async<Setup>;
  onRecheck: () => void;
}) {
  if (setup.status === "loading") {
    return (
      <Shell>
        <p className="mt-3 text-sm text-amber-200/70">Checking your system…</p>
      </Shell>
    );
  }
  if (setup.status === "error") {
    return (
      <Shell>
        <p className="mt-3 text-sm text-rose-300">Couldn't check setup: {setup.error}</p>
        <RecheckButton onRecheck={onRecheck} />
      </Shell>
    );
  }

  const steps = setup.data.steps;
  const currentIdx = steps.findIndex((s) => s.status === "todo");

  return (
    <Shell>
      <ol className="mt-4 space-y-3">
        {steps.map((s, i) => (
          <StepRow key={s.id} step={s} index={i} current={i === currentIdx} onRecheck={onRecheck} />
        ))}
      </ol>
      <RecheckButton onRecheck={onRecheck} />
    </Shell>
  );
}

function RecheckButton({ onRecheck }: { onRecheck: () => void }) {
  return (
    <button
      onClick={onRecheck}
      className="mt-4 rounded-lg border border-amber-500/30 px-3 py-1.5 text-sm text-amber-100 hover:bg-amber-500/10"
    >
      Recheck
    </button>
  );
}

function StepIcon({ done, current, index }: { done: boolean; current: boolean; index: number }) {
  if (done) {
    return (
      <span className="grid h-6 w-6 place-items-center rounded-full bg-emerald-500/15 text-xs font-bold text-emerald-300 ring-1 ring-emerald-500/30">
        ✓
      </span>
    );
  }
  return (
    <span
      className={`grid h-6 w-6 place-items-center rounded-full text-xs font-bold ring-1 ${
        current
          ? "bg-amber-400/20 text-amber-200 ring-amber-400/40"
          : "bg-zinc-800 text-zinc-400 ring-zinc-700"
      }`}
    >
      {index + 1}
    </span>
  );
}

function StepRow({
  step,
  index,
  current,
  onRecheck,
}: {
  step: SetupStep;
  index: number;
  current: boolean;
  onRecheck: () => void;
}) {
  const [pending, setPending] = useState(false);
  const [hint, setHint] = useState<string | null>(null);
  const [showManual, setShowManual] = useState(false);
  const [copied, setCopied] = useState(false);

  const done = step.status === "ok";

  async function runFix() {
    if (!step.action) return;
    setPending(true);
    setHint(null);
    try {
      const res = await api.runSetupStep(step.action.endpoint);
      setHint(res.hint);
    } catch (e) {
      setHint(e instanceof Error ? e.message : String(e));
    } finally {
      setPending(false);
      onRecheck();
    }
  }

  async function copyCommand() {
    if (!step.action) return;
    try {
      await navigator.clipboard.writeText(step.action.command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard may be unavailable; the command is shown for manual copy */
    }
  }

  return (
    <li className="flex gap-3">
      <div className="pt-0.5">
        <StepIcon done={done} current={current} index={index} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className={`text-sm font-medium ${done ? "text-zinc-400 line-through" : "text-zinc-100"}`}>
            {step.title}
          </span>
          {step.action?.needsAdmin && !done && (
            <span className="rounded-full bg-zinc-800 px-2 py-0.5 text-[10px] font-medium text-zinc-400 ring-1 ring-inset ring-zinc-700">
              admin
            </span>
          )}
          {step.action?.needsReboot && !done && (
            <span className="rounded-full bg-zinc-800 px-2 py-0.5 text-[10px] font-medium text-zinc-400 ring-1 ring-inset ring-zinc-700">
              may restart
            </span>
          )}
        </div>
        <p className="mt-0.5 text-sm text-zinc-400">{step.detail}</p>

        {!done && step.action && (
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <button
              onClick={runFix}
              disabled={pending}
              className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50"
            >
              {pending ? "Launching…" : step.action.label}
            </button>
            <button
              onClick={() => setShowManual((v) => !v)}
              className="text-xs text-zinc-500 underline-offset-2 hover:text-zinc-300 hover:underline"
            >
              {showManual ? "hide manual steps" : "run it manually"}
            </button>
          </div>
        )}

        {hint && <p className="mt-2 text-xs text-amber-200/80">{hint}</p>}

        {showManual && step.action && (
          <div className="mt-2">
            <div className="flex items-center justify-between gap-2 rounded-md bg-zinc-950/70 px-3 py-2 ring-1 ring-zinc-800">
              <code className="overflow-x-auto text-xs text-zinc-300">{step.action.command}</code>
              <button
                onClick={copyCommand}
                className="shrink-0 rounded border border-zinc-700 px-2 py-0.5 text-[11px] text-zinc-300 hover:bg-zinc-800"
              >
                {copied ? "copied" : "copy"}
              </button>
            </div>
            {step.action.needsAdmin && (
              <p className="mt-1 text-[11px] text-zinc-600">Run in an Administrator terminal.</p>
            )}
          </div>
        )}
      </div>
    </li>
  );
}
