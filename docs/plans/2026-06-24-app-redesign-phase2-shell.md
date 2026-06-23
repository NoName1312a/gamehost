# App Redesign — Phase 2: Shell + Navigation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the app's single centered column + hamburger-drawer navigation with a persistent **left sidebar** (logo · Dashboard · your servers · + New · Settings/Account/links) and a scrollable **main pane** that shows the Dashboard — the structural shell of the redesign.

**Architecture:** Refactor `ui/src/App.tsx` (one big `App()` whose navigation is `useState` flags). Extract the "Your servers" content into a new `Dashboard` component, add a new `Sidebar` component, and restructure `App`'s layout from `<header> + mx-auto max-w-6xl` into a `flex h-screen` shell: `<Sidebar/>` + `<main class="flex-1 overflow-y-auto">`. The existing overlays (GamePicker, ConfigureServerModal, ServerDetail, Settings, Changelog) stay as floating overlays launched from the sidebar/dashboard — they move *into* the pane (and ServerDetail becomes tabbed) in **Phase 3**, not here. The hamburger `Header` + `Menu` drawer are retired (their links fold into the sidebar).

**Tech Stack:** Tauri v2 · React 19 · Tailwind v4 · hand-rolled components (no UI library).

## Global Constraints

- **Phase 2 of 4** (spec: `docs/specs/2026-06-24-app-ux-overhaul-design.md`). Build the sidebar shell + Dashboard ONLY. **Do NOT** rework ServerDetail/Settings into in-pane tabs (Phase 3), and **do NOT** introduce a view/route-union — with only the Dashboard in the main pane, there's nothing to route yet (YAGNI). Server detail + Settings remain overlays/modals, just launched from the sidebar.
- **Preserve all existing behavior + the subscription UI:** ServerDetail (share/vanity/account), Settings, GamePicker→ConfigureServerModal create flow, Changelog, start/stop, the polling data hooks, EngineOffline + remote Login takeovers — all keep working unchanged. This is a navigation re-shell, not a feature change.
- Uses the Phase-1 brand layer: `font-display` for headings, `.panel`/`.divider`, the `Logo` component, `text-emerald-400` accent. Reuse those — don't reinvent styling.
- **No UI test runner.** Verify each task: `npm --prefix ui run build` (tsc + vite, must pass) + `npm --prefix ui run lint` (no NEW errors; 7 pre-existing `react-hooks/set-state-in-effect` are known debt) + the task's visual Acceptance.
- Tailwind v4, no UI library. Desktop app — a fixed-width sidebar is fine; no mobile/responsive collapse needed (YAGNI).
- Commits **DCO signed-off** (`git commit -s`). Branch `feat/app-ux-overhaul`; no new branch. Free AGPL core.

---

### Task 1: Extract the `Dashboard` component

Pure, behavior-preserving extraction: move the "Your servers" section (grid + empty state) and its helpers out of `App.tsx` into a new `Dashboard` component. App still renders it in the same place (no layout change yet). This isolates the main-pane content before the shell restructure.

**Files:**
- Create: `ui/src/components/Dashboard.tsx`
- Modify: `ui/src/App.tsx` (remove `ServerCard`/`Badge`/`statusStyle` defs ~`:72-87,:131-217`; replace the `<section>` ~`:362-409` with `<Dashboard …/>`; add the import)

**Interfaces:**
- Produces: `export function Dashboard(props: DashboardProps)` where
  ```ts
  type DashboardProps = {
    servers: ServerSummary[] | null;
    runtimeReady: boolean;
    busy: Record<string, string>;
    onNewServer: () => void;
    onOpenServer: (id: string) => void;
    onStart: (id: string) => void;
    onStop: (id: string) => void;
  }
  ```

- [ ] **Step 1: Create `ui/src/components/Dashboard.tsx`**

Move `Badge`, `statusStyle`, and `ServerCard` from `App.tsx` **verbatim** into this file (they're only used here), then add the `Dashboard` export wrapping the current `<section>`:
```tsx
import { type ReactNode } from "react";
import { type ServerSummary } from "../lib/api";
import { gameMetaFor } from "../lib/games";

function Badge({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${className}`}
    >
      {children}
    </span>
  );
}

function statusStyle(status: string): string {
  if (status === "running") return "text-emerald-400 bg-emerald-400/10 ring-emerald-400/20";
  if (status === "exited" || status === "created") return "text-amber-400 bg-amber-400/10 ring-amber-400/20";
  if (status === "not created") return "text-zinc-400 bg-zinc-400/10 ring-zinc-400/20";
  return "text-sky-400 bg-sky-400/10 ring-sky-400/20";
}

