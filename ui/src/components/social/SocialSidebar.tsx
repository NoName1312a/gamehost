import { useState } from 'react'
import { useAuth } from '../../lib/auth'
import { ProfileBlock } from './ProfileBlock'
import { SignInPanel } from './SignInPanel'
import { UsernameDialog } from './UsernameDialog'

export function SocialSidebar() {
  const { session, profile, loading, signOut } = useAuth()
  const [showSignIn, setShowSignIn] = useState(false)
  const [showMenu, setShowMenu] = useState(false)
  const [showRename, setShowRename] = useState(false)

  return (
    <aside className="hidden w-72 shrink-0 flex-col border-l border-zinc-800/80 bg-zinc-950/60 p-4 backdrop-blur lg:flex">
      {loading ? (
        <p className="text-xs text-zinc-600">…</p>
      ) : session && profile ? (
        <>
          <ProfileBlock profile={profile} onMenu={() => setShowMenu((v) => !v)} />
          {showMenu && (
            <div className="mt-2 flex flex-col rounded-lg border border-zinc-800 bg-zinc-900 p-1 text-sm">
              <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={() => { setShowMenu(false); setShowRename(true) }}>Change username</button>
              <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={() => { setShowMenu(false); void signOut() }}>Sign out</button>
            </div>
          )}
          <div className="my-4 h-px bg-zinc-800/80" />
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">Friends</span>
            <span className="rounded border border-zinc-800 px-1.5 text-xs text-zinc-600">+ Add</span>
          </div>
          <p className="mt-3 rounded-lg border border-dashed border-zinc-800 p-3 text-center text-[11px] text-zinc-600">
            Multiplayer presence &amp; activity — coming soon.
          </p>
          {showRename && <UsernameDialog current={profile.username} onClose={() => setShowRename(false)} />}
        </>
      ) : (
        <div className="flex flex-col items-center gap-2 text-center">
          <div className="grid h-12 w-12 place-items-center rounded-full bg-zinc-800 text-2xl">🎮</div>
          <div className="text-sm font-semibold text-zinc-100">Sign in to GameNest</div>
          <p className="text-xs text-zinc-500">Add friends, level up, and see what your friends are playing.</p>
          <button onClick={() => setShowSignIn(true)} className="mt-2 w-full rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 hover:bg-emerald-400">Sign in</button>
        </div>
      )}
      {showSignIn && <SignInPanel onClose={() => setShowSignIn(false)} />}
    </aside>
  )
}
