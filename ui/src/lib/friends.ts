import { supabase } from './supabase'
import { awardXp } from './xp'

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
  void awardXp(50)
}

export async function removeFriendship(id: string): Promise<void> {
  const { error } = await supabase.from('friendships').delete().eq('id', id)
  if (error) throw new Error(error.message)
}
