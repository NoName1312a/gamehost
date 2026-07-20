import { useState, type FormEvent } from 'react'
import { open } from '@tauri-apps/plugin-shell'
import { useAuth } from '../../lib/auth'
import { signInWithDiscord } from '../../lib/discord-oauth'
import { friendlyError } from '../../lib/errors'
import { WEB_BASE } from '../../lib/site'

export function SignInPanel({ onClose }: { onClose: () => void }) {
  const { signInEmail } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setErr(null)
    setBusy(true)
    try {
      await signInEmail(email, password)
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  async function discord() {
    setErr(null)
    setBusy(true)
    try {
      await signInWithDiscord()
      onClose()
    } catch (e) {
      setErr(friendlyError(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-6" onClick={onClose}>
      <div className="panel w-full max-w-sm p-6" onClick={(e) => e.stopPropagation()}>
        <h2 className="font-display mb-4 text-center text-base font-semibold text-zinc-100">
          Sign in to GameNest
        </h2>
        <button
          onClick={discord}
          disabled={busy}
          className="w-full rounded-lg bg-[#5865F2] px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
        >
          Continue with Discord
        </button>
        <div className="my-3 flex items-center gap-2 text-xs text-zinc-600">
          <div className="h-px flex-1 bg-zinc-800" /> or <div className="h-px flex-1 bg-zinc-800" />
        </div>
        <form onSubmit={submit} className="flex flex-col gap-2">
          <input
            placeholder="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
          />
          <input
            placeholder="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500"
          />
          {err && <p className="text-xs text-rose-400">{err}</p>}
          <button
            type="submit"
            disabled={busy}
            className="mt-1 w-full rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-50"
          >
            Sign in with email
          </button>
        </form>
        <p className="mt-3 text-center text-xs text-zinc-500">
          <button className="text-emerald-400" onClick={() => open(`${WEB_BASE}/reset`)}>Forgot password?</button>
        </p>
        <p className="mt-1 text-center text-xs text-zinc-500">
          No account?{' '}
          <button className="text-emerald-400" onClick={() => open(`${WEB_BASE}/signup`)}>Create one on the web</button>
        </p>
      </div>
    </div>
  )
}
