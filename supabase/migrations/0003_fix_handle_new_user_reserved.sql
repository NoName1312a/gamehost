-- Fix reserved-username bypass: the reserved check compared case-sensitively
-- (candidate = any(reserved)) while usernames are citext-unique (case-insensitive),
-- so 'Admin'/'GameNest'/'Support' slipped through and could impersonate staff.
-- Compare case-insensitively. Function body is otherwise unchanged.
create or replace function public.handle_new_user()
returns trigger
language plpgsql
security definer
set search_path to 'public'
as $function$
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
  while (lower(candidate) = any(reserved)) or exists(select 1 from public.profiles where username = candidate) loop
    candidate := left(base, 16) || lpad((floor(random()*9000)+1000)::int::text, 4, '0');
    tries := tries + 1;
    if tries > 8 then candidate := 'player' || lpad((floor(random()*900000)+100000)::int::text, 6, '0'); end if;
  end loop;
  insert into public.profiles (id, username, display_name, avatar_url)
  values (new.id, candidate, new.raw_user_meta_data->>'name', new.raw_user_meta_data->>'avatar_url');
  return new;
end;
$function$;
