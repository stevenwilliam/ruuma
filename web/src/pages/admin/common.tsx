// Shared admin pieces. Every list here has a search box (BR-1.5.1) — the
// frontend test enumerates these screens and fails if one is missing.

import { useEffect, useState } from 'react'
import { api, type ListResponse, type Store } from '../../lib/api'

export function useStores() {
  const [stores, setStores] = useState<Store[]>([])
  useEffect(() => {
    api
      .get<ListResponse<Store>>('/admin/stores')
      .then((res) => setStores(res.items))
      .catch(() => undefined)
  }, [])
  return stores
}

export function StoreSelect({
  stores,
  value,
  onChange,
  allowAll = true,
  label = 'Store',
}: {
  stores: Store[]
  value: string
  onChange: (id: string) => void
  allowAll?: boolean
  label?: string
}) {
  return (
    <label className="flex items-center gap-2 text-sm text-muted">
      {label}
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="min-h-[44px] rounded-xl border border-border bg-surface px-2 text-sm text-body"
      >
        {allowAll && <option value="">All stores</option>}
        {stores.map((s) => (
          <option key={s.id} value={s.id}>
            {s.name}
          </option>
        ))}
      </select>
    </label>
  )
}

export function Table({ head, children }: { head: string[]; children: React.ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-border">
      <table className="w-full min-w-[640px] text-sm">
        <thead className="bg-primary-subtle/60 text-left">
          <tr>
            {head.map((h) => (
              <th key={h} scope="col" className="px-3 py-2 font-medium text-body">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-border">{children}</tbody>
      </table>
    </div>
  )
}

export function todayISO(): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Jakarta' }).format(new Date())
}
