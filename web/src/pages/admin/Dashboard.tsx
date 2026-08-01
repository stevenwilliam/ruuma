import { useEffect, useState } from 'react'
import { api, type ListResponse, type Staff } from '../../lib/api'
import { Card, Spinner } from '../../components/ui'
import { StoreSelect, todayISO, useStores } from './common'
import { rupiah, slotLabel } from '../../lib/format'
import { t } from '../../i18n'

type BoardGroup = {
  slot_id: string
  starts_at: string
  ends_at: string
  order_count: number
  orders: Array<{ status: string; total: number; kitchen_units: number }>
}

export default function Dashboard({ staff }: { staff: Staff }) {
  const copy = t()
  const stores = useStores()
  const [storeId, setStoreId] = useState('')
  const [groups, setGroups] = useState<BoardGroup[] | null>(null)

  useEffect(() => {
    const params = new URLSearchParams({ date: todayISO() })
    if (storeId) params.set('store_id', storeId)
    api
      .get<ListResponse<BoardGroup>>(`/ops/orders?${params}`)
      .then((res) => setGroups(res.items))
      .catch(() => setGroups([]))
  }, [storeId])

  const orders = (groups ?? []).flatMap((g) => g.orders)
  const revenue = orders
    .filter((o) => !['CANCELLED', 'REFUNDED'].includes(o.status))
    .reduce((sum, o) => sum + o.total, 0)
  const cancelled = orders.filter((o) => o.status === 'CANCELLED').length

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-display text-2xl font-bold">Today</h1>
        <StoreSelect stores={stores} value={storeId} onChange={setStoreId} />
        <span className="ml-auto text-sm text-muted">{staff.role}</span>
      </div>

      <div className="grid gap-3 sm:grid-cols-4">
        <Stat label="Slots with orders" value={String(groups?.length ?? 0)} />
        <Stat label="Orders" value={String(orders.length)} />
        <Stat label="Revenue" value={rupiah(revenue)} />
        <Stat label="Cancellations" value={String(cancelled)} />
      </div>

      {!groups && <Spinner label={copy.common.loading} />}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(groups ?? []).map((g) => {
          const units = g.orders.reduce((n, o) => n + o.kitchen_units, 0)
          return (
            <Card key={g.slot_id} className="flex flex-col gap-1">
              <p className="tabular font-display text-lg font-semibold">
                {slotLabel(g.starts_at, g.ends_at)}
              </p>
              <p className="text-sm text-muted">
                {g.order_count} orders · {units} kitchen units
              </p>
            </Card>
          )
        })}
      </div>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <Card className="flex flex-col gap-1">
      <span className="text-xs uppercase tracking-wide text-muted">{label}</span>
      <span className="tabular font-display text-xl font-bold">{value}</span>
    </Card>
  )
}
