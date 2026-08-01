// Every list and table in ruuma has one of these (BR-1.5.1). It is debounced at
// 300ms per docs/10 §4.1 and is a real labelled input, not a decorated div.

import { useEffect, useId, useState } from 'react'
import { t } from '../i18n'

export function SearchBox({
  value,
  onChange,
  label,
  placeholder,
  autoFocus,
}: {
  value: string
  onChange: (next: string) => void
  label?: string
  placeholder?: string
  autoFocus?: boolean
}) {
  const id = useId()
  const [draft, setDraft] = useState(value)
  const copy = t()

  useEffect(() => setDraft(value), [value])

  useEffect(() => {
    if (draft === value) return
    const timer = setTimeout(() => onChange(draft), 300)
    return () => clearTimeout(timer)
    // onChange is intentionally excluded: callers pass inline closures.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft])

  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="sr-only">
        {label ?? copy.common.searchLabel}
      </label>
      <input
        id={id}
        type="search"
        role="searchbox"
        autoFocus={autoFocus}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        placeholder={placeholder ?? copy.common.search}
        className="min-h-[44px] w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-body placeholder:text-muted"
      />
    </div>
  )
}
