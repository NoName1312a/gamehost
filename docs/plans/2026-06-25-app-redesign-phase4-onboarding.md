# App Redesign — Phase 4: Onboarding — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Carry a brand-new user from first launch to a running server a friend can join — via a guided first-run flow (Welcome → Quick setup → Pick your first game → You're live) plus a persistent Dashboard "Get started" checklist as the safety net.

**Architecture:** Pure UI in `ui/` (no engine/API changes). Add two components: a `GetStartedChecklist` card rendered in the Dashboard branch of `App.tsx`, and a full-screen `Onboarding` flow shown as an early-return takeover (same pattern as `EngineOffline`/`Login`) for first-run users. The flow **reuses** the existing `SetupWizard` (Quick setup), `GamePicker` + `ConfigureServerModal` (Pick a game), and the share-address fields on `ServerSummary` (You're live). Onboarding-complete state persists in `localStorage` (the app already uses the `gamenest.*` namespace).

**Tech Stack:** Tauri v2 · React 19 · Tailwind v4 (`@theme` in CSS, no `tailwind.config.js`) · hand-rolled components (no UI library) · Vite.

## Global Constraints

- This is **Phase 4 of 4** (spec: `docs/specs/2026-06-24-app-ux-overhaul-design.md`, §5 + §65–75). It completes the redesign. Build the **guided first-run flow + the Dashboard "Get started" checklist** — nothing else.
- **Reuse the brand layer** (in `ui/src/index.css`): `.panel`, `.divider`, `font-display`/`font-sans`/`font-mono`, the `Logo` (`ui/src/components/icons.tsx`). **Keep the `emerald-400`/`emerald-500` literals** the app uses.
- **No engine/API changes** (`ui/src/lib/api.ts` is unchanged). Reuse existing endpoints/types.
- **Reuse, don't reimplement:** the Quick-setup step embeds the existing `SetupWizard` (driven by `api.setup`/`api.runSetupStep`); the Pick-a-game step reuses `GamePicker` → `ConfigureServerModal` (`api.createServer`); the You're-live step reads the share address off `ServerSummary` (`tunnelAddress ?? externalAddress ?? relayAddress`).
- **Persistence:** use `localStorage` (the app already uses key `gamenest.lastSeenVersion`). New keys: `gamenest.onboardingDone` and `gamenest.invitedFriend`. No engine setting.
- **Show the flow once:** the guided takeover appears only for a genuine first-run user (no servers yet, not already done); finishing OR skipping sets `gamenest.onboardingDone`; an existing user (servers already present) is auto-marked done so they never see it.
- **No UI test runner exists — do NOT write tests.** Verification per task = `npm --prefix ui run build` (tsc + vite, must pass) + `npm --prefix ui run lint` (**0 NEW** errors; **7 pre-existing `react-hooks/set-state-in-effect` errors are known debt** — don't add new ones; **note:** new `useEffect`s that call `setState` synchronously will add to this count and must be avoided/structured to not fire synchronously). **Task completion gates on build + lint passing**; the owner verifies visuals on the released build (they don't run the dev server). Tasks are implement → verify (build+lint) → commit.
- Commits **DCO signed-off** (`git commit -s`); free AGPL core (nothing under `ee/`); match surrounding style. Branch `feat/app-ux-overhaul` (already checked out; **no new branch**).

---

### Task 1: Restyle `SetupWizard` to the brand layer

`SetupWizard` is still flat amber (`rounded-lg border border-amber-500/20 bg-amber-500/5 p-5`) — the last screen not on the brand layer. Restyle it (it shows both in the Dashboard branch and, embedded, inside the onboarding flow), so onboarding can reuse it as-is.

**Files:**
- Modify: `ui/src/components/SetupWizard.tsx`

**Interfaces:** unchanged — keeps `export function SetupWizard({ setup, onRecheck }: { setup: Async<Setup>; onRecheck: () => void })`.

- [ ] **Step 1: Panelize the shell + display-font heading**

In `SetupWizard.tsx`, change the outer `Shell` card from the flat amber box to a `.panel`. Locate the shell wrapper (the `rounded-lg border border-amber-500/20 bg-amber-500/5 p-5` element) and replace those shell classes with `panel mx-6 mt-6 p-5` (match the `ReadyBanner`'s `mx-6 mt-6` placement so setup/ready sit identically in the dashboard column). Give the wizard's title/heading `font-display`. Keep the **amber accent only on the "needs setup / todo" cues** (e.g. the pending-step dot/label, the "Action needed" wording) — the card itself is now neutral glass, not amber-tinted.

- [ ] **Step 2: Brand the step rows + buttons**

Keep the `StepRow` structure and all logic (`currentIdx`, `api.runSetupStep(step.action.endpoint)`, `onRecheck()`). Restyle: done steps show an emerald `✓`; the current/todo step shows an amber dot; the action button uses the standard primary style `rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-50`; the "Re-check" button uses a ghost style `rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800`. Do not change any `api.*` call, the props, or the loading/error/ok state machine.

- [ ] **Step 3: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors).

- [ ] **Step 4: Commit**

```bash
git add ui/src/components/SetupWizard.tsx
git commit -s -m "feat(ui): restyle SetupWizard to the brand layer"
```

---

### Task 2: Dashboard "Get started" checklist (the safety net)

A persistent card on the Dashboard reflecting real progress — **Set up Docker · Create your first server · Invite a friend** — that auto-hides once complete. It lets a user who skips/exits the guided flow resume.

**Files:**
- Create: `ui/src/components/GetStartedChecklist.tsx`
- Modify: `ui/src/App.tsx` (render it in the dashboard branch; add the `invitedFriend` localStorage state)

**Interfaces:**
- Produces: `export function GetStartedChecklist(props)` (signature in Step 1).
- Consumes: `runtimeReady`, `servers`, an `invitedFriend` boolean, and two callbacks from `App`.

- [ ] **Step 1: Create `ui/src/components/GetStartedChecklist.tsx`**

```tsx
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
```

- [ ] **Step 2: Add `invitedFriend` state in `App.tsx`**

Near the other `useState` calls (~`:117-124`), add a **read-only** state (it reads the existing `gamenest.*` localStorage namespace, mirroring the `gamenest.lastSeenVersion` usage already in the file). The checklist only *reads* `invitedFriend`; the setter and the `markInvited` helper are added in **Task 4** when the "copy share address" moment first writes the flag — adding an unused setter/helper now would trip `noUnusedLocals`:
```tsx
const [invitedFriend] = useState<boolean>(
  () => localStorage.getItem("gamenest.invitedFriend") === "true",
);
```

- [ ] **Step 3: Render the checklist in the dashboard branch**

In the dashboard branch of the `<main>` switch, add `<GetStartedChecklist .../>` between the ReadyBanner/SetupWizard line (~`:269-270`) and `<Dashboard .../>` (~`:271`):
```tsx
{runtime.status !== "loading" &&
  (runtimeReady ? <ReadyBanner runtime={runtime} /> : <SetupWizard setup={setup} onRecheck={retry} />)}
<GetStartedChecklist
  runtimeReady={runtimeReady}
  servers={servers}
  invitedFriend={invitedFriend}
  onNewServer={() => setShowPicker(true)}
  onInvite={() => { if (servers && servers[0]) setView({ kind: "server", id: servers[0].id }); }}
/>
<Dashboard ... />
```
Add `import { GetStartedChecklist } from "./components/GetStartedChecklist";` at the top.

- [ ] **Step 4: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors).

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/GetStartedChecklist.tsx ui/src/App.tsx
git commit -s -m "feat(ui): Dashboard \"Get started\" checklist (onboarding safety net)"
```

---

### Task 3: Guided first-run flow — Welcome → Quick setup → hand-off

Build the full-screen `Onboarding` takeover with the first two steps (Welcome, Quick setup) plus a hand-off that drops the user on the Dashboard (where the Task-2 checklist guides them to create a server + invite a friend). Wire it into `App` as a first-run early-return, persisting `gamenest.onboardingDone`. Task 4 extends the flow with the in-flow Pick-a-game + You're-live steps.

**Files:**
- Create: `ui/src/components/Onboarding.tsx`
- Modify: `ui/src/App.tsx` (first-run trigger + persistence + veteran auto-mark)

**Interfaces:**
- Produces: `export function Onboarding(props)` (signature below). `type OnboardingStep = "welcome" | "setup" | "pick" | "live"` is declared here (Task 4 uses `"pick"`/`"live"`).
- Consumes: `Async<Setup>` + `runtimeReady` + `onRecheck` (for the embedded `SetupWizard`), and `onFinish`/`onSkip` callbacks from `App`.

- [ ] **Step 1: Create `ui/src/components/Onboarding.tsx` (Welcome + Quick setup)**

```tsx
import { useState } from "react";
import { type Async } from "../lib/api";
import { type Setup } from "../lib/api";
import { Logo } from "./icons";
import { SetupWizard } from "./SetupWizard";

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
```
Notes: the `bg-glow`/`grain` layers reuse the Phase-1 atmosphere (the normal app shell mounts them too — here they're mounted for the takeover, which renders instead of the shell). The embedded `SetupWizard` already polls `setup` via the parent and `runtimeReady` flips true when Docker connects, enabling **Continue**. (Task 4 replaces the step-2 **Continue** target from `onFinish` to advancing into the `"pick"` step.)

- [ ] **Step 2: Add the first-run trigger + persistence in `App.tsx`**

Add onboarding state near the other `useState` (~`:117-124`):
```tsx
const [onboardingDone, setOnboardingDone] = useState<boolean>(
  () => localStorage.getItem("gamenest.onboardingDone") === "true",
);
```
Add a stable finisher (with the other derivations, ~`:203`):
```tsx
function finishOnboarding() {
  localStorage.setItem("gamenest.onboardingDone", "true");
  setOnboardingDone(true);
}
```
Auto-mark existing users (so a returning user with servers never sees the flow) — add an effect alongside the other effects. **Guard it so it does not call `setState` synchronously every render** (it sets state only on the transition, and depends on the loaded servers):
```tsx
useEffect(() => {
  if (!onboardingDone && servers && servers.length > 0) {
    localStorage.setItem("gamenest.onboardingDone", "true");
    setOnboardingDone(true);
  }
}, [onboardingDone, servers]);
```
Derive the trigger (with the other derivations, after `runtimeReady`):
```tsx
const firstRun = !onboardingDone && servers !== null && servers.length === 0;
```

- [ ] **Step 3: Render `Onboarding` as a first-run early-return**

Add the takeover **after** the `EngineOffline` and `Login` early-returns (~`:188-201`) and before `const version = ...` (~`:203`). It is gated on `health.status === "ok"` implicitly (EngineOffline already returned otherwise) and on `firstRun`:
```tsx
if (firstRun) {
  return (
    <Onboarding
      setup={setup}
      runtimeReady={runtimeReady}
      onRecheck={retry}
      onFinish={finishOnboarding}
      onSkip={finishOnboarding}
    />
  );
}
```
(`firstRun` must be computed before this return — move the `firstRun`/`onboardingDone`/`runtimeReady` derivations above it as needed; `runtimeReady` already sits at ~`:204`, so hoist these three derivations above the early-returns block, right after the data hooks.) Add `import { Onboarding } from "./components/Onboarding";` at the top.

- [ ] **Step 4: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (**0 new** lint errors — confirm the new `useEffect` did not add a `react-hooks/set-state-in-effect` error; it sets state conditionally on a data transition, not synchronously on every render, so it should be clean. If lint flags it, gate the set inside the condition as shown and confirm the count stays at 7).

- [ ] **Step 5: Manual check (owner, at release)**

**Acceptance:** a brand-new user (no servers) lands on a full-screen **Welcome** (logo + promise + Get started / Skip). Get started → **Quick setup** embeds the restyled Docker wizard; **Continue** is disabled until Docker connects, then advances. Skipping or finishing drops them on the Dashboard and never auto-shows the flow again. A user who already has servers never sees it.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/Onboarding.tsx ui/src/App.tsx
git commit -s -m "feat(ui): guided first-run onboarding (welcome + quick setup)"
```

