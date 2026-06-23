# GameNest Desktop App — UX Overhaul (Redesign + Onboarding)

**Date:** 2026-06-24
**Status:** Approved (design); implementation planning next.
**Branch base:** `feat/app-ux-overhaul`, branched from `feat/subscription-client` so the redesign builds on the current app **including the subscription UI** (account linking, vanity name, share-with-friends) — those must be carried into the new design, not lost.

## Problem

The desktop app works but feels utilitarian next to the marketing site (`gamenest-web`). Two concrete gaps:

1. **Brand/structure.** The app already shares the site's bones (zinc-950 canvas, emerald-400 accent, rounded panel cards) but uses generic **Inter** instead of the site's fonts, has **none** of the site's atmosphere (grain, glow, glassmorphism), and shows a **"G" gradient box** instead of the real hexagon logo. Navigation is a **single pane with stacked overlays** (no router): `App.tsx` mounts `GamePicker`, `ConfigureServerModal`, a full-screen `ServerDetail`, `Settings` modal, `Menu` drawer, `Changelog`, plus an inline `SetupWizard` — the user moves around by opening/closing overlays.
2. **Onboarding.** A brand-new user sees the Docker `SetupWizard` inline, and after Docker connects gets a `ReadyBanner` and an **empty server grid** — no welcome, no explanation, no funnel to a first server. Dropped in cold.

Goal: make the app feel like the site's sibling and **carry a first-timer all the way to a running server a friend can join.**

## Locked decisions (validated with the owner via mockups)

1. **Scope = reskin + navigation rework** (not a reskin-only pass). New shell, new IA, restyle across screens, plus onboarding.
2. **Shell = persistent left sidebar.** Top: logo. Then **Dashboard**, then the **servers list** (status dots), then **+ New server**, with **Settings** + **Account** pinned at the bottom. The main pane shows the active view. Replaces the overlay-stacking model.
3. **Server detail = tabbed.** Sticky header (name · status · Start/Stop) + tabs **Overview · Console · Files · Settings · Backups · Mods** (Mods = Minecraft only). Console and Files become in-pane tabs, not separate full-screen layers.
4. **Onboarding = guided first-run flow + a Dashboard "Get started" checklist** as the safety net (chosen over coach-marks-only and checklist-only).
5. **Visual language = the website's, exactly.** Same tokens/atmosphere/logo; fonts **self-hosted/bundled** (desktop app must work offline — no Google CDN).
6. **Stack unchanged:** Tauri v2 + React 19 + Tailwind v4, hand-rolled components (no UI library) — just better organized.

## Design

### 1. Visual foundation (the brand layer)

Port the site's design system into `ui/`. Source of truth = `gamenest-web`:
- **Tokens** (`gamenest-web/app/globals.css` `@theme`): `--color-canvas:#09090b`, `--color-accent:#34d399` (emerald-400), `--color-accent-strong:#10b981`. The app's `ui/src/index.css` already sets a near-identical canvas; formalize the same token names.
- **Fonts:** Bricolage Grotesque (display/headings, `--font-display`), Hanken Grotesk (sans/body, `--font-sans`), JetBrains Mono (mono/console, `--font-mono`). **Bundle them** via `@fontsource` packages (or self-hosted woff2) and load locally — the current Inter usage is replaced.
- **Atmosphere classes** (port from `gamenest-web/app/globals.css`, applied *subtly* — an app is not a landing page): `.grain` (fixed SVG-noise overlay, very low opacity), a soft `.bg-glow` radial behind the shell, `.panel` (glassmorphism card: gradient bg + hairline border + backdrop blur + `rounded-2xl`), `.divider` (gradient hairline). Skip the heavy marketing motion (scroll-reveal, marquee).
- **Logo:** add a `Logo` component to `ui/src/components/icons` (port the SVG hex mark from `gamenest-web/components/icons.tsx`). Replace all three "G" gradient boxes (`App.tsx` header, login, `Menu`) with it.
- **Polish:** a single spacing scale, consistent type hierarchy (display font for headings, sans for body, mono for code/console), tighter and more deliberate use of space.

This phase alone should deliver most of the "a designer made it" feel without touching layout logic.

### 2. Shell & navigation

