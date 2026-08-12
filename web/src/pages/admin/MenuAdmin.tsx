// Per-store availability: 86 an item, lift it, or set today's stock countdown
// (BR-2.2.3/4). Price overrides need the price permission (docs/02 §3).

import { useCallback, useEffect, useState } from 'react'
import { api, type ListResponse, type MenuItem } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { AsyncButton, Badge, ErrorNote, Spinner } from '../../components/ui'
import { StoreSelect, Table, todayISO, useStores } from './common'
import { rupiah } from '../../lib/format'
import { t } from '../../i18n'

export default function MenuAdmin() {
  const copy = t()
  const stores = useStores()
  const [storeId, setStoreId] = useState('')
  const [items, setItems] = useState<MenuItem[] | null>(null)
  const [query, setQuery] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!storeId && stores.length > 0) setStoreId(stores[0].id)
  }, [stores, storeId])

  const load = useCallback(() => {
    if (!storeId) return
    const params = new URLSearchParams({ store_id: storeId, limit: '200' })
    if (query) params.set('q', query)
    api
      .get<ListResponse<MenuItem>>(`/menu?${params}`)
      .then((res) => setItems(res.items))
      .catch((err: Error) => setError(err.message))
  }, [storeId, query])

  useEffect(load, [load])

  async function eightySix(item: MenuItem) {
    setError('')
    try {
      await api.post(`/admin/stores/${storeId}/86`, {
        menu_item_id: item.id,
        reason: 'Out of stock',
      })
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function lift(item: MenuItem) {
    setError('')
    try {
      await api.del(`/admin/stores/${storeId}/86/${item.id}`)
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function setStock(item: MenuItem) {
    const value = window.prompt(`Stock for ${item.name_id} today:`, '20')
    if (!value) return
    setError('')
    try {
      await api.put(`/admin/stores/${storeId}/daily-stock`, {
        menu_item_id: item.id,
        business_date: todayISO(),
        stock_total: Number(value),
      })
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-display text-2xl font-bold">Menu availability</h1>
        <StoreSelect stores={stores} value={storeId} onChange={setStoreId} allowAll={false} />
      </div>

      <SearchBox value={query} onChange={setQuery} placeholder="Dish name" />
      {error && <ErrorNote>{error}</ErrorNote>}
      {!items && <Spinner label={copy.common.loading} />}

      {items && (
        <Table head={['Dish', 'Cuisine', 'Price', 'State', 'Stock', '']}>
          {items.map((item) => (
            <tr key={item.id}>
              <td className="px-3 py-2 font-medium">{item.name_id}</td>
              <td className="px-3 py-2 text-muted">{item.cuisine}</td>
              <td className="tabular px-3 py-2">{rupiah(item.price.value)}</td>
              <td className="px-3 py-2">
                {item.is_available ? (
                  <Badge tone="primary">available</Badge>
                ) : (
                  <Badge tone="danger">{item.availability}</Badge>
                )}
              </td>
              <td className="tabular px-3 py-2">{item.stock_left ?? '—'}</td>
              <td className="px-3 py-2">
                <div className="flex gap-2">
                  {/* 86 takes the dish off the customer menu straight away.
                      Lifting it is the safe direction, so only one of the two
                      asks. */}
                  {item.is_available ? (
                    <AsyncButton
                      variant="secondary"
                      busyLabel="…"
                      confirm={`Mark ${item.name_id} as 86? Customers stop being able to order it immediately.`}
                      onRun={() => eightySix(item)}
                    >
                      86
                    </AsyncButton>
                  ) : (
                    <AsyncButton variant="secondary" busyLabel="…" onRun={() => lift(item)}>
                      Lift 86
                    </AsyncButton>
                  )}
                  {/* setStock already prompts for the number, which doubles as
                      the confirmation step. */}
                  <AsyncButton variant="ghost" busyLabel="…" onRun={() => setStock(item)}>
                    Set stock
                  </AsyncButton>
                </div>
              </td>
            </tr>
          ))}
        </Table>
      )}
    </div>
  )
}
