import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@supabase/supabase-js', () => ({
  createClient: vi.fn(() => ({ from: vi.fn() })),
}))

beforeEach(() => {
  vi.stubEnv('VITE_SUPABASE_URL', 'https://example.supabase.co')
  vi.stubEnv('VITE_SUPABASE_ANON_KEY', 'anon-key')
})

describe('supabase client', () => {
  it('creates a client with the env URL + key and PKCE auth', async () => {
    const { createClient } = await import('@supabase/supabase-js')
    await import('./supabase')
    expect(createClient).toHaveBeenCalledWith(
      'https://example.supabase.co',
      'anon-key',
      expect.objectContaining({ auth: expect.objectContaining({ flowType: 'pkce' }) }),
    )
  })
})
