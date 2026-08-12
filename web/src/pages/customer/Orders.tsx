import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type ListResponse, type Order } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { Badge, Button, Card, EmptyState, ErrorNote, Spinner } from '../../components/ui'
import { longDate, rupiah, slotLabel } from '../../lib/format'
import { t } from '../../i18n'
import { useNavigate } from 'react-router-dom'
import { loadCart, saveCart, selectStore } from '../../lib/cart'
import { useSeo, useNoIndex } from '../../lib/seo'

export default function OrdersPage() {
  const copy = t()
  useSeo(copy.order.history)
  useNoIndex()
  const navigate = useNavigate()
  const [orders, setOrders] = useState<Order[] | null>(null)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')

  useEffect(() => {
    api
      .get<ListResponse<Order>>(`/orders${query ? `?q=${encodeURIComponent(query)}` : ''}`)
      .then((res) => setOrders(res.items))
      .catch((err: Error) => setError(err.message))
  }, [query])

  // Reorder revalidates against today's menu and reports what changed
  // (docs/01 §3.1).
  async function reorder(order: Order) {
    const result = await api.post<{
      lines: Array<{ menu_item_id: string; qty: number; notes: string; option_choice_ids: string[] }>
      warnings: Array<{ message: string }>
    }>(`/orders/${order.id}/reorder`)

    if (result.warnings.length > 0) {
      alert(result.warnings.map((w) => w.message).join('\n'))
    }
    selectStore(order.store.id)
    const cart = loadCart()
    saveCart({
      storeId: order.store.id,
      lines: result.lines.map((l, i) => ({
        key: `${l.menu_item_id}-${i}`,
        menuItemId: l.menu_item_id,
        name: order.lines.find((ol) => ol.menu_item_id === l.menu_item_id)?.name_id ?? '',
        unitPrice: order.lines.find((ol) => ol.menu_item_id === l.menu_item_id)?.unit_price.value ?? 0,
        optionsDelta: 0,
        qty: l.qty,
        notes: l.notes,
        optionChoiceIds: l.option_choice_ids,
        optionLabels: [],
      })),
    })
    void cart
    navigate('/cart')
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-display text-2xl font-bold">{copy.order.history}</h1>
      <SearchBox value={query} onChange={setQuery} />

      {error && <ErrorNote>{error}</ErrorNote>}
      {!orders && !error && <Spinner label={copy.common.loading} />}
      {orders && orders.length === 0 && <EmptyState>{copy.common.empty}</EmptyState>}

      <ul className="flex flex-col gap-3">
        {(orders ?? []).map((order) => (
          <li key={order.id}>
            <Card className="flex flex-wrap items-center gap-3">
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <Link to={`/orders/${order.id}`} className="font-display font-semibold tracking-wide">
                    {order.order_code}
                  </Link>
                  <Badge>{copy.order.status[order.status] ?? order.status}</Badge>
                </div>
                <p className="text-sm text-muted">
                  {order.store.name} · {longDate(order.slot.starts_at)} ·{' '}
                  {slotLabel(order.slot.starts_at, order.slot.ends_at)}
                </p>
              </div>
              <span className="tabular text-sm font-semibold">{rupiah(order.total)}</span>
              <Button variant="secondary" onClick={() => reorder(order)}>
                {copy.order.reorder}
              </Button>
            </Card>
          </li>
        ))}
      </ul>
    </div>
  )
}
