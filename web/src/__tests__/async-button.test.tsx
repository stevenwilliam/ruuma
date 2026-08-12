// AsyncButton is what stands between a double click and two writes. Every
// mutating admin action posts a fresh crypto.randomUUID() idempotency key, so
// two clicks are two *distinct* operations to the API rather than one retried
// — verifying a payment twice would write two immutable payment_events rows
// (D26). These tests pin that guard, and the confirmation step on the actions
// that cannot be walked back.
//
// fireEvent rather than user-event: user-event is not a dependency of this
// project and none of these assertions need its extra fidelity.

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AsyncButton } from '../components/ui'

// A promise resolved by hand, so the assertions run while the click is still
// in flight rather than racing a microtask.
function deferred() {
  let resolve!: () => void
  const promise = new Promise<void>((r) => {
    resolve = r
  })
  return { promise, resolve }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AsyncButton', () => {
  it('runs the action once however many times it is clicked in flight', async () => {
    const gate = deferred()
    const onRun = vi.fn(() => gate.promise)

    render(<AsyncButton onRun={onRun}>Verify</AsyncButton>)
    const button = screen.getByRole('button')

    fireEvent.click(button)
    expect(onRun).toHaveBeenCalledTimes(1)

    // Disabled while the request is outstanding, and announced as busy.
    await waitFor(() => expect(button).toBeDisabled())
    expect(button).toHaveAttribute('aria-busy', 'true')

    fireEvent.click(button)
    fireEvent.click(button)
    expect(onRun).toHaveBeenCalledTimes(1)

    gate.resolve()
    await waitFor(() => expect(button).toBeEnabled())
    expect(button).not.toHaveAttribute('aria-busy')
  })

  it('releases the button when the action rejects, without an unhandled rejection', async () => {
    const onRun = vi.fn().mockRejectedValue(new Error('nope'))
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})

    render(<AsyncButton onRun={onRun}>Verify</AsyncButton>)
    const button = screen.getByRole('button')

    fireEvent.click(button)

    // A failed write has to stay retryable — a stuck disabled button would
    // strand the operator with no way to try again.
    await waitFor(() => expect(button).toBeEnabled())
    expect(onRun).toHaveBeenCalledTimes(1)

    // Swallowed into the console rather than escaping as an unhandled
    // rejection: an async onClick that rejects is invisible to the operator
    // and drowns real errors.
    expect(logged).toHaveBeenCalled()
  })

  it('does not run the action when the confirmation is declined', async () => {
    const onRun = vi.fn().mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)

    render(
      <AsyncButton onRun={onRun} confirm="Close this date?">
        Close
      </AsyncButton>,
    )
    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => expect(confirmSpy).toHaveBeenCalledWith('Close this date?'))
    expect(onRun).not.toHaveBeenCalled()
  })

  it('runs the action when the confirmation is accepted', async () => {
    const onRun = vi.fn().mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    render(
      <AsyncButton onRun={onRun} confirm="Close this date?">
        Close
      </AsyncButton>,
    )
    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => expect(onRun).toHaveBeenCalledTimes(1))
  })

  it('shows the busy label while running and restores the original after', async () => {
    const gate = deferred()

    render(
      <AsyncButton onRun={() => gate.promise} busyLabel="Verifying…">
        Verify
      </AsyncButton>,
    )
    const button = screen.getByRole('button')
    expect(button).toHaveTextContent('Verify')

    fireEvent.click(button)
    await waitFor(() => expect(button).toHaveTextContent('Verifying…'))

    gate.resolve()
    await waitFor(() => expect(button).toHaveTextContent('Verify'))
  })

  it('stays disabled when the caller disables it, without confirming or running', () => {
    const onRun = vi.fn().mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    render(
      <AsyncButton onRun={onRun} confirm="Sure?" disabled>
        Close
      </AsyncButton>,
    )
    const button = screen.getByRole('button')
    expect(button).toBeDisabled()

    fireEvent.click(button)
    expect(confirmSpy).not.toHaveBeenCalled()
    expect(onRun).not.toHaveBeenCalled()
  })
})
