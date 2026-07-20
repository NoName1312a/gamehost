import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

// openMock is referenced inside a hoisted vi.mock factory, so it must be created
// with vi.hoisted (a plain const would be in the TDZ when the mock runs).
const { openMock } = vi.hoisted(() => ({ openMock: vi.fn(async () => {}) }))
const signInEmail = vi.fn(async () => {})
vi.mock('../../lib/auth', () => ({ useAuth: () => ({ signInEmail }) }))
vi.mock('../../lib/discord-oauth', () => ({ signInWithDiscord: vi.fn(async () => {}) }))
vi.mock('@tauri-apps/plugin-shell', () => ({ open: openMock }))

import { SignInPanel } from './SignInPanel'
import { WEB_BASE } from '../../lib/site'

describe('SignInPanel', () => {
  it('signs in with email', async () => {
    render(<SignInPanel onClose={() => {}} />)
    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'a@b.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret12' } })
    fireEvent.click(screen.getByRole('button', { name: /sign in with email/i }))
    await waitFor(() => expect(signInEmail).toHaveBeenCalledWith('a@b.com', 'secret12'))
  })

  it('opens the website to create an account', () => {
    render(<SignInPanel onClose={() => {}} />)
    fireEvent.click(screen.getByText(/create one on the web/i))
    expect(openMock).toHaveBeenCalledWith(`${WEB_BASE}/signup`)
  })

  it('opens the website to reset a password', () => {
    render(<SignInPanel onClose={() => {}} />)
    fireEvent.click(screen.getByText(/forgot password/i))
    expect(openMock).toHaveBeenCalledWith(`${WEB_BASE}/reset`)
  })
})
