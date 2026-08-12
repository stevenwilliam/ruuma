// Shared access to GET /public-config.
//
// Two components need it — the contact button and the backdrop override — and
// two components must not mean two requests. The promise is memoised, so the
// first caller starts the fetch and everyone else awaits the same one.
//
// Not cached across reloads: these are operator settings that must take effect
// within the parameter cache's 30s TTL, not whenever a customer happens to
// clear storage.

import { useEffect, useState } from 'react'
import { api } from './api'
import { applyBackdrop, type BackdropConfig } from './backdrop'

export type PublicConfig = {
  company_name: string
  whatsapp: {
    enabled: boolean
    number: string
    message_id: string
    message_en: string
  }
  backdrop?: BackdropConfig
}

let inflight: Promise<PublicConfig | null> | null = null

export function loadPublicConfig(): Promise<PublicConfig | null> {
  if (!inflight) {
    inflight = api
      .get<PublicConfig>('/public-config')
      // Chrome, not content: a failed read leaves every default in place and
      // the page is otherwise unaffected. No error banner, no thrown render.
      .catch(() => null)
  }
  return inflight
}

/** Test seam — lets a suite start from a clean slate. */
export function resetPublicConfig() {
  inflight = null
}

export function usePublicConfig(): PublicConfig | null {
  const [config, setConfig] = useState<PublicConfig | null>(null)

  useEffect(() => {
    let cancelled = false
    void loadPublicConfig().then((res) => {
      if (cancelled || !res) return
      setConfig(res)
      // Applied here rather than in a component's render: the backdrop is a
      // property of the document, not of any one screen, and it must survive
      // route changes without being re-applied on each.
      applyBackdrop(res.backdrop)
    })
    return () => {
      cancelled = true
    }
  }, [])

  return config
}
