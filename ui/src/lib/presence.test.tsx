import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import type { ServerSummary } from './api'

const { sb, mockAuth, apiMock, awardXp } = vi.hoisted(() => ({
  sb: { from: vi.fn() },
  mockAuth: { current: { session: null as unknown, profile: null as unknown } },
  apiMock: { servers: vi.fn(async (): Promise<ServerSummary[]> => []) },
  awardXp: vi.fn(),
}))
vi.mock('./supabase', () => ({ supabase: sb }))
vi.mock('./auth', () => ({ useAuth: () => mockAuth.current }))
vi.mock('./api', () => ({ api: apiMock }))
vi.mock('./xp', () => ({ awardXp }))
import { PresenceProvider } from './presence'

beforeEach(() => { sb.from.mockReset() })

describe('PresenceProvider', () => {
  it('upserts presence when signed in with activity enabled', async () => {
    const upsert = vi.fn(async () => ({ error: null }))
    sb.from.mockReturnValue({ upsert, delete: () => ({ eq: vi.fn(async () => ({})) }) })
    apiMock.servers.mockResolvedValue([{ running: true, name: 'Craftoria' }] as unknown as ServerSummary[])
    mockAuth.current = { session: { user: { id: 'me' } }, profile: { show_activity: true } }
    render(<PresenceProvider><div /></PresenceProvider>)
    await vi.waitFor(() => expect(upsert).toHaveBeenCalled())
    expect((upsert.mock.calls as unknown[][])[0][0]).toMatchObject({ user_id: 'me', activity: 'Hosting Craftoria' })
  })

  it('clears presence (no upsert) when activity disabled', async () => {
    const upsert = vi.fn(async () => ({ error: null }))
    const eq = vi.fn(async () => ({}))
    sb.from.mockReturnValue({ upsert, delete: () => ({ eq }) })
    mockAuth.current = { session: { user: { id: 'me' } }, profile: { show_activity: false } }
    render(<PresenceProvider><div /></PresenceProvider>)
    await vi.waitFor(() => expect(eq).toHaveBeenCalledWith('user_id', 'me'))
    expect(upsert).not.toHaveBeenCalled()
  })

  it('awards hosting XP on the no->running transition', async () => {
    const upsert = vi.fn(async () => ({ error: null }))
    sb.from.mockReturnValue({ upsert, delete: () => ({ eq: vi.fn(async () => ({})) }) })
    apiMock.servers.mockResolvedValue([{ running: true, name: 'Craftoria' }] as unknown as ServerSummary[])
    mockAuth.current = { session: { user: { id: 'me' } }, profile: { show_activity: true } }
    render(<PresenceProvider><div /></PresenceProvider>)
    await vi.waitFor(() => expect(awardXp).toHaveBeenCalledWith(25))
  })
})
