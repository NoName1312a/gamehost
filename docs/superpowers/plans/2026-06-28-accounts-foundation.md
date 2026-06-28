# GameNest Accounts Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional, in-app GameNest accounts (Discord + email via Supabase) with a right-hand social sidebar showing the signed-in profile — anonymous hosting unchanged.

**Architecture:** The React UI gets a `supabase-js` client and an auth context; sign-in is email/password or Discord (a custom Tauri loopback command captures the PKCE `code`). A `profiles` table in the existing Supabase project (RLS + new-user trigger) holds username/avatar/level/xp. The Go engine is untouched. New UI lives under `ui/src/components/social/` + `ui/src/lib/`.

**Tech Stack:** React 19, TypeScript, Tailwind v4, Vite 8, Vitest + Testing Library (added here), `@supabase/supabase-js`, Tauri 2 (Rust loopback command), `@tauri-apps/plugin-shell` (JS).

**Spec:** `docs/superpowers/specs/2026-06-28-accounts-foundation-design.md`

## Global Constraints

- **Optional accounts only** — never gate any anonymous flow (hosting, "friends can join") behind sign-in.
- **Engine untouched** — no changes under `engine/`. The engine's local admin "users" and the dormant Plus device-link (`Account.tsx`, `api.account*`) are separate; do not modify them.
- **Theme** — match the app: Tailwind, dark `zinc` palette, **`emerald-500` accent**, the `.panel` class. (Design mockups used blue; use emerald in code.)
- **Discord OAuth uses PKCE** (`flowType: "pkce"`); the redirect `code` arrives on a fixed loopback `http://localhost:8788/`.
- **`level`/`xp`** exist in schema + display but stay static (`1`/`0`) until sub-project C — no migration later.
- **Lint gate:** `npm --prefix ui run lint` — 0 new errors (7 pre-existing `react-hooks/set-state-in-effect` warnings allowed).
- **Build gate:** `npm --prefix ui run build` succeeds.
- **Username rule (verbatim):** regex `^[a-zA-Z0-9_]{3,20}$`; reserved (case-insensitive): `admin`, `administrator`, `system`, `gamenest`, `support`, `mod`, `moderator`, `root`, `null`, `undefined`.
- Commits: DCO sign-off (`git commit -s`) + trailer `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`. Work on branch `feat/accounts`.

## File Structure

**New (UI):**
- `ui/src/lib/supabase.ts` — supabase-js client (env, PKCE) + `Profile` type.
- `ui/src/lib/username.ts` — pure username validation + availability + reserved list.
- `ui/src/lib/auth.tsx` — `AuthProvider` + `useAuth()` (session, profile, email sign-in/up, sign-out, rename).
- `ui/src/lib/discord-oauth.ts` — Discord loopback PKCE flow.
- `ui/src/components/social/SocialSidebar.tsx` — right sidebar, two states.
- `ui/src/components/social/SignInPanel.tsx` — sign-in/up modal (Discord + email).
- `ui/src/components/social/ProfileBlock.tsx` — avatar/username/level/XP.
- `ui/src/components/social/UsernameDialog.tsx` — rename dialog.
- `ui/src/test/setup.ts` — Vitest/jest-dom setup.
- `ui/.env.example` — `VITE_SUPABASE_URL`, `VITE_SUPABASE_ANON_KEY`.
- Tests colocated: `*.test.ts(x)` next to each unit.

**New (Supabase):**
- `supabase/migrations/0001_profiles.sql` — table + RLS + trigger.

**New (docs):**
- `docs/accounts-owner-setup.md` — Discord app + Supabase provider config.

**Modified:**
- `ui/package.json` — deps + `test` script.
- `ui/vite.config.ts` — Vitest config.
- `ui/tsconfig.app.json` — include vitest globals types (if needed).
- `ui/src/App.tsx` — wrap in `AuthProvider`, mount `SocialSidebar` as a third column.
- `desktop/Cargo.toml` — (no new crate; loopback uses std).
- `desktop/src/main.rs` — `start_oauth_loopback` command + `invoke_handler`.
- `desktop/capabilities/default.json` — add `shell:allow-open`.
- `.github/workflows/release.yml` — pass `VITE_SUPABASE_*` to the build step.

---

### Task 1: Test harness + dependencies

**Files:**
- Modify: `ui/package.json`
- Modify: `ui/vite.config.ts`
- Create: `ui/src/test/setup.ts`
- Create: `ui/src/lib/smoke.test.ts`

**Interfaces:**
- Produces: a working `npm --prefix ui run test` (Vitest, jsdom) for all later tasks.

- [ ] **Step 1: Install dependencies**

Run:
```bash
cd ui
npm install @supabase/supabase-js @tauri-apps/plugin-shell
npm install -D vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom
```
Expected: `package.json` gains the deps; `package-lock.json` updates.

- [ ] **Step 2: Add the test script**

In `ui/package.json` `"scripts"`, add:
```json
"test": "vitest run"
```

- [ ] **Step 3: Configure Vitest in `ui/vite.config.ts`**

Replace the file with:
```ts
/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { fs: { allow: ['..'] } },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
})
```

- [ ] **Step 4: Create `ui/src/test/setup.ts`**

```ts
import '@testing-library/jest-dom/vitest'
```

- [ ] **Step 5: Write a smoke test `ui/src/lib/smoke.test.ts`**

```ts
import { describe, it, expect } from 'vitest'

describe('test harness', () => {
  it('runs', () => {
    expect(1 + 1).toBe(2)
  })
})
```

- [ ] **Step 6: Run it**

Run: `npm --prefix ui run test`
Expected: PASS (1 test).

