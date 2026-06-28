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
  base      text;
  candidate text;
  tries     int := 0;
  reserved  text[] := array['admin','administrator','system','gamenest','support','mod','moderator','root','null','undefined'];
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
