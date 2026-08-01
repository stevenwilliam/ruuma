// The audit log is append-only in the database itself (BR-2.10.2); this screen
// only reads it.

import { useCallback, useEffect, useState } from 'react'
import { api, type ListResponse } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { Badge, ErrorNote, Spinner } from '../../components/ui'
import { StoreSelect, Table, useStores } from './common'
import { t } from '../../i18n'

type AuditRow = {
  ID: string
  ActorEmail: string
  ActorType: string
  Action: string
  EntityType: string
  StoreID?: string
  CreatedAt: string
}

export default function AuditAdmin() {
  const copy = t()
  const stores = useStores()
  const [rows, setRows] = useState<AuditRow[] | null>(null)
  const [query, setQuery] = useState('')
  const [storeId, setStoreId] = useState('')
  const [error, setError] = useState('')

  const load = useCallback(() => {
    const params = new URLSearchParams({ limit: '200' })
    if (query) params.set('q', query)
    if (storeId) params.set('store_id', storeId)
    api
      .get<ListResponse<AuditRow>>(`/admin/audit-log?${params}`)
      .then((res) => setRows(res.items))
      .catch((err: Error) => setError(err.message))
  }, [query, storeId])

  useEffect(load, [load])

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-display text-2xl font-bold">Audit log</h1>
        <StoreSelect stores={stores} value={storeId} onChange={setStoreId} />
      </div>

      <SearchBox value={query} onChange={setQuery} placeholder="Action, entity or actor" />
      {error && <ErrorNote>{error}</ErrorNote>}
      {!rows && <Spinner label={copy.common.loading} />}

      {rows && (
        <Table head={['When', 'Actor', 'Action', 'Entity', 'Store']}>
          {rows.map((row) => (
            <tr key={row.ID}>
              <td className="tabular px-3 py-2 text-muted">
                {new Date(row.CreatedAt).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta' })}
              </td>
              <td className="px-3 py-2">
                {row.ActorEmail || row.ActorType}
              </td>
              <td className="px-3 py-2">
                <Badge>{row.Action}</Badge>
              </td>
              <td className="px-3 py-2 text-muted">{row.EntityType}</td>
              <td className="px-3 py-2 text-muted">
                {stores.find((s) => s.id === row.StoreID)?.code ?? '—'}
              </td>
            </tr>
          ))}
        </Table>
      )}
    </div>
  )
}
