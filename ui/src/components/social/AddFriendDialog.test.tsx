import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const { sendRequest } = vi.hoisted(() => ({ sendRequest: vi.fn() }))
vi.mock('../../lib/friends', () => ({ sendRequest }))
import { AddFriendDialog } from './AddFriendDialog'

describe('AddFriendDialog', () => {
  it('sends a request then calls onSent', async () => {
    sendRequest.mockResolvedValue(undefined)
    const onSent = vi.fn()
    render(<AddFriendDialog onClose={() => {}} onSent={onSent} />)
    fireEvent.change(screen.getByPlaceholderText(/username/i), { target: { value: 'Tom' } })
    fireEvent.click(screen.getByRole('button', { name: /send request/i }))
    await waitFor(() => expect(sendRequest).toHaveBeenCalledWith('Tom'))
    await waitFor(() => expect(onSent).toHaveBeenCalled())
  })

  it('shows an error and does not call onSent on failure', async () => {
    sendRequest.mockRejectedValue(new Error('No user with that username.'))
    const onSent = vi.fn()
    render(<AddFriendDialog onClose={() => {}} onSent={onSent} />)
    fireEvent.change(screen.getByPlaceholderText(/username/i), { target: { value: 'ghost' } })
    fireEvent.click(screen.getByRole('button', { name: /send request/i }))
    await waitFor(() => expect(screen.getByText(/no user/i)).toBeInTheDocument())
    expect(onSent).not.toHaveBeenCalled()
  })
})
