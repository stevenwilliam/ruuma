// The slot picker is keyboard-operable and every disabled slot states its
// reason — never a bare grey box (BR-2.3.6, docs/10 §4/§5).

import type { Slot } from '../lib/api'
import { t } from '../i18n'
import { slotLabel } from '../lib/format'

export function SlotPicker({
  slots,
  selected,
  onSelect,
  timeZone,
}: {
  slots: Slot[]
  selected?: string
  onSelect: (slot: Slot) => void
  timeZone?: string
}) {
  const copy = t()
  const firstBookable = slots.find((s) => s.is_bookable)

  return (
    <div className="flex flex-col gap-3">
      {firstBookable && selected !== firstBookable.slot_id && (
        <button
          type="button"
          onClick={() => onSelect(firstBookable)}
          className="self-start text-sm font-medium text-primary underline underline-offset-4"
        >
          {copy.checkout.nextAvailable}: {slotLabel(firstBookable.starts_at, firstBookable.ends_at, timeZone)}
        </button>
      )}

      <ul role="listbox" aria-label={copy.checkout.pickSlot} className="grid grid-cols-2 gap-2 sm:grid-cols-3">
        {slots.map((slot) => {
          const isSelected = slot.slot_id === selected
          const reason = slot.reason ? (copy.checkout.reasons[slot.reason] ?? slot.reason) : ''
          return (
            <li key={slot.slot_id}>
              <button
                type="button"
                role="option"
                aria-selected={isSelected}
                disabled={!slot.is_bookable}
                onClick={() => onSelect(slot)}
                className={[
                  'flex min-h-[64px] w-full flex-col items-start justify-center rounded-xl border px-3 py-2 text-left transition-colors',
                  isSelected
                    ? 'border-primary bg-primary-subtle'
                    : slot.is_bookable
                      ? 'border-border bg-surface hover:border-primary'
                      : 'cursor-not-allowed border-border bg-surface opacity-70',
                ].join(' ')}
              >
                <span className="tabular text-sm font-medium text-body">
                  {slotLabel(slot.starts_at, slot.ends_at, timeZone)}
                </span>
                {slot.is_bookable ? (
                  <span className={`text-xs ${slot.almost_full ? 'text-warning' : 'text-muted'}`}>
                    {slot.almost_full
                      ? copy.checkout.almostFull
                      : copy.checkout.remaining.replace('{n}', String(slot.remaining_orders))}
                  </span>
                ) : (
                  <span className="text-xs text-muted">{reason}</span>
                )}
              </button>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
