import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProfileBlock } from './ProfileBlock'

const profile = { id: 'u1', username: 'Tom', display_name: null, avatar_url: null, level: 2, xp: 250, show_activity: true }

describe('ProfileBlock', () => {
  it('shows username and the curve-derived level', () => {
    render(<ProfileBlock profile={profile} onMenu={() => {}} />)
    expect(screen.getByText('Tom')).toBeInTheDocument()
    expect(screen.getByText(/level 2/i)).toBeInTheDocument()   // xp 250 -> level 2
    expect(screen.getByText(/150 \/ 300 XP/)).toBeInTheDocument()
  })
  it('falls back to an initial when there is no avatar', () => {
    render(<ProfileBlock profile={profile} onMenu={() => {}} />)
    expect(screen.getByText('T')).toBeInTheDocument()
  })
})
