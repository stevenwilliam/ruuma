// Floating "chat with us" button, bottom right of every customer page.
//
// The number, the on/off switch and the greeting all come from sys_parameters
// via GET /public-config (BR-1.4.5) — a restaurant changes the number it
// answers on far more often than it deploys, so none of it is hard-coded.
//
// It renders nothing at all until the config arrives, and nothing ever if the
// number is blank. A contact button that opens a chat with nobody is worse
// than no button: the customer believes they have asked and waits.

import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { currentLang, t } from '../i18n'

type PublicConfig = {
  company_name: string
  whatsapp: {
    enabled: boolean
    number: string
    message_id: string
    message_en: string
  }
}

export function WhatsAppButton() {
  const copy = t()
  const [config, setConfig] = useState<PublicConfig | null>(null)

  useEffect(() => {
    let cancelled = false
    api
      .get<PublicConfig>('/public-config')
      // Chrome, not content: if this read fails the button stays hidden and
      // the page is otherwise unaffected. Nothing is worth an error banner.
      .then((res) => {
        if (!cancelled) setConfig(res)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [])

  if (!config?.whatsapp.enabled || !config.whatsapp.number) return null

  const greeting =
    currentLang() === 'en' ? config.whatsapp.message_en : config.whatsapp.message_id
  const href = `https://wa.me/${config.whatsapp.number}${
    greeting ? `?text=${encodeURIComponent(greeting)}` : ''
  }`

  return (
    <a
      href={href}
      target="_blank"
      // noreferrer as well as noopener: wa.me is a third party and has no
      // business seeing which ruuma page the customer left from.
      rel="noopener noreferrer"
      aria-label={copy.a11y.whatsapp}
      // z-40 matches the sticky header rather than exceeding it — the two never
      // overlap, and a higher value would put this above a future modal.
      // The bottom offset carries the iOS home-indicator inset on top of the
      // base spacing, which is why index.html sets viewport-fit=cover.
      // 112px, double the original 56. That is a large object on a 375px
      // phone, so it is the one thing allowed to sit over the page and it
      // stays out of the way of the sticky add-to-cart bar (see Item.tsx).
      className="no-print fixed right-4 z-40 flex h-28 w-28 items-center justify-center rounded-full bg-[#25D366] text-white shadow-lg transition-transform hover:scale-105 focus-visible:scale-105"
      style={{ bottom: 'calc(1rem + env(safe-area-inset-bottom, 0px))' }}
    >
      {/* An SVG, not the emoji: emoji render as someone else's artwork per
          platform, do not inherit currentColor, and are announced by a screen
          reader on top of the aria-label. WhatsApp green #25D366 is the
          brand's own and is deliberately exempt from the ruuma palette — a
          recoloured logo would not read as WhatsApp. */}
      <svg
        viewBox="0 0 24 24"
        className="h-14 w-14"
        fill="currentColor"
        aria-hidden="true"
        focusable="false"
      >
        <path d="M17.47 14.38c-.3-.15-1.75-.86-2.02-.96-.27-.1-.47-.15-.67.15-.2.3-.77.96-.94 1.16-.17.2-.35.22-.64.07-.3-.15-1.25-.46-2.38-1.47-.88-.78-1.47-1.75-1.64-2.05-.17-.3-.02-.46.13-.6.13-.13.3-.35.45-.52.15-.17.2-.3.3-.5.1-.2.05-.37-.02-.52-.08-.15-.67-1.6-.92-2.2-.24-.58-.49-.5-.67-.51h-.57c-.2 0-.52.07-.79.37-.27.3-1.04 1.02-1.04 2.48s1.06 2.88 1.21 3.08c.15.2 2.1 3.2 5.08 4.49.71.3 1.26.49 1.69.63.71.22 1.36.19 1.87.12.57-.09 1.75-.72 2-1.41.25-.7.25-1.29.17-1.41-.07-.13-.27-.2-.57-.35z" />
        <path d="M12.04 2C6.58 2 2.13 6.45 2.13 11.91c0 1.75.46 3.45 1.32 4.95L2 22l5.28-1.38a9.86 9.86 0 0 0 4.76 1.21h.01c5.46 0 9.91-4.45 9.91-9.91 0-2.65-1.03-5.14-2.9-7.01A9.82 9.82 0 0 0 12.04 2zm0 18.13h-.01a8.2 8.2 0 0 1-4.18-1.15l-.3-.18-3.11.82.83-3.04-.2-.31a8.17 8.17 0 0 1-1.26-4.36c0-4.54 3.7-8.24 8.24-8.24 2.2 0 4.27.86 5.82 2.42a8.18 8.18 0 0 1 2.41 5.83c0 4.54-3.69 8.24-8.24 8.24z" />
      </svg>
    </a>
  )
}
