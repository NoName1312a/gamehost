import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const signInEmail = vi.fn(async () => {})
const signUpEmail = vi.fn(async () => {})
vi.mock('../../lib/auth', () => ({ useAuth: () => ({ signInEmail, signUpEmail }) }))
vi.mock('../../lib/discord-oauth', () => ({ signInWithDiscord: vi.fn(async () => {}) }))
vi.mock('../../lib/username', () => ({
  validateUsername: (n: string) => (n.length < 3 ? 'too short' : null),
  isUsernameAvailable: vi.fn(async () => true),
}))

import { SignInPanel } from './SignInPanel'

describe('SignInPanel', () => {
  it('signs in with email', async () => {
    render(<SignInPanel onClose={() => {}} />)
    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'a@b.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret12' } })
    fireEvent.click(screen.getByRole('button', { name: /sign in with email/i }))
    await waitFor(() => expect(signInEmail).toHaveBeenCalledWith('a@b.com', 'secret12'))
  })

  it('blocks sign-up with an invalid username', async () => {
    render(<SignInPanel onClose={() => {}} />)
    fireEvent.click(screen.getByText(/create one/i))
    fireEvent.change(screen.getByPlaceholderText('Username'), { target: { value: 'ab' } })
    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'a@b.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret12' } })
    fireEvent.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText('too short')).toBeInTheDocument())
    expect(signUpEmail).not.toHaveBeenCalled()
  })
})
