import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import { cartCount, loadCart } from '../lib/cart'
import { WhatsAppButton } from './WhatsAppButton'
import { LanguagePicker } from './LanguagePicker'
import { t } from '../i18n'

export function CustomerLayout() {
  const copy = t()
  const { pathname } = useLocation()
  const [count, setCount] = useState(() => cartCount(loadCart()))

  useEffect(() => {
    const update = () => setCount(cartCount(loadCart()))
    window.addEventListener('ruuma:cart', update)
    window.addEventListener('storage', update)
    return () => {
      window.removeEventListener('ruuma:cart', update)
      window.removeEventListener('storage', update)
    }
  }, [])

  return (
    // Sticky footer, the layout kind rather than position:fixed. A column flex
    // container the height of the viewport, with <main> taking the slack, puts
    // the footer at the bottom of the window on a short page (an empty cart,
    // the sign-in screen) and below the content on a long one.
    //
    // Deliberately not `fixed`: this footer carries the wordmark, the cuisine
    // line, the copyright and the photo-credits link, and pinning that much
    // over the viewport would eat the bottom of every phone screen — the space
    // the menu grid and the checkout CTA need most.
    <div className="flex min-h-dvh flex-col">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded-lg focus:bg-surface focus:px-3 focus:py-2"
      >
        {copy.a11y.skipToContent}
      </a>

      {/* Emerald header and footer: --primary as a full-bleed fill, with the
          reversed-out wordmark because the emerald logo would disappear into
          it (docs/10 §1, §2). */}
      <header className="sticky top-0 z-40 bg-primary text-primary-fg shadow-sm">
        <div className="mx-auto flex max-w-5xl items-center gap-3 px-4 py-3">
          <Link to="/" className="flex items-center gap-2">
            <img
              src="/brand/ruuma-logo-white.png"
              alt={copy.brand}
              width={112}
              height={58}
              className="h-8 w-auto"
            />
          </Link>

          <nav className="ml-auto flex items-center gap-1 text-sm">
            <NavLink to="/menu" className={navClass}>
              {copy.nav.menu}
            </NavLink>
            <NavLink to="/orders" className={navClass}>
              {copy.nav.orders}
            </NavLink>
            {/* The badge has to invert with the pill: on the active link the
                pill is already white, so a white badge would disappear. */}
            <NavLink to="/cart" className={navClass}>
              {({ isActive }) => (
                <>
                  {copy.nav.cart}
                  {count > 0 && (
                    <span
                      className={[
                        'ml-1 rounded-full px-1.5 text-xs font-semibold tabular',
                        isActive ? 'bg-primary text-primary-fg' : 'bg-white text-primary',
                      ].join(' ')}
                    >
                      {count}
                    </span>
                  )}
                </>
              )}
            </NavLink>
            <LanguagePicker tone="light" />
          </nav>
        </div>
      </header>

      {/* w-full alongside max-w-5xl: as a flex item in a column container the
          main would otherwise shrink to its content width.
          key={pathname} restarts the entrance on every route change, which is
          the whole page transition — no exit tween, because an exit blocks
          navigation and back/forward has to stay instant. */}
      {/* pb-32 reserves the height of the floating contact button so the last
          card in a list is never permanently sitting underneath it. A FAB
          overlapping mid-scroll content is inherent to the pattern; content
          you can never scroll clear of is a bug. */}
      <main
        key={pathname}
        id="main"
        className="rise-in mx-auto w-full max-w-5xl flex-1 px-4 pb-32 pt-6"
      >
        <Outlet />
      </main>

      <WhatsAppButton />

      {/* mt-10 keeps the gap on a long page; flex-1 on the main is what pushes
          this down on a short one. */}
      <footer className="mt-10 bg-primary text-primary-fg">
        <div className="mx-auto flex max-w-5xl flex-col gap-1 px-4 py-8 text-xs">
          {/* self-start is load-bearing: this is a flex item in a COLUMN
              container, where the default align-items: stretch stretches it
              along the cross axis — the width — and silently overrides
              w-auto. That is what distorted the wordmark. */}
          <img
            src="/brand/ruuma-logo-white.png"
            alt={copy.brand}
            width={112}
            height={58}
            className="mb-2 h-7 w-auto self-start"
          />
          <p>{copy.footer.cuisines}</p>
          <p className="text-primary-fg/80">{copy.footer.rights}</p>
          {/* CC BY / BY-SA dish photography has to name its photographers
              wherever it is published, so this link is on every page. */}
          <Link to="/credits" className="mt-1 w-fit underline underline-offset-4">
            {copy.footer.credits}
          </Link>
        </div>
      </footer>
    </div>
  )
}

// On the emerald header the old light-on-light pill had no contrast: both the
// resting text and the hover fill were tuned for a white surface.
function navClass({ isActive }: { isActive: boolean }) {
  return [
    'inline-flex min-h-[44px] items-center rounded-lg px-3',
    isActive
      ? 'bg-white font-medium text-primary'
      : 'text-primary-fg/90 hover:bg-white/15 hover:text-primary-fg',
  ].join(' ')
}

