// The floating contact button reads its number from sys_parameters through
// GET /public-config (BR-1.4.5). The cases that matter are the ones where it
// must NOT appear: a button that opens a chat with nobody is worse than no
// button, because the customer believes they have asked and then waits.

import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { WhatsAppButton } from '../components/WhatsAppButton'
import { resetPublicConfig } from '../lib/config'

type Config = {
  company_name: string
  whatsapp: {
    enabled: boolean
    number: string
    message_id: string
    message_en: string
  }
}

function mockConfig(whatsapp: Partial<Config['whatsapp']>) {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    text: async () =>
      JSON.stringify({
        company_name: 'Ruuma Eatery',
        whatsapp: {
          enabled: true,
          number: '628176315568',
          message_id: 'Halo Ruuma',
          message_en: 'Hello Ruuma',
          ...whatsapp,
        },
      }),
  }) as unknown as typeof fetch
}

beforeEach(() => {
  localStorage.clear()
  // The config fetch is memoised so two components share one request. That
  // cache is module state, so it survives between tests and would hand the
  // next case the previous case's config.
  resetPublicConfig()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('WhatsAppButton', () => {
  it('links to wa.me with the number and the prefilled greeting', async () => {
    mockConfig({})
    render(<WhatsAppButton />)

    const link = await screen.findByRole('link')
    expect(link).toHaveAttribute(
      'href',
      `https://wa.me/628176315568?text=${encodeURIComponent('Halo Ruuma')}`,
    )
  })

  it('has an accessible name, since the icon carries no text', async () => {
    mockConfig({})
    render(<WhatsAppButton />)

    // ID is the default language (D9).
    expect(await screen.findByLabelText('Hubungi kami lewat WhatsApp')).toBeInTheDocument()
  })

  it('opens in a new tab without leaking the referrer to WhatsApp', async () => {
    mockConfig({})
    render(<WhatsAppButton />)

    const link = await screen.findByRole('link')
    expect(link).toHaveAttribute('target', '_blank')
    const rel = link.getAttribute('rel') ?? ''
    expect(rel).toContain('noopener')
    expect(rel).toContain('noreferrer')
  })

  it('renders nothing when the operator has switched it off', async () => {
    mockConfig({ enabled: false })
    const { container } = render(<WhatsAppButton />)

    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when no number is configured', async () => {
    mockConfig({ enabled: true, number: '' })
    const { container } = render(<WhatsAppButton />)

    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('stays silent when the config request fails', async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new Error('offline')) as unknown as typeof fetch
    const { container } = render(<WhatsAppButton />)

    // Chrome, not content: a failed read hides the button and leaves the page
    // alone. No error banner, no thrown render.
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('uses the English greeting when the language is EN', async () => {
    localStorage.setItem('ruuma.lang', 'en')
    mockConfig({})
    render(<WhatsAppButton />)

    const link = await screen.findByRole('link')
    expect(link.getAttribute('href')).toContain(encodeURIComponent('Hello Ruuma'))
  })
})
