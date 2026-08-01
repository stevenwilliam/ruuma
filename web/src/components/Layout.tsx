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

      <header className="sticky top-0 z-40 border-b border-border bg-surface/95 backdrop-blur">
        <div className="mx-auto flex max-w-5xl items-center gap-3 px-4 py-3">
          <Link to="/" className="flex items-center gap-2">
            <img
              src="/brand/ruuma-logo-emerald.png"
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
            <NavLink to="/cart" className={navClass}>
              {copy.nav.cart}
              {count > 0 && (
                <span className="ml-1 rounded-full bg-primary px-1.5 text-xs text-primary-fg tabular">
                  {count}
                </span>
              )}
            </NavLink>
            <LanguageToggle />
          </nav>
        </div>
      </header>

      <main id="main" className="mx-auto max-w-5xl px-4 py-6">
        <Outlet />
      </main>

      <footer className="mx-auto max-w-5xl px-4 py-10 text-xs text-muted">
        {copy.brand} · Indonesian · Chinese · Western
      </footer>
    </div>
  )
}

function navClass({ isActive }: { isActive: boolean }) {
  return [
    'inline-flex min-h-[44px] items-center rounded-lg px-3',
    isActive ? 'bg-primary-subtle font-medium text-primary' : 'text-body hover:bg-primary-subtle',
  ].join(' ')
}

export function LanguageToggle() {
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
      className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-lg px-3 text-sm font-medium uppercase text-muted hover:bg-primary-subtle"
    >
      {lang}
    </button>
  )
}
