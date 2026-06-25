# Desktop Client Switch — playit.gg → GameNest Tunnel (Stage B) — Design

**Date:** 2026-06-26
**Status:** Approved (design); implementation planning next.
**Branch base:** `feat/relay-client-switch`, branched from `feat/app-ux-overhaul` (the shipped v0.5.0 redesign + the tunnel/subscription client code).

## Problem

GameNest's "friends can join" networking is **direct-first**: UPnP/direct when reachable, else a **playit.gg** relay fallback — but playit is clunky (the user leaves the app, makes a tunnel on playit's dashboard, and pastes the address back). The team's **own** frp-based tunnel is fully built into the engine + UI but **dormant** (gated behind `GAMEHOST_TUNNEL_URL`, which has no baked-in default, so users never see it). This is **Stage B** of the playit→own-relay program (A relay hardening — built; **B this** — client switch; C GameNest Plus — later): make our tunnel the **seamless automatic fallback** and retire playit.

## Locked decisions (from brainstorming 2026-06-25/26)

1. **Sharing model = direct-first, tunnel as the automatic seamless fallback.** When a shared server's port is reachable directly (UPnP) → show the direct address (no relay, no cost). When it isn't → the tunnel engages **transparently** (no extra clicks), and the share address just works. The relay is used only by users who actually need it.
2. **Remove playit.gg entirely** (UI + engine touch-points), not relegate it.
3. **Free tier stays anonymous** (`gn-…` slugs); the per-server **vanity name** (Plus) UI stays gated/dormant — that's Stage C.
4. **Gated rollout, no fallback gap.** Build now on the branch; it goes live only in the release cut **after** the owner verifies the relay (Stage A M1) on the box. Playit-removal and the baked-in relay URL **ship together** in that release, so users never lose a fallback in between.
5. **Relay is `gamenest-relay`** (frps 0.69.1, hardened in Stage A). The bundled tunnel client is `frpc`.
6. **Stack unchanged:** Tauri v2 · React 19 · Tailwind v4 · Go engine. No engine/API *protocol* break — the tunnel endpoints already exist (`/api/system/tunnel`, `use-tunnel`, vanity); this wires the client behavior + activation around them.

## Design

### 1. Auto-fallback sharing flow (UI-orchestrated)

The engine already exposes everything: per-server `useTunnel`/`tunnelAddress`, the global `TunnelStatus.configured`, the connectivity probe (`api.connectivity` + `api.testConnectivity`), and the direct fields (`externalAddress`/`shared`). The engine does **not** pick "the" address — the UI does (in `ConnectionPanel`). So the auto-fallback lives in `ConnectionPanel`:

- On a **running** server, resolve the friend-facing address:
  1. If a **direct** address is confirmed reachable (`externalAddress` + forwarded/reachability-test pass) → show it, pill **"direct"**. The tunnel is **not** engaged (save the relay hop + cost).
  2. Else, if the tunnel is **configured** → **auto-enable** it (`api.setUseTunnel(id, true)` if not already on) and show `tunnelAddress` when it resolves, pill **"via GameNest"**. This is automatic — no button.
  3. Else (tunnel not configured — pre-green-light builds only) → show the direct address as "unverified" with manual port-forward guidance (the existing not-reachable copy, minus the playit path).
- The probe → decide flow is seamless: while resolving, show "checking how friends can reach you…"; settle to the best address. Re-resolve when the server's running/connectivity state changes.
- **Cost posture:** the tunnel auto-engages only when (a) the server is running/shared **and** (b) direct isn't reachable. Direct-capable users never touch the relay. The relay's own caps + kill-switch (Stage A) bound the rest.

### 2. UI changes (`ui/src`)

- **`ConnectionPanel`** (in `ServerDetail.tsx`): rework the precedence to §1. Surface **one** primary "friends connect to" address + a status pill; drop the three-branch direct/tunnel/playit tree.
- **`TunnelShare`**: the explicit "Share with friends (no setup)" enable becomes the automatic behavior; keep the address display + copy. The **vanity-name** control stays gated on `account?.configured && account?.linked` (dormant until Stage C) — unchanged, just carried.
- **Delete `RelaySetup`** (the playit component) and its `relay?: Relay` prop threading (`ServerDetail` → `ConnectionPanel` → `RelaySetup`); remove the playit `api` calls from client use (`relay`, `relayAction`, `relayLink`, `setRelayAddress`) and the `Relay`-driven UI. (The engine endpoints may remain server-side; the client stops using/showing them — see §4 for engine removal.)
- **Onboarding** ("You're live") already derives `tunnelAddress ?? externalAddress ?? relayAddress` — once the tunnel is active it surfaces the tunnel address automatically. Drop the now-dead `relayAddress` leg.
- **Existing playit users:** if a server still has a pasted `relayAddress`, display it read-only with a one-line "GameNest now handles this automatically" nudge; don't hard-break. (Lightweight migration.)

