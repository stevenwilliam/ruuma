// Everything configurable lives here (BR-1.4.1/2). A change takes effect
// without a deploy and is audited before/after (BR-2.9.2, BR-2.10.1).

import { useCallback, useEffect, useState } from 'react'
import { api, type ListResponse } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { Badge, Button, Card, ErrorNote, Spinner } from '../../components/ui'
import { Table } from './common'
import { t } from '../../i18n'

type ParamRow = {
  Key: string
  Value: string
  DataType: string
  Description: string
  IsSecret: boolean
  IsStoreOverridable: boolean
  Source: string
}

export default function ParametersAdmin() {
  const copy = t()
  const [rows, setRows] = useState<ParamRow[] | null>(null)
  const [query, setQuery] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const load = useCallback(() => {
    api
      .get<ListResponse<ParamRow>>(`/admin/sys-parameters${query ? `?q=${encodeURIComponent(query)}` : ''}`)
      .then((res) => setRows(res.items))
      .catch((err: Error) => setError(err.message))
  }, [query])

  useEffect(load, [load])

  async function edit(row: ParamRow) {
    const value = window.prompt(`${row.Key}\n${row.Description}`, row.Value)
    if (value === null) return
    setError('')
    try {
      const res = await api.put<{ applies: string }>('/admin/sys-parameters', {
        key: row.Key,
        value,
      })
      setNotice(`${row.Key} updated — applies ${res.applies}.`)
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-display text-2xl font-bold">Parameters</h1>
      <Card className="text-sm text-muted">
        Slot length, capacity, lead time, cutoffs, tax and service-charge rates, notification
        templates and feature switches all live here. Changes apply immediately — no deploy
        (BR-1.4.1, BR-2.9.2).
      </Card>

      <SearchBox value={query} onChange={setQuery} placeholder="Key or description" />
      {error && <ErrorNote>{error}</ErrorNote>}
      {notice && <Card className="text-sm">{notice}</Card>}
      {!rows && <Spinner label={copy.common.loading} />}

      {rows && (
        <Table head={['Key', 'Value', 'Description', 'Scope', '']}>
          {rows.map((row) => (
            <tr key={row.Key}>
              <td className="px-3 py-2 font-mono text-xs">{row.Key}</td>
              <td className="tabular px-3 py-2 font-medium">
                {row.IsSecret ? '••••••••' : row.Value}
              </td>
              <td className="px-3 py-2 text-muted">{row.Description}</td>
              <td className="px-3 py-2">
                {row.IsStoreOverridable ? <Badge tone="primary">per store</Badge> : <Badge>group</Badge>}
              </td>
              <td className="px-3 py-2">
                <Button variant="secondary" onClick={() => edit(row)}>
                  Edit
                </Button>
              </td>
            </tr>
          ))}
        </Table>
      )}
    </div>
  )
}
