# App Redesign — Phase 1: Brand Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the GameNest desktop app share the website's exact visual language — bundled brand fonts, the website's color tokens, its subtle atmosphere (grain/glow/glassmorphism/dividers), and the real hexagon logo — so it immediately feels "designed," without touching layout or navigation yet.

**Architecture:** Pure presentation-layer change in `ui/`. Port the design system from `gamenest-web` (its `app/globals.css` `@theme` + atmosphere classes + the `Logo` SVG) into the app's Tailwind v4 theme (`ui/src/index.css`) and a new `ui/src/components/icons.tsx`. Fonts are bundled via Fontsource (offline-safe). No engine/API changes; no component logic changes beyond swapping the placeholder logo and applying classes.

**Tech Stack:** Tauri v2 · React 19 · Tailwind v4 (`@theme` in CSS, no `tailwind.config.js`) · Fontsource (self-hosted fonts) · Vite.

## Global Constraints

- This is **Phase 1 of 4** (spec: `docs/specs/2026-06-24-app-ux-overhaul-design.md`). Brand foundation ONLY — do not change navigation, the overlay model, or screen structure here.
- **Fonts bundled, never CDN** (the app must work offline). Use Fontsource packages.
- **Source of truth = `C:\Users\leop1\projects\gamenest-web`** — match its tokens/atmosphere/logo exactly. Key files there: `app/globals.css` (tokens + `.grain`/`.bg-glow`/`.panel`/`.divider`), `components/icons.tsx` (`Logo`).
- Color tokens (verbatim): `--color-canvas: #09090b`, `--color-accent: #34d399`, `--color-accent-strong: #10b981`.
- Atmosphere is applied **subtly** — an app is not a marketing page. Skip the site's heavy motion (scroll-reveal, marquee).
- Stack stays: Tailwind v4, hand-rolled components, **no UI library**.
- **No UI test runner exists.** Verification per task = `npm --prefix ui run build` (tsc + vite, must pass) + `npm --prefix ui run lint` (no NEW errors; 7 pre-existing `react-hooks/set-state-in-effect` errors are known debt) + a **manual visual check** against the task's Acceptance. Tasks are implement → verify → visual-check → commit (not red/green TDD — there's nothing to assert against).
- Commits are **DCO signed-off** (`git commit -s`); free AGPL core (nothing under `ee/`); match surrounding style. Branch `feat/app-ux-overhaul`.

---

### Task 1: Bundle fonts + define theme tokens

**Files:**
- Modify: `ui/package.json` (add Fontsource deps)
- Modify: `ui/src/main.tsx` (import the font CSS)
- Modify: `ui/src/index.css` (the `@theme` block + body)

**Interfaces:**
- Produces: Tailwind utilities `font-display`, `font-sans`, `font-mono` and the `--color-canvas`/`--color-accent`/`--color-accent-strong` tokens, available app-wide for later tasks.

- [ ] **Step 1: Install the bundled fonts**

