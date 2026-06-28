# GameNest Accounts Foundation — Design Spec

**Date:** 2026-06-28
**Status:** Approved (design), ready for implementation plan
**Sub-project:** A of 3 (A · Accounts → B · Friends + presence → C · Levels/XP)

## Summary

Add **optional** user accounts to the GameNest desktop app, Modrinth-style. Anonymous use — including the zero-signup "friends can join" hosting — stays fully working. Signing in unlocks a social/gamification layer that lands incrementally: this sub-project (A) delivers sign-in (Discord + email), a profile, and the right-hand **social sidebar** that later hosts friends (B) and levels/XP (C).

This is the first of three independently shippable sub-projects. A is the prerequisite for B and C. B and C each get their own spec → plan → implementation cycle.

## Goal

A signed-in GameNest user has a persistent identity (username + avatar) and a profile shown in a right-hand sidebar, signing in with either Discord or email/password — while anonymous users notice no change.

## Settled decisions

| Decision | Choice | Why |
|---|---|---|
| Account requirement | **Optional** | Preserve zero-friction anonymous hosting + "friends can join"; matches Modrinth |
| Sign-in methods | **Discord OAuth + email/password** | Discord = one-click + auto identity for gamers; email = inclusive fallback |
| Where identity lives | **In-app via `supabase-js`** | Native in-app UX; fastest; makes friends/presence (B) easy; engine stays decoupled |
| UI placement | **Right social sidebar** (Modrinth model) | Houses profile now, friends + presence next |
| Backend | **Existing Supabase project** (reused, the one running web auth + the waitlist) | No new infra; auth already configured |

## Scope

### In scope (A)
- `supabase-js` client in the React UI, pointed at the existing Supabase project.
- Email sign-up (email, password, **chosen username**) + sign-in.
- Discord OAuth sign-in via a **loopback PKCE** flow (desktop-safe).
- A `profiles` table + RLS + a new-user trigger.
- Session persistence across app launches; sign-out.
- The **right social sidebar** with two states: signed-out prompt, signed-in profile (avatar, username, **Level**, **XP bar**).
- A profile/account menu: sign-out + **username rename**.
- The friends area is rendered as a labeled placeholder only (its content is B).

### Out of scope (deferred)
- **B:** friends (add/accept/remove), presence ("hosting Craftoria"), the live friends list.
- **C:** XP-earning rules, leveling logic. `level`/`xp` exist in the schema and display, but stay static at `1`/`0` until C — **no migration later**.
- Converging the social account with the dormant **Plus device-link** (engine-side entitlements). They coexist; convergence is a later, separate decision.
- Account deletion (recommended fast-follow; see Future).
- Hardened token storage (see Future).

### Explicitly unchanged
- **The Go engine.** A touches no engine code. Server hosting, tunnels, and the engine's local admin "users" (a separate concept used for remote-access auth) are untouched. The account is a pure UI↔Supabase identity layer. (A *does* add a small OAuth-loopback handler in the **Tauri desktop shell** — the Rust app process — which is distinct from the Go engine.)
- **Anonymous flows.** No anonymous path gains a sign-in requirement.

## Architecture

```
React UI (Tauri webview)
  ├── supabase-js client  ──────────────►  Supabase (existing project)
  │     • auth (email + Discord PKCE)         • auth.users (GoTrue)
  │     • profiles read/write                 • profiles table (+ RLS, trigger)
  │     • session persisted (localStorage)
  └── Discord loopback handler (Tauri/Rust)
        • one-shot http://localhost:<port>/auth-callback
        • captures ?code, returns a close-tab page, emits code to the UI
Go engine ── UNCHANGED (not involved in A)
```

A single shared `supabase-js` client instance owns auth state. The UI subscribes to `onAuthStateChange` to drive the sidebar between signed-out and signed-in.

## Data model

One new table in the existing Supabase project.

