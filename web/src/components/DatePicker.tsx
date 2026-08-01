// A horizontal date strip. Unbookable dates stay visible with their reason so a
// customer can see that a store simply closes on Sundays (BR-2.1.4).

import type { DateAvailability } from '../lib/api'
import { t, currentLang } from '../i18n'
import { shortDate } from '../lib/format'

export function DatePicker({
  dates,
  selected,
  onSelect,
}: {
  dates: DateAvailability[]
  selected?: string
  onSelect: (date: string) => void
}) {
  const copy = t()
  const locale = currentLang() === 'en' ? 'en-GB' : 'id-ID'

  return (
    <ul
      role="listbox"
      aria-label={copy.checkout.pickDate}
      className="flex snap-x gap-2 overflow-x-auto pb-2"
    >
      {dates.map((d) => {
        const iso = `${d.date}T00:00:00+07:00`
        const isSelected = d.date === selected
        const weekday = new Intl.DateTimeFormat(locale, {
          weekday: 'short',
          timeZone: 'Asia/Jakarta',
        }).format(new Date(iso))
        return (
          <li key={d.date} className="snap-start">
            <button
              type="button"
              role="option"
              aria-selected={isSelected}
              disabled={!d.is_bookable}
              onClick={() => onSelect(d.date)}
              title={!d.is_bookable && d.reason ? (copy.checkout.reasons[d.reason] ?? d.reason) : undefined}
              className={[
                'flex min-h-[68px] w-[76px] flex-col items-center justify-center rounded-xl border px-2 py-2',
                isSelected
                  ? 'border-primary bg-primary-subtle'
                  : d.is_bookable
                    ? 'border-border bg-surface hover:border-primary'
                    : 'cursor-not-allowed border-border bg-surface opacity-60',
              ].join(' ')}
            >
              <span className="text-xs text-muted">{weekday}</span>
              <span className="tabular text-sm font-medium text-body">{shortDate(iso, locale)}</span>
              {!d.is_bookable && d.reason && (
                <span className="mt-0.5 text-[10px] leading-tight text-muted">
                  {copy.checkout.reasons[d.reason] ?? ''}
                </span>
              )}
            </button>
          </li>
        )
      })}
    </ul>
  )
}
