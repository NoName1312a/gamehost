import { useState } from 'react'
import { supabase } from '../../lib/supabase'
import { useAuth } from '../../lib/auth'
import { validateUsername, isUsernameAvailable } from '../../lib/username'
import { friendlyError } from '../../lib/errors'

export function UsernameDialog({ current, onClose }: { current: string; onClose: () => void }) {
  const { refreshProfile } = useAuth()
  const [name, setName] = useState(current)
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function save() {
    setErr(null)
    setBusy(true)
    try {
      const v = validateUsername(name)
      if (v) throw new Error(v)
      if (name.toLowerCase() !== current.toLowerCase() && !(await isUsernameAvailable(name))) {
        throw new Error('That username is taken.')
      }
      const { error } = await supabase.from('profiles').update({ username: name }).eq('username', current)
      if (error) throw new Error(error.message)
      await refreshProfile()
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
      <div className="panel w-full max-w-xs p-6" onClick={(e) => e.stopPropagation()}>
        <h3 className="mb-3 text-sm font-semibold text-zinc-100">Change username</h3>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
        />
        {err && <p className="mt-2 text-xs text-rose-400">{err}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-lg px-3 py-1.5 text-sm text-zinc-400 hover:text-zinc-200">Cancel</button>
          <button onClick={save} disabled={busy} className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50">Save</button>
        </div>
      </div>
    </div>
  )
}
