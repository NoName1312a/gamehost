# App Redesign — Phase 3: Tabbed Server Detail + In-Pane Views — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the full-screen `ServerDetail`/`ServerConsole`/`FileManager` overlays into a single **in-pane tabbed server detail** (Overview · Console · Files · Settings · Backups · Mods), introduce a lightweight **view/route model** so the main pane renders Dashboard vs. a server vs. Settings vs. Account, and restyle the remaining screens (Settings, Account, GamePicker, ConfigureServerModal, Changelog) into the Phase-1 brand layer.

**Architecture:** Today (post Phase 2) the app is a sidebar shell + a main pane that always shows the `Dashboard`; everything else (`ServerDetail`, `Settings`, `Account`, the create flow, Changelog) floats as a `fixed inset-0` overlay launched from the sidebar via boolean/string flags (`detailId`, `showSettings`, `showAccount`). Phase 3 replaces those three destination flags with one typed `view` union and makes the main pane a switch. `ServerDetail` becomes a pane-filling tabbed component; `ServerConsole`/`FileManager` lose their overlay chrome and render as tab content; `Settings`/`Account` lose their modal chrome and render as in-pane views. The create flow (`GamePicker` → `ConfigureServerModal`) and `Changelog` **stay modals** (transient, layered over any view) — they only get restyled. `EngineOffline` and `Login` stay whole-screen takeovers, untouched.

**Tech Stack:** Tauri v2 · React 19 · Tailwind v4 (`@theme` in CSS, no `tailwind.config.js`) · hand-rolled components (no UI library) · Vite.

## Global Constraints