A new top-level shell component owns: the **sidebar** + the **main pane**. The sidebar sections:
- **Brand** (logo + "GameNest").
- **Dashboard** nav item.
- **Servers** — the live list (each row: game icon, name, a status dot, a quick start/stop affordance); selecting one routes to its detail in the main pane.
- **+ New server** — opens the create flow (the existing `GamePicker` → `ConfigureServerModal`, restyled; may stay a modal or become a main-pane view — implementer's call, keep it 2–3 steps).
- **Settings** + **Account** pinned at the bottom (Account surfaces the GameNest Plus link/status built in the subscription work).

**Navigation model:** introduce a lightweight **view/route state** (a small typed `view` union or a minimal router) so the main pane renders Dashboard vs. a selected server vs. Settings — replacing the boolean-overlay flags in `App.tsx`. The full-screen `ServerDetail`/`ServerConsole`/`FileManager` overlays collapse into in-pane views/tabs. `EngineOffline` and the remote-mode `Login` remain whole-screen takeovers (correct as-is).

### 3. Dashboard

The sidebar's **Dashboard** = an at-a-glance overview: all servers as cards (status, quick start/stop, the share address when running), light aggregate info. It is also where the onboarding **"Get started" checklist** lives (§5).

### 4. Server detail (tabbed)

Main pane when a server is selected:
- **Sticky header:** game icon · name · status pill · Start/Stop.
- **Tabs:**
  - **Overview** — the share/connection panel (tunnel address + copy, the "Share with friends" / vanity control from the subscription work) and live CPU/RAM sparklines + quick actions.
  - **Console** — the existing `ServerConsole` (WS log stream + command input) as an in-pane tab.
  - **Files** — the existing `FileManager` as an in-pane tab.
  - **Settings** — per-server settings form.
  - **Backups** — backups list + schedules.
  - **Mods** — Minecraft only; hidden otherwise.

### 5. Onboarding

**Guided first-run flow** (shown on first launch / when no servers exist and setup is incomplete), a brief full-screen journey:
1. **Welcome** — logo + the promise ("Host a server your friends can actually join — one click, no port-forwarding") + Get started.
2. **Quick setup** — wraps the existing Docker `SetupWizard` steps (driven by the engine's `/api/setup`), restyled into the flow.
3. **Pick your first game** — the `GamePicker`, framed as "let's make your first server," → configure → create.
4. **You're live** — the server is running; show the **share address** and a "send this to a friend" moment, then hand off into the app.

**Safety net — Dashboard "Get started" checklist:** a persistent card on the Dashboard — **Set up Docker · Create your first server · Invite a friend** — that reflects real progress (derived from setup status + server count) and lets a user who exits or skips the flow resume. It auto-hides once complete.

Onboarding state (has the user finished / dismissed the flow) persists locally (engine data dir or a settings flag) so it shows once.

## Scope & phasing

One spec, built in four phases — each leaves the app shippable:

1. **Brand foundation** — fonts (bundled), tokens, atmosphere classes, `Logo`, the polish pass. Low-risk, touches the theme layer + swaps the logo. Biggest visual win per effort.
2. **Shell + navigation** — the sidebar shell, the view/route model, the Dashboard; collapse the overlay flags.
3. **Tabbed server detail** + restyle the remaining screens (Settings, Files, Console, GamePicker, ConfigureServerModal, Menu, Changelog) into the new system.
4. **Onboarding** — the guided flow + the Dashboard checklist.

## Out of scope (separate efforts)

UI-only work — the **engine/backend API is unchanged**. Also out: the rest of the product backlog — custom Windows installer, background-update toggle, 7-day analytics + geo insights, own-domain support, port-tier bump, and the homelab/VPS web edition. Marketing is its own track.

## Constraints

- Tauri v2 desktop · React 19 · **Tailwind v4** (`@theme` in CSS, no `tailwind.config.js`) · hand-rolled components (keep — no shadcn/Radix/MUI).
- **Fonts bundled, not CDN** (offline).
- **No UI test runner** exists — verification is `npm --prefix ui run build` (tsc + vite) + `npm --prefix ui run lint` + manual. Keep lint clean (7 known pre-existing `react-hooks/set-state-in-effect` errors are existing debt — don't add new ones; fixing them is welcome where touched).
- **Must preserve + restyle the subscription UI** (account linking in Settings/Account, the per-server vanity control, the share-with-friends panel) — this branch is based on `feat/subscription-client`.
- Commits are **DCO signed-off** (`git commit -s`); free AGPL core (nothing under `ee/`); match surrounding style.

## Open questions (non-blocking, decide during implementation)

- New-server flow: stays a modal vs. becomes a main-pane view — either is fine; keep it short.
- Where onboarding-complete state lives (engine setting vs. local) — implementer's call.
