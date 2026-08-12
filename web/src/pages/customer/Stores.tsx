// Store picker. It is honest about hours: closed days, today's state and the
// next open date (docs/01 §3.1, BR-2.1.4).

import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type ListResponse, type Store } from '../../lib/api'
import { selectStore, selectedStoreId } from '../../lib/cart'
import { SearchBox } from '../../components/SearchBox'
import { Badge, Button, Card, EmptyState, ErrorNote, Spinner } from '../../components/ui'
import { longDate } from '../../lib/format'
import { t } from '../../i18n'
import { useSeo } from '../../lib/seo'

export default function StoresPage() {
  const copy = t()
  useSeo(copy.stores.title)
  const navigate = useNavigate()
  const [stores, setStores] = useState<Store[] | null>(null)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const chosen = selectedStoreId()

  useEffect(() => {
    let cancelled = false
    api
      .get<ListResponse<Store>>(`/stores${query ? `?q=${encodeURIComponent(query)}` : ''}`)
      .then((res) => !cancelled && setStores(res.items))
      .catch((err: Error) => !cancelled && setError(err.message))
    return () => {
      cancelled = true
    }
  }, [query])

  const sorted = useMemo(
    () => (stores ?? []).slice().sort((a, b) => Number(b.open_today) - Number(a.open_today)),
    [stores],
  )

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-2xl font-bold">{copy.stores.title}</h1>
        <p className="text-sm text-muted">{copy.stores.subtitle}</p>
      </header>

      {/* BR-1.5.1: every list has a search box. */}
      <SearchBox value={query} onChange={setQuery} />

      {error && <ErrorNote>{error}</ErrorNote>}
      {!stores && !error && <Spinner label={copy.common.loading} />}
      {stores && sorted.length === 0 && <EmptyState>{copy.common.empty}</EmptyState>}

      <ul className="grid gap-3 sm:grid-cols-2">
        {sorted.map((store) => (
          <li key={store.id}>
            <Card className="flex h-full flex-col gap-3">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <h2 className="font-display text-lg font-semibold">{store.name}</h2>
                  <p className="text-sm text-muted">{store.address_line}</p>
                  <p className="text-sm text-muted">{store.city}</p>
                </div>
                {store.open_today ? (
                  <Badge tone="primary">{copy.stores.openToday}</Badge>
                ) : (
                  <Badge tone="warning">{copy.stores.closedToday}</Badge>
                )}
              </div>

              <div className="flex flex-wrap gap-1.5">
                {store.fulfilment_modes.map((m) => (
                  <Badge key={m}>{copy.stores.modes[m as 'pickup' | 'delivery'] ?? m}</Badge>
                ))}
              </div>

              {store.open_today && store.today_hours?.length ? (
                <p className="tabular text-sm text-body">{store.today_hours.join(' · ')}</p>
              ) : (
                <p className="text-sm text-muted">
                  {store.today_reason ? (copy.stores.reasons[store.today_reason] ?? '') : ''}
                  {store.next_open_date && (
                    <>
                      {' — '}
                      {copy.stores.nextOpen} {longDate(store.next_open_date)}
                    </>
                  )}
                </p>
              )}

              <Button
                className="mt-auto"
                variant={chosen === store.id ? 'secondary' : 'primary'}
                onClick={() => {
                  selectStore(store.id)
                  navigate('/menu')
                }}
              >
                {chosen === store.id ? copy.stores.chosen : copy.stores.choose}
              </Button>
            </Card>
          </li>
        ))}
      </ul>
    </div>
  )
}
