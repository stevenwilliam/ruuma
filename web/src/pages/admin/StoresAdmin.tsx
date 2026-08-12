import { useEffect, useState } from 'react'
import { api, type ListResponse, type Store } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { AsyncButton, Badge, Card, EmptyState, ErrorNote } from '../../components/ui'
import { Table } from './common'

export default function StoresAdmin() {
  const [stores, setStores] = useState<Store[] | null>(null)
  const [query, setQuery] = useState('')
  const [error, setError] = useState('')

  function load() {
    api
      .get<ListResponse<Store>>(`/admin/stores${query ? `?q=${encodeURIComponent(query)}` : ''}`)
      .then((res) => setStores(res.items))
      .catch((err: Error) => setError(err.message))
  }
  useEffect(load, [query])

  async function toggle(store: Store) {
    setError('')
    try {
      await api.post(`/admin/stores/${store.id}/${store.is_active ? 'deactivate' : 'activate'}`)
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-display text-2xl font-bold">Stores</h1>
      <SearchBox value={query} onChange={setQuery} placeholder="Name, code or address" />
      {error && <ErrorNote>{error}</ErrorNote>}
      {stores && stores.length === 0 && <EmptyState>No stores.</EmptyState>}

      {stores && stores.length > 0 && (
        <Table head={['Code', 'Name', 'City', 'Modes', 'Active', '']}>
          {stores.map((s) => (
            <tr key={s.id}>
              <td className="px-3 py-2 font-medium">{s.code}</td>
              <td className="px-3 py-2">{s.name}</td>
              <td className="px-3 py-2 text-muted">{s.city}</td>
              <td className="px-3 py-2">
                <div className="flex gap-1">
                  {s.fulfilment_modes?.map((m) => (
                    <Badge key={m}>{m}</Badge>
                  ))}
                </div>
              </td>
              <td className="px-3 py-2">
                {s.is_active ? <Badge tone="primary">active</Badge> : <Badge tone="warning">hidden</Badge>}
              </td>
              <td className="px-3 py-2">
                {/* Deactivating hides the store but never touches its orders
                    (BR-2.1.11). */}
                <AsyncButton
                  variant="secondary"
                  busyLabel="Saving…"
                  confirm={
                    s.is_active
                      ? `Deactivate ${s.name}? It disappears from the customer site at once; existing orders are untouched.`
                      : undefined
                  }
                  onRun={() => toggle(s)}
                >
                  {s.is_active ? 'Deactivate' : 'Activate'}
                </AsyncButton>
              </td>
            </tr>
          ))}
        </Table>
      )}

      <Card className="text-sm text-muted">
        Deactivating a store hides it from customers immediately. Historical orders are never
        deleted or reassigned (BR-2.1.11).
      </Card>
    </div>
  )
}