- This is **Phase 3 of 4** (spec: `docs/specs/2026-06-24-app-ux-overhaul-design.md`). Build the tabbed server detail + the view/route model + the restyle of remaining screens. **Do NOT** build onboarding (the guided first-run flow + Dashboard "Get started" checklist) — that is Phase 4.
- **Reuse the Phase-1 brand layer** already in `ui/src/index.css`: `.panel` (glassmorphism card), `.divider` (gradient hairline), `.grain`/`.bg-glow` (atmosphere, already mounted in `App`), and font utilities `font-display` (headings), `font-sans` (body, default), `font-mono` (code/console/addresses). The `Logo` component lives in `ui/src/components/icons.tsx`. Don't reinvent styling.
- **Accent literals:** the app uses `text-emerald-400` / `bg-emerald-500` / `focus:border-emerald-500` throughout. These equal the brand tokens (`--color-accent` = `#34d399` = emerald-400, `--color-accent-strong` = `#10b981` = emerald-500). **Keep using the `emerald-400`/`emerald-500` literals** to match surrounding code — do not churn them into `text-accent` utilities.
- **Preserve ALL existing behavior + the subscription UI.** The share/connection panel, "Share with friends", the per-server **vanity name** control (all inside `ConnectionPanel`/`TunnelShare` in `ServerDetail.tsx`), account linking/Plus status (`Account.tsx`), backups, schedules, mods, files, console, start/stop/delete, the polling data hooks, `EngineOffline` + remote `Login` — all keep working. This is a navigation + presentation change, not a feature change.
- **No engine/API changes.** `ui/src/lib/api.ts` is unchanged. All endpoints/types already exist.
- **No UI test runner exists.** Verification per task = `npm --prefix ui run build` (tsc + vite, must pass) + `npm --prefix ui run lint` (no NEW errors; **7 pre-existing `react-hooks/set-state-in-effect` errors are known debt** — don't add new ones; fixing them where you touch code is welcome). **Task completion gates on build + lint passing** (objective). The owner verifies visuals on the **released build** — they do **not** run the dev server — so each task's visual/behavioral Acceptance documents intent and is confirmed by the owner after the `0.4.9` release. Tasks are implement → verify (build + lint) → commit (not red/green TDD — there's nothing to assert against).
- Commits are **DCO signed-off** (`git commit -s`); free AGPL core (nothing under `ee/`); match surrounding style. Branch `feat/app-ux-overhaul` (already checked out; **no new branch**).
- Desktop app — a fixed two-column shell is fine; no mobile/responsive collapse needed (YAGNI).

---

### Task 1: Introduce the `view` route model (behavior-preserving)

Replace the three destination flags (`detailId`, `showSettings`, `showAccount`) in `App.tsx` with one typed `view` union, and teach the `Sidebar` to highlight the active destination. **This task changes navigation *state* only — every screen still renders exactly as it does today** (ServerDetail/Settings/Account stay full-screen overlays for now; Tasks 2–4 move them in-pane). This isolates the routing seam before the bigger restructure.

**Files:**
- Modify: `ui/src/App.tsx` (state + sidebar wiring + the three overlay conditions)
- Modify: `ui/src/components/Sidebar.tsx` (add `activeView`, highlight Settings/Account)

**Interfaces:**
- Produces: a `View` discriminated union (file-local to `App.tsx`) and `const [view, setView] = useState<View>(...)`. Sidebar gains an `activeView: "dashboard" | "server" | "settings" | "account"` prop. Tasks 2–4 render the main pane off `view.kind`.

- [ ] **Step 1: Define the `View` type and state in `App.tsx`**

At the top of the `App()` component, add the type (above the component or just inside the module) and replace the `detailId`/`showSettings`/`showAccount` state declarations (currently ~`:110`, `:113`, `:114`).

Add the type (module scope, near the other top-level types in the file):
```tsx
type View =
  | { kind: "dashboard" }
  | { kind: "server"; id: string }
  | { kind: "settings" }
  | { kind: "account" };
```
Remove these three lines:
```tsx
const [detailId, setDetailId] = useState<string | null>(null);
const [showSettings, setShowSettings] = useState(false);
const [showAccount, setShowAccount] = useState(false);
```
And add in their place:
```tsx
const [view, setView] = useState<View>({ kind: "dashboard" });
```
Keep `configureGroup`, `busy`, `toast`, `showPicker`, `updateInfo`, `appVer`, `whatsNew`, `authed`, `nonce`, `tick` exactly as they are — the create flow and Changelog stay flag-driven modals.

- [ ] **Step 2: Re-derive `activeServerId` + `detailServer` from `view`**

Replace the derived `detailServer` line (currently ~`:202`):
```tsx
const detailServer = detailId ? servers?.find((s) => s.id === detailId) ?? null : null;
```
with:
```tsx
const activeServerId = view.kind === "server" ? view.id : null;
const detailServer = activeServerId ? servers?.find((s) => s.id === activeServerId) ?? null : null;
```

- [ ] **Step 3: Rewire the `<Sidebar … />` call (App, ~`:209-222`)**

Change the navigation props to drive `view` (and pass the new `activeView`):
```tsx
<Sidebar
  servers={servers}
  activeView={view.kind}
  activeServerId={activeServerId}
  runtimeReady={runtimeReady}
  appVersion={appVer}
  engineVersion={version}
  account={account.status === "ok" ? account.data : undefined}
  onDashboard={() => setView({ kind: "dashboard" })}
  onSelectServer={(id) => setView({ kind: "server", id })}
  onNewServer={() => setShowPicker(true)}
  onOpenSettings={() => setView({ kind: "settings" })}
  onOpenAccount={() => setView({ kind: "account" })}
  onWhatsNew={() => setWhatsNew({ title: "What's New", entries: changelogEntries })}
/>
```

- [ ] **Step 4: Rewire the remaining `view` consumers in `App.tsx`**

Find every other use of the removed flags and repoint it:
- The **update banner** "Update" button (~`:230`): `onClick={() => setShowSettings(true)}` → `onClick={() => setView({ kind: "settings" })}`.
- The **`<Dashboard … />`** call's `onOpenServer` (~`:240`): `onOpenServer={setDetailId}` → `onOpenServer={(id) => setView({ kind: "server", id })}`.
- The **ServerDetail overlay** condition + handlers (~`:269-292`): change `{detailServer && (` → `{view.kind === "server" && detailServer && (`; change `onClose={() => setDetailId(null)}` → `onClose={() => setView({ kind: "dashboard" })}`; in `onDelete`, change `setDetailId(null)` → `setView({ kind: "dashboard" })`.
- The **Settings overlay** (~`:293-299`): `{showSettings && (` → `{view.kind === "settings" && (`; `onClose={() => setShowSettings(false)}` → `onClose={() => setView({ kind: "dashboard" })}`.
- The **Account overlay** (~`:300`): `{showAccount && <Account onClose={() => setShowAccount(false)} />}` → `{view.kind === "account" && <Account onClose={() => setView({ kind: "dashboard" })} />}`.

Leave the GamePicker, ConfigureServerModal, Changelog, and toast blocks untouched.

- [ ] **Step 5: Add `activeView` to `Sidebar.tsx`**

In the props (destructure ~`:15-17` + type ~`:18-31`), add `activeView`:
```tsx
activeView: "dashboard" | "server" | "settings" | "account";
```
(Add `activeView` to the destructured params too.) Then:
- Replace `const dashboardActive = activeServerId === null;` (~`:32`) with:
```tsx
const dashboardActive = activeView === "dashboard";
```
- The per-server row `active` (~`:58`) stays `const active = s.id === activeServerId;` (App now passes `activeServerId = null` unless on a server view, so this is correct).
- Give the footer NavItems an `active` prop (~`:89-90`):
```tsx
<NavItem onClick={onOpenSettings} label="Settings" icon="⚙" active={activeView === "settings"} />
<NavItem onClick={onOpenAccount} label={account?.linked ? "Account · Plus" : "Account"} icon="◍" active={activeView === "account"} />
```
(`NavItem` already supports an optional `active` prop and renders the emerald-ring treatment when set.)

- [ ] **Step 6: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors); no references to the removed `detailId`/`showSettings`/`showAccount` remain (tsc would flag them).
**Acceptance (behavioral):** the app behaves **identically** to before — clicking a server opens its (still full-screen) detail, Settings and Account open as before, the sidebar highlights Dashboard / the open server, and now **also** highlights Settings/Account when those overlays are open. Pure state refactor.

- [ ] **Step 7: Commit**

```bash
git add ui/src/App.tsx ui/src/components/Sidebar.tsx
git commit -s -m "refactor(ui): single view/route model, replacing nav flags"
```

---

### Task 2: Tabbed in-pane `ServerDetail` (+ Console & Files as tabs)

The core of Phase 3. Convert `ServerConsole` and `FileManager` from full-screen overlays into pane-filling components, then rebuild `ServerDetail` as a pane-filling **tabbed** view (sticky header with Start/Stop · tab bar · tab content) and render it inside `<main>` instead of as an overlay. The existing sub-panels (`ConnectionPanel`, `ResourcesPanel`, the settings `<form>`, `BackupsPanel`, `SchedulesPanel`, `ModsPanel`) move into tabs **unchanged** — only their wrappers and the surrounding chrome change.

**Files:**
- Modify: `ui/src/components/ServerConsole.tsx` (drop overlay chrome → pane-fill)
- Modify: `ui/src/components/FileManager.tsx` (drop overlay chrome → pane-fill)
- Modify: `ui/src/components/ServerDetail.tsx` (overlay → tabbed pane)
- Modify: `ui/src/App.tsx` (render ServerDetail in `<main>`; remove its overlay block; adjust `<main>` layout)

**Interfaces:**
- Consumes: `view` (Task 1), the Phase-1 `.panel` class.
- Produces: `ServerConsole` and `FileManager` now take **only** `{ server }` (no `onClose`). `ServerDetail` now takes its existing props **minus `onClose`** and renders pane-filling.

- [ ] **Step 1: Convert `ServerConsole` to pane-fill**

In `ui/src/components/ServerConsole.tsx`:
- Props: remove `onClose`. New signature:
```tsx
export function ServerConsole({ server }: { server: ServerSummary })
```
- Root element (~`:47`): change `<div className="fixed inset-0 z-50 flex flex-col bg-zinc-950">` → `<div className="flex h-full flex-col bg-zinc-950">`.
- Remove the `← Back` button element (~`:50-55`) entirely (the tab replaces it). Keep the rest of the header row (the live/disconnected status pill + `server.image`), the log viewport, and the command-input form **unchanged** — the tabbed detail header already shows the server name/status, but the console's own connection pill is still useful.
- The WebSocket effect, `lines` cap, auto-scroll, and command send stay exactly as they are.

- [ ] **Step 2: Verify Step 1 compiles**

```bash
npm --prefix ui run build
```
Expected: a **type error in `ServerDetail.tsx`** at the `<ServerConsole … onClose=… />` call site (it still passes `onClose`) — that's expected and fixed in Step 4. If there are OTHER errors in `ServerConsole.tsx` itself, fix them now. (You may skip lint until Step 7.)

- [ ] **Step 3: Convert `FileManager` to pane-fill**

In `ui/src/components/FileManager.tsx`:
- Props: remove `onClose`. New signature:
```tsx
export function FileManager({ server }: { server: ServerSummary })
```
- Outer root (~`:109`): change `<div className="fixed inset-0 z-50 flex flex-col bg-zinc-950">` → `<div className="flex h-full flex-col bg-zinc-950">`.
- Remove the `← Back` button element (~`:112-114`) entirely. Keep the title (`Files — {server.name}`), the action buttons (`+ File`, `+ Folder`, `↻`), the breadcrumb bar, the file list, and **the nested file-editor overlay (the inner `fixed inset-0 z-50` at ~`:176`) UNCHANGED** — that editor is a modal *over* the pane and should stay a true overlay.
- All file ops (`api.listFiles`/`readFile`/`writeFile`/`deleteFile`/`makeDir`) stay as-is.

- [ ] **Step 4: Rebuild `ServerDetail` as a tabbed pane**

In `ui/src/components/ServerDetail.tsx`:

**(a)** Add a `Tab` type at module scope (near the other top-level consts, e.g. above `statusStyle`):
```tsx
type Tab = "overview" | "console" | "files" | "settings" | "backups" | "mods";
```

**(b)** In the props type + destructure (~`:768-792`), **remove `onClose`** (it's no longer used — the sidebar handles navigation). Keep all other props (`server`, `template`, `relay`, `tunnel`, `account`, `busy`, `onChanged`, `onStart`, `onStop`, `onDelete`).

**(c)** Replace the two overlay-toggle state lines (~`:810-811`):
```tsx
const [showConsole, setShowConsole] = useState(false);
const [showFiles, setShowFiles] = useState(false);
```
with the tab state + tab list (place after the existing `saved` state):
```tsx
const [tab, setTab] = useState<Tab>("overview");
const showMods = template?.runtime === "java";
const tabs: { id: Tab; label: string }[] = [
  { id: "overview", label: "Overview" },
  { id: "console", label: "Console" },
  { id: "files", label: "Files" },
  { id: "settings", label: "Settings" },
  { id: "backups", label: "Backups" },
  ...(showMods ? [{ id: "mods" as Tab, label: "Mods" }] : []),
];
```
Keep `name`/`port`/`memory`/`vars`/`saving`/`saveError`/`saved` state, the `save()` handler, and the `field`/`label`/`status`/`meta` consts (~`:830-834`) — the Settings tab still uses them.

**(d)** Replace the **entire `return (…)`** (currently ~`:836-1031`, the `<div className="fixed inset-0 z-40 …">` overlay) with the pane-filling tabbed structure below. **Move the existing sub-panels and the settings `<form>` verbatim** where indicated (they are unchanged — only their `<section>` wrapper class changes from `rounded-2xl border border-zinc-800 bg-zinc-900/30 p-5` to `panel p-5`, and the `<h3>` gains `font-display`):

```tsx
return (
  <div className="flex h-full flex-col">
    {/* Sticky header: icon · name · status · Start/Stop */}
    <header className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-zinc-800/80 bg-zinc-950/70 px-6 py-3 backdrop-blur">
      <div className="flex min-w-0 items-center gap-3">
        <div className={`grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-gradient-to-br ${meta.gradient} text-base`}>
          {meta.glyph}
        </div>
        <div className="min-w-0">
          <h2 className="truncate font-display text-lg font-semibold text-zinc-100">{server.name}</h2>
          <p className="text-xs text-zinc-600">{server.game}</p>
        </div>
        <span
          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${statusStyle(server.status)}`}
        >
          {status}
        </span>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {server.running ? (
          <button onClick={onStop} disabled={!!busy} className={ghostBtn}>Stop</button>
        ) : (
          <button onClick={onStart} disabled={!!busy} className={primaryBtn}>Start</button>
        )}
      </div>
    </header>

    {/* Tab bar */}
    <nav className="flex shrink-0 items-center gap-1 border-b border-zinc-800/80 px-4">
      {tabs.map((t) => (
        <button
          key={t.id}
          onClick={() => setTab(t.id)}
          className={`relative px-3 py-2.5 text-sm font-medium transition ${
            tab === t.id ? "text-zinc-100" : "text-zinc-500 hover:text-zinc-300"
          }`}
        >
          {t.label}
          {tab === t.id && <span className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-emerald-400" />}
        </button>
      ))}
    </nav>

    {/* Tab content */}
    <div className="min-h-0 flex-1">
      {tab === "overview" && (
        <div className="h-full overflow-y-auto px-6 py-6">
          <div className="mx-auto max-w-3xl space-y-5">
            {server.pulling && (
              /* …move the pulling banner verbatim from the old lines ~861-872… */
            )}
            <section className="panel p-5">
              <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Connection &amp; sharing</h3>
              <ConnectionPanel s={server} relay={relay} tunnel={tunnel} account={account} onChanged={onChanged} />
            </section>
            <section className="panel p-5">
              <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Resources</h3>
              <ResourcesPanel s={server} />
            </section>
          </div>
        </div>
      )}

      {tab === "console" && <ServerConsole server={server} />}
      {tab === "files" && <FileManager server={server} />}

      {tab === "settings" && (
        <div className="h-full overflow-y-auto px-6 py-6">
          <div className="mx-auto max-w-3xl space-y-5">
            <section className="panel p-5">
              <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Server settings</h3>
              <form onSubmit={save} className="space-y-4">
                {/* …move the existing form body verbatim from the old lines ~918-1002
                     (Server name, Host port + Memory grid, the variables.map block,
                     the no-template + running-restart + saveError notices, and the
                     Save changes / Saved ✓ row). Unchanged. … */}
              </form>
            </section>
            <section className="panel p-5">
              <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-rose-300">Danger zone</h3>
              <p className="mb-3 text-sm text-zinc-400">Permanently delete this server and all of its data. This can't be undone.</p>
              <button
                onClick={onDelete}
                disabled={!!busy}
                className="rounded-lg border border-rose-500/30 px-3 py-1.5 text-sm text-rose-300 hover:bg-rose-500/10 disabled:opacity-50"
              >
                Delete server
              </button>
            </section>
          </div>
        </div>
      )}

      {tab === "backups" && (
        <div className="h-full overflow-y-auto px-6 py-6">
          <div className="mx-auto max-w-3xl space-y-5">
            <section className="panel p-5">
              <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Backups</h3>
              <BackupsPanel s={server} />
            </section>
            <section className="panel p-5">
              <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Schedules</h3>
              <SchedulesPanel s={server} onChanged={onChanged} />
            </section>
          </div>
        </div>
      )}

      {tab === "mods" && showMods && (
        <div className="h-full overflow-y-auto px-6 py-6">
          <div className="mx-auto max-w-3xl space-y-5">
            <section className="panel p-5">
              <h3 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">Mods &amp; plugins</h3>
              <ModsPanel s={server} onChanged={onChanged} />
            </section>
          </div>
        </div>
      )}
    </div>
  </div>
);
```

**(e)** Delete the old Actions bar (the `{/* Actions */}` `<section>`, old ~`:874-898`) and the two overlay mounts at the end (old ~`:1028-1029`, `{showConsole && …}` / `{showFiles && …}`) — Start/Stop now live in the header, Delete in the Settings tab, and Console/Files are tabs. The `ServerConsole`/`FileManager` imports stay (now used as tab content).

- [ ] **Step 5: Render `ServerDetail` inside `<main>` (App.tsx)**

Change the `<main>` element (~`:223`) and its contents. Replace:
```tsx
<main className="flex-1 overflow-y-auto">
  <div className="mx-auto max-w-5xl">
    {/* update banner, ReadyBanner/SetupWizard, <Dashboard/> */}
  </div>
</main>
```
with a switch that renders the server detail pane-filling, else the dashboard (note `<main>` becomes a flex column that hides its own overflow — each branch scrolls itself):
```tsx
<main className="flex min-h-0 flex-1 flex-col overflow-hidden">
  {view.kind === "server" && detailServer ? (
    <ServerDetail
      key={detailServer.id}
      server={detailServer}
      template={templates.status === "ok" ? templates.data.find((t) => t.id === detailServer.templateId) : undefined}
      relay={relay.status === "ok" ? relay.data : undefined}
      tunnel={tunnel.status === "ok" ? tunnel.data : undefined}
      account={account.status === "ok" ? account.data : undefined}
      busy={busy[detailServer.id]}
      onChanged={() => { refresh(); retry(); }}
      onStart={() => action(detailServer.id, "starting…", () => api.startServer(detailServer.id))}
      onStop={() => action(detailServer.id, "stopping…", () => api.stopServer(detailServer.id))}
      onDelete={() => {
        if (confirm(`Delete "${detailServer.name}" and its data? This can't be undone.`)) {
          action(detailServer.id, "deleting…", () => api.deleteServer(detailServer.id));
          setView({ kind: "dashboard" });
        }
      }}
    />
  ) : (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-5xl">
        {/* …keep the existing update banner block (~:225-232) verbatim… */}
        {/* …keep the existing ReadyBanner/SetupWizard line (~:233-234) verbatim… */}
        <Dashboard
          servers={servers}
          runtimeReady={runtimeReady}
          busy={busy}
          onNewServer={() => setShowPicker(true)}
          onOpenServer={(id) => setView({ kind: "server", id })}
          onStart={(id) => action(id, "starting…", () => api.startServer(id))}
          onStop={(id) => action(id, "stopping…", () => api.stopServer(id))}
        />
      </div>
    </div>
  )}
</main>
```
Then **delete the standalone `ServerDetail` overlay block** (the `{view.kind === "server" && detailServer && ( <ServerDetail … onClose=… /> )}` you edited in Task 1, ~`:269-292`) — it's now rendered inside `<main>` above, without `onClose`.

- [ ] **Step 6: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors); no leftover `onClose`/`showConsole`/`showFiles` references.

- [ ] **Step 7: Manual visual/behavioral check**

Run `npm --prefix ui run dev`. **Acceptance:**
- Selecting a server shows its detail **inside the main pane, with the sidebar still visible** (no full-screen takeover).
- A **sticky header** shows the game glyph, name (display font), status pill, and a working **Start/Stop** button.
- A **tab bar** shows Overview · Console · Files · Settings · Backups, plus **Mods only for a Minecraft/Java server**. The active tab has the emerald underline.
- **Overview** shows the Connection & sharing panel (incl. Share-with-friends + vanity when Plus-linked) and the CPU/RAM sparklines, each in a glass `.panel`.
- **Console** streams logs and accepts commands (when RCON-capable); **Files** browses/edits; both fill the pane with no "Back" button.
- **Settings** tab edits name/port/memory/variables and saves; its **Danger zone** deletes the server (returns to Dashboard).
- **Backups** lists/creates/restores backups and edits schedules.
- Switching servers in the sidebar swaps the detail; switching tabs is instant.

- [ ] **Step 8: Commit**

```bash
git add ui/src/components/ServerDetail.tsx ui/src/components/ServerConsole.tsx ui/src/components/FileManager.tsx ui/src/App.tsx
git commit -s -m "feat(ui): in-pane tabbed server detail (console/files become tabs)"
```

---

### Task 3: `Settings` → in-pane view + brand restyle

Convert `Settings` from a centered modal to an in-pane view rendered in `<main>` for `view.kind === "settings"`, and restyle its sections into glass `.panel` cards with display-font headings. All settings behavior (updates, remote access, supporter key, off-site backups, diagnostics, danger zone, users) is preserved.

**Files:**
- Modify: `ui/src/components/Settings.tsx`
- Modify: `ui/src/App.tsx` (render Settings in `<main>`; remove its overlay block)

**Interfaces:**
- Consumes: `view` (Task 1), `.panel`/`.divider`.
- Produces: `Settings` now takes `{ engineVersion?: string; initialUpdate?: UpdateInfo | null }` (no `onClose`).

- [ ] **Step 1: Strip the modal chrome from `Settings.tsx`**

- Props (~`:6-14`): remove `onClose`. New signature:
```tsx
export function Settings({
  engineVersion,
  initialUpdate,
}: {
  engineVersion?: string;
  initialUpdate?: UpdateInfo | null;
})
```
- Remove the **body scroll-lock** `useEffect` (~`:82-88`, the one setting `document.body.style.overflow = "hidden"`) — it's modal-only.
- Replace the modal wrapper (~`:244-248`):
```tsx
<div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
  <div className="max-h-[calc(100vh-3rem)] w-full max-w-md overflow-y-auto overscroll-contain rounded-xl border border-zinc-800 bg-zinc-900 p-6" onClick={(e) => e.stopPropagation()}>
```
with an in-pane scroll container:
```tsx
<div className="h-full overflow-y-auto">
  <div className="mx-auto max-w-2xl px-6 py-8">
```
(The matching closing `</div></div>` at the end of the component stays — you're swapping two opening `<div>`s and dropping the backdrop `onClick`/`stopPropagation`.)
- Header (~`:249-254`): remove the `✕` close button. Make the title an in-pane page heading:
```tsx
<header className="mb-6">
  <h1 className="font-display text-2xl font-semibold text-zinc-100">Settings</h1>
</header>
```

- [ ] **Step 2: Restyle the sections into `.panel` cards**

The body currently is a flat list of sections separated by `mt-5 border-t border-zinc-800 pt-4`. Convert **each** section (Version info, Updates, Remote access, Supporter, Off-site backups, Diagnostics, Danger zone, Users) into its own glass card. Wrap the section group as `space-y-5` and give each section:
```tsx
<section className="panel p-5">
  <h2 className="mb-3 font-display text-sm font-semibold uppercase tracking-wide text-zinc-400">…section title…</h2>
  …existing section body unchanged…
</section>
```
- Drop the now-redundant `mt-5 border-t border-zinc-800 pt-4` separators (the `.panel` cards + `space-y-5` provide the separation).
- Keep the **Danger zone** heading rose (`text-rose-300`) and its destructive button styling.
- Apply `font-mono` to value displays already shown as code (app/engine versions in the version `<dl>`, the remote-access URL, the supporter key — these already use `font-mono` in spots; ensure version numbers + URLs are mono).
- Keep all inputs/buttons as-is (they already use the emerald accent); only the card shells + headings change. **Do not touch any of the `api.*` calls, the `useEffect` data-load, or the component's state.**

- [ ] **Step 3: Render `Settings` in `<main>` (App.tsx)**

In the `<main>` switch from Task 2, add a `settings` branch between the `server` branch and the dashboard fallback:
```tsx
) : view.kind === "settings" ? (
  <Settings engineVersion={version} initialUpdate={updateInfo} />
) : (
```
(Settings' own root is `h-full overflow-y-auto`, so it fills the pane and scrolls itself.) Then **delete the Settings overlay block** (~`:293-299`).

- [ ] **Step 4: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors); no leftover `onClose` reference in `Settings.tsx`.

- [ ] **Step 5: Manual check**

**Acceptance:** clicking **Settings** in the sidebar shows the Settings page **in the main pane** (sidebar visible, Settings nav item highlighted), as a stack of glass `.panel` cards with display-font headings. Every control still works (check for updates, toggle remote access, redeem/clear a supporter key, set off-site dir, toggle diagnostics, the danger-zone purge, and — if owner — the users list). No backdrop, no `✕`; you leave by clicking another sidebar item.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/Settings.tsx ui/src/App.tsx
git commit -s -m "feat(ui): Settings as an in-pane view, restyled to the brand layer"
```

---

### Task 4: `Account` → in-pane view + brand restyle

Same treatment for `Account`: modal → in-pane view rendered for `view.kind === "account"`, restyled. Account linking / Plus status / coming-soon states are preserved.

**Files:**
- Modify: `ui/src/components/Account.tsx`
- Modify: `ui/src/App.tsx` (render Account in `<main>`; remove its overlay block)

**Interfaces:**
- Consumes: `view` (Task 1), `.panel`.
- Produces: `Account` now takes **no props** (`export function Account()`).

- [ ] **Step 1: Strip the modal chrome from `Account.tsx`**

- Props (~`:9`): change `export function Account({ onClose }: { onClose: () => void })` → `export function Account()`.
- Remove the **body scroll-lock** `useEffect` (~`:17-23`).
- Replace the modal wrapper (~`:60-64`):
```tsx
<div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
  <div className="max-h-[calc(100vh-3rem)] w-full max-w-md overflow-y-auto overscroll-contain rounded-xl border border-zinc-800 bg-zinc-900 p-6" onClick={(e) => e.stopPropagation()}>
```
with:
```tsx
<div className="h-full overflow-y-auto">
  <div className="mx-auto max-w-md px-6 py-8">
```
(Keep the matching trailing `</div></div>`.)
- Header (~`:65-77`): remove the `✕` close button; keep the `font-display` title and the "Plus" badge. (The title `<h2 className="font-display …">` is already correct — leave it, or bump to `text-2xl` to match the Settings page heading.)
- Wrap the main account card body in `panel p-5` instead of the old `rounded-xl border border-zinc-800 bg-zinc-900` shell. Keep all three states (linked / configured-not-linked / coming-soon) and their buttons unchanged.

- [ ] **Step 2: Render `Account` in `<main>` (App.tsx)**

In the `<main>` switch, add an `account` branch:
```tsx
) : view.kind === "account" ? (
  <Account />
) : (
```
Then **delete the Account overlay block** (~`:300`).

- [ ] **Step 3: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors); no leftover `onClose` in `Account.tsx`.