```bash
npm --prefix ui install @fontsource-variable/bricolage-grotesque @fontsource-variable/hanken-grotesk @fontsource-variable/jetbrains-mono
```
(These are the variable-font packages. If a name 404s, check the exact package on npmjs — Fontsource also ships non-variable `@fontsource/<name>`. The CSS family names exposed are `"Bricolage Grotesque Variable"`, `"Hanken Grotesk Variable"`, `"JetBrains Mono Variable"` respectively — confirm in each package's README.)

- [ ] **Step 2: Import the font CSS at the entry point**

In `ui/src/main.tsx`, add these imports at the very top (before the `./index.css` import):
```ts
import "@fontsource-variable/bricolage-grotesque"
import "@fontsource-variable/hanken-grotesk"
import "@fontsource-variable/jetbrains-mono"
```

- [ ] **Step 3: Define the tokens in `ui/src/index.css`**

Replace the current file contents with:
```css
@import "tailwindcss";

/* Design tokens — match gamenest-web. Tailwind v4 generates utilities
   (font-display, font-sans, font-mono, text-accent, …) from these. */
@theme {
  --font-display: "Bricolage Grotesque Variable", ui-sans-serif, system-ui, sans-serif;
  --font-sans: "Hanken Grotesk Variable", ui-sans-serif, system-ui, sans-serif;
  --font-mono: "JetBrains Mono Variable", ui-monospace, SFMono-Regular, monospace;

  --color-canvas: #09090b;        /* zinc-950 */
  --color-accent: #34d399;        /* emerald-400 */
  --color-accent-strong: #10b981; /* emerald-500 */
}

body {
  background-color: var(--color-canvas);
  color: #e4e4e7; /* zinc-200 */
  font-family: var(--font-sans);
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
}

::selection {
  background: rgba(52, 211, 153, 0.25);
  color: #ecfdf5;
}
```

- [ ] **Step 4: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build succeeds (tsc + vite), lint shows only the 7 known pre-existing errors / 0 new.
**Acceptance (visual):** run `npm --prefix ui run dev`, open the app — body text now renders in **Hanken Grotesk** (rounder, friendlier than Inter), and DevTools → Network shows the fonts served locally (no `fonts.googleapis.com` / `fonts.gstatic.com` requests).

- [ ] **Step 5: Commit**

```bash
git add ui/package.json ui/package-lock.json ui/src/main.tsx ui/src/index.css
git commit -s -m "feat(ui): bundle brand fonts + theme tokens (match website)"
```

---

### Task 2: Logo component + replace the placeholder "G" boxes

**Files:**
- Create: `ui/src/components/icons.tsx`
- Modify: `ui/src/App.tsx` (the header logo ~`:103` and the login logo ~`:519`)
- Modify: `ui/src/components/Menu.tsx` (the drawer logo ~`:90`)

**Interfaces:**
- Consumes: nothing.
- Produces: `export function Logo({ className }: { className?: string }): JSX.Element` — the hex brand mark; later tasks/phases reuse it.

- [ ] **Step 1: Create the Logo component**

Create `ui/src/components/icons.tsx` (port the mark verbatim from `gamenest-web/components/icons.tsx`):
```tsx
type IconProps = { className?: string }

/** The GameNest hex mark — matches the website. Stroke uses currentColor;
 *  the emerald core is fixed so the brand color is consistent. */
export function Logo({ className }: IconProps) {
  return (
    <svg viewBox="0 0 32 32" fill="none" className={className ?? "h-7 w-7"} aria-hidden>
      <path
        d="M16 3 27 9.3v13.4L16 29 5 22.7V9.3z"
        stroke="currentColor"
        strokeWidth={1.8}
        opacity={0.9}
      />
      <circle cx="16" cy="16" r="3.6" fill="#34d399" />
      <circle cx="16" cy="16" r="6.4" stroke="#34d399" strokeOpacity={0.4} />
    </svg>
  )
}
```

- [ ] **Step 2: Find and replace the three placeholder logos**

Grep first to find them: `git -C C:/Users/leop1/projects/gamehost grep -n "from-emerald-400 to-cyan-500" ui/src`. Each is an inline `div` rendering the letter "G", e.g.:
```tsx
<div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-emerald-400 to-cyan-500 text-lg font-black text-zinc-950 shadow-lg shadow-emerald-500/20">
  G
</div>
```
Replace each occurrence (in `App.tsx` header + `App.tsx` login + `Menu.tsx`) with the real logo, keeping its size and the adjacent "GameNest" wordmark untouched:
```tsx
<Logo className="h-9 w-9 text-emerald-400" />
```
Add `import { Logo } from "@/components/icons"` (or the repo's relative-import style — check the file's existing imports) to each file you edit. (Note: the app may use relative imports like `./icons` rather than the `@/` alias — match what the file already does.)

- [ ] **Step 3: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new lint errors).
**Acceptance (visual):** the header, the menu drawer, and the remote-login screen all show the **hexagon mark with the glowing emerald core** instead of a gradient "G" box.

- [ ] **Step 4: Commit**

```bash
git add ui/src/components/icons.tsx ui/src/App.tsx ui/src/components/Menu.tsx
git commit -s -m "feat(ui): real hexagon logo, replacing the placeholder G mark"
```

---

### Task 3: Atmosphere — grain, glow, panel, divider

**Files:**
- Modify: `ui/src/index.css` (add the atmosphere classes)
- Modify: `ui/src/App.tsx` (mount the grain overlay + a soft glow on the app root)

**Interfaces:**
- Consumes: the tokens from Task 1.
- Produces: CSS classes `.grain`, `.bg-glow`, `.panel`, `.divider` for this and later phases.

- [ ] **Step 1: Add the atmosphere classes to `ui/src/index.css`**

Append (ported from `gamenest-web/app/globals.css`, glow toned down for an app):
```css
/* ---- Atmosphere (subtle — this is an app, not a landing page) ---- */
.grain {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  opacity: 0.025;
  mix-blend-mode: overlay;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='160' height='160'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
}

.bg-glow {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 0;
  background:
    radial-gradient(50% 40% at 78% 0%, rgba(16, 185, 129, 0.10), transparent 70%),
    radial-gradient(40% 35% at 10% 8%, rgba(20, 184, 166, 0.07), transparent 70%);
}

/* Glassmorphism card — use in place of flat zinc-900 panels. */
.panel {
  background: linear-gradient(180deg, rgba(24, 24, 27, 0.7), rgba(9, 9, 11, 0.6));
  border: 1px solid rgba(63, 63, 70, 0.55);
  border-radius: 1rem;
  backdrop-filter: blur(8px);
}

.divider {
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(63, 63, 70, 0.7), transparent);
}
```

- [ ] **Step 2: Mount the grain + glow on the app root**

