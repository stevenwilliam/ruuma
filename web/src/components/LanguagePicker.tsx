// Language picker.
//
// It replaces a single button that read "ID" or "EN", which was ambiguous in
// the way every one-button language toggle is: the label is the *current*
// language, but a button label normally describes what pressing it does. Half
// of users read "EN" as "you are in English" and half as "switch to English".
//
// Two options side by side, with the active one marked, removes the question —
// you can see both languages and which one you are in.
//
// On flags: a flag is a country, not a language, and the usual objection is
// real. It survives here because ruuma ships exactly two languages for one
// market — Indonesian and English for an Indonesian restaurant — and both
// flags are unambiguous in that context. The flag is never the only signal:
// every option carries a visible ID/EN label beside it and an accessible name
// on the control, so nothing depends on recognising the artwork.

import { currentLang, setLang, t, type Lang } from '../i18n'

// Inline SVG rather than the flag emoji. Emoji flags do not render at all on
// Windows (they fall back to the letter pair), and they cannot inherit size or
// alignment the way an SVG can.
function FlagID({ className = '' }: { className?: string }) {
  return (
    <svg viewBox="0 0 21 14" className={className} aria-hidden="true" focusable="false">
      <rect width="21" height="7" fill="#E30A17" />
      <rect y="7" width="21" height="7" fill="#FFFFFF" />
      <rect width="21" height="14" fill="none" stroke="rgba(0,0,0,.18)" strokeWidth="1" />
    </svg>
  )
}

function FlagEN({ className = '' }: { className?: string }) {
  return (
    <svg viewBox="0 0 60 40" className={className} aria-hidden="true" focusable="false">
      <rect width="60" height="40" fill="#012169" />
      {/* White saltire, then the red one inset on top of it. */}
      <path d="M0 0 60 40M60 0 0 40" stroke="#FFFFFF" strokeWidth="8" />
      <path d="M0 0 60 40M60 0 0 40" stroke="#C8102E" strokeWidth="4" />
      {/* White cross, then red. */}
      <path d="M30 0V40M0 20H60" stroke="#FFFFFF" strokeWidth="13" />
      <path d="M30 0V40M0 20H60" stroke="#C8102E" strokeWidth="8" />
    </svg>
  )
}

const OPTIONS: Array<{ lang: Lang; label: string; Flag: typeof FlagID }> = [
  { lang: 'id', label: 'ID', Flag: FlagID },
  { lang: 'en', label: 'EN', Flag: FlagEN },
]

// tone selects the contrast set. The customer header is an emerald fill and the
// admin header is a light surface, and this appears in both — a single
// hard-coded colour is invisible in one of them.
export function LanguagePicker({ tone = 'dark' }: { tone?: 'light' | 'dark' }) {
  const copy = t()
  const active = currentLang()

  return (
    <div
      // A group label, so a screen reader announces what the pair is for
      // before reading the two options.
      role="group"
      aria-label={copy.a11y.languageToggle}
      className={[
        'inline-flex items-center gap-0.5 rounded-lg p-0.5',
        tone === 'light' ? 'bg-white/15' : 'bg-primary-subtle',
      ].join(' ')}
    >
      {OPTIONS.map(({ lang, label, Flag }) => {
        const isActive = lang === active
        return (
          <button
            key={lang}
            type="button"
            lang={lang}
            aria-pressed={isActive}
            onClick={() => {
              if (isActive) return
              setLang(lang)
              // The catalogues are read at render time, so a reload is the
              // simplest correct way to re-render every string.
              window.location.reload()
            }}
            className={[
              'inline-flex min-h-[44px] items-center gap-1.5 rounded-md px-2 text-xs font-semibold',
              isActive
                ? tone === 'light'
                  ? 'bg-white text-primary'
                  : 'bg-surface text-primary-ink'
                : tone === 'light'
                  ? 'text-primary-fg/80 hover:text-primary-fg'
                  : 'text-muted hover:text-body',
            ].join(' ')}
          >
            {/* Fixed 4:3-ish box with a hairline, so neither flag reflows the
                row and the white half of the ID flag stays visible on a white
                active pill. */}
            <Flag className="h-3.5 w-5 rounded-[2px] object-cover" />
            {label}
          </button>
        )
      })}
    </div>
  )
}
