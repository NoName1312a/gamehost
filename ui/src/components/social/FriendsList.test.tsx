import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

const { listFriends, removeFriendship } = vi.hoisted(() => ({ listFriends: vi.fn(), removeFriendship: vi.fn() }))
vi.mock('../../lib/friends', () => ({ listFriends, removeFriendship }))
import { FriendsList } from './FriendsList'

describe('FriendsList', () => {
  it('renders a friend with online activity', async () => {
    listFriends.mockResolvedValue([{ id: 'f1', userId: 'u1', username: 'Tom', avatarUrl: null, online: true, activity: 'Hosting Craftoria' }])
    render(<FriendsList refreshKey={0} onChanged={() => {}} />)
    await waitFor(() => expect(screen.getByText('Tom')).toBeInTheDocument())
    expect(screen.getByText(/hosting craftoria/i)).toBeInTheDocument()
  })

  it('shows an empty state when there are no friends', async () => {
    listFriends.mockResolvedValue([])
    render(<FriendsList refreshKey={0} onChanged={() => {}} />)
    await waitFor(() => expect(screen.getByText(/no friends yet/i)).toBeInTheDocument())
  })
})