- [ ] **Step 4: Manual check**

**Acceptance:** clicking **Account** in the sidebar shows the Account page in the main pane (Account nav item highlighted). The correct state renders for the engine's account config: signed-in + Sign out, or the link-code form, or the "GameNest Plus is on the way" coming-soon card with the `Logo`. Linking/unlinking still works when configured.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/Account.tsx ui/src/App.tsx
git commit -s -m "feat(ui): Account as an in-pane view, restyled to the brand layer"
```

---

### Task 5: Restyle the create-flow modals (`GamePicker` + `ConfigureServerModal`)

These two **stay modals** (the create flow is a transient, focused task that overlays whatever view is active) — only restyle them into the brand system: display-font headings, consistent inputs/buttons, the accent, and a card shell consistent with the rest of the app. Behavior is unchanged.

**Files:**
- Modify: `ui/src/components/GamePicker.tsx`
- Modify: `ui/src/components/ConfigureServerModal.tsx`

**Interfaces:** none change — both keep their current props (`GamePicker`: `{ groups, onPick, onClose }`; `ConfigureServerModal`: `{ group, onClose, onCreated }`).

- [ ] **Step 1: Restyle `GamePicker.tsx`**

- Modal card (~`:99-103`): keep the command-palette behavior; update the card shell to a consistent solid-glass look. Change `rounded-2xl border border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/40` → `rounded-2xl border border-zinc-800 bg-zinc-900/90 shadow-2xl shadow-black/40 backdrop-blur` (the backdrop already blurs; a solid-ish card reads better than full glass over it — do **not** use the `.panel` class here).
- Give any section/empty-state heading text `font-display` where it's a heading.
- Leave the search input, keyboard nav, `Thumb` tiles, category accents, and the footer hint as-is (they already match the system).

- [ ] **Step 2: Restyle `ConfigureServerModal.tsx`**

- Modal card (~`:177`): `rounded-2xl border border-zinc-800 bg-zinc-950 shadow-2xl` → keep, but ensure the header title uses `font-display` (the "New {group.name} server" `<h2>`/heading ~`:184-190`).
- The module-level field consts (~`:6-8`):
```tsx
const field = "w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500";
const labelCls = "mb-1 block text-xs font-medium text-zinc-400";
```
are already on-system — leave them. Just ensure the **section/edition headings** and the submit button area read consistently (submit button already `bg-emerald-500 … text-zinc-950 hover:bg-emerald-400`). Edition tiles already use `hover:border-emerald-500/40` — leave them.
- Do not change `EditionPicker`/`OptionsForm` logic or the `api.createServer` call.

- [ ] **Step 3: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors).

- [ ] **Step 4: Manual check**

**Acceptance:** clicking **+ New server** opens the game picker (command-palette modal) with display-font headings and a card consistent with the app; picking a game opens the configure modal (edition picker when multi-edition, else the options form); creating a server works and lands you back on the Dashboard with the new server. Keyboard nav in the picker (↑/↓/Enter/Esc) still works.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/GamePicker.tsx ui/src/components/ConfigureServerModal.tsx
git commit -s -m "feat(ui): restyle the create-server flow modals to the brand layer"
```

