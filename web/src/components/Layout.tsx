import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet } from 'react-router-dom'
import { cartCount, loadCart } from '../lib/cart'
import { currentLang, setLang, t } from '../i18n'

export function CustomerLayout() {
  const copy = t()
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
    <div className="min-h-dvh">
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
            <LanguageToggle tone="light" />
          </nav>
        </div>
      </header>

      <main id="main" className="mx-auto max-w-5xl px-4 py-6">
        <Outlet />
      </main>

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

// tone selects the contrast set. The customer header is an emerald fill and the
// admin header is a light surface, and this button appears in both — a single
// hard-coded colour is invisible in one of them.
export function LanguageToggle({ tone = 'dark' }: { tone?: 'light' | 'dark' }) {
  const [lang, setCurrent] = useState(currentLang())
  const copy = t()

  return (
    <button
      type="button"
      aria-label={copy.a11y.languageToggle}
      onClick={() => {
        const next = lang === 'id' ? 'en' : 'id'
        setLang(next)
        setCurrent(next)
        // The catalogues are read at render time, so a reload is the simplest
        // correct way to re-render every string.
        window.location.reload()
      }}
      className={[
        'inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-lg px-3 text-sm font-medium uppercase',
        tone === 'light'
          ? 'text-primary-fg/90 hover:bg-white/15 hover:text-primary-fg'
          : 'text-muted hover:bg-primary-subtle',
      ].join(' ')}
    >
      {lang}
    </button>
  )
}