### 3. Engine activation + `frpc` bundling

- **Bake the relay control-plane URL** in as the default for `GAMEHOST_TUNNEL_URL` (e.g. `https://cp.coderaum.com` — owner-confirm the exact value at release) so `TunnelStatus.configured` is true in shipped builds. A **client-side disable safety-valve** env (e.g. `GAMEHOST_TUNNEL_DISABLE=1`) lets the owner force the tunnel off without a rebuild.
- **Bundle the `frpc` sidecar binary** in the Tauri installer, mirroring how the engine + playit sidecars are bundled today (the engine's `frpc` locator already checks the bundle path; the binary must physically ship). Pin the `frpc` version compatible with the relay's frps 0.69.1.
- **Remove playit from the engine** (~8–10 touch-points: `internal/relay/*`, the `/api/system/relay*` routes, the `Relay` interface + `syncRelay`/`anyRelayServerRunning` in the manager, the `relay.New`/`Stop` in `main.go`, and the playit sidecar from the bundle). Keep `ServerSummary.relayAddress` for read-only migration display, or retire it — implementer's call, but don't break stored data.

### 4. Rollout gate

Everything above is **built on `feat/relay-client-switch` and held**. It becomes user-facing only in the release cut **after** the owner green-lights the relay (Stage A M1 on-box pass). Because playit-removal + the baked URL ship in the *same* release, there is **no interim window** where users have lost playit but don't yet have the tunnel. The `GAMEHOST_TUNNEL_DISABLE` valve is the emergency off.

## Out of scope

- **GameNest Plus / vanity activation** (Stage C — accounts/billing platform + entitlement issuance). The vanity UI stays dormant.
- **Anything relay-side** (Stage A — already built/branched).
- The subscription platform; marketing.

## Constraints

- Tauri v2 · React 19 · **Tailwind v4** (`@theme`, no config) · hand-rolled components · Go engine. Reuse the Phase-1 brand layer.
- **No UI test runner** — UI verification = `npm --prefix ui run build` + `npm --prefix ui run lint` (0 new errors; 7 known pre-existing `react-hooks/set-state-in-effect`) + owner visual on a release (owner can't run `dev`). **Engine** = `go test ./...` where tests exist (tunnel/connectivity/manager).
- **No engine/API protocol break** — the tunnel/connectivity endpoints already exist; this changes client behavior + activation + removes playit.
- Commits **DCO signed-off** (`git commit -s`); free **AGPL** core (nothing under `ee/`); match surrounding style. **Installer bundle size** grows by the `frpc` binary (acceptable; playit binary is removed, partially offsetting).

## Open items (owner-confirm, at/near release)

- The exact **relay control-plane URL** to bake in (Stage A box).
- The **`frpc` version** + binary source for the bundle (compatible with relay frps 0.69.1).
- Whether to **retire `relayAddress`** from the type or keep it for migration display.

## Testing & success criteria

- **Engine:** `go test ./...` green; the playit removal compiles + doesn't break the manager/sidecar lifecycle; the baked URL makes `api.tunnel()` report `configured: true`; `frpc` locates from the bundle.
- **UI:** build + lint clean (0 new); `ConnectionPanel` resolves the single best address with the right pill; `RelaySetup` and the `relay` prop chain are gone; onboarding shows the tunnel address.
- **End-to-end (owner, on a release after relay green-light):** a server with working UPnP shows a **direct** address (no relay); a server behind CGNAT/UPnP-failure **automatically** shows a **via-GameNest** address a friend can actually join — with no playit step anywhere. The `GAMEHOST_TUNNEL_DISABLE` valve turns the tunnel off.

When these hold and the relay is green-lit, the release ships and the playit→own-relay switch is live for users.
