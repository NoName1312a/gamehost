import { useState } from 'react'
import { sendRequest } from '../../lib/friends'
import { friendlyError } from '../../lib/errors'

export function AddFriendDialog({ onClose, onSent }: { onClose: () => void; onSent: () => void }) {
  const [name, setName] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit() {
    setErr(null)
    setBusy(true)
    try {
      await sendRequest(name.trim())
      onSent()
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={busy ? undefined : onClose}>
      <div className="panel w-full max-w-xs p-6" onClick={(e) => e.stopPropagation()}>
        <h3 className="mb-3 text-sm font-semibold text-zinc-100">Add a friend</h3>
        <input
          placeholder="Username"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
        />
        {err && <p className="mt-2 text-xs text-rose-400">{err}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-lg px-3 py-1.5 text-sm text-zinc-400 hover:text-zinc-200">Cancel</button>
          <button onClick={submit} disabled={busy || !name.trim()} className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50">Send request</button>
        </div>
      </div>
    </div>
  )
}
