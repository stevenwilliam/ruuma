// Opening hours per weekday per mode, per-date overrides, and blackout dates —
// including today, which is the whole point of an emergency closure
// (BR-2.1.4/6/7, D27).

import { useCallback, useEffect, useState } from 'react'
import { api, type ListResponse } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { Badge, Button, Card, ErrorNote, Field, TextInput } from '../../components/ui'
import { StoreSelect, Table, todayISO, useStores } from './common'

type HoursRow = {
  Weekday: number
  Mode: string
  BlockIndex: number
  IsClosed: boolean
  OpensAt: string
  ClosesAt: string
}

type Blackout = { Date: string; Reason: string }

const DAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

export default function ScheduleAdmin() {
  const stores = useStores()
  const [storeId, setStoreId] = useState('')
  const [hours, setHours] = useState<HoursRow[]>([])
  const [blackouts, setBlackouts] = useState<Blackout[]>([])
  const [query, setQuery] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [newDate, setNewDate] = useState(todayISO())
  const [reason, setReason] = useState('')

  useEffect(() => {
    if (!storeId && stores.length > 0) setStoreId(stores[0].id)
  }, [stores, storeId])

  const load = useCallback(() => {
    if (!storeId) return
    api
      .get<ListResponse<HoursRow>>(`/admin/stores/${storeId}/hours`)
      .then((res) => setHours(res.items))
      .catch((err: Error) => setError(err.message))
    api
      .get<ListResponse<Blackout>>(`/admin/stores/${storeId}/blackouts`)
      .then((res) => setBlackouts(res.items))
      .catch(() => undefined)
  }, [storeId])

  useEffect(load, [load])

  async function save() {
    setError('')
    setNotice('')
    try {
      const res = await api.put<{ note: string }>(`/admin/stores/${storeId}/hours`, {
        hours: hours.map((h) => ({
          weekday: h.Weekday,
          fulfilment_type: h.Mode,
          block_index: h.BlockIndex,
          is_closed: h.IsClosed,
          opens_at: h.IsClosed ? '' : h.OpensAt,
          closes_at: h.IsClosed ? '' : h.ClosesAt,
        })),
      })
      setNotice(res.note)
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function addBlackout() {
    setError('')
    setNotice('')
    try {
      const res = await api.post<{ affected_orders: number; note: string }>(
        `/admin/stores/${storeId}/blackouts`,
        { business_date: newDate, reason },
      )
      setNotice(`${res.affected_orders} existing order(s) affected. ${res.note}`)
      setReason('')
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const filtered = hours.filter((h) => {
    if (!query) return true
    const needle = query.toLowerCase()
    return DAYS[h.Weekday].toLowerCase().includes(needle) || h.Mode.includes(needle)
  })

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-display text-2xl font-bold">Schedule</h1>
        <StoreSelect stores={stores} value={storeId} onChange={setStoreId} allowAll={false} />
      </div>

      {error && <ErrorNote>{error}</ErrorNote>}
      {notice && <Card className="text-sm">{notice}</Card>}

      <SearchBox value={query} onChange={setQuery} placeholder="Day or mode" />

      <Table head={['Day', 'Mode', 'Closed', 'Opens', 'Closes']}>
        {filtered.map((h, i) => (
          <tr key={`${h.Weekday}-${h.Mode}-${h.BlockIndex}`}>
            <td className="px-3 py-2">{DAYS[h.Weekday]}</td>
            <td className="px-3 py-2">
              <Badge>{h.Mode}</Badge>
            </td>
            <td className="px-3 py-2">
              <input
                type="checkbox"
                aria-label={`${DAYS[h.Weekday]} ${h.Mode} closed`}
                checked={h.IsClosed}
                onChange={(e) => {
                  const next = [...hours]
                  next[hours.indexOf(filtered[i])] = { ...h, IsClosed: e.target.checked }
                  setHours(next)
                }}
                className="h-4 w-4 accent-[var(--primary)]"
              />
            </td>
            <td className="px-3 py-2">
              <input
                type="time"
                aria-label={`${DAYS[h.Weekday]} ${h.Mode} opens`}
                disabled={h.IsClosed}
                value={(h.OpensAt || '').slice(0, 5)}
                onChange={(e) => {
                  const next = [...hours]
                  next[hours.indexOf(filtered[i])] = { ...h, OpensAt: `${e.target.value}:00` }
                  setHours(next)
                }}
                className="min-h-[40px] rounded-lg border border-border bg-surface px-2 text-sm"
              />
            </td>
            <td className="px-3 py-2">
              <input
                type="time"
                aria-label={`${DAYS[h.Weekday]} ${h.Mode} closes`}
                disabled={h.IsClosed}
                value={(h.ClosesAt || '').slice(0, 5)}
                onChange={(e) => {
                  const next = [...hours]
                  next[hours.indexOf(filtered[i])] = { ...h, ClosesAt: `${e.target.value}:00` }
                  setHours(next)
                }}
                className="min-h-[40px] rounded-lg border border-border bg-surface px-2 text-sm"
              />
            </td>
          </tr>
        ))}
      </Table>

      <Button className="self-start" onClick={save}>
        Save opening hours
      </Button>

      <Card className="flex flex-col gap-3">
        <h2 className="font-display text-lg font-semibold">Blackout dates</h2>
        <p className="text-sm text-muted">
          A blackout may be today. It stops new orders immediately and never cancels orders that are
          already booked — review those under Orders (BR-2.1.9, D27).
        </p>
        <div className="flex flex-wrap items-end gap-3">
          <Field label="Date">
            {(id) => (
              <input
                id={id}
                type="date"
                value={newDate}
                onChange={(e) => setNewDate(e.target.value)}
                className="min-h-[44px] rounded-xl border border-border bg-surface px-3 text-sm"
              />
            )}
          </Field>
          <div className="flex-1">
            <Field label="Reason">
              {(id) => <TextInput id={id} value={reason} onChange={(e) => setReason(e.target.value)} />}
            </Field>
          </div>
          <Button onClick={addBlackout} disabled={!reason}>
            Close this date
          </Button>
        </div>

        <ul className="flex flex-col gap-1 text-sm">
          {blackouts.map((b) => (
            <li key={String(b.Date)} className="flex items-center justify-between border-b border-border py-1">
              <span className="tabular">{String(b.Date).slice(0, 10)}</span>
              <span className="text-muted">{b.Reason}</span>
            </li>
          ))}
        </ul>
      </Card>
    </div>
  )
}
