import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import type { Session } from '@supabase/supabase-js'
import { supabase, fetchProfile, type Profile } from './supabase'

interface AuthState {
  session: Session | null
  profile: Profile | null
  loading: boolean
  signInEmail: (email: string, password: string) => Promise<void>
  signUpEmail: (email: string, password: string, username: string) => Promise<void>
  signOut: () => Promise<void>
  refreshProfile: () => Promise<void>
}

const Ctx = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null)
  const [profile, setProfile] = useState<Profile | null>(null)
  const [loading, setLoading] = useState(true)

  const loadProfile = useCallback(async (s: Session | null) => {
    setProfile(s ? await fetchProfile(s.user.id) : null)
  }, [])

  useEffect(() => {
    let alive = true
    supabase.auth.getSession().then(async ({ data }) => {
      if (!alive) return
      setSession(data.session)
      await loadProfile(data.session)
      setLoading(false)
    })
    const { data: sub } = supabase.auth.onAuthStateChange((_e, s) => {
      setSession(s)
      void loadProfile(s)
    })
    return () => { alive = false; sub.subscription.unsubscribe() }
  }, [loadProfile])

  const signInEmail = useCallback(async (email: string, password: string) => {
    const { error } = await supabase.auth.signInWithPassword({ email, password })
    if (error) throw new Error(error.message)
  }, [])

  const signUpEmail = useCallback(async (email: string, password: string, username: string) => {
    const { error } = await supabase.auth.signUp({ email, password, options: { data: { username } } })
    if (error) throw new Error(error.message)
  }, [])

  const signOut = useCallback(async () => { await supabase.auth.signOut() }, [])
  const refreshProfile = useCallback(() => loadProfile(session), [loadProfile, session])

  return (
    <Ctx.Provider value={{ session, profile, loading, signInEmail, signUpEmail, signOut, refreshProfile }}>
      {children}
    </Ctx.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthState {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth must be used within AuthProvider')
  return v
}