- [ ] **Step 7: Confirm lint + build still pass**

Run: `npm --prefix ui run lint && npm --prefix ui run build`
Expected: lint clean (≤7 pre-existing warnings), build succeeds.

- [ ] **Step 8: Commit**

```bash
git add ui/package.json ui/package-lock.json ui/vite.config.ts ui/src/test/setup.ts ui/src/lib/smoke.test.ts
git commit -s -m "test: add Vitest + Testing Library harness; supabase-js + shell deps"
```

---

### Task 2: Supabase schema (profiles + RLS + trigger)

**Files:**
- Create: `supabase/migrations/0001_profiles.sql`

**Interfaces:**
- Produces: a `public.profiles` table (`id`, `username` citext unique, `display_name`, `avatar_url`, `level` int=1, `xp` int=0, `created_at`); RLS (authenticated read-all, write-own); `handle_new_user` trigger.

- [ ] **Step 1: Write the migration `supabase/migrations/0001_profiles.sql`**

```sql
-- Case-insensitive unique usernames.
create extension if not exists citext;

create table if not exists public.profiles (
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

create policy profiles_read on public.profiles
  for select to authenticated using (true);
create policy profiles_insert_own on public.profiles
  for insert to authenticated with check (auth.uid() = id);
create policy profiles_update_own on public.profiles
  for update to authenticated using (auth.uid() = id) with check (auth.uid() = id);

-- Create a profile row on signup. Username comes from signUp metadata (email)
-- or Discord identity; collisions get a 4-digit suffix; reserved names rejected.
create or replace function public.handle_new_user()
returns trigger
language plpgsql
security definer set search_path = public
as $$
declare
  base    text;
  candidate text;
  tries   int := 0;
  reserved text[] := array['admin','administrator','system','gamenest','support','mod','moderator','root','null','undefined'];
begin
  base := coalesce(
    new.raw_user_meta_data->>'username',
    new.raw_user_meta_data->>'name',
    new.raw_user_meta_data->>'user_name',
    'player'
  );
  base := regexp_replace(base, '[^a-zA-Z0-9_]', '', 'g');
  if length(base) < 3 then base := base || 'player'; end if;
  base := left(base, 16);
  candidate := base;
  while (candidate = any(reserved)) or exists(select 1 from public.profiles where username = candidate) loop
    candidate := left(base, 16) || lpad((floor(random()*9000)+1000)::int::text, 4, '0');
    tries := tries + 1;
    if tries > 8 then candidate := 'player' || lpad((floor(random()*900000)+100000)::int::text, 6, '0'); end if;
  end loop;
  insert into public.profiles (id, username, display_name, avatar_url)
  values (new.id, candidate, new.raw_user_meta_data->>'name', new.raw_user_meta_data->>'avatar_url');
  return new;
end;
$$;

drop trigger if exists on_auth_user_created on auth.users;
create trigger on_auth_user_created
  after insert on auth.users
  for each row execute function public.handle_new_user();
```

- [ ] **Step 2: Apply the migration to the existing Supabase project**

Apply via the Supabase MCP `apply_migration` tool (name `0001_profiles`, the SQL above) **or** paste it into the Supabase dashboard SQL editor. (This shares the existing GameNest project.)

- [ ] **Step 3: Verify RLS + trigger (SQL editor or MCP `execute_sql`)**

Run these checks and confirm each result:
```sql
-- table + columns exist with defaults
select column_name, column_default from information_schema.columns
where table_schema='public' and table_name='profiles' order by column_name;
-- RLS enabled
select relrowsecurity from pg_class where relname='profiles';   -- expect: t
-- policies present
select policyname, cmd from pg_policies where tablename='profiles';
```
Expected: `level` default `1`, `xp` default `0`; `relrowsecurity = t`; three policies (`profiles_read` select, `profiles_insert_own` insert, `profiles_update_own` update).

- [ ] **Step 4: Commit**

```bash
git add supabase/migrations/0001_profiles.sql
git commit -s -m "feat(db): profiles table with RLS + new-user trigger"
```

---

### Task 3: Supabase client + env

**Files:**
- Create: `ui/src/lib/supabase.ts`
- Create: `ui/.env.example`
- Create: `ui/src/lib/supabase.test.ts`

**Interfaces:**
- Produces: `supabase` (a `SupabaseClient`), `type Profile`, `fetchProfile(userId): Promise<Profile | null>`.

- [ ] **Step 1: Write the failing test `ui/src/lib/supabase.test.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@supabase/supabase-js', () => ({
  createClient: vi.fn(() => ({ from: vi.fn() })),
}))

beforeEach(() => {
  vi.stubEnv('VITE_SUPABASE_URL', 'https://example.supabase.co')
  vi.stubEnv('VITE_SUPABASE_ANON_KEY', 'anon-key')
})

describe('supabase client', () => {
  it('creates a client with the env URL + key and PKCE auth', async () => {
    const { createClient } = await import('@supabase/supabase-js')
    await import('./supabase')
    expect(createClient).toHaveBeenCalledWith(
      'https://example.supabase.co',
      'anon-key',
      expect.objectContaining({ auth: expect.objectContaining({ flowType: 'pkce' }) }),
    )
  })
})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `npm --prefix ui run test -- src/lib/supabase.test.ts`
Expected: FAIL (`Cannot find module './supabase'`).

- [ ] **Step 3: Write `ui/src/lib/supabase.ts`**

```ts
import { createClient } from '@supabase/supabase-js'

const url = import.meta.env.VITE_SUPABASE_URL as string
const anon = import.meta.env.VITE_SUPABASE_ANON_KEY as string

