// The picker replaced a one-button toggle whose label was the *current*
// language — ambiguous, because a button label normally says what pressing it
// does. These tests pin the properties that make the replacement unambiguous:
// both options are always present, the active one is marked, and the flag is
// never the only thing carrying the meaning.

import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { LanguagePicker } from '../components/LanguagePicker'

beforeEach(() => {
  localStorage.clear()
  // jsdom has no navigation; the component reloads to re-read the catalogues.
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, reload: vi.fn() },
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('LanguagePicker', () => {
  it('shows both languages, not just the current one', () => {
    render(<LanguagePicker />)

    expect(screen.getByRole('button', { name: /ID/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /EN/ })).toBeInTheDocument()
  })

  it('marks the active language with aria-pressed', () => {
    render(<LanguagePicker />)

    // ID is the default (D9).
    expect(screen.getByRole('button', { name: /ID/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /EN/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('stores the choice and reloads when a different language is picked', () => {
    render(<LanguagePicker />)

    fireEvent.click(screen.getByRole('button', { name: /EN/ }))

    expect(localStorage.getItem('ruuma.lang')).toBe('en')
    expect(window.location.reload).toHaveBeenCalled()
  })

  it('does nothing when the active language is clicked again', () => {
    render(<LanguagePicker />)

    fireEvent.click(screen.getByRole('button', { name: /ID/ }))

    // No pointless reload, and nothing written for a no-op.
    expect(window.location.reload).not.toHaveBeenCalled()
  })

  it('reflects a stored language on mount', () => {
    localStorage.setItem('ruuma.lang', 'en')
    render(<LanguagePicker />)

    expect(screen.getByRole('button', { name: /EN/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /ID/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('never depends on the flag alone — each option carries a text label', () => {
    const { container } = render(<LanguagePicker />)

    // The flags are decorative; the accessible name comes from the text.
    for (const svg of container.querySelectorAll('svg')) {
      expect(svg).toHaveAttribute('aria-hidden', 'true')
    }
    for (const name of ['ID', 'EN']) {
      expect(screen.getByRole('button', { name: new RegExp(name) })).toHaveTextContent(name)
    }
  })

  it('groups the pair under one accessible name', () => {
    render(<LanguagePicker />)

    expect(screen.getByRole('group', { name: 'Ganti bahasa' })).toBeInTheDocument()
  })

  it('keeps a 44px touch target on each option', () => {
    render(<LanguagePicker />)

    for (const button of screen.getAllByRole('button')) {
      expect(button.className).toContain('min-h-[44px]')
    }
  })
})
