import { useCallback, useEffect, useState } from 'react'
import { useAuth } from '../../lib/auth'
import { PresenceProvider } from '../../lib/presence'
import { ProfileBlock } from './ProfileBlock'
import { SignInPanel } from './SignInPanel'
import { UsernameDialog } from './UsernameDialog'
import { FriendsList } from './FriendsList'
import { AddFriendDialog } from './AddFriendDialog'
import { RequestsInbox } from './RequestsInbox'
import { listRequests } from '../../lib/friends'
import { supabase } from '../../lib/supabase'

export function SocialSidebar() {
  const { session, profile, loading, signOut, refreshProfile } = useAuth()
  const [showSignIn, setShowSignIn] = useState(false)
  const [showMenu, setShowMenu] = useState(false)
  const [showRename, setShowRename] = useState(false)
  const [showAdd, setShowAdd] = useState(false)
  const [showRequests, setShowRequests] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)
  const [requestCount, setRequestCount] = useState(0)
  const bump = useCallback(() => setRefreshKey((k) => k + 1), [])
  const refreshRequestCount = useCallback(async () => {
    try { setRequestCount((await listRequests()).filter((r) => r.direction === 'in').length) } catch { /* keep */ }
  }, [])
  // poll friends + request count every 15s while signed in
  useEffect(() => {
    if (!session) return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void refreshRequestCount()
    const t = setInterval(() => { bump(); void refreshRequestCount() }, 15_000)
    return () => clearInterval(t)
  }, [session, bump, refreshRequestCount])

  return (
    <aside className="hidden w-72 shrink-0 flex-col border-l border-zinc-800/80 bg-zinc-950/60 p-4 backdrop-blur lg:flex">
      {loading ? (
        <p className="text-xs text-zinc-600">…</p>
      ) : session && profile ? (
        <PresenceProvider>
          <>
            <ProfileBlock profile={profile} onMenu={() => setShowMenu((v) => !v)} />
            {showMenu && (
              <div className="mt-2 flex flex-col rounded-lg border border-zinc-800 bg-zinc-900 p-1 text-sm">
                <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={() => { setShowMenu(false); setShowRename(true) }}>Change username</button>
                <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={async () => {
                  setShowMenu(false)
                  if (!profile) return
                  await supabase.from('profiles').update({ show_activity: !profile.show_activity }).eq('id', profile.id)
                  await refreshProfile()
                }}>{profile?.show_activity === false ? 'Show my activity to friends' : 'Hide my activity from friends'}</button>
                <button className="rounded px-2 py-1 text-left text-zinc-300 hover:bg-zinc-800" onClick={() => { setShowMenu(false); void signOut() }}>Sign out</button>
              </div>
            )}
            <div className="my-4 h-px bg-zinc-800/80" />
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">Friends</span>
              <div className="flex items-center gap-1">
                <button onClick={() => setShowRequests((v) => !v)} className="relative rounded border border-zinc-800 px-1.5 text-xs text-zinc-400 hover:text-zinc-200">
                  Requests{requestCount > 0 ? <span className="ml-1 rounded-full bg-emerald-500 px-1 text-[10px] font-semibold text-zinc-950">{requestCount}</span> : null}
                </button>
                <button onClick={() => setShowAdd(true)} className="rounded border border-zinc-800 px-1.5 text-xs text-zinc-400 hover:text-zinc-200">+ Add</button>
              </div>
            </div>
            {showRequests && <div className="mt-2"><RequestsInbox onChanged={() => { bump(); void refreshRequestCount() }} /></div>}
            <div className="mt-3 min-h-0 flex-1 overflow-y-auto"><FriendsList refreshKey={refreshKey} onChanged={bump} /></div>
            {showAdd && <AddFriendDialog onClose={() => setShowAdd(false)} onSent={() => { bump(); void refreshRequestCount() }} />}
            {showRename && <UsernameDialog current={profile.username} onClose={() => setShowRename(false)} />}
          </>
        </PresenceProvider>
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
