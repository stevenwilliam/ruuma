// The kitchen and counter board: orders grouped by slot, with the aggregated
// production summary and a printable ticket (BR-2.8.x).

import { useCallback, useEffect, useState } from 'react'
import { api, type ListResponse, type Staff } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { Badge, Button, Card, EmptyState, ErrorNote, Spinner } from '../../components/ui'
import { StoreSelect, todayISO, useStores } from './common'
import { rupiah, slotLabel } from '../../lib/format'
import { t } from '../../i18n'

type BoardOrder = {
  id: string
  order_code: string
  status: string
  contact_name: string
  contact_phone: string
  total: number
  kitchen_units: number
  lines: Array<{ name: string; qty: number; options: string[]; notes: string }>
}

type BoardGroup = {
  slot_id: string
  starts_at: string
  ends_at: string
  order_count: number
  orders: BoardOrder[]
}

type ProductionRow = { item: string; option: string; qty: number; prep_minutes: number }

export default function OrdersBoard({ staff }: { staff: Staff }) {
  const copy = t()
  const stores = useStores()
  const [storeId, setStoreId] = useState('')
  const [date, setDate] = useState(todayISO())
  const [query, setQuery] = useState('')
  const [groups, setGroups] = useState<BoardGroup[] | null>(null)
  const [error, setError] = useState('')
  const [production, setProduction] = useState<{ slot: string; rows: ProductionRow[] } | null>(null)

  const load = useCallback(() => {
    const params = new URLSearchParams({ date })
    if (storeId) params.set('store_id', storeId)
    if (query) params.set('q', query)
    api
      .get<ListResponse<BoardGroup>>(`/ops/orders?${params}`)
      .then((res) => setGroups(res.items))
      .catch((err: Error) => setError(err.message))
  }, [storeId, date, query])

  useEffect(load, [load])

  async function advance(orderId: string, status: string) {
    setError('')
    try {
      await api.post(`/ops/orders/${orderId}/status`, { status }, crypto.randomUUID())
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function showProduction(slotId: string, label: string) {
    const res = await api.get<ListResponse<ProductionRow>>(`/ops/slots/${slotId}/production`)
    setProduction({ slot: label, rows: res.items })
  }

  const canCook = staff.permissions.includes('order.status.kitchen')
  const canHandOver = staff.permissions.includes('order.status.handover')

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-display text-2xl font-bold">Orders</h1>
        <StoreSelect stores={stores} value={storeId} onChange={setStoreId} />
        <label className="flex items-center gap-2 text-sm text-muted">
          Date
          <input
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="min-h-[44px] rounded-xl border border-border bg-surface px-2 text-sm text-body"
          />
        </label>
      </div>

      <SearchBox value={query} onChange={setQuery} placeholder="Order code, name or phone" />

      {error && <ErrorNote>{error}</ErrorNote>}
      {!groups && <Spinner label={copy.common.loading} />}
      {groups && groups.length === 0 && <EmptyState>{copy.common.empty}</EmptyState>}

      {production && (
        <Card className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <h2 className="font-display text-lg font-semibold">Production · {production.slot}</h2>
            <div className="flex gap-2 no-print">
              <Button variant="secondary" onClick={() => window.print()}>
                Print
              </Button>
              <Button variant="ghost" onClick={() => setProduction(null)}>
                {copy.common.close}
              </Button>
            </div>
          </div>
          {/* Aggregated per item and per option, longest prep first (BR-2.8.2/3). */}
          <ul className="flex flex-col gap-1 text-sm">
            {production.rows.map((r, i) => (
              <li key={i} className="flex justify-between border-b border-border py-1">
                <span>
                  {r.item}
                  {r.option ? ` · ${r.option}` : ''}
                </span>
                <span className="tabular font-semibold">{r.qty}×</span>
              </li>
            ))}
          </ul>
        </Card>
      )}

      {(groups ?? []).map((group) => (
        <section key={group.slot_id} className="flex flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="tabular font-display text-lg font-semibold">
              {slotLabel(group.starts_at, group.ends_at)}
            </h2>
            <Badge>{group.order_count} orders</Badge>
            <Button
              variant="ghost"
              onClick={() => showProduction(group.slot_id, slotLabel(group.starts_at, group.ends_at))}
            >
              Production summary
            </Button>
          </div>

          <ul className="grid gap-2 sm:grid-cols-2">
            {group.orders.map((order) => (
              <li key={order.id}>
                <Card className="flex flex-col gap-2">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-display font-semibold tracking-wide">{order.order_code}</span>
                    <Badge tone={order.status === 'READY' ? 'primary' : 'neutral'}>
                      {copy.order.status[order.status] ?? order.status}
                    </Badge>
                  </div>
                  <p className="text-sm text-muted">{order.contact_name}</p>
                  <ul className="text-sm">
                    {order.lines.map((l, i) => (
                      <li key={i}>
                        <span className="tabular font-medium">{l.qty}×</span> {l.name}
                        {l.options.length > 0 && (
                          <span className="text-muted"> · {l.options.join(', ')}</span>
                        )}
                        {l.notes && <span className="italic text-muted"> · “{l.notes}”</span>}
                      </li>
                    ))}
                  </ul>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="tabular text-sm">{rupiah(order.total)}</span>
                    <div className="ml-auto flex gap-2 no-print">
                      {canCook && order.status === 'ACCEPTED' && (
                        <Button variant="secondary" onClick={() => advance(order.id, 'IN_KITCHEN')}>
                          Start
                        </Button>
                      )}
                      {canCook && order.status === 'IN_KITCHEN' && (
                        <Button variant="secondary" onClick={() => advance(order.id, 'READY')}>
                          Ready
                        </Button>
                      )}
                      {canHandOver && order.status === 'READY' && (
                        <Button onClick={() => advance(order.id, 'PICKED_UP')}>Picked up</Button>
                      )}
                    </div>
                  </div>
                </Card>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}
