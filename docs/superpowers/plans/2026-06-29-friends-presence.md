# GameNest Friends + Presence — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the social sidebar's Friends section live — add friends by username (mutual request), and show each friend's online/offline + activity ("Hosting Craftoria").

**Architecture:** A friends data layer (`lib/friends.ts`) over a `friendships` table, a `PresenceProvider` (`lib/presence.tsx`) that heartbeats the signed-in user's activity (from `api.servers()`) into a `presence` table honoring a privacy toggle, and three UI pieces (FriendsList, AddFriendDialog, RequestsInbox) wired into `SocialSidebar`. Presence is friends-only (RLS) and read by polling. Engine untouched. Builds on sub-project A.

**Tech Stack:** React 19, TypeScript, Tailwind v4, Vitest + Testing Library, `@supabase/supabase-js`. Reuses A: `useAuth()`, `supabase`, `Profile`, `friendlyError`, and `api.servers()`.

**Spec:** `docs/superpowers/specs/2026-06-29-friends-presence-design.md`

## Global Constraints

- **Optional / additive** — only renders when signed in; never gates anonymous flows.
- **Engine untouched** — `lib/presence.tsx` only READS `api.servers()` (existing); no `engine/` changes.
- **Theme** — Tailwind, `zinc` palette, `emerald-500` accent, `.panel` for dialogs (match A's components).
- **Presence privacy** — friends-only read (RLS); a per-user `show_activity` toggle (default true) gates publishing; online = `presence.updated_at` within **60s**.
- **Friend model** — mutual requests: insert `status:'pending'`; addressee accepts (→`accepted`); either party deletes (decline/cancel/remove). Reject self / existing-either-direction in `sendRequest`.
- **Lint gate:** `npm --prefix ui run lint` — 0 NEW errors (7 pre-existing `react-hooks/set-state-in-effect` allowed).
- **Build gate:** `npm --prefix ui run build`; **Full suite:** `npm --prefix ui run test`.
- Commits: `git commit -s` + trailer `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`. Branch `feat/accounts` (continues A; B+C ship together after the one owner setup).

## File Structure

**New:** `supabase/migrations/0002_friends_presence.sql`; `ui/src/lib/friends.ts` (+ `.test.ts`); `ui/src/lib/presence.tsx` (+ `.test.tsx`); `ui/src/components/social/{AddFriendDialog,RequestsInbox,FriendsList}.tsx` (+ `.test.tsx`).
**Modified:** `ui/src/lib/supabase.ts` (add `show_activity` to `Profile` + `fetchProfile`); `ui/src/components/social/SocialSidebar.tsx` (live Friends section + PresenceProvider + privacy toggle); `docs/accounts-owner-setup.md` (append the `0002` migration).

---

### Task 1: Migration 0002 + Profile.show_activity (controller-handled)

**Files:** Create `supabase/migrations/0002_friends_presence.sql`; Modify `ui/src/lib/supabase.ts`.

**Interfaces:**
- Produces: the `friendships` + `presence` tables (+ RLS) and `profiles.show_activity`; the `Profile` type gains `show_activity: boolean`; `fetchProfile` selects it.

- [ ] **Step 1: Write `supabase/migrations/0002_friends_presence.sql`** — exactly the SQL from the spec's "Data model" section (friendships table + 4 policies; presence table + 4 policies; `alter table profiles add column show_activity boolean not null default true`).
- [ ] **Step 2: Extend `Profile` in `ui/src/lib/supabase.ts`** — add `show_activity: boolean` to the `Profile` interface, and add `show_activity` to the `fetchProfile` select list (`'id, username, display_name, avatar_url, level, xp, show_activity'`).
- [ ] **Step 3: Verify** `npm --prefix ui run build` (type-checks the Profile change) + `npm --prefix ui run test` (existing suite still green).
- [ ] **Step 4: Commit** `feat(social): friends+presence schema (0002) + Profile.show_activity`.

> Controller note: the DB `apply` is DEFERRED to owner-setup (shared prod Supabase); unit tests mock Supabase.

---

### Task 2: Friends data layer (`lib/friends.ts`)

**Files:** Create `ui/src/lib/friends.ts`, `ui/src/lib/friends.test.ts`.

**Interfaces:**
- Consumes: `supabase` (A).
- Produces: `FriendView`, `RequestView`, `listFriends()`, `listRequests()`, `sendRequest(username)`, `acceptRequest(id)`, `removeFriendship(id)`.

- [ ] **Step 1: Write the failing test `ui/src/lib/friends.test.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const { sb } = vi.hoisted(() => ({ sb: { auth: { getSession: vi.fn() }, from: vi.fn() } }))
vi.mock('./supabase', () => ({ supabase: sb }))
import { sendRequest } from './friends'

beforeEach(() => {
  sb.auth.getSession.mockResolvedValue({ data: { session: { user: { id: 'me' } } } })
})

describe('sendRequest', () => {
  it('rejects sending to yourself', async () => {
    sb.from.mockImplementation((t: string) => {
      if (t === 'profiles') return { select: () => ({ ilike: () => ({ maybeSingle: async () => ({ data: { id: 'me' } }) }) }) }
      return {}
    })
    await expect(sendRequest('Me')).rejects.toThrow(/you/i)
  })

  it('rejects an unknown username', async () => {
    sb.from.mockImplementation((t: string) => {
      if (t === 'profiles') return { select: () => ({ ilike: () => ({ maybeSingle: async () => ({ data: null }) }) }) }
      return {}
    })
    await expect(sendRequest('ghost')).rejects.toThrow(/no user/i)
  })

  it('inserts a pending request for a valid new friend', async () => {
    const insert = vi.fn(async () => ({ error: null }))
    sb.from.mockImplementation((t: string) => {
      if (t === 'profiles') return { select: () => ({ ilike: () => ({ maybeSingle: async () => ({ data: { id: 'them' } }) }) }) }
      if (t === 'friendships') return {
        select: () => ({ or: () => ({ maybeSingle: async () => ({ data: null }) }) }),
        insert,
      }
      return {}
    })
    await sendRequest('Them')
    expect(insert).toHaveBeenCalledWith({ requester: 'me', addressee: 'them', status: 'pending' })
  })
})
```

- [ ] **Step 2: Run it — fails** (`Cannot find module './friends'`). `npm --prefix ui run test -- src/lib/friends.test.ts`
- [ ] **Step 3: Write `ui/src/lib/friends.ts`**

```ts
import { supabase } from './supabase'

const ONLINE_MS = 60_000

export interface FriendView { id: string; userId: string; username: string; avatarUrl: string | null; online: boolean; activity: string | null }
export interface RequestView { id: string; userId: string; username: string; avatarUrl: string | null; direction: 'in' | 'out' }

async function myId(): Promise<string> {
  const { data } = await supabase.auth.getSession()
  const id = data.session?.user.id
  if (!id) throw new Error('Not signed in.')
  return id
}

export async function listFriends(): Promise<FriendView[]> {
  const me = await myId()
  const { data: rows, error } = await supabase
    .from('friendships').select('id, requester, addressee')
    .eq('status', 'accepted').or(`requester.eq.${me},addressee.eq.${me}`)
  if (error) throw new Error(error.message)
  const list = rows ?? []
  if (list.length === 0) return []
  const ids = list.map((r) => (r.requester === me ? r.addressee : r.requester))
  const [{ data: profiles }, { data: presence }] = await Promise.all([
    supabase.from('profiles').select('id, username, avatar_url').in('id', ids),
    supabase.from('presence').select('user_id, activity, updated_at').in('user_id', ids),
  ])
  const prof = new Map((profiles ?? []).map((p) => [p.id, p]))
  const pres = new Map((presence ?? []).map((p) => [p.user_id, p]))
  const now = Date.now()
  return list.map((r) => {
    const uid = r.requester === me ? r.addressee : r.requester
    const p = pres.get(uid)
    const online = !!p && now - new Date(p.updated_at).getTime() < ONLINE_MS
    return { id: r.id, userId: uid, username: prof.get(uid)?.username ?? 'unknown', avatarUrl: prof.get(uid)?.avatar_url ?? null, online, activity: online ? p?.activity ?? null : null }
  })
}

export async function listRequests(): Promise<RequestView[]> {
  const me = await myId()
  const { data: rows, error } = await supabase
    .from('friendships').select('id, requester, addressee')
    .eq('status', 'pending').or(`requester.eq.${me},addressee.eq.${me}`)
  if (error) throw new Error(error.message)
  const list = rows ?? []
  if (list.length === 0) return []
  const ids = list.map((r) => (r.requester === me ? r.addressee : r.requester))
  const { data: profiles } = await supabase.from('profiles').select('id, username, avatar_url').in('id', ids)
  const prof = new Map((profiles ?? []).map((p) => [p.id, p]))
  return list.map((r) => {
    const out = r.requester === me
    const uid = out ? r.addressee : r.requester
    return { id: r.id, userId: uid, username: prof.get(uid)?.username ?? 'unknown', avatarUrl: prof.get(uid)?.avatar_url ?? null, direction: out ? 'out' : 'in' }
  })
}

export async function sendRequest(username: string): Promise<void> {
  const me = await myId()
  const { data: target } = await supabase.from('profiles').select('id').ilike('username', username).maybeSingle()
  if (!target) throw new Error('No user with that username.')
  if (target.id === me) throw new Error("That's you!")
  const { data: existing } = await supabase.from('friendships').select('id, status')
    .or(`and(requester.eq.${me},addressee.eq.${target.id}),and(requester.eq.${target.id},addressee.eq.${me})`).maybeSingle()
  if (existing) throw new Error(existing.status === 'accepted' ? 'Already friends.' : 'A request already exists.')
  const { error } = await supabase.from('friendships').insert({ requester: me, addressee: target.id, status: 'pending' })
  if (error) throw new Error(error.message)
}

export async function acceptRequest(id: string): Promise<void> {
  const { error } = await supabase.from('friendships').update({ status: 'accepted', responded_at: new Date().toISOString() }).eq('id', id)
  if (error) throw new Error(error.message)
}

export async function removeFriendship(id: string): Promise<void> {
  const { error } = await supabase.from('friendships').delete().eq('id', id)
  if (error) throw new Error(error.message)
}
```

- [ ] **Step 4: Run the test — passes.** `npm --prefix ui run test -- src/lib/friends.test.ts`
- [ ] **Step 5: Commit** `feat(social): friends data layer (requests/accept/remove/list)`.

---

### Task 3: Presence publisher (`lib/presence.tsx`)

**Files:** Create `ui/src/lib/presence.tsx`, `ui/src/lib/presence.test.tsx`.

**Interfaces:**
- Consumes: `useAuth()` (A), `supabase` (A), `api.servers()` (`./api`).
- Produces: `<PresenceProvider>` that heartbeats the signed-in user's presence (activity from running servers), honoring `profile.show_activity`; clears on sign-out / toggle-off.

- [ ] **Step 1: Write the failing test `ui/src/lib/presence.test.tsx`**

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'

const { sb, mockAuth, apiMock } = vi.hoisted(() => ({
  sb: { from: vi.fn() },
  mockAuth: { current: { session: null as unknown, profile: null as unknown } },
  apiMock: { servers: vi.fn(async () => []) },
}))
vi.mock('./supabase', () => ({ supabase: sb }))
vi.mock('./auth', () => ({ useAuth: () => mockAuth.current }))
vi.mock('./api', () => ({ api: apiMock }))
import { PresenceProvider } from './presence'

beforeEach(() => { sb.from.mockReset() })

describe('PresenceProvider', () => {
  it('upserts presence when signed in with activity enabled', async () => {
    const upsert = vi.fn(async () => ({ error: null }))
    sb.from.mockReturnValue({ upsert, delete: () => ({ eq: vi.fn(async () => ({})) }) })
    apiMock.servers.mockResolvedValue([{ running: true, name: 'Craftoria' }])
    mockAuth.current = { session: { user: { id: 'me' } }, profile: { show_activity: true } }
    render(<PresenceProvider><div /></PresenceProvider>)
    await vi.waitFor(() => expect(upsert).toHaveBeenCalled())
    expect(upsert.mock.calls[0][0]).toMatchObject({ user_id: 'me', activity: 'Hosting Craftoria' })
  })

  it('clears presence (no upsert) when activity disabled', async () => {
    const upsert = vi.fn(async () => ({ error: null }))
    const eq = vi.fn(async () => ({}))
    sb.from.mockReturnValue({ upsert, delete: () => ({ eq }) })
    mockAuth.current = { session: { user: { id: 'me' } }, profile: { show_activity: false } }
    render(<PresenceProvider><div /></PresenceProvider>)
    await vi.waitFor(() => expect(eq).toHaveBeenCalledWith('user_id', 'me'))
    expect(upsert).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run it — fails.** `npm --prefix ui run test -- src/lib/presence.test.tsx`
- [ ] **Step 3: Write `ui/src/lib/presence.tsx`**

```tsx
import { useEffect, type ReactNode } from 'react'
import { supabase } from './supabase'
import { useAuth } from './auth'
import { api } from './api'

const HEARTBEAT_MS = 30_000

async function currentActivity(): Promise<string | null> {
  try {
    const servers = await api.servers()
    const running = servers.find((s) => s.running)
    return running ? `Hosting ${running.name}` : null
  } catch {
    return null
  }
}

export function PresenceProvider({ children }: { children: ReactNode }) {
  const { session, profile } = useAuth()
  const userId = session?.user.id
  const enabled = !!userId && profile?.show_activity !== false

  useEffect(() => {
    if (!userId) return
    if (!enabled) {
      void supabase.from('presence').delete().eq('user_id', userId)
      return
    }
    let alive = true
    const beat = async () => {
      const activity = await currentActivity()
      if (alive) await supabase.from('presence').upsert({ user_id: userId, activity, updated_at: new Date().toISOString() })
    }
    void beat()
    const t = setInterval(() => void beat(), HEARTBEAT_MS)
    return () => {
      alive = false
      clearInterval(t)
      void supabase.from('presence').delete().eq('user_id', userId)
    }
  }, [userId, enabled])

  return <>{children}</>
}
```

- [ ] **Step 4: Run the test — passes.** `npm --prefix ui run test -- src/lib/presence.test.tsx`
- [ ] **Step 5: Commit** `feat(social): PresenceProvider (heartbeat activity, privacy-gated)`.

---

### Task 4: AddFriendDialog

**Files:** Create `ui/src/components/social/AddFriendDialog.tsx`, `…/AddFriendDialog.test.tsx`.

**Interfaces:** Consumes `sendRequest` (Task 2), `friendlyError`. Produces `<AddFriendDialog onClose={() => void} onSent={() => void} />`.

- [ ] **Step 1: Write the failing test `…/AddFriendDialog.test.tsx`**

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const { sendRequest } = vi.hoisted(() => ({ sendRequest: vi.fn() }))
vi.mock('../../lib/friends', () => ({ sendRequest }))
import { AddFriendDialog } from './AddFriendDialog'

describe('AddFriendDialog', () => {
  it('sends a request then calls onSent', async () => {
    sendRequest.mockResolvedValue(undefined)
    const onSent = vi.fn()
    render(<AddFriendDialog onClose={() => {}} onSent={onSent} />)
    fireEvent.change(screen.getByPlaceholderText(/username/i), { target: { value: 'Tom' } })
    fireEvent.click(screen.getByRole('button', { name: /send request/i }))
    await waitFor(() => expect(sendRequest).toHaveBeenCalledWith('Tom'))
    await waitFor(() => expect(onSent).toHaveBeenCalled())
  })

  it('shows an error and does not call onSent on failure', async () => {
    sendRequest.mockRejectedValue(new Error('No user with that username.'))
    const onSent = vi.fn()
    render(<AddFriendDialog onClose={() => {}} onSent={onSent} />)
    fireEvent.change(screen.getByPlaceholderText(/username/i), { target: { value: 'ghost' } })
    fireEvent.click(screen.getByRole('button', { name: /send request/i }))
    await waitFor(() => expect(screen.getByText(/no user/i)).toBeInTheDocument())
    expect(onSent).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run it — fails.**
- [ ] **Step 3: Write `ui/src/components/social/AddFriendDialog.tsx`**

```tsx
import { useState } from 'react'
import { sendRequest } from '../../lib/friends'
import { friendlyError } from '../../lib/errors'

export function AddFriendDialog({ onClose, onSent }: { onClose: () => void; onSent: () => void }) {
  const [name, setName] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit() {
    setErr(null)
    setBusy(true)
    try {
      await sendRequest(name.trim())
      onSent()
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={busy ? undefined : onClose}>
      <div className="panel w-full max-w-xs p-6" onClick={(e) => e.stopPropagation()}>
        <h3 className="mb-3 text-sm font-semibold text-zinc-100">Add a friend</h3>
        <input
          placeholder="Username"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
        />
        {err && <p className="mt-2 text-xs text-rose-400">{err}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-lg px-3 py-1.5 text-sm text-zinc-400 hover:text-zinc-200">Cancel</button>
          <button onClick={submit} disabled={busy || !name.trim()} className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50">Send request</button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Run the test — passes.**
- [ ] **Step 5: Commit** `feat(social): AddFriendDialog`.

---

### Task 5: RequestsInbox

**Files:** Create `ui/src/components/social/RequestsInbox.tsx`, `…/RequestsInbox.test.tsx`.

**Interfaces:** Consumes `listRequests`, `acceptRequest`, `removeFriendship` (Task 2), `friendlyError`. Produces `<RequestsInbox onChanged={() => void} />` (re-fetches on mount; calls `onChanged` after accept/decline so the parent refreshes counts + friends).

- [ ] **Step 1: Write the failing test `…/RequestsInbox.test.tsx`**

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const { listRequests, acceptRequest, removeFriendship } = vi.hoisted(() => ({
  listRequests: vi.fn(), acceptRequest: vi.fn(), removeFriendship: vi.fn(),
}))
vi.mock('../../lib/friends', () => ({ listRequests, acceptRequest, removeFriendship }))
import { RequestsInbox } from './RequestsInbox'

describe('RequestsInbox', () => {
  it('accepts an incoming request', async () => {
    listRequests.mockResolvedValue([{ id: 'r1', userId: 'u1', username: 'Tom', avatarUrl: null, direction: 'in' }])
    acceptRequest.mockResolvedValue(undefined)
    render(<RequestsInbox onChanged={() => {}} />)
    await waitFor(() => expect(screen.getByText('Tom')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /accept/i }))
    await waitFor(() => expect(acceptRequest).toHaveBeenCalledWith('r1'))
  })
})
```

- [ ] **Step 2: Run it — fails.**
- [ ] **Step 3: Write `ui/src/components/social/RequestsInbox.tsx`**

```tsx
import { useCallback, useEffect, useState } from 'react'
import { listRequests, acceptRequest, removeFriendship, type RequestView } from '../../lib/friends'
import { friendlyError } from '../../lib/errors'

export function RequestsInbox({ onChanged }: { onChanged: () => void }) {
  const [reqs, setReqs] = useState<RequestView[]>([])
  const [err, setErr] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setReqs(await listRequests())
    } catch (e) {
      setErr(friendlyError(e))
    }
  }, [])
  useEffect(() => { void load() }, [load])

  async function act(id: string, fn: (id: string) => Promise<void>) {
    setErr(null)
    try {
      await fn(id)
      await load()
      onChanged()
    } catch (e) {
      setErr(friendlyError(e))
    }
  }

  if (reqs.length === 0) return <p className="px-1 py-2 text-[11px] text-zinc-600">No pending requests.</p>
  return (
    <div className="flex flex-col gap-2">
      {err && <p className="text-xs text-rose-400">{err}</p>}
      {reqs.map((r) => (
        <div key={r.id} className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-sm text-zinc-200">{r.username}</span>
          {r.direction === 'in' ? (
            <>
              <button onClick={() => act(r.id, acceptRequest)} className="rounded bg-emerald-500 px-2 py-0.5 text-xs font-semibold text-zinc-950 hover:bg-emerald-400">Accept</button>
              <button onClick={() => act(r.id, removeFriendship)} className="rounded border border-zinc-700 px-2 py-0.5 text-xs text-zinc-400 hover:text-zinc-200">Decline</button>
            </>
          ) : (
            <button onClick={() => act(r.id, removeFriendship)} className="rounded border border-zinc-700 px-2 py-0.5 text-xs text-zinc-500 hover:text-zinc-300">Pending · Cancel</button>
          )}
        </div>
      ))}
    </div>
  )
}
```

- [ ] **Step 4: Run the test — passes.**
- [ ] **Step 5: Commit** `feat(social): RequestsInbox (accept/decline/cancel)`.

---

### Task 6: FriendsList

**Files:** Create `ui/src/components/social/FriendsList.tsx`, `…/FriendsList.test.tsx`.

**Interfaces:** Consumes `listFriends`, `removeFriendship` (Task 2), `friendlyError`. Produces `<FriendsList refreshKey={number} onChanged={() => void} />` — fetches on mount and whenever `refreshKey` changes (the parent bumps it on a poll); renders rows; remove calls `removeFriendship` then `onChanged`.

- [ ] **Step 1: Write the failing test `…/FriendsList.test.tsx`**

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

const { listFriends, removeFriendship } = vi.hoisted(() => ({ listFriends: vi.fn(), removeFriendship: vi.fn() }))
vi.mock('../../lib/friends', () => ({ listFriends, removeFriendship }))
import { FriendsList } from './FriendsList'

describe('FriendsList', () => {
  it('renders a friend with online activity', async () => {
    listFriends.mockResolvedValue([{ id: 'f1', userId: 'u1', username: 'Tom', avatarUrl: null, online: true, activity: 'Hosting Craftoria' }])
    render(<FriendsList refreshKey={0} onChanged={() => {}} />)
    await waitFor(() => expect(screen.getByText('Tom')).toBeInTheDocument())
    expect(screen.getByText(/hosting craftoria/i)).toBeInTheDocument()
  })

  it('shows an empty state when there are no friends', async () => {
    listFriends.mockResolvedValue([])
    render(<FriendsList refreshKey={0} onChanged={() => {}} />)
    await waitFor(() => expect(screen.getByText(/no friends yet/i)).toBeInTheDocument())
  })
})
```

- [ ] **Step 2: Run it — fails.**
- [ ] **Step 3: Write `ui/src/components/social/FriendsList.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { listFriends, removeFriendship, type FriendView } from '../../lib/friends'
import { friendlyError } from '../../lib/errors'

export function FriendsList({ refreshKey, onChanged }: { refreshKey: number; onChanged: () => void }) {
  const [friends, setFriends] = useState<FriendView[] | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [menuFor, setMenuFor] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    listFriends()
      .then((f) => alive && setFriends(f))
      .catch((e) => alive && setErr(friendlyError(e)))
    return () => { alive = false }
  }, [refreshKey])

  async function remove(id: string) {
    setMenuFor(null)
    try {
      await removeFriendship(id)
      onChanged()
    } catch (e) {
      setErr(friendlyError(e))
    }
  }

  if (friends && friends.length === 0) return <p className="px-1 py-2 text-[11px] text-zinc-600">No friends yet — add some.</p>
  return (
    <div className="flex flex-col gap-2">
      {err && <p className="text-xs text-rose-400">{err}</p>}
      {(friends ?? []).map((f) => (
        <div key={f.id} className="group flex items-center gap-2">
          <span className="relative inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-zinc-800 text-xs text-zinc-300">
            {f.avatarUrl ? <img src={f.avatarUrl} alt="" className="h-7 w-7 rounded-full" /> : f.username.charAt(0).toUpperCase()}
            <span className={`absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border-2 border-zinc-950 ${f.online ? 'bg-emerald-400' : 'bg-zinc-600'}`} />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm text-zinc-200">{f.username}</span>
            <span className="block truncate text-[11px] text-zinc-500">{f.online ? f.activity ?? 'Online' : 'Offline'}</span>
          </span>
          <button onClick={() => setMenuFor(menuFor === f.id ? null : f.id)} aria-label="Friend options" className="text-zinc-600 opacity-0 group-hover:opacity-100 hover:text-zinc-300">⋯</button>
          {menuFor === f.id && (
            <button onClick={() => remove(f.id)} className="rounded border border-zinc-700 px-2 py-0.5 text-xs text-rose-300 hover:bg-zinc-800">Remove</button>
          )}
        </div>
      ))}
    </div>
  )
}
```

- [ ] **Step 4: Run the test — passes.**
- [ ] **Step 5: Commit** `feat(social): FriendsList (presence rows + remove)`.

---

### Task 7: Wire the live Friends section into SocialSidebar

**Files:** Modify `ui/src/components/social/SocialSidebar.tsx`, `…/SocialSidebar.test.tsx`.

**Interfaces:** Consumes `FriendsList`, `AddFriendDialog`, `RequestsInbox` (Tasks 4–6), `PresenceProvider` (Task 3), `useAuth().refreshProfile` + `supabase` (toggle), `listRequests` (badge count).

- [ ] **Step 1: Replace the placeholder Friends block (current lines 26–33) and wire state.** The signed-in branch becomes (preserving the existing ProfileBlock + menu above the divider):

```tsx
          <div className="my-4 h-px bg-zinc-800/80" />
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">Friends</span>
            <div className="flex items-center gap-1">
              <button onClick={() => setShowRequests((v) => !v)} className="relative rounded border border-zinc-800 px-1.5 text-xs text-zinc-400 hover:text-zinc-200">
                Requests{requestCount > 0 ? <span className="ml-1 rounded-full bg-emerald-500 px-1 text-[10px] font-semibold text-zinc-950">{requestCount}</span> : null}
              </button>
              <button onClick={() => setShowAdd(true)} className="rounded border border-zinc-800 px-1.5 text-xs text-zinc-400 hover:text-zinc-200">+ Add</button>
            </div>
          </div>
          {showRequests && <div className="mt-2"><RequestsInbox onChanged={() => { bump(); void refreshRequestCount() }} /></div>}
          <div className="mt-3 min-h-0 flex-1 overflow-y-auto"><FriendsList refreshKey={refreshKey} onChanged={bump} /></div>
          {showAdd && <AddFriendDialog onClose={() => setShowAdd(false)} onSent={() => { bump(); void refreshRequestCount() }} />}
```

Add the imports and, inside the component, this state + helpers (alongside the existing `useState`s):

```tsx
import { useCallback, useEffect, useState } from 'react'
import { PresenceProvider } from '../../lib/presence'
import { FriendsList } from './FriendsList'
import { AddFriendDialog } from './AddFriendDialog'
import { RequestsInbox } from './RequestsInbox'
import { listRequests } from '../../lib/friends'
import { supabase } from '../../lib/supabase'
// …existing imports…

  const [showAdd, setShowAdd] = useState(false)
  const [showRequests, setShowRequests] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)
  const [requestCount, setRequestCount] = useState(0)
  const bump = useCallback(() => setRefreshKey((k) => k + 1), [])
  const refreshRequestCount = useCallback(async () => {
    try { setRequestCount((await listRequests()).filter((r) => r.direction === 'in').length) } catch { /* keep */ }
  }, [])
  // poll friends + request count every 15s while signed in
  useEffect(() => {
    if (!session) return
    void refreshRequestCount()
    const t = setInterval(() => { bump(); void refreshRequestCount() }, 15_000)
    return () => clearInterval(t)
  }, [session, bump, refreshRequestCount])
```

- [ ] **Step 2: Mount `PresenceProvider` and add the privacy toggle to the account menu.** Wrap the signed-in branch's content in `<PresenceProvider>…</PresenceProvider>`, and add to the account menu (next to Change username / Sign out):

```tsx
              <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={async () => {
                setShowMenu(false)
                if (!profile) return
                await supabase.from('profiles').update({ show_activity: !profile.show_activity }).eq('id', profile.id)
                await refreshProfile()
              }}>{profile?.show_activity === false ? 'Show my activity to friends' : 'Hide my activity from friends'}</button>
```

(Add `refreshProfile` to the `useAuth()` destructure.)

- [ ] **Step 3: Update `SocialSidebar.test.tsx`** — the existing signed-in test asserted the placeholder; now mock the new children so the signed-in test stays unit-scoped:

```tsx
vi.mock('./FriendsList', () => ({ FriendsList: () => <div>friends-list</div> }))
vi.mock('./RequestsInbox', () => ({ RequestsInbox: () => <div>requests</div> }))
vi.mock('./AddFriendDialog', () => ({ AddFriendDialog: () => <div>add-friend</div> }))
vi.mock('../../lib/presence', () => ({ PresenceProvider: ({ children }: { children: React.ReactNode }) => <>{children}</> }))
vi.mock('../../lib/friends', () => ({ listRequests: vi.fn(async () => []) }))
```
Change the signed-in assertion from the old placeholder text to `expect(screen.getByText('friends-list')).toBeInTheDocument()` and keep the signed-out test. Ensure the signed-in `useAuth` mock includes `refreshProfile: vi.fn()` and `profile.id`/`profile.show_activity`.

- [ ] **Step 4: Verify** `npm --prefix ui run lint` (0 new) + `npm --prefix ui run build` + `npm --prefix ui run test` (full suite green).
- [ ] **Step 5: Commit** `feat(social): live Friends section + presence + privacy toggle in SocialSidebar`.

---

### Task 8: Owner-setup doc — append the 0002 migration (controller-handled)

**Files:** Modify `docs/accounts-owner-setup.md`.

- [ ] **Step 1:** In section 4 (Database), add a line to also apply `supabase/migrations/0002_friends_presence.sql` (after `0001`). No new providers/secrets.
- [ ] **Step 2: Commit** `docs: owner-setup — apply the 0002 friends/presence migration`.

---

## Self-Review Notes
- Spec coverage: friendships+presence schema + RLS (T1); data layer requests/accept/remove/list (T2); presence heartbeat + privacy gate (T3); add/requests/list UI (T4–6); live sidebar + toggle + polling (T7); owner doc (T8). All spec sections map to a task.
- Out-of-scope held: no realtime (polling), no blocking/DMs/notifications.
- Type consistency: `FriendView`/`RequestView` defined in T2 and consumed in T5/T6; `PresenceProvider` (T3) consumed in T7; `Profile.show_activity` added in T1 and consumed in T3/T7; `sendRequest/acceptRequest/removeFriendship/listFriends/listRequests` names consistent across T2 and consumers.
- Known external dependency: end-to-end needs the owner to apply `0002` (+ A's setup) and two accounts; unit tests mock Supabase + `api.servers()`.
