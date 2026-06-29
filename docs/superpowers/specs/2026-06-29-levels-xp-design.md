# GameNest Levels / XP — Design Spec

**Date:** 2026-06-29
**Status:** Designed autonomously (owner approved the scheme + said "continue with C"); **review on return**
**Sub-project:** C of 3 (A · Accounts ✓ → B · Friends + presence ✓ → **C · Levels/XP**)
**Builds on:** `feat/accounts` (A + B). Continues on the same branch — ships with A + B after the one owner Supabase setup.

## Summary

A light, **cosmetic** progression layer: signed-in users earn XP from activity, which fills the XP bar already in their profile and raises their level. No gameplay unlocks, no leaderboard — just a fun "you've been active" badge. Reuses A's `profiles.xp` / `profiles.level` columns, so **no new DB migration** and **no owner-setup change**.

## Decisions made (review these)

| Decision | Choice | Rationale |
|---|---|---|
| What it does | **Cosmetic only** — a level badge + the XP bar; no unlocks/leaderboard | Pressure-free, fair, no balancing burden |
| Earn XP from | **+50** when you accept a friend request · **+25** per hosting *session* (a server going from stopped→running) | Rewards the two core activities (social + hosting); both hook into existing B code |
| Hosting reward shape | **+25 on the rising edge** (session start), NOT continuous per-hour accrual | Owner's proposal was "10 XP/hr"; rising-edge is the same spirit with a trivially-testable no-timer MVP. Tunable / swappable for time-accrual later |
| Curve | `level = floor(sqrt(xp/100)) + 1` → L2 @ 100, L3 @ 400, L4 @ 900, L5 @ 1600 | Gentle, slows naturally; a few sessions + friends → L3-4 |
| Anti-cheat | **Client-awarded** for the MVP (RLS lets users write their own `xp`) | Cosmetic, nothing to gain by faking; server-side enforcement (an RPC / column trigger) is noted as future, needed only if levels ever gate something |
| Live update | The sidebar's existing 15s poll also calls `refreshProfile()` | The bar/level reflect new XP within ~15s without a manual reload |

## Scope

### In scope (C)
- `ui/src/lib/xp.ts`: pure `levelForXp(xp)`, `xpProgress(xp)` (level + into/span/percent for the bar), and `awardXp(delta)` (reads the current user's xp, writes `xp + delta` and the recomputed `level`).
- `ProfileBlock.tsx` (modify): use `xpProgress(profile.xp)` for the bar percent + an "x / y XP" label, and show the level from the curve. (Replaces the placeholder `xpPercent` and its century-collapse quirk noted in A.)
- Award hooks: `friends.ts` `acceptRequest` → `awardXp(50)`; `presence.tsx` → on a hosting rising-edge (a server starts running) → `awardXp(25)`.
- `SocialSidebar.tsx`: the existing 15s poll also calls `refreshProfile()` so the level/bar update live.

### Out of scope (deferred)
- Continuous hosting-time accrual (rising-edge instead), server-creation XP, leaderboards/rankings, gameplay unlocks/perks, achievements/badges beyond the level number, and **server-side anti-cheat** (client-awarded MVP; harden with an `increment_xp` RPC + restricted `xp` writes if levels ever gate anything).

### Explicitly unchanged
- The Go engine (hosting edge is detected from the already-read `api.servers()`). Anonymous flows. The DB schema (reuses A's `profiles.xp`/`level` — no migration). A's auth + B's friends/presence contracts (C only adds calls + a display change).

## Architecture

```
ui/src/lib/xp.ts        — levelForXp / xpProgress (pure) + awardXp (supabase update on own profile)
ui/src/components/social/ProfileBlock.tsx  — (modified) renders level + bar from xpProgress
ui/src/lib/friends.ts   — (modified) acceptRequest → awardXp(50)
ui/src/lib/presence.tsx — (modified) hosting rising-edge → awardXp(25)
ui/src/components/social/SocialSidebar.tsx — (modified) 15s poll also refreshProfile()
```

`awardXp` is fire-and-forget at call sites (a failed award is silently ignored — cosmetic). It's racy under concurrent awards (last-write-wins on `xp`); acceptable for an infrequent, cosmetic counter — the future RPC makes it atomic.

## Curve (`xp.ts`)

```ts
const PER_LEVEL = 100
export function levelForXp(xp: number): number { return Math.floor(Math.sqrt(Math.max(0, xp) / PER_LEVEL)) + 1 }
export function xpForLevel(level: number): number { return (level - 1) * (level - 1) * PER_LEVEL } // inverse
export interface XpProgress { level: number; into: number; span: number; percent: number }
export function xpProgress(xp: number): XpProgress {
  const level = levelForXp(xp)
  const base = xpForLevel(level)
  const next = xpForLevel(level + 1)
  const span = next - base
  const into = Math.max(0, xp) - base
  return { level, into, span, percent: span > 0 ? Math.min(100, Math.round((into / span) * 100)) : 0 }
}
```

`awardXp(delta)`: get the session user id; read `profiles.xp`; write `{ xp: xp+delta, level: levelForXp(xp+delta) }` for that id. No-op if signed out.

## Error handling
- `awardXp` failures are swallowed (best-effort, cosmetic). Curve functions are total (guard negative xp → level 1, 0%).

## Testing
- **`xp.test.ts`:** `levelForXp` boundaries (0→1, 99→1, 100→2, 399→2, 400→3); `xpProgress` (into/span/percent at a mid-level value, and 0 XP → level 1, small percent); `awardXp` (mocked supabase) reads then writes `xp+delta` + the recomputed level for the current user; no-op when signed out.
- **`ProfileBlock.test.tsx` (update):** a profile with xp shows the right "Level N" and a non-zero bar from the curve (no longer the placeholder).
- **`friends.test.ts` (update):** `acceptRequest` calls `awardXp(50)` after the update succeeds.
- **`presence.test.tsx` (update):** a transition from no-running-server to a running server awards `awardXp(25)` once (not on every heartbeat while it stays running).

## Owner setup
**None.** C adds no providers, secrets, or migrations.

## Future
Continuous hosting-time XP, server-creation XP, achievements, a friends leaderboard, and server-side XP enforcement (atomic `increment_xp` RPC + a trigger/policy restricting direct `xp` writes) — required only if levels ever unlock or rank anything.
