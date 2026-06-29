import { describe, it, expect, vi, beforeEach } from 'vitest'

const { sb, awardXp } = vi.hoisted(() => ({ sb: { auth: { getSession: vi.fn() }, from: vi.fn() }, awardXp: vi.fn() }))
vi.mock('./supabase', () => ({ supabase: sb }))
vi.mock('./xp', () => ({ awardXp }))
import { sendRequest, acceptRequest } from './friends'

beforeEach(() => {
  sb.auth.getSession.mockResolvedValue({ data: { session: { user: { id: 'me' } } } })
})

describe('sendRequest', () => {
  it('rejects sending to yourself', async () => {
    sb.from.mockImplementation((t: string) => {
      if (t === 'profiles') return { select: () => ({ ilike: () => ({ maybeSingle: async () => ({ data: { id: 'me' } }) }) }) }
      return {}
    })
    await expect(sendRequest('Me')).rejects.toThrow(/you/i)
  })

  it('rejects an unknown username', async () => {
    sb.from.mockImplementation((t: string) => {
      if (t === 'profiles') return { select: () => ({ ilike: () => ({ maybeSingle: async () => ({ data: null }) }) }) }
      return {}
    })
    await expect(sendRequest('ghost')).rejects.toThrow(/no user/i)
  })

  it('inserts a pending request for a valid new friend', async () => {
    const insert = vi.fn(async () => ({ error: null }))
    sb.from.mockImplementation((t: string) => {
      if (t === 'profiles') return { select: () => ({ ilike: () => ({ maybeSingle: async () => ({ data: { id: 'them' } }) }) }) }
      if (t === 'friendships') return {
        select: () => ({ or: () => ({ maybeSingle: async () => ({ data: null }) }) }),
        insert,
      }
      return {}
    })
    await sendRequest('Them')
    expect(insert).toHaveBeenCalledWith({ requester: 'me', addressee: 'them', status: 'pending' })
  })
})

describe('acceptRequest', () => {
  it('awards XP after accepting a request', async () => {
    const eq = vi.fn(async () => ({ error: null }))
    sb.from.mockImplementation((t: string) => (t === 'friendships' ? { update: () => ({ eq }) } : {}))
    await acceptRequest('r1')
    expect(awardXp).toHaveBeenCalledWith(50)
  })
})