---

### Task 6: Restyle the `Changelog` ("What's New") modal

`Changelog` stays a modal (transient, layered over any view) — restyle only.

**Files:**
- Modify: `ui/src/components/Changelog.tsx`

**Interfaces:** unchanged (`{ title, subtitle, entries, onClose }`).

- [ ] **Step 1: Restyle `Changelog.tsx`**

- Modal card (~`:63-65`): `rounded-2xl border border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/40` → `rounded-2xl border border-zinc-800 bg-zinc-900/90 shadow-2xl shadow-black/40 backdrop-blur`.
- Title (~`:67-79`): give the `title` `<h2>` `font-display`.
- Between version `<section>`s, you may replace the existing inter-section borders with the `.divider` hairline (a `<div className="divider my-4" />`) for the brand look — optional but preferred.
- Keep the `typeColor` labels, the `renderInline` markdown, the bullets, the Escape-to-close `useEffect`, and the "Got it" button (already `bg-emerald-500 … hover:bg-emerald-400`) as-is.

- [ ] **Step 2: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors).

- [ ] **Step 3: Manual check**

**Acceptance:** clicking **What's New** in the sidebar opens the changelog modal with a display-font title and the brand card; versions/entries render correctly; Escape and "Got it" close it.

- [ ] **Step 4: Commit**

