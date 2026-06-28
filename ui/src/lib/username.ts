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