// PKCE so the Discord redirect carries a code in the query (loopback-readable);
// detectSessionInUrl off because the desktop webview never has the URL.
export const supabase = createClient(url, anon, {
  auth: {
    flowType: 'pkce',
    persistSession: true,
    autoRefreshToken: true,
    detectSessionInUrl: false,
  },
})

export interface Profile {
  id: string
  username: string
  display_name: string | null
  avatar_url: string | null
  level: number
  xp: number
}

export async function fetchProfile(userId: string): Promise<Profile | null> {
  const { data } = await supabase
    .from('profiles')
    .select('id, username, display_name, avatar_url, level, xp')
    .eq('id', userId)
    .maybeSingle()
  return (data as Profile | null) ?? null
}
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `npm --prefix ui run test -- src/lib/supabase.test.ts`
Expected: PASS.

- [ ] **Step 5: Create `ui/.env.example`**

```
# Supabase project the desktop app authenticates against (public values).
VITE_SUPABASE_URL=https://<project-ref>.supabase.co
VITE_SUPABASE_ANON_KEY=<anon-public-key>
```

- [ ] **Step 6: Commit**

```bash
git add ui/src/lib/supabase.ts ui/src/lib/supabase.test.ts ui/.env.example
git commit -s -m "feat(ui): supabase client (PKCE) + Profile type + env example"
```

---

### Task 4: Username validation utilities

**Files:**
- Create: `ui/src/lib/username.ts`
- Create: `ui/src/lib/username.test.ts`

**Interfaces:**
- Produces: `validateUsername(name: string): string | null` (error message or null), `isUsernameAvailable(name: string): Promise<boolean>`.

- [ ] **Step 1: Write the failing test `ui/src/lib/username.test.ts`**

```ts
import { describe, it, expect, vi } from 'vitest'
import { validateUsername } from './username'

describe('validateUsername', () => {
  it('accepts a valid name', () => {
    expect(validateUsername('Tom_99')).toBeNull()
  })
  it('rejects too short / too long / bad chars', () => {
    expect(validateUsername('ab')).toMatch(/3/)
    expect(validateUsername('x'.repeat(21))).toMatch(/3/)
    expect(validateUsername('has space')).toMatch(/letters/i)
  })
  it('rejects reserved names case-insensitively', () => {
    expect(validateUsername('Admin')).toMatch(/reserved/i)
    expect(validateUsername('gamenest')).toMatch(/reserved/i)
  })
})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `npm --prefix ui run test -- src/lib/username.test.ts`
Expected: FAIL (`Cannot find module './username'`).

- [ ] **Step 3: Write `ui/src/lib/username.ts`**

```ts
import { supabase } from './supabase'

const RESERVED = new Set([
  'admin', 'administrator', 'system', 'gamenest', 'support',
  'mod', 'moderator', 'root', 'null', 'undefined',
])

/** Returns an error message, or null when the name is valid. */
export function validateUsername(name: string): string | null {
  if (!/^[a-zA-Z0-9_]{3,20}$/.test(name)) {
    return '3–20 letters, numbers, or underscores.'
  }
  if (RESERVED.has(name.toLowerCase())) {
    return 'That username is reserved.'
  }
  return null
}

/** True if no profile already uses this username (case-insensitive). */
export async function isUsernameAvailable(name: string): Promise<boolean> {
  const { data } = await supabase
    .from('profiles')
    .select('id')
    .ilike('username', name)
    .maybeSingle()
  return !data
}
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `npm --prefix ui run test -- src/lib/username.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/username.ts ui/src/lib/username.test.ts
git commit -s -m "feat(ui): username validation + availability"
```

---

### Task 5: Auth context (`useAuth`)

**Files:**
- Create: `ui/src/lib/auth.tsx`
- Create: `ui/src/lib/auth.test.tsx`

**Interfaces:**
- Consumes: `supabase`, `fetchProfile`, `type Profile` (Task 3).
- Produces: `<AuthProvider>` and `useAuth(): { session, profile, loading, signInEmail(email,password), signUpEmail(email,password,username), signOut(), refreshProfile() }`. `session` is `Session | null`, `profile` is `Profile | null`.

- [ ] **Step 1: Write the failing test `ui/src/lib/auth.test.tsx`**

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

const onAuthStateChange = vi.fn(() => ({ data: { subscription: { unsubscribe: vi.fn() } } }))
const getSession = vi.fn(async () => ({ data: { session: null } }))
vi.mock('./supabase', () => ({
  supabase: { auth: { getSession, onAuthStateChange, signOut: vi.fn() } },
  fetchProfile: vi.fn(async () => ({ id: 'u1', username: 'Tom', display_name: null, avatar_url: null, level: 1, xp: 0 })),
}))

import { AuthProvider, useAuth } from './auth'

function Probe() {
  const { loading, session } = useAuth()
  return <div>{loading ? 'loading' : session ? 'in' : 'out'}</div>
}

beforeEach(() => { getSession.mockClear(); onAuthStateChange.mockClear() })

