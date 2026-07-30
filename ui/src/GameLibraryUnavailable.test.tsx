import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { GameLibraryUnavailable } from './App'

// Regression: "New server" was rendered only when the template list had loaded.
// If the load failed the button did nothing at all — no dialog, no message —
// and the user was left clicking a dead control.
describe('GameLibraryUnavailable', () => {
  it('explains what happened instead of leaving the click silent', () => {
    render(<GameLibraryUnavailable loading={false} onRetry={() => {}} onClose={() => {}} />)

    expect(screen.getByText(/game library didn't load/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('reassures that existing servers are unaffected', () => {
    render(<GameLibraryUnavailable loading={false} onRetry={() => {}} onClose={() => {}} />)

    expect(screen.getByText(/keep running/i)).toBeInTheDocument()
  })

  it('shows the diagnosis the engine reported, when there is one', () => {
    render(
      <GameLibraryUnavailable
        loading={false}
        detail="read templates dir: no such file or directory"
        dir="C:/GameNest/resources/templates"
        onRetry={() => {}}
        onClose={() => {}}
      />,
    )

    expect(screen.getByText(/no such file or directory/i)).toBeInTheDocument()
    expect(screen.getByText(/C:\/GameNest\/resources\/templates/)).toBeInTheDocument()
  })

  it('does not accuse the install while the list is still loading', () => {
    render(<GameLibraryUnavailable loading onRetry={() => {}} onClose={() => {}} />)

    expect(screen.getByText(/loading the game library/i)).toBeInTheDocument()
    expect(screen.queryByText(/didn't load/i)).not.toBeInTheDocument()
  })

  it('retries and closes on demand', () => {
    const onRetry = vi.fn()
    const onClose = vi.fn()
    render(<GameLibraryUnavailable loading={false} onRetry={onRetry} onClose={onClose} />)

    fireEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(onRetry).toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: /close/i }))
    expect(onClose).toHaveBeenCalled()
  })
})
