import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

const { onAuthStateChange, getSession } = vi.hoisted(() => {
  const onAuthStateChange = vi.fn(() => ({ data: { subscription: { unsubscribe: vi.fn() } } }))
  const getSession = vi.fn(async () => ({ data: { session: null } }))
  return { onAuthStateChange, getSession }
})

vi.mock('./supabase', () => ({
  supabase: { auth: { getSession, onAuthStateChange, signOut: vi.fn() } },
  fetchProfile: vi.fn(async () => ({ id: 'u1', username: 'Tom', display_name: null, avatar_url: null, level: 1, xp: 0 })),
}))

import { AuthProvider, useAuth } from './auth'

function Probe() {
  const { loading, session } = useAuth()
  return <div>{loading ? 'loading' : session ? 'in' : 'out'}</div>
}

beforeEach(() => { getSession.mockClear(); onAuthStateChange.mockClear() })

describe('AuthProvider', () => {
  it('resolves to signed-out when there is no session', async () => {
    render(<AuthProvider><Probe /></AuthProvider>)
    await waitFor(() => expect(screen.getByText('out')).toBeInTheDocument())
    expect(onAuthStateChange).toHaveBeenCalled()
  })
})
