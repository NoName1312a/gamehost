import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProfileBlock } from './ProfileBlock'

const profile = { id: 'u1', username: 'Tom', display_name: null, avatar_url: null, level: 3, xp: 1240 }

describe('ProfileBlock', () => {
  it('shows username and level', () => {
    render(<ProfileBlock profile={profile} onMenu={() => {}} />)
    expect(screen.getByText('Tom')).toBeInTheDocument()
    expect(screen.getByText(/level 3/i)).toBeInTheDocument()
  })
  it('falls back to an initial when there is no avatar', () => {
    render(<ProfileBlock profile={profile} onMenu={() => {}} />)
    expect(screen.getByText('T')).toBeInTheDocument()
  })
})