```sql
-- citext for case-insensitive unique usernames
create extension if not exists citext;

create table public.profiles (
  id           uuid primary key references auth.users(id) on delete cascade,
  username     citext unique not null,
  display_name text,
  avatar_url   text,
  level        int  not null default 1,
  xp           int  not null default 0,
  created_at   timestamptz not null default now(),
  constraint username_format check (username ~ '^[a-zA-Z0-9_]{3,20}$')
);

alter table public.profiles enable row level security;

-- any signed-in user may read any profile (friends in B need this)
create policy profiles_read on public.profiles
  for select to authenticated using (true);

-- a user may insert/update only their own row
create policy profiles_insert_own on public.profiles
  for insert to authenticated with check (auth.uid() = id);
create policy profiles_update_own on public.profiles
  for update to authenticated using (auth.uid() = id) with check (auth.uid() = id);
```

**Profile creation trigger** (runs on `auth.users` insert, `SECURITY DEFINER`):
- Resolves a base username:
  - **email:** the username passed in `options.data.username` at `signUp`.
  - **Discord:** `raw_user_meta_data->>'name'` (fallback `'user_name'`), sanitized to the allowed charset.
- **Dedupe:** if the base username is taken, append a 4-digit random suffix and retry (bounded to a few attempts).
- Sets `avatar_url` from `raw_user_meta_data->>'avatar_url'` when present (Discord supplies it).
- `level`/`xp` take their column defaults (`1`/`0`).

Reserved usernames (e.g. `admin`, `system`, `gamenest`, `support`) are rejected by the availability check (a small denylist) before sign-up proceeds, and the trigger applies the same denylist as a backstop.

## Auth flows

### Email
- **Sign up:** form (email, password, username). The UI checks username availability (`select … where username = ? ` + format + denylist) and password rules, then `supabase.auth.signUp({ email, password, options: { data: { username } } })`. The trigger creates the profile. (Supabase email-confirmation setting is the project default; if confirmation is on, show a "check your email" state.)
- **Sign in:** `supabase.auth.signInWithPassword({ email, password })`.

### Discord (loopback PKCE)
PKCE is required so the redirect carries an authorization **code in the query string** (a URL fragment would never reach the loopback server).

1. UI: `supabase.auth.signInWithOAuth({ provider: 'discord', options: { redirectTo: 'http://localhost:<fixedPort>/auth-callback', skipBrowserRedirect: true } })` → returns the provider URL. (The supabase client is configured `flowType: 'pkce'`; it stores the code-verifier in its own storage.)
2. UI asks Tauri to (a) start a **one-shot loopback HTTP listener** on `<fixedPort>` — via `tauri-plugin-oauth`, which runs a localhost server and hands the captured callback URL back to the frontend — and (b) open the provider URL in the system browser (`tauri-plugin-shell` open).
3. User authorizes in Discord → Supabase redirects to `http://localhost:<fixedPort>/auth-callback?code=…`.
4. The loopback handler reads `code`, responds with a minimal "You're signed in — you can close this tab" HTML page, then **emits a Tauri event** carrying the code to the UI and shuts the listener down.
5. UI: `supabase.auth.exchangeCodeForSession(code)` (same client instance → has the verifier) → session established → trigger had already created the profile on first sign-in.

`<fixedPort>` is a single fixed port (constant in the app); the exact `http://localhost:<fixedPort>/auth-callback` URL is allow-listed in both Supabase (Auth → URL config) and the Discord application's redirect URIs. If the port is busy, sign-in surfaces a clear error ("close the other instance and retry") rather than silently failing.

### Session
- The supabase client is created with `persistSession: true, autoRefreshToken: true` using the webview's `localStorage` as the store. The session restores on launch; the user stays signed in.
- **Sign-out:** `supabase.auth.signOut()` → clears the session → sidebar returns to the signed-out state.

## UI

The right social sidebar is a persistent panel in the existing app shell, following the current redesign's component patterns. (Reference mockups produced during design: `account-layout.html`, `account-states.html` in `.superpowers/brainstorm/`.)

