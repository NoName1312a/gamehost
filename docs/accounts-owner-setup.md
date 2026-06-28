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