In `ui/src/App.tsx`, find the outermost layout wrapper (the `<div className="mx-auto min-h-screen max-w-6xl">` around the whole app, ~`:338`). Wrap or precede it so the atmosphere sits behind everything. The simplest: at the top of the returned tree, add the two fixed layers (they're `pointer-events:none`, so they never block clicks), and ensure the content sits above them with `relative z-10`:
```tsx
<>
  <div className="bg-glow" aria-hidden />
  <div className="grain" aria-hidden />
  <div className="relative z-10 mx-auto min-h-screen max-w-6xl">
    {/* …existing app content… */}
  </div>
</>
```
(Adapt to the file's actual root element — keep all existing children and classes; only add the two atmosphere layers and the `relative z-10` wrapper. The full-screen takeovers like `EngineOffline`/`Login`/`ServerDetail` can stay as-is — the fixed atmosphere shows through their transparent areas or sits behind; if any takeover needs the glow too, that's a later-phase polish, not now.)

- [ ] **Step 3: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: pass, 0 new lint errors.
**Acceptance (visual):** a faint film grain over the whole window and a soft emerald glow bleeding from the top-right — both barely-there, never distracting, and clicks still work everywhere (the layers don't intercept input).

- [ ] **Step 4: Commit**

```bash
git add ui/src/index.css ui/src/App.tsx
git commit -s -m "feat(ui): subtle atmosphere — grain, glow, panel + divider utilities"
```

---

### Task 4: Typography hierarchy + glassmorphism on the key cards (polish pass)

**Files:**
- Modify: `ui/src/App.tsx` (headings + server cards)
- Modify: `ui/src/components/ServerConsole.tsx` (console → mono font)
- Modify: a few high-visibility spots that show code/addresses (e.g. the connection address) → mono

**Interfaces:**
- Consumes: `font-display`/`font-mono` (Task 1), `.panel` (Task 3).

- [ ] **Step 1: Apply the display font to headings**

In `ui/src/App.tsx`, add `font-display` (and keep existing weight/size classes) to the brand wordmark and the main section headings (e.g. the "GameNest" title next to the logo, and the "+ New server" section heading / empty-state heading). Headings should read in **Bricolage Grotesque**.

- [ ] **Step 2: Apply the mono font where it belongs**

In `ui/src/components/ServerConsole.tsx`, ensure the log output + command input use `font-mono` (JetBrains Mono). Do the same for any inline server **address / connection string** shown on a card or in the header (grep for `coderaum` / `gn.` / port displays) so technical values render monospace.

- [ ] **Step 3: Glassmorphism on the primary cards**

In `ui/src/App.tsx`, change the main **server cards** (and the `ReadyBanner`/empty-state card) from their flat `bg-zinc-900/40 border border-zinc-800 rounded-2xl` styling to the `.panel` class (drop the now-redundant bg/border/rounded utilities the `.panel` class supplies; keep padding/layout utilities). Cards should now have the soft glass look with the hairline border.

- [ ] **Step 4: Verify**

```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: pass, 0 new lint errors.
**Acceptance (visual):** headings are in the display font, the console + addresses are monospace, and the server cards have the glassmorphism panel look. Side-by-side with `gamenest.cc`, the app now clearly reads as the same brand.

- [ ] **Step 5: Commit**

```bash
git add ui/src/App.tsx ui/src/components/ServerConsole.tsx
git commit -s -m "feat(ui): type hierarchy + glassmorphism cards (brand polish)"
```

---

## Self-Review

**Spec coverage (Phase 1 = "Visual foundation"):**
- Bundled fonts (display/sans/mono), offline → Task 1. ✓
- Tokens (canvas/accent/accent-strong) → Task 1. ✓
- Atmosphere (grain, glow, panel, divider), subtle → Task 3. ✓
- Real hexagon logo replacing the "G" box everywhere → Task 2. ✓
- Polish: type hierarchy + better-looking cards → Task 4. ✓
- Explicitly NOT here: sidebar/nav, tabs, onboarding (later phases) — respected. ✓

**Placeholder scan:** No TBD/TODO. The two hedges — exact Fontsource package/family names (Task 1) and the file's import style `@/` vs relative (Task 2) — are "confirm against the real package/file" instructions for external/existing facts, with concrete most-likely values given, not missing logic. Every code step shows complete code.

**Type consistency:** `Logo({ className })` defined in Task 2 and used with `className` in Tasks 2/4. Token names (`--color-canvas/accent/accent-strong`, `--font-display/sans/mono`) and class names (`.grain/.bg-glow/.panel/.divider`) are identical across Tasks 1/3/4. Font family strings identical between the import (Task 1 Step 1) and the `@theme` (Task 1 Step 3).

**No-test-runner adaptation:** every task gates on `build` + `lint` + a concrete visual Acceptance, since there's no UI test framework — stated in Global Constraints and applied per task.