---

### Task 4: Extend the flow — Pick your first game → You're live

Carry the funnel all the way: after Quick setup, let the user pick + create their first server in-flow, then start it and show the share address ("send this to a friend") before handing off into the app.

**Files:**
- Modify: `ui/src/components/Onboarding.tsx` (add the `pick` + `live` steps)
- Modify: `ui/src/components/ConfigureServerModal.tsx` (pass the created server up)
- Modify: `ui/src/App.tsx` (provide `groups`, `servers`, start/open callbacks, `onMarkInvited` to `Onboarding`)

**Interfaces:**
- Consumes (new `Onboarding` props): `groups: GameGroup[]`, `servers: ServerSummary[] | null`, `onStartServer: (id: string) => void`, `onOpenServer: (id: string) => void`, `onMarkInvited: () => void`.
- `ConfigureServerModal`'s `onCreated` changes from `() => void` to `(server: ServerSummary) => void`.

- [ ] **Step 1: Make `ConfigureServerModal` report the created server**

In `ui/src/components/ConfigureServerModal.tsx`: change the prop type `onCreated: () => void` → `onCreated: (server: ServerSummary) => void` (both the `ConfigureServerModal` props and the inner `OptionsForm` props that thread it). In `OptionsForm.submit`, the `api.createServer(req)` call already returns the created `ServerSummary` — capture it and pass it: `const created = await api.createServer(req); onCreated(created);`. Add `ServerSummary` to the `../lib/api` import if not present.

