import { useEffect, type ReactNode } from 'react'
import { supabase } from './supabase'
import { useAuth } from './auth'
import { api } from './api'

const HEARTBEAT_MS = 30_000

async function currentActivity(): Promise<string | null> {
  try {
    const servers = await api.servers()
    const running = servers.find((s) => s.running)
    return running ? `Hosting ${running.name}` : null
  } catch {
    return null
  }
}

export function PresenceProvider({ children }: { children: ReactNode }) {
  const { session, profile } = useAuth()
  const userId = session?.user.id
  const enabled = !!userId && profile?.show_activity !== false

  useEffect(() => {
    if (!userId) return
    if (!enabled) {
      void supabase.from('presence').delete().eq('user_id', userId)
      return
    }
    let alive = true
    const beat = async () => {
      const activity = await currentActivity()
      if (alive) await supabase.from('presence').upsert({ user_id: userId, activity, updated_at: new Date().toISOString() })
    }
    void beat()
    const t = setInterval(() => void beat(), HEARTBEAT_MS)
    return () => {
      alive = false
      clearInterval(t)
      void supabase.from('presence').delete().eq('user_id', userId)
    }
  }, [userId, enabled])

  return <>{children}</>
}
