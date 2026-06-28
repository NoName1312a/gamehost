import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const update = vi.fn(() => ({ eq: vi.fn(async () => ({ error: null })) }))
const refreshProfile = vi.fn(async () => {})
vi.mock('../../lib/supabase', () => ({ supabase: { from: () => ({ update }) } }))
vi.mock('../../lib/auth', () => ({ useAuth: () => ({ refreshProfile }) }))
vi.mock('../../lib/username', () => ({
  validateUsername: (n: string) => (n.length < 3 ? 'too short' : null),
  isUsernameAvailable: vi.fn(async () => true),
}))

import { UsernameDialog } from './UsernameDialog'

beforeEach(() => { update.mockClear(); refreshProfile.mockClear() })

describe('UsernameDialog', () => {
  it('saves a valid new username', async () => {
    render(<UsernameDialog current="Old" onClose={() => {}} />)
    fireEvent.change(screen.getByDisplayValue('Old'), { target: { value: 'NewName' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))
    await waitFor(() => expect(update).toHaveBeenCalledWith({ username: 'NewName' }))
    await waitFor(() => expect(refreshProfile).toHaveBeenCalled())
  })
})