// ServerCard — moved verbatim from App.tsx (the role="button" card with the
// glyph, status Badge, download progress bar, and Start/Stop quick action).
// Keep its body EXACTLY as in App.tsx; only the surrounding file changes.
function ServerCard({
  s, busy, onOpen, onStart, onStop,
}: {
  s: ServerSummary; busy?: string; onOpen: () => void; onStart: () => void; onStop: () => void;
}) {
  /* …paste the existing ServerCard JSX body unchanged… */
}

export function Dashboard({
  servers, runtimeReady, busy, onNewServer, onOpenServer, onStart, onStop,
}: {
  servers: ServerSummary[] | null;
  runtimeReady: boolean;
  busy: Record<string, string>;
  onNewServer: () => void;
  onOpenServer: (id: string) => void;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
}) {
  return (
    <section className="px-6 py-8">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="font-display text-lg font-semibold text-zinc-100">Your servers</h2>
        <button
          onClick={onNewServer}
          disabled={!runtimeReady}
          title={runtimeReady ? "" : "Finish Docker setup first"}
          className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span className="text-base leading-none">+</span> New server
        </button>
      </div>
      {servers && servers.length > 0 ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {servers.map((s) => (
            <ServerCard
              key={s.id}
              s={s}
              busy={busy[s.id]}
              onOpen={() => onOpenServer(s.id)}
              onStart={() => onStart(s.id)}
              onStop={() => onStop(s.id)}
            />
          ))}
        </div>
      ) : (
        <div className="panel grid place-items-center py-14 text-center">
          <div className="mb-3 grid h-12 w-12 place-items-center rounded-2xl bg-zinc-900 text-2xl ring-1 ring-inset ring-zinc-800">🎮</div>
          <p className="text-zinc-300">No servers yet.</p>
          <p className="mt-1 text-sm text-zinc-600">
            {runtimeReady ? "Click “+ New server” to create your first one." : "Finish Docker setup above, then add a server."}
          </p>
          {runtimeReady && (
            <button
              onClick={onNewServer}
              className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400"
            >
              <span className="text-base leading-none">+</span> New server
            </button>
          )}
        </div>
      )}
    </section>
  );
}
```

- [ ] **Step 2: Update `App.tsx`**

Remove the now-moved `Badge`, `statusStyle`, and `ServerCard` definitions. Add `import { Dashboard } from "./components/Dashboard";`. Replace the entire `{/* Servers */} <section …>…</section>` block with:
```tsx
<Dashboard
  servers={servers}
  runtimeReady={runtimeReady}
  busy={busy}
  onNewServer={() => setShowPicker(true)}
  onOpenServer={setDetailId}
  onStart={(id) => action(id, "starting…", () => api.startServer(id))}
  onStop={(id) => action(id, "stopping…", () => api.stopServer(id))}
/>
```

- [ ] **Step 3: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors).
**Acceptance:** the app looks and behaves identically to before — same server grid, same empty state, same Start/Stop and "+ New server". This task changes file structure only.

- [ ] **Step 4: Commit**
```bash
git add ui/src/components/Dashboard.tsx ui/src/App.tsx
git commit -s -m "refactor(ui): extract Dashboard component from App"
```

---

### Task 2: Build the `Sidebar` component

Create the persistent sidebar. Standalone in this task (wired into `App` in Task 3). It owns the brand, primary nav, servers list, and the footer links that currently live in the `Menu` drawer.

**Files:**
- Create: `ui/src/components/Sidebar.tsx`
- Read (for the GitHub/Discord URLs + any icons to reuse): `ui/src/components/Menu.tsx`

**Interfaces:**
- Consumes: `gameMetaFor` (`../lib/games`), `Logo` (`./icons`), types `ServerSummary` + `AccountStatus` (`../lib/api`).
- Produces: `export function Sidebar(props: SidebarProps)` where
  ```ts
  type SidebarProps = {
    servers: ServerSummary[] | null;
    activeServerId: string | null;   // highlight the open server; null = Dashboard active
    runtimeReady: boolean;
    appVersion: string | null;
    engineVersion?: string;
    account?: AccountStatus;
    onDashboard: () => void;
    onSelectServer: (id: string) => void;
    onNewServer: () => void;
    onOpenSettings: () => void;
    onWhatsNew: () => void;
  }
  ```

- [ ] **Step 1: Read `Menu.tsx`** to copy the exact GitHub + Discord link URLs (and reuse any small SVG icons it has). The sidebar footer replaces the Menu drawer, so those links must carry over.

- [ ] **Step 2: Create `ui/src/components/Sidebar.tsx`**

```tsx
import { type ServerSummary, type AccountStatus } from "../lib/api";
import { gameMetaFor } from "../lib/games";
import { Logo } from "./icons";

