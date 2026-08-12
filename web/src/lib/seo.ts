// Per-route document title and meta description.
//
// Without this every route is "Ruuma Eatery" — in the search result, the
// browser tab and the history entry. The tab and history are the part users
// feel immediately: three open ruuma tabs are indistinguishable otherwise.
//
// Deliberately not react-helmet or any head-management library. This is two
// DOM writes; a dependency for that is not worth the bytes on a menu people
// open on mobile data (CLAUDE.md §3 pins the stack).
//
// This runs in the browser, so it reaches Google — which renders JS — but not
// link-preview bots, which do not. Those read the static tags in index.html.
// See docs/10 §6.

import { useEffect } from 'react'

const SUFFIX = 'Ruuma Eatery'

function setMeta(name: string, content: string) {
  let tag = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!tag) {
    tag = document.createElement('meta')
    tag.name = name
    document.head.appendChild(tag)
  }
  tag.content = content
}

/**
 * Sets the page title and, optionally, the meta description.
 *
 * Pass the bare page name — the brand suffix is appended here so it cannot
 * drift between pages. Pass an empty title on a page whose name is not known
 * yet (a dish still loading) and the document keeps the previous title rather
 * than flashing "— Ruuma Eatery" on its own.
 */
export function useSeo(title: string, description?: string) {
  useEffect(() => {
    if (title) {
      document.title = `${title} — ${SUFFIX}`
    }
    if (description) {
      setMeta('description', description)
    }
  }, [title, description])
}

/**
 * Marks a route as not for indexing.
 *
 * robots.txt already disallows the transactional pages, but robots.txt only
 * asks a crawler not to *fetch* — a URL linked from elsewhere can still be
 * indexed without being fetched. `noindex` is what actually keeps a checkout
 * or an order-tracking page out of results.
 *
 * The tag is removed on unmount, or it would leak onto the next route in a
 * client-rendered app and quietly de-index the menu.
 */
export function useNoIndex() {
  useEffect(() => {
    const tag = document.createElement('meta')
    tag.name = 'robots'
    tag.content = 'noindex, nofollow'
    document.head.appendChild(tag)
    return () => {
      tag.remove()
    }
  }, [])
}