describe('AuthProvider', () => {
  it('resolves to signed-out when there is no session', async () => {
    render(<AuthProvider><Probe /></AuthProvider>)
    await waitFor(() => expect(screen.getByText('out')).toBeInTheDocument())
    expect(onAuthStateChange).toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `npm --prefix ui run test -- src/lib/auth.test.tsx`
Expected: FAIL (`Cannot find module './auth'`).

- [ ] **Step 3: Write `ui/src/lib/auth.tsx`**

```tsx
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import type { Session } from '@supabase/supabase-js'
import { supabase, fetchProfile, type Profile } from './supabase'

interface AuthState {
  session: Session | null
  profile: Profile | null
  loading: boolean
  signInEmail: (email: string, password: string) => Promise<void>
  signUpEmail: (email: string, password: string, username: string) => Promise<void>
  signOut: () => Promise<void>
  refreshProfile: () => Promise<void>
}

const Ctx = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null)
  const [profile, setProfile] = useState<Profile | null>(null)
  const [loading, setLoading] = useState(true)

  const loadProfile = useCallback(async (s: Session | null) => {
    setProfile(s ? await fetchProfile(s.user.id) : null)
  }, [])

  useEffect(() => {
    let alive = true
    supabase.auth.getSession().then(async ({ data }) => {
      if (!alive) return
      setSession(data.session)
      await loadProfile(data.session)
      setLoading(false)
    })
    const { data: sub } = supabase.auth.onAuthStateChange((_e, s) => {
      setSession(s)
      void loadProfile(s)
    })
    return () => { alive = false; sub.subscription.unsubscribe() }
  }, [loadProfile])

  const signInEmail = useCallback(async (email: string, password: string) => {
    const { error } = await supabase.auth.signInWithPassword({ email, password })
    if (error) throw new Error(error.message)
  }, [])

  const signUpEmail = useCallback(async (email: string, password: string, username: string) => {
    const { error } = await supabase.auth.signUp({ email, password, options: { data: { username } } })
    if (error) throw new Error(error.message)
  }, [])

  const signOut = useCallback(async () => { await supabase.auth.signOut() }, [])
  const refreshProfile = useCallback(() => loadProfile(session), [loadProfile, session])

  return (
    <Ctx.Provider value={{ session, profile, loading, signInEmail, signUpEmail, signOut, refreshProfile }}>
      {children}
    </Ctx.Provider>
  )
}

export function useAuth(): AuthState {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth must be used within AuthProvider')
  return v
}
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `npm --prefix ui run test -- src/lib/auth.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/auth.tsx ui/src/lib/auth.test.tsx
git commit -s -m "feat(ui): auth context (session + profile + email auth + signout)"
```

---

### Task 6: Discord OAuth loopback (Tauri command + JS flow)

**Files:**
- Modify: `desktop/src/main.rs`
- Modify: `desktop/capabilities/default.json`
- Create: `ui/src/lib/discord-oauth.ts`

**Interfaces:**
- Consumes: `supabase` (Task 3).
- Produces: Rust command `start_oauth_loopback() -> Result<u16, String>` (emits a `oauth-code` event with the captured `code`); JS `signInWithDiscord(): Promise<void>`.

- [ ] **Step 1: Add the loopback command to `desktop/src/main.rs`**

After the existing `use` lines, add:
```rust
use std::io::{Read, Write};
use std::net::TcpListener;
use tauri::Emitter;
```
Add this function above `fn main()`:
```rust
/// Start a one-shot loopback server on a fixed port to capture the OAuth
/// redirect's `code` query param, then emit it to the frontend as `oauth-code`.
/// Fixed port 8788 so the redirect URL can be allow-listed in Supabase + Discord.
#[tauri::command]
fn start_oauth_loopback(app: tauri::AppHandle) -> Result<u16, String> {
    let listener = TcpListener::bind("127.0.0.1:8788")
        .map_err(|_| "Sign-in port 8788 is busy (another GameNest instance?).".to_string())?;
    let port = listener.local_addr().map_err(|e| e.to_string())?.port();
    std::thread::spawn(move || {
        if let Ok((mut stream, _)) = listener.accept() {
            let mut buf = [0u8; 4096];
            let n = stream.read(&mut buf).unwrap_or(0);
            let req = String::from_utf8_lossy(&buf[..n]);
            let code = req
                .lines()
                .next()
                .and_then(|l| l.split_whitespace().nth(1))
                .and_then(|path| path.split('?').nth(1))
                .and_then(|q| q.split('&').find_map(|kv| kv.strip_prefix("code=")))
                .map(|c| c.to_string());
            let body = "<html><body style='font-family:sans-serif;background:#0a0a0a;color:#eee;text-align:center;padding-top:80px'><h2>\u{2713} Signed in</h2><p>You can close this tab and return to GameNest.</p></body></html>";
            let _ = write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nConnection: close\r\nContent-Length: {}\r\n\r\n{}",
                body.len(),
                body
            );
            let _ = stream.flush();
            if let Some(code) = code {
                let _ = app.emit("oauth-code", code);
            }
        }
    });
    Ok(port)
}
```

- [ ] **Step 2: Register the command**

In `fn main()`, on the `tauri::Builder`, add an invoke handler. Change:
```rust
        .manage(EngineProcess(Mutex::new(None)))
```
to:
```rust
        .manage(EngineProcess(Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![start_oauth_loopback])
```

- [ ] **Step 3: Grant `shell:allow-open` in `desktop/capabilities/default.json`**

Add `"shell:allow-open"` to the `permissions` array (after `"process:allow-restart"`):
```json
    "process:allow-restart",
    "shell:allow-open",
```

- [ ] **Step 4: Verify the Rust shell compiles**

Run: `cargo build --manifest-path desktop/Cargo.toml`
Expected: builds (may be slow). If Smart App Control blocks local build, note it and rely on the CI build to verify (the change is a standard std TCP listener + a registered command).

- [ ] **Step 5: Write `ui/src/lib/discord-oauth.ts`**

```ts
import { invoke } from '@tauri-apps/api/core'
import { listen } from '@tauri-apps/api/event'
import { open } from '@tauri-apps/plugin-shell'
import { supabase } from './supabase'

/** Sign in with Discord via a loopback PKCE flow: start a local server,
 *  open the provider URL in the browser, capture the code, exchange it. */
export async function signInWithDiscord(): Promise<void> {
  const port = await invoke<number>('start_oauth_loopback')
  const redirectTo = `http://localhost:${port}/`

  const codePromise = new Promise<string>((resolve, reject) => {
    let unlisten: (() => void) | undefined
    const timer = setTimeout(() => {
      unlisten?.()
      reject(new Error('Sign-in timed out. Please try again.'))
    }, 120_000)
    listen<string>('oauth-code', (e) => {
      clearTimeout(timer)
      unlisten?.()
      resolve(e.payload)
    }).then((u) => { unlisten = u })
  })

  const { data, error } = await supabase.auth.signInWithOAuth({
    provider: 'discord',
    options: { redirectTo, skipBrowserRedirect: true },
  })
  if (error) throw new Error(error.message)
  if (data.url) await open(data.url)

  const code = await codePromise
  const { error: exErr } = await supabase.auth.exchangeCodeForSession(code)
  if (exErr) throw new Error(exErr.message)
}
```

- [ ] **Step 6: Confirm UI build + lint**

Run: `npm --prefix ui run lint && npm --prefix ui run build`
Expected: clean + build succeeds.

- [ ] **Step 7: Commit**

```bash
git add desktop/src/main.rs desktop/capabilities/default.json ui/src/lib/discord-oauth.ts
git commit -s -m "feat: Discord OAuth via Tauri loopback (PKCE) + shell open permission"
```

---

### Task 7: SignInPanel component

**Files:**
- Create: `ui/src/components/social/SignInPanel.tsx`
- Create: `ui/src/components/social/SignInPanel.test.tsx`

**Interfaces:**
- Consumes: `useAuth` (Task 5), `validateUsername` + `isUsernameAvailable` (Task 4), `signInWithDiscord` (Task 6).
- Produces: `<SignInPanel onClose={() => void} />` — a modal with Discord + email sign-in and a sign-up toggle (adds a username field with validation).

- [ ] **Step 1: Write the failing test `ui/src/components/social/SignInPanel.test.tsx`**

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const signInEmail = vi.fn(async () => {})
const signUpEmail = vi.fn(async () => {})
vi.mock('../../lib/auth', () => ({ useAuth: () => ({ signInEmail, signUpEmail }) }))
vi.mock('../../lib/discord-oauth', () => ({ signInWithDiscord: vi.fn(async () => {}) }))
vi.mock('../../lib/username', () => ({
  validateUsername: (n: string) => (n.length < 3 ? 'too short' : null),
  isUsernameAvailable: vi.fn(async () => true),
}))

import { SignInPanel } from './SignInPanel'

describe('SignInPanel', () => {
  it('signs in with email', async () => {
    render(<SignInPanel onClose={() => {}} />)
    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'a@b.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret12' } })
    fireEvent.click(screen.getByRole('button', { name: /sign in with email/i }))
    await waitFor(() => expect(signInEmail).toHaveBeenCalledWith('a@b.com', 'secret12'))
  })

  it('blocks sign-up with an invalid username', async () => {
    render(<SignInPanel onClose={() => {}} />)
    fireEvent.click(screen.getByText(/create one/i))
    fireEvent.change(screen.getByPlaceholderText('Username'), { target: { value: 'ab' } })
    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'a@b.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret12' } })
    fireEvent.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText('too short')).toBeInTheDocument())
    expect(signUpEmail).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `npm --prefix ui run test -- src/components/social/SignInPanel.test.tsx`
Expected: FAIL (`Cannot find module './SignInPanel'`).

- [ ] **Step 3: Write `ui/src/components/social/SignInPanel.tsx`**

```tsx
import { useState, type FormEvent } from 'react'
import { useAuth } from '../../lib/auth'
import { signInWithDiscord } from '../../lib/discord-oauth'
import { validateUsername, isUsernameAvailable } from '../../lib/username'
import { friendlyError } from '../../lib/errors'

export function SignInPanel({ onClose }: { onClose: () => void }) {
  const { signInEmail, signUpEmail } = useAuth()
  const [mode, setMode] = useState<'in' | 'up'>('in')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [username, setUsername] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setErr(null)
    setBusy(true)
    try {
      if (mode === 'up') {
        const v = validateUsername(username)
        if (v) throw new Error(v)
        if (!(await isUsernameAvailable(username))) throw new Error('That username is taken.')
        await signUpEmail(email, password, username)
      } else {
        await signInEmail(email, password)
      }
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  async function discord() {
    setErr(null)
    setBusy(true)
    try {
      await signInWithDiscord()
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
      <div className="panel w-full max-w-sm p-6" onClick={(e) => e.stopPropagation()}>
        <h2 className="font-display mb-4 text-center text-base font-semibold text-zinc-100">
          Welcome to GameNest
        </h2>
        <button
          onClick={discord}
          disabled={busy}
          className="w-full rounded-lg bg-[#5865F2] px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
        >
          Continue with Discord
        </button>
        <div className="my-3 flex items-center gap-2 text-xs text-zinc-600">
          <div className="h-px flex-1 bg-zinc-800" /> or <div className="h-px flex-1 bg-zinc-800" />
        </div>
        <form onSubmit={submit} className="flex flex-col gap-2">
          {mode === 'up' && (
            <input
              placeholder="Username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
            />
          )}
          <input
            placeholder="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
          />
          <input
            placeholder="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
          />
          {err && <p className="text-xs text-rose-400">{err}</p>}
          <button
            type="submit"
            disabled={busy}
            className="mt-1 w-full rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-50"
          >
            {mode === 'up' ? 'Create account' : 'Sign in with email'}
          </button>
        </form>
        <p className="mt-3 text-center text-xs text-zinc-500">
          {mode === 'in' ? (
            <>No account? <button className="text-emerald-400" onClick={() => setMode('up')}>Create one</button></>
          ) : (
            <>Have an account? <button className="text-emerald-400" onClick={() => setMode('in')}>Sign in</button></>
          )}
        </p>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `npm --prefix ui run test -- src/components/social/SignInPanel.test.tsx`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/social/SignInPanel.tsx ui/src/components/social/SignInPanel.test.tsx
git commit -s -m "feat(ui): SignInPanel (Discord + email + sign-up)"
```

---

### Task 8: ProfileBlock component

**Files:**
- Create: `ui/src/components/social/ProfileBlock.tsx`
- Create: `ui/src/components/social/ProfileBlock.test.tsx`

**Interfaces:**
- Consumes: `type Profile` (Task 3).
- Produces: `<ProfileBlock profile={Profile} onMenu={() => void} />` — avatar (or initials fallback), username, "Level N", XP bar.

- [ ] **Step 1: Write the failing test `ui/src/components/social/ProfileBlock.test.tsx`**

```tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProfileBlock } from './ProfileBlock'

const profile = { id: 'u1', username: 'Tom', display_name: null, avatar_url: null, level: 3, xp: 1240 }

describe('ProfileBlock', () => {
  it('shows username and level', () => {
    render(<ProfileBlock profile={profile} onMenu={() => {}} />)
    expect(screen.getByText('Tom')).toBeInTheDocument()
    expect(screen.getByText(/level 3/i)).toBeInTheDocument()
  })
  it('falls back to an initial when there is no avatar', () => {
    render(<ProfileBlock profile={profile} onMenu={() => {}} />)
    expect(screen.getByText('T')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `npm --prefix ui run test -- src/components/social/ProfileBlock.test.tsx`
Expected: FAIL (`Cannot find module './ProfileBlock'`).

- [ ] **Step 3: Write `ui/src/components/social/ProfileBlock.tsx`**

```tsx
import type { Profile } from '../../lib/supabase'

export function ProfileBlock({ profile, onMenu }: { profile: Profile; onMenu: () => void }) {
  const initial = profile.username.charAt(0).toUpperCase()
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3">
        {profile.avatar_url ? (
          <img src={profile.avatar_url} alt="" className="h-10 w-10 rounded-full" />
        ) : (
          <div className="grid h-10 w-10 place-items-center rounded-full bg-gradient-to-br from-emerald-500 to-sky-500 text-sm font-semibold text-zinc-950">
            {initial}
          </div>
        )}
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-semibold text-zinc-100">{profile.username}</div>
          <div className="text-xs text-zinc-500">Level {profile.level}</div>
        </div>
        <button onClick={onMenu} aria-label="Account menu" className="text-zinc-500 hover:text-zinc-300">⚙</button>
      </div>
      <div>
        <div className="h-1.5 overflow-hidden rounded-full bg-zinc-800">
          <div className="h-full bg-gradient-to-r from-emerald-500 to-sky-500" style={{ width: `${xpPercent(profile)}%` }} />
        </div>
        <div className="mt-1 text-right text-[10px] text-zinc-600">{profile.xp} XP</div>
      </div>
    </div>
  )
}

// Static early-state until sub-project C defines leveling. Keeps a sensible bar.
function xpPercent(p: Profile): number {
  if (p.xp <= 0) return 4
  return Math.min(100, p.xp % 100)
}
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `npm --prefix ui run test -- src/components/social/ProfileBlock.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/social/ProfileBlock.tsx ui/src/components/social/ProfileBlock.test.tsx
git commit -s -m "feat(ui): ProfileBlock (avatar/username/level/XP)"
```

---

### Task 9: UsernameDialog (rename)

**Files:**
- Create: `ui/src/components/social/UsernameDialog.tsx`
- Create: `ui/src/components/social/UsernameDialog.test.tsx`

**Interfaces:**
- Consumes: `validateUsername` + `isUsernameAvailable` (Task 4), `supabase` (Task 3), `useAuth` (Task 5, `refreshProfile`).
- Produces: `<UsernameDialog current={string} onClose={() => void} />`.

- [ ] **Step 1: Write the failing test `ui/src/components/social/UsernameDialog.test.tsx`**

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const update = vi.fn(() => ({ eq: vi.fn(async () => ({ error: null })) }))
const refreshProfile = vi.fn(async () => {})
vi.mock('../../lib/supabase', () => ({ supabase: { from: () => ({ update }) } }))
vi.mock('../../lib/auth', () => ({ useAuth: () => ({ refreshProfile }) }))
vi.mock('../../lib/username', () => ({
  validateUsername: (n: string) => (n.length < 3 ? 'too short' : null),
  isUsernameAvailable: vi.fn(async () => true),
}))

import { UsernameDialog } from './UsernameDialog'

beforeEach(() => { update.mockClear(); refreshProfile.mockClear() })

describe('UsernameDialog', () => {
  it('saves a valid new username', async () => {
    render(<UsernameDialog current="Old" onClose={() => {}} />)
    fireEvent.change(screen.getByDisplayValue('Old'), { target: { value: 'NewName' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))
    await waitFor(() => expect(update).toHaveBeenCalledWith({ username: 'NewName' }))
    await waitFor(() => expect(refreshProfile).toHaveBeenCalled())
  })
})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `npm --prefix ui run test -- src/components/social/UsernameDialog.test.tsx`
Expected: FAIL (`Cannot find module './UsernameDialog'`).

- [ ] **Step 3: Write `ui/src/components/social/UsernameDialog.tsx`**

```tsx
import { useState } from 'react'
import { supabase } from '../../lib/supabase'
import { useAuth } from '../../lib/auth'
import { validateUsername, isUsernameAvailable } from '../../lib/username'
import { friendlyError } from '../../lib/errors'

export function UsernameDialog({ current, onClose }: { current: string; onClose: () => void }) {
  const { refreshProfile } = useAuth()
  const [name, setName] = useState(current)
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function save() {
    setErr(null)
    setBusy(true)
    try {
      const v = validateUsername(name)
      if (v) throw new Error(v)
      if (name.toLowerCase() !== current.toLowerCase() && !(await isUsernameAvailable(name))) {
        throw new Error('That username is taken.')
      }
      const { error } = await supabase.from('profiles').update({ username: name }).eq('username', current)
      if (error) throw new Error(error.message)
      await refreshProfile()
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
      <div className="panel w-full max-w-xs p-6" onClick={(e) => e.stopPropagation()}>
        <h3 className="mb-3 text-sm font-semibold text-zinc-100">Change username</h3>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
        />
        {err && <p className="mt-2 text-xs text-rose-400">{err}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-lg px-3 py-1.5 text-sm text-zinc-400 hover:text-zinc-200">Cancel</button>
          <button onClick={save} disabled={busy} className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50">Save</button>
        </div>
      </div>
    </div>
  )
}
```

Note: the `update().eq('username', current)` targets the row by its current username; RLS still restricts the write to the user's own row.

- [ ] **Step 4: Run the test to confirm it passes**

Run: `npm --prefix ui run test -- src/components/social/UsernameDialog.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/social/UsernameDialog.tsx ui/src/components/social/UsernameDialog.test.tsx
git commit -s -m "feat(ui): UsernameDialog (rename)"
```

---

### Task 10: SocialSidebar (two states)

**Files:**
- Create: `ui/src/components/social/SocialSidebar.tsx`
- Create: `ui/src/components/social/SocialSidebar.test.tsx`

**Interfaces:**
- Consumes: `useAuth` (Task 5), `ProfileBlock` (Task 8), `SignInPanel` (Task 7), `UsernameDialog` (Task 9).
- Produces: `<SocialSidebar />` — renders the signed-out prompt or the signed-in profile + a labeled "Friends — coming soon" placeholder.

- [ ] **Step 1: Write the failing test `ui/src/components/social/SocialSidebar.test.tsx`**

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

let mockAuth: { session: unknown; profile: unknown; loading: boolean; signOut: () => void }
vi.mock('../../lib/auth', () => ({ useAuth: () => mockAuth }))
vi.mock('./SignInPanel', () => ({ SignInPanel: () => <div>signin-panel</div> }))

import { SocialSidebar } from './SocialSidebar'

describe('SocialSidebar', () => {
  it('shows the sign-in prompt when signed out', () => {
    mockAuth = { session: null, profile: null, loading: false, signOut: vi.fn() }
    render(<SocialSidebar />)
    expect(screen.getByText(/sign in to gamenest/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^sign in$/i }))
    expect(screen.getByText('signin-panel')).toBeInTheDocument()
  })
  it('shows the profile when signed in', () => {
    mockAuth = { session: { user: { id: 'u1' } }, profile: { id: 'u1', username: 'Tom', display_name: null, avatar_url: null, level: 2, xp: 50 }, loading: false, signOut: vi.fn() }
    render(<SocialSidebar />)
    expect(screen.getByText('Tom')).toBeInTheDocument()
    expect(screen.getByText(/friends/i)).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `npm --prefix ui run test -- src/components/social/SocialSidebar.test.tsx`
Expected: FAIL (`Cannot find module './SocialSidebar'`).

- [ ] **Step 3: Write `ui/src/components/social/SocialSidebar.tsx`**

```tsx
import { useState } from 'react'
import { useAuth } from '../../lib/auth'
import { ProfileBlock } from './ProfileBlock'
import { SignInPanel } from './SignInPanel'
import { UsernameDialog } from './UsernameDialog'

export function SocialSidebar() {
  const { session, profile, loading, signOut } = useAuth()
  const [showSignIn, setShowSignIn] = useState(false)
  const [showMenu, setShowMenu] = useState(false)
  const [showRename, setShowRename] = useState(false)

  return (
    <aside className="hidden w-72 shrink-0 flex-col border-l border-zinc-800/80 bg-zinc-950/60 p-4 backdrop-blur lg:flex">
      {loading ? (
        <p className="text-xs text-zinc-600">…</p>
      ) : session && profile ? (
        <>
          <ProfileBlock profile={profile} onMenu={() => setShowMenu((v) => !v)} />
          {showMenu && (
            <div className="mt-2 flex flex-col rounded-lg border border-zinc-800 bg-zinc-900 p-1 text-sm">
              <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={() => { setShowMenu(false); setShowRename(true) }}>Change username</button>
              <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={() => { setShowMenu(false); void signOut() }}>Sign out</button>
            </div>
          )}
          <div className="my-4 h-px bg-zinc-800/80" />
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">Friends</span>
            <span className="rounded border border-zinc-800 px-1.5 text-xs text-zinc-600">+ Add</span>
          </div>
          <p className="mt-3 rounded-lg border border-dashed border-zinc-800 p-3 text-center text-[11px] text-zinc-600">
            Friends &amp; presence are coming soon.
          </p>
          {showRename && <UsernameDialog current={profile.username} onClose={() => setShowRename(false)} />}
        </>
      ) : (
        <div className="flex flex-col items-center gap-2 text-center">
          <div className="grid h-12 w-12 place-items-center rounded-full bg-zinc-800 text-2xl">🎮</div>
          <div className="text-sm font-semibold text-zinc-100">Sign in to GameNest</div>
          <p className="text-xs text-zinc-500">Add friends, level up, and see what your friends are playing.</p>
          <button onClick={() => setShowSignIn(true)} className="mt-2 w-full rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 hover:bg-emerald-400">Sign in</button>
        </div>
      )}
      {showSignIn && <SignInPanel onClose={() => setShowSignIn(false)} />}
    </aside>
  )
}
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `npm --prefix ui run test -- src/components/social/SocialSidebar.test.tsx`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/social/SocialSidebar.tsx ui/src/components/social/SocialSidebar.test.tsx
git commit -s -m "feat(ui): SocialSidebar (signed-out prompt / signed-in profile)"
```

---

### Task 11: Wire into App.tsx

**Files:**
- Modify: `ui/src/App.tsx`

**Interfaces:**
- Consumes: `AuthProvider` (Task 5), `SocialSidebar` (Task 10).

- [ ] **Step 1: Import the new pieces**

In `ui/src/App.tsx`, add after the existing imports (near line 27):
```tsx
import { AuthProvider } from "./lib/auth";
import { SocialSidebar } from "./components/social/SocialSidebar";
```

- [ ] **Step 2: Mount the SocialSidebar as a third column**

In the main return (the layout `div` at line ~266), add `<SocialSidebar />` after the closing `</main>`. Change:
```tsx
        </main>
      </div>
```
to:
```tsx
        </main>
        <SocialSidebar />
      </div>
```

- [ ] **Step 3: Wrap the app export in AuthProvider**

Rename the current `export default function App()` to `function AppInner()`, then add at the end of the file:
```tsx
export default function App() {
  return (
    <AuthProvider>
      <AppInner />
    </AuthProvider>
  )
}
```
(Update the `function App(` declaration to `function AppInner(` — it is no longer the default export.)

- [ ] **Step 4: Verify lint + build**

Run: `npm --prefix ui run lint && npm --prefix ui run build`
Expected: lint clean (≤7 pre-existing warnings, 0 new), build succeeds.

- [ ] **Step 5: Run the whole test suite**

Run: `npm --prefix ui run test`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add ui/src/App.tsx
git commit -s -m "feat(ui): mount SocialSidebar + AuthProvider in the app shell"
```

---

### Task 12: Build env + owner setup doc

**Files:**
- Modify: `.github/workflows/release.yml`
- Create: `docs/accounts-owner-setup.md`

**Interfaces:**
- Produces: the release build receives `VITE_SUPABASE_URL` + `VITE_SUPABASE_ANON_KEY`; documented owner setup.

- [ ] **Step 1: Pass Supabase env to the build step**

In `.github/workflows/release.yml`, the "Build, sign manifest, and publish" step's `env:` block currently has `GH_TOKEN`. Add the two Supabase vars from secrets:
```yaml
        env:
          GH_TOKEN: ${{ secrets.GH_PUBLISH_TOKEN }}
          VITE_SUPABASE_URL: ${{ secrets.VITE_SUPABASE_URL }}
          VITE_SUPABASE_ANON_KEY: ${{ secrets.VITE_SUPABASE_ANON_KEY }}
```

- [ ] **Step 2: Write `docs/accounts-owner-setup.md`**

```markdown
# Accounts — owner setup (one-time)

These are manual steps the owner performs; they unblock end-to-end Discord sign-in.

## 1. Supabase env (GitHub secrets)
Add to the `NoName1312a/gamehost` repo secrets (public values, but kept as secrets for tidiness):
- `VITE_SUPABASE_URL` — `https://<project-ref>.supabase.co`
- `VITE_SUPABASE_ANON_KEY` — the project's anon public key

For local dev, copy `ui/.env.example` to `ui/.env` with the same values.

## 2. Discord application
1. https://discord.com/developers/applications → New Application.
2. OAuth2 → copy the Client ID + Client Secret.
3. OAuth2 → Redirects → add **both**:
   - the Supabase callback shown in Supabase (Auth → Providers → Discord), and
   - `http://localhost:8788/`

## 3. Supabase Auth config
1. Auth → Providers → Discord → enable, paste Client ID + Secret.
2. Auth → URL Configuration → Redirect URLs → add `http://localhost:8788/`.
3. Confirm the email provider settings (email confirmation on/off) — if on, sign-up shows a "check your email" state.

## 4. Database
Apply `supabase/migrations/0001_profiles.sql` to the project (dashboard SQL editor or Supabase MCP `apply_migration`).
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml docs/accounts-owner-setup.md
git commit -s -m "ci+docs: Supabase build env + accounts owner-setup guide"
```

---

## Self-Review Notes

- **Spec coverage:** optional accounts (Task 11 — anonymous untouched, sidebar is additive); Discord+email (Tasks 5–7); supabase-js in-app (Task 3); profiles + RLS + trigger (Task 2); right sidebar two states (Task 10); session persistence (Task 5 `persistSession`); sign-out + rename (Tasks 9–10); engine untouched (no `engine/` edits); owner setup (Task 12); level/xp static + displayed (Task 8). All spec sections map to a task.
- **Out-of-scope held:** no friends/presence/leveling logic; friends area is a static placeholder (Task 10).
- **Type consistency:** `Profile` defined in Task 3 and consumed unchanged in Tasks 5/8/10; `useAuth()` shape defined in Task 5 and consumed in 7/9/10; `signInWithDiscord` (Task 6) consumed in Task 7; `start_oauth_loopback`/`oauth-code` names consistent across Rust (Task 6) and JS (Task 6).
- **Known external dependency:** end-to-end Discord requires the Task 12 owner setup; until then, email sign-up/in and the full UI are testable, and unit tests mock Supabase.
```