const GITHUB_URL = "…"; // copy from Menu.tsx
const DISCORD_URL = "…"; // copy from Menu.tsx

function dotColor(s: ServerSummary): string {
  if (s.pulling) return "bg-sky-400";
  if (s.running) return "bg-emerald-400";
  if (s.status === "exited" || s.status === "created") return "bg-amber-400";
  return "bg-zinc-600";
}

export function Sidebar({
  servers, activeServerId, runtimeReady, appVersion, engineVersion, account,
  onDashboard, onSelectServer, onNewServer, onOpenSettings, onWhatsNew,
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
        <NavItem active={dashboardActive} onClick={onDashboard} label="Dashboard" icon="▦" />
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
        <NavItem onClick={onOpenSettings} label="Settings" icon="⚙" />
        <NavItem onClick={onOpenSettings} label={account?.linked ? "Account · Plus" : "Account"} icon="◍" />
        <NavItem onClick={onWhatsNew} label="What's New" icon="✦" />
        <a href={GITHUB_URL} target="_blank" rel="noreferrer" className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-zinc-400 transition hover:bg-zinc-800/60 hover:text-zinc-200">GitHub ↗</a>
        <a href={DISCORD_URL} target="_blank" rel="noreferrer" className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-zinc-400 transition hover:bg-zinc-800/60 hover:text-zinc-200">Discord ↗</a>
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
```
(If `AccountStatus` has no `linked` field, read `ui/src/lib/api.ts` and use the correct boolean for "is the user signed in to Plus" — fall back to a plain "Account" label if unsure. The `account` prop is optional and may be `undefined` when the platform integration is dormant — handle that, don't crash.)

- [ ] **Step 3: Verify**
```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass. (The component isn't rendered yet — Task 3 wires it; this confirms it compiles + typechecks against the real `ServerSummary`/`AccountStatus`/`gameMetaFor`.)

- [ ] **Step 4: Commit**
```bash
git add ui/src/components/Sidebar.tsx
git commit -s -m "feat(ui): add Sidebar component (brand, nav, servers, footer links)"
```

---

### Task 3: Restructure `App` into the sidebar shell

Replace the `<header> + mx-auto max-w-6xl` layout with the two-column shell, wire the sidebar, and retire the hamburger `Header` + `Menu` drawer. The existing overlays stay as floating overlays.

**Files:**
- Modify: `ui/src/App.tsx`
- Delete: `ui/src/components/Menu.tsx` (now unused)

**Interfaces:**
- Consumes: `Sidebar` (Task 2), `Dashboard` (Task 1).

- [ ] **Step 1: Swap imports**

In `App.tsx`: remove `import { Menu } from "./components/Menu";`. Add `import { Sidebar } from "./components/Sidebar";`. Remove the `Header` function definition (~`:91-111`) and the `menuOpen` state (`const [menuOpen, setMenuOpen] = useState(false);`).

- [ ] **Step 2: Restructure the main return**

Replace the main return block (the `return ( <> <div className="bg-glow"/> … </div> </> )` at ~`:336-497`) so the shell is a `flex h-screen` with the sidebar and a scrolling main pane. Keep the atmosphere layers, the banners (updateInfo / ReadyBanner / SetupWizard), the `<Dashboard/>`, ALL the overlays, and the toast — only the outer layout and the header/menu change:
```tsx
return (
  <>
    <div className="bg-glow" aria-hidden />
    <div className="grain" aria-hidden />
    <div className="relative z-10 flex h-screen overflow-hidden">
      <Sidebar
        servers={servers}
        activeServerId={detailId}
        runtimeReady={runtimeReady}
        appVersion={appVer}
        engineVersion={version}
        account={account.status === "ok" ? account.data : undefined}
        onDashboard={() => setDetailId(null)}
        onSelectServer={setDetailId}
        onNewServer={() => setShowPicker(true)}
        onOpenSettings={() => setShowSettings(true)}
        onWhatsNew={() => setWhatsNew({ title: "What's New", entries: changelogEntries })}
      />
      <main className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-5xl">
          {updateInfo && (
            <div className="mx-6 mt-6 flex items-center justify-between gap-3 rounded-2xl border border-sky-500/20 bg-sky-500/5 px-4 py-3">
              <p className="text-sm text-sky-200">
                GameNest <span className="font-semibold">v{updateInfo.version}</span> is available.
              </p>
              <button onClick={() => setShowSettings(true)} className="rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-sky-400">Update</button>
            </div>
          )}
          {runtime.status !== "loading" &&
            (runtimeReady ? <ReadyBanner runtime={runtime} /> : <SetupWizard setup={setup} onRecheck={retry} />)}
          <Dashboard
            servers={servers}
            runtimeReady={runtimeReady}
            busy={busy}
            onNewServer={() => setShowPicker(true)}
            onOpenServer={setDetailId}
            onStart={(id) => action(id, "starting…", () => api.startServer(id))}
            onStop={(id) => action(id, "stopping…", () => api.stopServer(id))}
          />
        </div>
      </main>
    </div>

    {/* Overlays — unchanged, still launched via the state flags */}
    {showPicker && templates.status === "ok" && (
      /* …existing GamePicker block verbatim… */
    )}
    {configureGroup && (/* …existing ConfigureServerModal block verbatim… */)}
    {detailServer && (/* …existing ServerDetail block verbatim… */)}
    {showSettings && (/* …existing Settings block verbatim… */)}
    {whatsNew && (/* …existing Changelog block verbatim… */)}

    {toast && (
      <div className="fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-lg border border-rose-500/30 bg-rose-500/15 px-4 py-2 text-sm text-rose-200 shadow-lg">
        {toast}
        <button onClick={() => setToast(null)} className="ml-3 text-rose-400 hover:text-rose-200">✕</button>
      </div>
    )}
  </>
);
```
Notes for this step: (a) remove the `{menuOpen && <Menu …/>}` overlay block entirely; (b) add `z-50` to the toast (it had none — prevents it slipping under the sidebar/overlays); (c) keep the `GamePicker`/`ConfigureServerModal`/`ServerDetail`/`Settings`/`Changelog` overlay blocks exactly as they are today — do not change their props; (d) `EngineOffline` and `Login` early-returns above stay untouched.

- [ ] **Step 3: Delete the dead `Menu` component**
```bash
git rm ui/src/components/Menu.tsx
```
(Confirm nothing else imports it first: `git -C C:/Users/leop1/projects/gamehost grep -n "components/Menu" ui/src` should return nothing after the import removal.)

- [ ] **Step 4: Verify**
```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors); no unused-import or missing-reference errors from the `Header`/`Menu`/`menuOpen` removals.
**Acceptance (visual):** the app now has a **left sidebar** (logo + GameNest, Dashboard, your servers with status dots, + New server, and a footer with Settings/Account/What's New/GitHub/Discord/version) beside a scrolling main pane showing the Dashboard. Clicking a server still opens its detail; "+ New server", Settings, What's New all work from the sidebar; the hamburger menu is gone; the atmosphere (grain/glow) still shows behind the shell; a transient error toast appears above everything.

- [ ] **Step 5: Commit**
```bash
git add ui/src/App.tsx
git commit -s -m "feat(ui): sidebar shell layout, retiring the hamburger menu"
```

---

## Self-Review

**Spec coverage (Phase 2 = "Shell + navigation"):**
- Persistent left sidebar: logo · Dashboard · servers (status dots) · + New · Settings/Account pinned bottom → Task 2 + Task 3. ✓
- Main pane shows the active view (Dashboard) → Task 3. ✓
- Replaces overlay-stacking *for navigation* (retires hamburger/Menu drawer) → Task 3. ✓
- Dashboard = at-a-glance servers (+ later the checklist, Phase 4) → Task 1. ✓
- Deliberately deferred per spec: ServerDetail/Settings into the pane + tabs = Phase 3; the view/route-union is unnecessary until then (only one in-pane view) — documented in Global Constraints. ✓
- Carry-forward fix applied: toast gets `z-50` (Task 3 Step 2). ✓

**Placeholder scan:** The `GITHUB_URL`/`DISCORD_URL` and `AccountStatus.linked` are "copy the real value from `Menu.tsx`/`api.ts`" instructions for existing facts (with a stated fallback), not missing logic. The `/* …verbatim… */` markers in Tasks 1 & 3 point at clearly-identified existing blocks the engineer is moving unchanged (they have `App.tsx` open) — deliberately not re-pasted to avoid transcription drift in a 60-line JSX move. Every NEW or CHANGED line shows complete code.

**Type consistency:** `Dashboard` props (Task 1) match its call sites in Tasks 1 & 3. `Sidebar` props (Task 2) match the `<Sidebar .../>` call in Task 3 exactly (`servers`, `activeServerId={detailId}`, `runtimeReady`, `appVersion={appVer}`, `engineVersion={version}`, `account`, and the five callbacks). `onOpenServer`/`onSelectServer`/`onDashboard` all funnel to the existing `setDetailId`. No new types introduced; reuses `ServerSummary`/`AccountStatus`/`Runtime` from `lib/api`.

**No-test-runner adaptation:** each task gates on build + lint + a concrete visual/behavioral Acceptance.
