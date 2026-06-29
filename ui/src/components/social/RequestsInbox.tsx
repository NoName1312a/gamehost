import { useCallback, useEffect, useState } from 'react'
import { listRequests, acceptRequest, removeFriendship, type RequestView } from '../../lib/friends'
import { friendlyError } from '../../lib/errors'

export function RequestsInbox({ onChanged }: { onChanged: () => void }) {
  const [reqs, setReqs] = useState<RequestView[]>([])
  const [err, setErr] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setReqs(await listRequests())
    } catch (e) {
      setErr(friendlyError(e))
    }
  }, [])
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { void load() }, [load])

  async function act(id: string, fn: (id: string) => Promise<void>) {
    setErr(null)
    try {
      await fn(id)
      await load()
      onChanged()
    } catch (e) {
      setErr(friendlyError(e))
    }
  }

  if (reqs.length === 0) return <p className="px-1 py-2 text-[11px] text-zinc-600">No pending requests.</p>
  return (
    <div className="flex flex-col gap-2">
      {err && <p className="text-xs text-rose-400">{err}</p>}
      {reqs.map((r) => (
        <div key={r.id} className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-sm text-zinc-200">{r.username}</span>
          {r.direction === 'in' ? (
            <>
              <button onClick={() => act(r.id, acceptRequest)} className="rounded bg-emerald-500 px-2 py-0.5 text-xs font-semibold text-zinc-950 hover:bg-emerald-400">Accept</button>
              <button onClick={() => act(r.id, removeFriendship)} className="rounded border border-zinc-700 px-2 py-0.5 text-xs text-zinc-400 hover:text-zinc-200">Decline</button>
            </>
          ) : (
            <button onClick={() => act(r.id, removeFriendship)} className="rounded border border-zinc-700 px-2 py-0.5 text-xs text-zinc-500 hover:text-zinc-300">Pending · Cancel</button>
          )}
        </div>
      ))}
    </div>
  )
}
