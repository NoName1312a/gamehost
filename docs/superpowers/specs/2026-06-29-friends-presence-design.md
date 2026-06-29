# GameNest Friends + Presence — Design Spec

**Date:** 2026-06-29
**Status:** Designed autonomously (owner delegated "continue the next steps"); **review on return**
**Sub-project:** B of 3 (A · Accounts ✓ → **B · Friends + presence** → C · Levels/XP)
**Builds on:** `feat/accounts` (sub-project A). Continues on the same branch — A + B + C ship together after the one owner Supabase/Discord setup.

## Summary

Make the social sidebar's Friends section live: signed-in users add friends by username (mutual request → accept/decline), and see each friend's **presence** — online/offline plus what they're doing ("Hosting Craftoria"). Anonymous use is unchanged; this only appears when signed in.

## Decisions made (review these)

| Decision | Choice | Rationale |
|---|---|---|
| Friend model | **Mutual requests** (send by username → accept/decline → friends) | Standard (Discord/Modrinth); avoids one-sided follows; symmetric |
| Decline / remove | Decline **deletes** the request row; remove deletes the friendship | Simpler than a `declined` tombstone; re-request allowed |
| Presence transport | **Polling** (refresh friends+presence every ~15s) for the MVP | Robust, simple; Supabase **Realtime** is a noted upgrade |
| Activity source | The engine's running servers (`api.servers()`) → "Hosting `<name>`" else "Online" | No engine change; the UI already has this data |
| Presence privacy | **Friends-only read** (RLS) + a per-user **"Show my activity to friends"** toggle (default ON) | Matches Steam/Discord norm; respects privacy; can go invisible |
| Online definition | `presence.updated_at` within **60s** = online; older/absent = offline | No explicit "offline" write needed; stale = offline |
| Blocking, DMs/chat, push notifications | **Out of scope** (future) | Keep B shippable |

## Scope

### In scope (B)
- `friendships` + `presence` tables (+ RLS) in the existing Supabase project (migration `0002`).
- A friends data layer (`ui/src/lib/friends.ts`): list friends (with presence), list incoming/outgoing requests, send/accept/decline/cancel/remove.
- A presence publisher (`ui/src/lib/presence.tsx`): while signed-in + toggle-on, upsert my presence on a ~30s heartbeat and on server start/stop; activity derived from `api.servers()`; clear on sign-out / toggle-off.
- The SocialSidebar Friends section goes live: friend rows (avatar, username, presence dot + activity), an **Add friend** dialog (search username → send request), a **requests** inbox (incoming count badge → accept/decline), and a per-friend **remove**.
- A privacy toggle ("Show my activity to friends") in the account menu.

### Out of scope (deferred)
- **C:** levels/XP (separate sub-project).
- Realtime presence (polling MVP; Realtime upgrade later), blocking, DMs/chat, OS/push notifications, friend suggestions, profiles-of-others pages.

### Explicitly unchanged
- The Go engine (B reads `api.servers()`, already exposed). Anonymous flows. A's auth/profile code (B consumes `useAuth()` + the `profiles` table; it does not modify them).

## Architecture

```
React UI (signed-in only)
  ├── lib/friends.ts     — friendships queries/mutations (supabase)
  ├── lib/presence.tsx   — PresenceProvider: heartbeat upsert of my presence;
  │                         activity from api.servers(); honors the privacy toggle
  ├── components/social/
  │     ├── SocialSidebar.tsx  — (modified) live Friends section
  │     ├── FriendsList.tsx    — friend rows + presence + remove
  │     ├── AddFriendDialog.tsx— search username → send request
  │     └── RequestsInbox.tsx  — incoming requests → accept/decline
  └── reuses A: useAuth(), supabase, Profile, validateUsername
Supabase: friendships + presence tables (RLS) ── engine UNCHANGED
```

Friends + presence are **read by polling** a `useFriends()` hook (re-query every ~15s). Presence is **written** by `PresenceProvider` (mounted inside `AuthProvider`, in the signed-in tree).

## Data model (migration `supabase/migrations/0002_friends_presence.sql`)

```sql
-- one row per relationship; requester initiates, addressee responds
create table public.friendships (
  id           uuid primary key default gen_random_uuid(),
  requester    uuid not null references auth.users(id) on delete cascade,
  addressee    uuid not null references auth.users(id) on delete cascade,
  status       text not null default 'pending' check (status in ('pending','accepted')),
  created_at   timestamptz not null default now(),
  responded_at timestamptz,
  constraint friendship_distinct check (requester <> addressee),
  constraint friendship_unique unique (requester, addressee)
);
-- prevent duplicate inverse pairs at the app layer (check both directions before insert)

alter table public.friendships enable row level security;
-- see rows you're part of
create policy friendships_read on public.friendships for select to authenticated
  using (auth.uid() = requester or auth.uid() = addressee);
-- send a request as yourself
create policy friendships_insert on public.friendships for insert to authenticated
  with check (auth.uid() = requester and status = 'pending');
-- accept: only the addressee may flip pending->accepted
create policy friendships_update on public.friendships for update to authenticated
  using (auth.uid() = addressee) with check (auth.uid() = addressee);
-- cancel/decline/remove: either party may delete
create policy friendships_delete on public.friendships for delete to authenticated
  using (auth.uid() = requester or auth.uid() = addressee);

create table public.presence (
  user_id    uuid primary key references auth.users(id) on delete cascade,
  activity   text,                       -- e.g. 'Hosting Craftoria'; null = just online
  updated_at timestamptz not null default now()
);
alter table public.presence enable row level security;
-- read presence of yourself or an accepted friend only
create policy presence_read on public.presence for select to authenticated using (
  user_id = auth.uid()
  or exists (
    select 1 from public.friendships f
    where f.status = 'accepted'
      and ((f.requester = auth.uid() and f.addressee = presence.user_id)
        or (f.addressee = auth.uid() and f.requester = presence.user_id))
  )
);
-- write only your own presence
create policy presence_write on public.presence for insert to authenticated with check (user_id = auth.uid());
create policy presence_update on public.presence for update to authenticated using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy presence_delete on public.presence for delete to authenticated using (user_id = auth.uid());
```