```bash
git add ui/src/components/Changelog.tsx
git commit -s -m "feat(ui): restyle the What's New changelog modal to the brand layer"
```

---

### Task 7: Panelize the `Login` + `EngineOffline` takeover cards

A Phase-1 carry-forward (logged in the SDD progress ledger): the two full-screen takeovers — `EngineOffline` (engine unreachable) and the remote-mode `Login` — still use flat `bg-zinc-900`/`border-zinc-800` cards while the rest of the app is on the brand layer. They **stay whole-screen takeovers** (the early-return components in `App.tsx`); restyle their inner cards only.

**Files:**
- Modify: `ui/src/App.tsx` (the `Login` component ~`:322-366` and the `EngineOffline` component ~`:368-394`)

**Interfaces:** none change — both are file-internal components with identical behavior.

- [ ] **Step 1: Panelize `EngineOffline`** (~`:368-394`)

Swap the flat card shell (the `rounded-* border border-zinc-800 bg-zinc-900*` wrapper around the message) for the `.panel` class, and give its heading `font-display`. Keep the logo/icon, the error message, and the `onRetry` button (emerald) exactly as they are; the outer full-screen centering stays.

- [ ] **Step 2: Panelize `Login`** (~`:322-366`)

Same treatment: the login card shell → `.panel`, the heading → `font-display`. Keep the `Logo`, the password `<input>` (`focus:border-emerald-500`), the submit button, the error display, and the `onLoggedIn` flow unchanged.

