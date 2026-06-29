import { describe, it, expect, vi } from 'vitest'

const { sb } = vi.hoisted(() => ({ sb: { auth: { getSession: vi.fn() }, from: vi.fn() } }))
vi.mock('./supabase', () => ({ supabase: sb }))
import { levelForXp, xpProgress, awardXp } from './xp'

describe('levelForXp', () => {
  it('maps xp to levels on the sqrt curve', () => {
    expect(levelForXp(0)).toBe(1)
    expect(levelForXp(99)).toBe(1)
    expect(levelForXp(100)).toBe(2)
    expect(levelForXp(399)).toBe(2)
    expect(levelForXp(400)).toBe(3)
    expect(levelForXp(900)).toBe(4)
    expect(levelForXp(-50)).toBe(1)
  })
})

describe('xpProgress', () => {
  it('computes level + into/span/percent inside a level', () => {
    const p = xpProgress(250) // level 2 spans [100,400): into 150 of 300 = 50%
    expect(p).toEqual({ level: 2, into: 150, span: 300, percent: 50 })
  })
  it('handles 0 xp', () => {
    expect(xpProgress(0)).toEqual({ level: 1, into: 0, span: 100, percent: 0 })
  })
})

describe('awardXp', () => {
  it('adds delta to current xp and writes the recomputed level', async () => {
    const eq = vi.fn(async () => ({ error: null }))
    const update = vi.fn(() => ({ eq }))
    sb.auth.getSession.mockResolvedValue({ data: { session: { user: { id: 'me' } } } })
    sb.from.mockReturnValue({
      select: () => ({ eq: () => ({ maybeSingle: async () => ({ data: { xp: 80 } }) }) }),
      update,
    })
    await awardXp(50) // 80 + 50 = 130 -> level 2
    expect(update).toHaveBeenCalledWith({ xp: 130, level: 2 })
  })

  it('no-ops when signed out', async () => {
    const update = vi.fn(() => ({ eq: vi.fn() }))
    sb.auth.getSession.mockResolvedValue({ data: { session: null } })
    sb.from.mockReturnValue({ update })
    await awardXp(50)
    expect(update).not.toHaveBeenCalled()
  })
})