**Signed-out state:** a centered prompt — game icon, "Sign in to GameNest", subtext "Add friends, level up, and see what your friends are playing.", a primary **Sign in** button, and a **Create account** link. Anonymous use is otherwise unchanged.

**Signed-in state:**
- Profile block: avatar (email users without one get a generated default, e.g. initials), username, "Level N" (+ an optional small role tag), and an **XP progress bar** with "x / y XP" text. (Level/XP are static at 1 / "0 / —" until C; the bar renders a sensible empty/early state.)
- A settings affordance (gear or kebab) → menu: **Sign out**, **Change username**.
- Below a divider: the **Friends** section header with a disabled "+ Add" and a labeled placeholder ("friends + presence = step B"). No friend data in A.

**Sign-in panel** (opens from "Sign in"): "Continue with Discord" (primary), an "or" divider, email + password fields with an email sign-in button, and a "Create one (pick a username)" link that switches the panel to the sign-up form (adds the username field + availability check).

**Username rename:** a small dialog from the profile menu — new username → availability check (format + denylist + uniqueness) → `update profiles set username = …`. Errors shown inline.

## Error handling

- Network/Supabase errors surface as inline messages in the panel (never a dead button); the UI's existing request patterns apply.
- Username taken / invalid / reserved → inline validation before submit and on the server (unique constraint + trigger backstop), with a friendly message.
- Discord loopback: port busy, user closes the browser, or no code within a timeout → the flow cancels cleanly and re-enables the Discord button.
- All auth state changes flow through `onAuthStateChange`, so a token expiry or external sign-out updates the sidebar without a manual refresh.

## Owner setup tasks (one-time, manual)

1. **Discord application:** create one at the Discord developer portal; copy client id + secret.
2. **Supabase Auth:** enable the Discord provider (paste client id/secret); add `http://localhost:<fixedPort>/auth-callback` to the redirect allow-list; confirm the email provider settings (confirmation on/off).
3. **Discord app redirect URIs:** add the Supabase callback URL (Supabase shows it) and `http://localhost:<fixedPort>/auth-callback`.
4. Apply the `profiles` migration (table + RLS + trigger) to the existing project.

These are documented for the owner; they are not code tasks but block end-to-end Discord testing.

## Testing

- **UI unit tests** (the UI's test runner; add one if absent):
  - Sign-up form: username format, password rule, and availability-check logic (mocked supabase) — valid/invalid/taken/reserved cases.
  - Auth-state hook: signed-out → signed-in → signed-out transitions drive the right sidebar state (mocked `onAuthStateChange`).
  - Discord callback parsing: a handler unit test that a well-formed `?code=…` yields the code and a malformed/empty callback is rejected.
- **Loopback handler (Tauri/Rust) unit test:** given a request to `/auth-callback?code=X`, it returns the close-tab page, emits `X`, and stops; a request without `code` returns an error page and emits nothing.
- **Supabase RLS + trigger (SQL tests):**
  - An authenticated user can read any profile; cannot insert/update another user's row; can update only their own.
  - The new-user trigger creates a row with the right username (email path), applies dedupe when the base is taken, sets defaults `level=1`/`xp=0`, and copies `avatar_url` from metadata.
- **Manual end-to-end checklist** (needs the owner setup): email sign-up → profile appears; Discord sign-in via browser → returns to app signed in with avatar; restart app → still signed in; sign out → signed-out sidebar; rename username → reflected.

## Future (noted, not in A)

- **B — Friends + presence:** add/accept/remove friends by username; live presence sourced from the engine API (what servers the user is running) published via Supabase Realtime. The sidebar's friends section becomes live.
- **C — Levels/XP:** XP-earning rules (e.g. hosting uptime, inviting friends) + leveling; the static `level`/`xp` become dynamic.
- **Plus-link convergence:** let the social account also carry the Plus entitlement, retiring the separate device-link code flow.
- **Hardened token storage:** move the refresh token from `localStorage` to Tauri secure storage.
- **Account deletion:** a self-serve delete (auth user + cascade), for privacy/GDPR.
```
