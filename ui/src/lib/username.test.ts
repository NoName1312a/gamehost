import { describe, it, expect, vi } from 'vitest'
import { validateUsername } from './username'

vi.mock('./supabase')

describe('validateUsername', () => {
  it('accepts a valid name', () => {
    expect(validateUsername('Tom_99')).toBeNull()
  })
  it('rejects too short / too long / bad chars', () => {
    expect(validateUsername('ab')).toMatch(/3/)
    expect(validateUsername('x'.repeat(21))).toMatch(/3/)
    expect(validateUsername('has space')).toMatch(/letters/i)
  })
  it('rejects reserved names case-insensitively', () => {
    expect(validateUsername('Admin')).toMatch(/reserved/i)
    expect(validateUsername('gamenest')).toMatch(/reserved/i)
  })
})
