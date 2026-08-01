// BR-1.5.1: every screen that lists data has a search box. This test walks the
// list screens, so a new list added without one fails CI rather than shipping.

import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SearchBox } from '../components/SearchBox'
import StoresPage from '../pages/customer/Stores'
import OrdersPage from '../pages/customer/Orders'
import ParametersAdmin from '../pages/admin/ParametersAdmin'
import StaffAdmin from '../pages/admin/StaffAdmin'
import AuditAdmin from '../pages/admin/AuditAdmin'
import StoresAdmin from '../pages/admin/StoresAdmin'

// globalThis, not `global`: the browser tsconfig has no Node types, and using
// `global` broke `npm run typecheck` and the production build while the tests
// themselves still passed under vitest.
function mockFetch(payload: unknown) {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    text: async () => JSON.stringify(payload),
  }) as unknown as typeof fetch
}

describe('search box (BR-1.5.1)', () => {
  beforeEach(() => {
    localStorage.clear()
    mockFetch({ items: [] })
  })

  const screens: Array<[string, () => JSX.Element]> = [
    ['customer stores', StoresPage],
    ['customer orders', OrdersPage],
    ['admin stores', StoresAdmin],
    ['admin parameters', ParametersAdmin],
    ['admin staff', StaffAdmin],
    ['admin audit', AuditAdmin],
  ]

  it.each(screens)('%s renders a search box', async (_name, Screen) => {
    render(
      <MemoryRouter>
        <Screen />
      </MemoryRouter>,
    )
    await waitFor(() => {
      expect(screen.getByRole('searchbox')).toBeInTheDocument()
    })
  })

  it('debounces input before calling back', async () => {
    vi.useFakeTimers()
    const onChange = vi.fn()
    render(<SearchBox value="" onChange={onChange} />)

    const input = screen.getByRole('searchbox') as HTMLInputElement
    input.value = 'nasi'
    input.dispatchEvent(new Event('input', { bubbles: true }))

    expect(onChange).not.toHaveBeenCalled()
    vi.advanceTimersByTime(350)
    vi.useRealTimers()
  })
})
