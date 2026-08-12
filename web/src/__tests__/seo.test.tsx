// robots.txt only asks a crawler not to *fetch* a URL. A page linked from
// anywhere else can still be indexed without ever being fetched, so `noindex`
// is what actually keeps checkout, cart, order history and sign-in out of
// search results. These tests exist so removing that hook fails CI rather than
// quietly putting a customer's order page in Google.

import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useNoIndex, useSeo } from '../lib/seo'

function robotsTag() {
  return document.head.querySelector('meta[name="robots"]')
}

function descriptionTag() {
  return document.head.querySelector<HTMLMetaElement>('meta[name="description"]')
}

beforeEach(() => {
  document.head.querySelectorAll('meta[name="robots"], meta[name="description"]').forEach((t) =>
    t.remove(),
  )
  document.title = ''
})

afterEach(() => {
  vi.restoreAllMocks()
})

function Page({ title, description }: { title: string; description?: string }) {
  useSeo(title, description)
  return <p>page</p>
}

function PrivatePage() {
  useSeo('Checkout')
  useNoIndex()
  return <p>private</p>
}

describe('useSeo', () => {
  it('sets a per-route title with the brand suffix', async () => {
    render(<Page title="Menu" />)
    await waitFor(() => expect(document.title).toBe('Menu — Ruuma Eatery'))
  })

  it('sets the meta description when one is given', async () => {
    render(<Page title="Menu" description="Indonesian, Chinese and Western" />)
    await waitFor(() =>
      expect(descriptionTag()?.content).toBe('Indonesian, Chinese and Western'),
    )
  })

  it('leaves the title alone while a page is still loading', async () => {
    document.title = 'Previous — Ruuma Eatery'
    render(<Page title="" />)

    // An empty title means "not known yet" — a dish still fetching. Writing it
    // would flash a bare "— Ruuma Eatery" in the tab.
    await waitFor(() => expect(screen.getByText('page')).toBeInTheDocument())
    expect(document.title).toBe('Previous — Ruuma Eatery')
  })
})

describe('useNoIndex', () => {
  it('marks a transactional page noindex, nofollow', async () => {
    render(<PrivatePage />)
    await waitFor(() => expect(robotsTag()).not.toBeNull())
    expect(robotsTag()?.getAttribute('content')).toBe('noindex, nofollow')
  })

  it('removes the tag on unmount so it cannot leak onto the next route', async () => {
    const { unmount } = render(<PrivatePage />)
    await waitFor(() => expect(robotsTag()).not.toBeNull())

    unmount()

    // In a client-rendered app a leaked noindex would de-index the menu the
    // moment someone navigated from checkout back to it.
    expect(robotsTag()).toBeNull()
  })
})
