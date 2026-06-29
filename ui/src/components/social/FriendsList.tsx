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
