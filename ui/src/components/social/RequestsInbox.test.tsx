import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const { listRequests, acceptRequest, removeFriendship } = vi.hoisted(() => ({
  listRequests: vi.fn(), acceptRequest: vi.fn(), removeFriendship: vi.fn(),
}))
vi.mock('../../lib/friends', () => ({ listRequests, acceptRequest, removeFriendship }))
import { RequestsInbox } from './RequestsInbox'

describe('RequestsInbox', () => {
  it('accepts an incoming request', async () => {
    listRequests.mockResolvedValue([{ id: 'r1', userId: 'u1', username: 'Tom', avatarUrl: null, direction: 'in' }])
    acceptRequest.mockResolvedValue(undefined)
    render(<RequestsInbox onChanged={() => {}} />)
    await waitFor(() => expect(screen.getByText('Tom')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /accept/i }))
    await waitFor(() => expect(acceptRequest).toHaveBeenCalledWith('r1'))
  })
})
