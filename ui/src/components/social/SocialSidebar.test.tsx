import React from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

let mockAuth: { session: unknown; profile: unknown; loading: boolean; signOut: () => void; refreshProfile: () => Promise<void> }
vi.mock('../../lib/auth', () => ({ useAuth: () => mockAuth }))
vi.mock('./SignInPanel', () => ({ SignInPanel: () => <div>signin-panel</div> }))
vi.mock('./FriendsList', () => ({ FriendsList: () => <div>friends-list</div> }))
vi.mock('./RequestsInbox', () => ({ RequestsInbox: () => <div>requests</div> }))
vi.mock('./AddFriendDialog', () => ({ AddFriendDialog: () => <div>add-friend</div> }))
vi.mock('../../lib/presence', () => ({ PresenceProvider: ({ children }: { children: React.ReactNode }) => <>{children}</> }))
vi.mock('../../lib/friends', () => ({ listRequests: vi.fn(async () => []) }))

import { SocialSidebar } from './SocialSidebar'

describe('SocialSidebar', () => {
  it('shows the sign-in prompt when signed out', () => {
    mockAuth = { session: null, profile: null, loading: false, signOut: vi.fn(), refreshProfile: vi.fn(async () => {}) }
    render(<SocialSidebar />)
    expect(screen.getByText(/sign in to gamenest/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^sign in$/i }))
    expect(screen.getByText('signin-panel')).toBeInTheDocument()
  })
  it('shows the profile when signed in', () => {
    mockAuth = { session: { user: { id: 'u1' } }, profile: { id: 'u1', username: 'Tom', display_name: null, avatar_url: null, level: 2, xp: 50, show_activity: true }, loading: false, signOut: vi.fn(), refreshProfile: vi.fn(async () => {}) }
    render(<SocialSidebar />)
    expect(screen.getByText('Tom')).toBeInTheDocument()
    expect(screen.getByText('friends-list')).toBeInTheDocument()
  })
})
