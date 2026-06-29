import { supabase } from './supabase'

const PER_LEVEL = 100

export function levelForXp(xp: number): number {
  return Math.floor(Math.sqrt(Math.max(0, xp) / PER_LEVEL)) + 1
}

export function xpForLevel(level: number): number {
  return (level - 1) * (level - 1) * PER_LEVEL
}

export interface XpProgress { level: number; into: number; span: number; percent: number }

export function xpProgress(xp: number): XpProgress {
  const level = levelForXp(xp)
  const base = xpForLevel(level)
  const span = xpForLevel(level + 1) - base
  const into = Math.max(0, xp) - base
  return { level, into, span, percent: span > 0 ? Math.min(100, Math.round((into / span) * 100)) : 0 }
}

/** Best-effort, cosmetic. Adds delta to the current user's xp and stores the
 *  recomputed level. Silently no-ops when signed out. */
export async function awardXp(delta: number): Promise<void> {
  const { data: s } = await supabase.auth.getSession()
  const id = s.session?.user.id
  if (!id) return
  const { data: prof } = await supabase.from('profiles').select('xp').eq('id', id).maybeSingle()
  const xp = ((prof?.xp as number | undefined) ?? 0) + delta
  await supabase.from('profiles').update({ xp, level: levelForXp(xp) }).eq('id', id)
}
