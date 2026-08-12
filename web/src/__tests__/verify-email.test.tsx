// Email verification could not have worked for anyone: authsvc mails a link to
// {baseURL}/verify-email, the API route is /api/v1/auth/verify-email, and the
// SPA declared neither — so the link rendered the chrome around an empty page.
// These tests exist so the route cannot quietly disappear again.

import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import VerifyEmailPage from '../pages/customer/VerifyEmail'
import NotFoundPage from '../pages/customer/NotFound'

function mockFetch(ok: boolean, body: unknown = {}) {
  const spy = vi.fn().mockResolvedValue({
    ok,
    status: ok ? 200 : 422,
    text: async () => JSON.stringify(body),
  })
  globalThis.fetch = spy as unknown as typeof fetch
  return spy
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/verify-email" element={<VerifyEmailPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  document.head.querySelectorAll('meta[name="robots"]').forEach((t) => t.remove())
})

describe('VerifyEmailPage', () => {
  it('calls the API with the token from the query string', async () => {
    const spy = mockFetch(true, { status: 'verified' })
    renderAt('/verify-email?token=ABC123')

    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(String(spy.mock.calls[0][0])).toContain('/auth/verify-email?token=ABC123')
  })

  it('confirms success in words, not JSON', async () => {
    mockFetch(true, { status: 'verified' })
    renderAt('/verify-email?token=ABC123')

    expect(await screen.findByText(/sudah terverifikasi|is verified/i)).toBeInTheDocument()
  })

  it('calls the API exactly once, because the token is single-use', async () => {
    const spy = mockFetch(true, { status: 'verified' })
    const { rerender } = renderAt('/verify-email?token=ABC123')

    rerender(
      <MemoryRouter initialEntries={['/verify-email?token=ABC123']}>
        <Routes>
          <Route path="/verify-email" element={<VerifyEmailPage />} />
        </Routes>
      </MemoryRouter>,
    )

    // A second call always fails server-side, so a double-invoked effect would
    // paint an error over a verification that actually succeeded.
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
  })

  it('explains a rejected link instead of showing a blank page', async () => {
    mockFetch(false, { error: { code: 'VALIDATION_FAILED', message: 'Token expired' } })
    renderAt('/verify-email?token=STALE')

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByText(/tidak berlaku|not valid/i)).toBeInTheDocument()
  })

  it('handles a link that arrived without a token', async () => {
    const spy = mockFetch(true)
    renderAt('/verify-email')

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(spy).not.toHaveBeenCalled()
  })

  it('keeps the one-shot token out of search indexes', async () => {
    mockFetch(true, { status: 'verified' })
    renderAt('/verify-email?token=ABC123')

    await waitFor(() =>
      expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe(
        'noindex, nofollow',
      ),
    )
  })
})

describe('NotFoundPage', () => {
  it('renders for an unknown path rather than an empty layout', async () => {
    renderAt('/no-such-page')

    expect(await screen.findByText('404')).toBeInTheDocument()
    expect(screen.getByRole('heading')).toBeInTheDocument()
  })
})