- [ ] **Step 3: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors).

- [ ] **Step 4: Manual check**

**Acceptance (owner, at release):** when the engine is unreachable, the EngineOffline takeover shows a glass `.panel` card with a display-font heading + Retry; in remote mode, the Login takeover shows a `.panel` card. Both still fully take over the screen.

- [ ] **Step 5: Commit**

```bash
git add ui/src/App.tsx
git commit -s -m "feat(ui): panelize the Login + EngineOffline takeover cards"
```

---

## Definition of Done (manual QA — run after all tasks)

Walk the whole app once with `npm --prefix ui run dev`, then `npm --prefix ui run build && npm --prefix ui run lint` one final time (0 new lint errors). Confirm:

- [ ] Sidebar nav routes the **main pane** (no center-screen overlays) for: Dashboard, each server's tabbed detail, Settings, Account. The active item is highlighted in every case.
- [ ] Server detail tabs all work: Overview (share + vanity + sparklines), Console (live logs + commands), Files (browse + edit + the editor sub-modal), Settings (save + danger-zone delete), Backups (backups + schedules), Mods (Minecraft only — hidden for non-Java games).
- [ ] **Subscription UI intact:** Share-with-friends + per-server vanity name (Overview tab) and account linking / Plus status (Account view) all function.
- [ ] Create flow (+ New server → pick → configure → create) works and remains a modal.
- [ ] Changelog opens as a modal; the error toast still appears above everything (`z-50`).
- [ ] `EngineOffline` and remote `Login` still take over the full screen when triggered, now with brand-styled (`.panel`) cards (Task 7).
- [ ] Atmosphere (grain/glow) still shows behind the shell; brand fonts + hex logo intact.

