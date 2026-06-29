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
  show_activity: boolean
}

export async function fetchProfile(userId: string): Promise<Profile | null> {
  const { data } = await supabase
    .from('profiles')
    .select('id, username, display_name, avatar_url, level, xp, show_activity')
    .eq('id', userId)
    .maybeSingle()
  return (data as Profile | null) ?? null
}
