-- Friends (mutual requests) + presence (friends-only) for the social layer.

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

alter table public.friendships enable row level security;
create policy friendships_read on public.friendships for select to authenticated
  using (auth.uid() = requester or auth.uid() = addressee);
create policy friendships_insert on public.friendships for insert to authenticated
  with check (auth.uid() = requester and status = 'pending');
create policy friendships_update on public.friendships for update to authenticated
  using (auth.uid() = addressee) with check (auth.uid() = addressee);
create policy friendships_delete on public.friendships for delete to authenticated
  using (auth.uid() = requester or auth.uid() = addressee);

create table public.presence (
  user_id    uuid primary key references auth.users(id) on delete cascade,
  activity   text,
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
create policy presence_write on public.presence for insert to authenticated with check (user_id = auth.uid());
create policy presence_update on public.presence for update to authenticated using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy presence_delete on public.presence for delete to authenticated using (user_id = auth.uid());

-- privacy toggle for activity broadcast (default on)
alter table public.profiles add column if not exists show_activity boolean not null default true;