Then invoke **superpowers:finishing-a-development-branch**.

## Self-Review

**Spec coverage (Phase 3 = "Tabbed server detail + restyle the remaining screens"):**
- View/route model replacing the boolean-overlay nav (spec §47) → Task 1. ✓
- Tabbed server detail: sticky header (icon · name · status · Start/Stop) + tabs Overview · Console · Files · Settings · Backups · Mods (Mods = Minecraft only) (spec §53-64) → Task 2. ✓
- Console + Files collapse from full-screen overlays into in-pane tabs (spec §47) → Task 2. ✓
- Overview = share/connection + vanity (subscription UI preserved) + CPU/RAM sparklines (spec §58) → Task 2 (moves `ConnectionPanel` + `ResourcesPanel`). ✓
- Restyle remaining screens into the new system: Settings → Task 3; Account → Task 4; GamePicker + ConfigureServerModal → Task 5; Changelog → Task 6; Console/Files/ServerDetail sections → restyled within Task 2. (Menu was already deleted in Phase 2.) ✓
- `EngineOffline`/`Login` remain whole-screen takeovers (spec §47); their cards are panelized to the brand layer (Phase-1 carry-forward, logged in the SDD ledger) → Task 7. ✓
- Onboarding explicitly deferred to Phase 4 → stated in Global Constraints. ✓