- [ ] **Step 2: Update the existing `ConfigureServerModal` call site in `App.tsx`**

The current overlay (~`:297-305`) passes `onCreated={() => { setConfigureGroup(null); refresh(); }}`. Change it to accept (and ignore) the arg so the normal create flow is unchanged:
```tsx
onCreated={() => { setConfigureGroup(null); refresh(); }}
```
(No behavior change — it already ignores any argument; this step is just to confirm the call site still type-checks against the new `(server) => void` signature, which it does since a `() => void` is assignable. If tsc complains, write `onCreated={(_server) => { setConfigureGroup(null); refresh(); }}`.)

- [ ] **Step 3: Add the `pick` + `live` steps to `Onboarding.tsx`**

Extend the component. Add these props to the signature + type:
```tsx
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
```
Add imports: `import { GamePicker } from "./GamePicker";`, `import { ConfigureServerModal } from "./ConfigureServerModal";`, and the types `ServerSummary`, `GameGroup` (`GameGroup` from `../lib/games`). Add local state:
```tsx
const [pickerGroup, setPickerGroup] = useState<GameGroup | null>(null);
const [createdId, setCreatedId] = useState<string | null>(null);
const liveServer = createdId ? servers?.find((s) => s.id === createdId) ?? null : null;
```
Change the step-2 **Continue** button's `onClick` from `onFinish` to `() => setStep("pick")`.