Add to `profiles` (extends A's table; migration `0002`): `show_activity boolean not null default true` (the privacy toggle).

**Apply is DEFERRED** to the same owner-setup as A (shared prod Supabase). Unit tests mock Supabase, so the build doesn't need it. (Doc updated in the owner-setup guide.)

## Friends data layer (`ui/src/lib/friends.ts`)

```ts
export interface FriendView { id: string; userId: string; username: string; avatarUrl: string | null; online: boolean; activity: string | null }
export interface RequestView { id: string; userId: string; username: string; avatarUrl: string | null; direction: 'in' | 'out' }
export async function listFriends(): Promise<FriendView[]>          // accepted; joins profiles + presence; online = updated_at < 60s
export async function listRequests(): Promise<RequestView[]>        // pending; in = addressee me, out = requester me
export async function sendRequest(username: string): Promise<void>  // resolve username->id; reject self; reject if a row already exists either direction; insert pending
export async function acceptRequest(id: string): Promise<void>      // update status='accepted', responded_at=now()
export async function removeFriendship(id: string): Promise<void>   // delete (covers decline / cancel / unfriend)
```

`online` is computed in the UI from `updated_at` (within 60s). `activity` comes from the joined `presence` row.

## Presence publisher (`ui/src/lib/presence.tsx`)

`PresenceProvider` (mounted in the signed-in tree): when there's a session and `profile.show_activity` is true, every ~30s (and immediately on mount and whenever the running-server set changes) it upserts `presence` with `activity` derived from `api.servers()` — the first running server → `"Hosting <name>"`, else `null`. When signed out or the toggle is off, it deletes the user's presence row and stops. Uses the existing engine polling already present in the app (or its own light `api.servers()` poll). No engine change.

## UI

- **SocialSidebar.tsx (modified):** replace the placeholder (lines 26–33) with the live Friends section — `<FriendsList />` + an active **+ Add** button (opens `<AddFriendDialog />`) + a requests affordance showing the incoming count (opens `<RequestsInbox />`). Mount `<PresenceProvider>` so presence publishes while the sidebar is shown signed-in. Add "Show my activity to friends" to the account menu (toggles `profiles.show_activity`).
- **FriendsList.tsx:** rows of avatar + username + a presence dot (emerald online / zinc offline) + activity text; a per-row menu with **Remove**. Empty state: "No friends yet — add some."
- **AddFriendDialog.tsx:** username input → `sendRequest`; inline errors (not found / already friends / yourself / request exists). Success → toast/close.
- **RequestsInbox.tsx:** incoming requests with Accept / Decline; outgoing shown as "Pending." Count badge drives the sidebar affordance.

Theme: emerald/zinc, `.panel` for dialogs, mirrors A's components.

## Error handling
- All Supabase calls surface via `friendlyError`; failed reads keep the last list (no forced wipe). Duplicate/again requests fail with a clear message. Presence write failures are silent (best-effort, retried next heartbeat).

## Testing
- **UI unit (Vitest + Testing Library, mocked supabase/engine):** `friends.ts` (sendRequest rejects self / existing / resolves username→id; listFriends maps online from updated_at; listRequests direction); `PresenceProvider` (upserts when toggle on + session present; deletes/stops when off or signed out; activity from a mocked `api.servers()`); `AddFriendDialog` (invalid → error, no insert); `RequestsInbox` (accept calls update, decline calls delete); `FriendsList` (renders online/offline + activity; remove calls delete).
- **Supabase RLS (SQL tests):** can't read non-friends' presence; can't accept a request you didn't receive; can't insert a request as someone else; either party can delete.
- **Manual e2e** (needs owner setup + two accounts): send/accept → both see each other; host a server → friend sees "Hosting <name>"; toggle privacy off → friend sees offline.

## Owner setup (additional to A)
Apply `supabase/migrations/0002_friends_presence.sql` alongside A's `0001`. No new providers/secrets. (Append to `docs/accounts-owner-setup.md`.)

## Future (not B)
Realtime presence (replace polling with Supabase Realtime channels), blocking, DMs/chat, OS notifications for requests, "what your friends are hosting" discovery feed. C (levels/XP) is the next sub-project.
