import type { Profile } from '../../lib/supabase'

export function ProfileBlock({ profile, onMenu }: { profile: Profile; onMenu: () => void }) {
  const initial = profile.username.charAt(0).toUpperCase()
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3">
        {profile.avatar_url ? (
          <img src={profile.avatar_url} alt="" className="h-10 w-10 rounded-full" />
        ) : (
          <div className="grid h-10 w-10 place-items-center rounded-full bg-gradient-to-br from-emerald-500 to-sky-500 text-sm font-semibold text-zinc-950">
            {initial}
          </div>
        )}
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-semibold text-zinc-100">{profile.username}</div>
          <div className="text-xs text-zinc-500">Level {profile.level}</div>
        </div>
        <button onClick={onMenu} aria-label="Account menu" className="text-zinc-500 hover:text-zinc-300">⚙</button>
      </div>
      <div>
        <div className="h-1.5 overflow-hidden rounded-full bg-zinc-800">
          <div className="h-full bg-gradient-to-r from-emerald-500 to-sky-500" style={{ width: `${xpPercent(profile)}%` }} />
        </div>
        <div className="mt-1 text-right text-[10px] text-zinc-600">{profile.xp} XP</div>
      </div>
    </div>
  )
}

// Static early-state until sub-project C defines leveling. Keeps a sensible bar.
function xpPercent(p: Profile): number {
  if (p.xp <= 0) return 4
  return Math.min(100, p.xp % 100)
}