Add the `pick` step (renders the existing modals; on create, advance to `live` and start the server):
```tsx
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
```
where `showPicker` is local onboarding state: add `const [showPicker, setShowPicker] = useState(false);`. Render the create-flow modals (outside the step blocks, before the closing `</div>`s) so they layer over the takeover:
```tsx
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
```
Add the `live` step (shows pull/boot progress, then the share address):
```tsx
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
```
The `live` step reads the live server from the polled `servers` (passed in), so the address/pull-progress update as the engine resolves them — no new effect needed.

- [ ] **Step 4: Wire the new props from `App.tsx`**

First add the invite helper that this task introduces (Task 2 added `invitedFriend` read-only). Change `const [invitedFriend] = useState(...)` to include the setter, and add the `markInvited` helper alongside the other derivations (~`:203`):
```tsx
const [invitedFriend, setInvitedFriend] = useState<boolean>(
  () => localStorage.getItem("gamenest.invitedFriend") === "true",
);
// ...
function markInvited() {
  localStorage.setItem("gamenest.invitedFriend", "true");
  setInvitedFriend(true);
}
```
Then update the `<Onboarding .../>` early-return (from Task 3) to pass the new props:
```tsx
if (firstRun) {
  return (
    <Onboarding
      setup={setup}
      runtimeReady={runtimeReady}
      onRecheck={retry}
      onFinish={finishOnboarding}
      onSkip={finishOnboarding}
      groups={templates.status === "ok" ? groupGames(templates.data) : []}
      servers={servers}
      onStartServer={(id) => action(id, "starting…", () => api.startServer(id))}
      onOpenServer={(id) => { finishOnboarding(); setView({ kind: "server", id }); }}
      onMarkInvited={markInvited}
    />
  );
}
```
Ensure `groupGames` is imported in `App.tsx` (it's used by the picker overlay already — confirm the import exists; if not, add `import { groupGames } from "./lib/games";`). `markInvited` is added in this task's Step 4 above; `action`/`api`/`setView`/`templates`/`servers` already exist.

Note: `onOpenServer` finishes onboarding **and** navigates to the new server's detail; the plain **Open my server** path works even if `onFinish` was already called (idempotent — it just re-sets the flag).

- [ ] **Step 5: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors); the `ConfigureServerModal` signature change type-checks at both call sites (onboarding + the App overlay).