**Placeholder scan:** The `/* …move verbatim… */` markers (the pulling banner in Task 2, the settings `<form>` body in Task 2, and the kept App banner blocks in Task 2 Step 5) point at clearly-identified existing blocks the engineer relocates **unchanged** — they have the files open, and re-pasting 80+ lines of unchanged JSX risks transcription drift (same convention as the Phase 2 plan). Every NEW or CHANGED line shows complete code. No TBD/TODO.

**Type consistency:**
- `View` (Task 1) is `{kind:"dashboard"} | {kind:"server";id} | {kind:"settings"} | {kind:"account"}`; `Sidebar.activeView` is the matching string union `"dashboard"|"server"|"settings"|"account"` and App passes `view.kind` (assignable). ✓
- `ServerConsole`/`FileManager` drop to `{ server }` (Task 2) and are called with only `server=` from `ServerDetail`. ✓
- `ServerDetail` drops `onClose` (Task 2); App's `<ServerDetail/>` call (Task 2 Step 5) passes no `onClose`. ✓
- `Settings` → `{ engineVersion?, initialUpdate? }` (Task 3); App calls `<Settings engineVersion={version} initialUpdate={updateInfo} />`. ✓
- `Account` → no props (Task 4); App calls `<Account />`. ✓
- Sub-panel call sites moved into tabs match their current signatures verbatim: `ConnectionPanel{ s, relay, tunnel, account, onChanged }`, `ResourcesPanel{ s }`, `BackupsPanel{ s }`, `SchedulesPanel{ s, onChanged }`, `ModsPanel{ s, onChanged }`. ✓
- Mods gating reuses the existing `template?.runtime === "java"` predicate. ✓

**No-test-runner adaptation:** every task gates on `build` + `lint` (0 new errors) + a concrete visual/behavioral Acceptance, since there is no UI test framework — stated in Global Constraints and applied per task. A holistic Definition-of-Done QA pass covers the cross-cutting navigation change before the finishing skill.
