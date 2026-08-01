// Home for a chosen store: the menu grid with filters, sort and a debounced
// search box (BR-1.5.1). Sold-out dishes stay visible with their state — hiding
// them makes the menu look broken (docs/10 §4.2).

import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, type ListResponse, type MenuItem, type Store } from '../../lib/api'
import { selectedStoreId } from '../../lib/cart'
import { SearchBox } from '../../components/SearchBox'
import { Badge, Card, EmptyState, ErrorNote, Spinner } from '../../components/ui'
import { rupiah } from '../../lib/format'
import { localeDesc, localeName, t } from '../../i18n'

const CUISINES = ['indonesian', 'chinese', 'western', 'other'] as const

export default function MenuPage() {
  const copy = t()
  const navigate = useNavigate()
  const storeId = selectedStoreId()

  const [store, setStore] = useState<Store | null>(null)
  const [items, setItems] = useState<MenuItem[] | null>(null)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [cuisine, setCuisine] = useState('')
  const [diet, setDiet] = useState('')
  const [sort, setSort] = useState('')

  useEffect(() => {
    if (!storeId) navigate('/')
  }, [storeId, navigate])

  useEffect(() => {
    if (!storeId) return
    api
      .get<ListResponse<Store>>('/stores')
      .then((res) => setStore(res.items.find((s) => s.id === storeId) ?? null))
      .catch(() => undefined)
  }, [storeId])

  useEffect(() => {
    if (!storeId) return
    let cancelled = false
    setItems(null)

    const params = new URLSearchParams({ store_id: storeId, limit: '100' })
    if (query) params.set('q', query)
    if (cuisine) params.set('cuisine', cuisine)
    if (diet) params.set('diet', diet)
    if (sort) params.set('sort', sort)

    api
      .get<ListResponse<MenuItem>>(`/menu?${params.toString()}`)
      .then((res) => !cancelled && setItems(res.items))
      .catch((err: Error) => !cancelled && setError(err.message))
    return () => {
      cancelled = true
    }
  }, [storeId, query, cuisine, diet, sort])

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-baseline justify-between gap-2">
        <div>
          <h1 className="font-display text-2xl font-bold">{store?.name ?? copy.menu.title}</h1>
          {store && <p className="text-sm text-muted">{store.address_line}</p>}
        </div>
        <Link to="/" className="text-sm font-medium text-primary underline underline-offset-4">
          {copy.stores.change}
        </Link>
      </header>

      <SearchBox value={query} onChange={setQuery} />

      <div className="flex flex-wrap gap-2">
        <Chip active={cuisine === ''} onClick={() => setCuisine('')}>
          {copy.menu.all}
        </Chip>
        {CUISINES.map((c) => (
          <Chip key={c} active={cuisine === c} onClick={() => setCuisine(cuisine === c ? '' : c)}>
            {c === 'indonesian'
              ? 'Indonesia'
              : c === 'chinese'
                ? 'Tionghoa'
                : c === 'western'
                  ? 'Barat'
                  : 'Lainnya'}
          </Chip>
        ))}
        <span className="mx-1 w-px bg-border" aria-hidden />
        <Chip active={diet === 'halal'} onClick={() => setDiet(diet === 'halal' ? '' : 'halal')}>
          {copy.menu.halal}
        </Chip>
        <Chip
          active={diet === 'vegetarian'}
          onClick={() => setDiet(diet === 'vegetarian' ? '' : 'vegetarian')}
        >
          {copy.menu.vegetarian}
        </Chip>
        <Chip active={diet === 'no_pork'} onClick={() => setDiet(diet === 'no_pork' ? '' : 'no_pork')}>
          {copy.menu.noPork}
        </Chip>
        <Chip active={diet === 'no_nuts'} onClick={() => setDiet(diet === 'no_nuts' ? '' : 'no_nuts')}>
          {copy.menu.noNuts}
        </Chip>

        <label className="ml-auto flex items-center gap-2 text-sm text-muted">
          {copy.menu.sort}
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value)}
            className="min-h-[44px] rounded-xl border border-border bg-surface px-2 text-sm text-body"
          >
            <option value="">{copy.menu.sortName}</option>
            <option value="price_asc">{copy.menu.sortPriceAsc}</option>
            <option value="price_desc">{copy.menu.sortPriceDesc}</option>
          </select>
        </label>
      </div>

      {error && <ErrorNote>{error}</ErrorNote>}
      {!items && !error && <Spinner label={copy.common.loading} />}
      {items && items.length === 0 && <EmptyState>{copy.menu.empty}</EmptyState>}

      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(items ?? []).map((item) => (
          <li key={item.id}>
            <Link to={`/menu/${item.id}`} className="block h-full">
              <Card className="flex h-full flex-col gap-2 transition-colors hover:border-primary">
                <div className="flex items-start justify-between gap-2">
                  <h2 className="font-display text-base font-semibold">{localeName(item)}</h2>
                  <span className="tabular whitespace-nowrap text-sm font-semibold">
                    {rupiah(item.price.value)}
                  </span>
                </div>
                <p className="line-clamp-2 text-sm text-muted">{localeDesc(item)}</p>

                <div className="mt-auto flex flex-wrap gap-1.5 pt-1">
                  {item.tags.halal && <Badge>{copy.menu.halal}</Badge>}
                  {item.tags.vegetarian && <Badge>{copy.menu.vegetarian}</Badge>}
                  {item.tags.spice_level > 0 && (
                    <Badge tone="warning">
                      {copy.menu.spicy} {'🌶'.repeat(item.tags.spice_level)}
                    </Badge>
                  )}
                  {item.tags.contains_pork && <Badge>Pork</Badge>}
                  {item.tags.contains_nuts && <Badge>Nuts</Badge>}
                  {item.min_lead_minutes > 0 && (
                    <Badge tone="warning">
                      {copy.menu.leadTime.replace('{n}', String(item.min_lead_minutes))}
                    </Badge>
                  )}
                  {!item.is_available && <Badge tone="danger">{copy.menu.soldOut}</Badge>}
                  {item.is_available && item.stock_left !== null && item.stock_left <= 5 && (
                    <Badge tone="warning">
                      {copy.menu.stockLeft.replace('{n}', String(item.stock_left))}
                    </Badge>
                  )}
                </div>
              </Card>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={[
        'inline-flex min-h-[40px] items-center rounded-full border px-3 text-sm',
        active ? 'border-primary bg-primary-subtle text-primary' : 'border-border text-body',
      ].join(' ')}
    >
      {children}
    </button>
  )
}