- [ ] **Step 6: Manual check (owner, at release)**

**Acceptance:** from Quick setup → **Continue** → **Make your first server** → **Choose a game** opens the game picker → configure → create. The flow advances to **You're live!**, starts the server, shows download progress, then surfaces the share address with a **Copy** button (copying marks the "Invite a friend" checklist item done). **Open my server** lands on the new server's tabbed detail. The whole flow shows only once.

- [ ] **Step 7: Commit**

```bash
git add ui/src/components/Onboarding.tsx ui/src/components/ConfigureServerModal.tsx ui/src/App.tsx
git commit -s -m "feat(ui): onboarding — pick a game + you're-live share moment"
```

---

## Definition of Done (manual QA — run after all tasks)

Run `npm --prefix ui run build && npm --prefix ui run lint` (0 new lint errors). Then, conceptually walking a fresh user:

- [ ] First launch (no servers, `gamenest.onboardingDone` absent) → full-screen **Welcome**; Get started → **Quick setup** (restyled wizard) → **Continue** (enabled once Docker connects) → **Make your first server** → pick/configure/create → **You're live!** with the share address + Copy → **Open my server** → tabbed detail.
- [ ] **Skip for now** (from Welcome or Quick setup) → lands on the Dashboard; the **Get started checklist** is visible and reflects real progress; the guided flow does not auto-reappear.
- [ ] An existing user (≥1 server) never sees the guided flow (auto-marked done); their checklist hides once Docker + a server + a shareable address all exist.
- [ ] Copying the address (in the flow) ticks "Invite a friend"; the checklist auto-hides when all three items are done.
- [ ] `EngineOffline` / `Login` still take over correctly; the rest of the app (sidebar, tabbed detail, Settings/Account, create flow, Changelog) is unchanged.

Then invoke **superpowers:finishing-a-development-branch**.

## Self-Review

**Spec coverage (Phase 4 = "Onboarding", §5 / §65–75):**
- Guided first-run flow, shown on first launch / no servers (§67) → Task 3 trigger + Task 3/4 steps. ✓
- Step 1 Welcome (logo + promise + Get started) (§68) → Task 3. ✓
- Step 2 Quick setup wrapping the Docker `SetupWizard`, restyled (§69) → Task 1 (restyle) + Task 3 (embed). ✓
- Step 3 Pick your first game → configure → create (§70) → Task 4 (reuses GamePicker → ConfigureServerModal). ✓
- Step 4 You're live → share address + "send to a friend", then hand off (§71) → Task 4 `live` step. ✓
- Dashboard "Get started" checklist (Docker · first server · invite a friend), reflects real progress, auto-hides (§73) → Task 2. ✓
- Onboarding-complete state persists locally so it shows once (§75) → `gamenest.onboardingDone` (Tasks 2/3). ✓
- Implementer's-call open questions (§98–101): onboarding state lives in `localStorage` (chosen); the create flow stays the existing modals (reused). ✓
- Out of scope respected: no engine/API changes; no other backlog items. ✓

**Placeholder scan:** No TBD/TODO. New components have complete code; App/SetupWizard/ConfigureServerModal edits are precise locate-by-content instructions with the exact current anchors from research. The one hedge — the `groupGames` import "confirm it exists, else add" — is a verify-against-existing-file instruction with the concrete fallback given.

**Type consistency:** `Onboarding` props are introduced in Task 3 and extended in Task 4 with matching names/types at the `App` call site. `ConfigureServerModal.onCreated: (server: ServerSummary) => void` (Task 4) matches both call sites (onboarding passes `(server) =>`, App passes a `() =>` which is assignable). `GetStartedChecklist` props (Task 2) match its `App` call site. `hasShareAddress` uses the real `ServerSummary` fields (`tunnelAddress`/`externalAddress`/`relayAddress`). `OnboardingStep` union covers all four steps used.

**No-test-runner adaptation:** every task gates on build + lint (0 new errors) + a concrete owner-verified Acceptance; new effects are explicitly checked against the known `set-state-in-effect` debt so the count stays at 7.
