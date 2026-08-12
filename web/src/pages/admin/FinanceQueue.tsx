// The finance queue, oldest first. Verify needs an explicit reason when the
// amount differs (BR-2.6.7); reject needs a reason from the closed set
// (BR-2.6.8) and sends no automated message (D28).

import { useCallback, useEffect, useState } from 'react'
import { api, type ListResponse } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { AsyncButton, Badge, Button, Card, EmptyState, ErrorNote, Spinner } from '../../components/ui'
import { StoreSelect, Table, todayISO, useStores } from './common'
import { rupiah } from '../../lib/format'
import { t } from '../../i18n'

type QueueItem = {
  payment_id: string
  order_id: string
  order_code: string
  customer_name: string
  store_id: string
  store_name: string
  status: string
  amount_due: number
  declared_amount: number
  mismatch: number
  sender_name: string
  has_proof: boolean
  uploaded_at?: string
  age_minutes: number
}

const REJECT_REASONS = ['AMOUNT_MISMATCH', 'PROOF_UNREADABLE', 'NOT_RECEIVED', 'DUPLICATE', 'OTHER']

export default function FinanceQueue() {
  const copy = t()
  const stores = useStores()
  const [storeId, setStoreId] = useState('')
  const [status, setStatus] = useState('pending')
  const [query, setQuery] = useState('')
  const [items, setItems] = useState<QueueItem[] | null>(null)
  const [error, setError] = useState('')
  const [recon, setRecon] = useState<{ rows: Array<Record<string, number | string>> } | null>(null)

  const load = useCallback(() => {
    const params = new URLSearchParams({ status })
    if (storeId) params.set('store_id', storeId)
    if (query) params.set('q', query)
    api
      .get<ListResponse<QueueItem>>(`/finance/payments?${params}`)
      .then((res) => setItems(res.items))
      .catch((err: Error) => setError(err.message))
  }, [storeId, status, query])

  useEffect(load, [load])

  async function openProof(paymentId: string) {
    const res = await api.get<{ url: string }>(`/finance/payments/${paymentId}/proof`)
    window.open(res.url, '_blank', 'noopener')
  }

  async function verify(item: QueueItem) {
    setError('')
    let reason = ''
    if (item.mismatch !== 0) {
      // A mismatch can never pass silently (BR-2.6.7).
      const entered = window.prompt(
        `Declared ${rupiah(item.declared_amount)} vs due ${rupiah(item.amount_due)} (${rupiah(item.mismatch)}). Reason for accepting:`,
      )
      if (!entered) return
      reason = entered
    }
    try {
      await api.post(
        `/finance/payments/${item.payment_id}/verify`,
        { accept_mismatch: item.mismatch !== 0, mismatch_reason: reason },
        crypto.randomUUID(),
      )
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function reject(item: QueueItem) {
    const reason = window.prompt(`Reject reason (${REJECT_REASONS.join(' / ')}):`, 'NOT_RECEIVED')
    if (!reason) return
    const note = window.prompt('Note (optional):') ?? ''
    setError('')
    try {
      await api.post(
        `/finance/payments/${item.payment_id}/reject`,
        { reason, note },
        crypto.randomUUID(),
      )
      load()
      alert('Rejected. No automated message is sent — please contact the customer.')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function loadReconciliation() {
    const params = new URLSearchParams({ date: todayISO() })
    if (storeId) params.set('store_id', storeId)
    const res = await api.get<{ rows: Array<Record<string, number | string>> }>(
      `/finance/reconciliation?${params}`,
    )
    setRecon(res)
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-display text-2xl font-bold">Finance</h1>
        <StoreSelect stores={stores} value={storeId} onChange={setStoreId} />
        <label className="flex items-center gap-2 text-sm text-muted">
          Status
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="min-h-[44px] rounded-xl border border-border bg-surface px-2 text-sm text-body"
          >
            <option value="pending">Awaiting verification</option>
            <option value="verified">Verified</option>
            <option value="rejected">Rejected</option>
            <option value="all">All</option>
          </select>
        </label>
        <Button variant="secondary" className="ml-auto" onClick={loadReconciliation}>
          Today's reconciliation
        </Button>
      </div>

      <SearchBox value={query} onChange={setQuery} placeholder="Order code, customer or sender" />

      {error && <ErrorNote>{error}</ErrorNote>}
      {!items && <Spinner label={copy.common.loading} />}
      {items && items.length === 0 && <EmptyState>{copy.common.empty}</EmptyState>}

      {recon && (
        <Card className="flex flex-col gap-2">
          <h2 className="font-display text-lg font-semibold">Reconciliation · {todayISO()}</h2>
          <Table head={['Store', 'Orders', 'Order total', 'Kode unik', 'Declared', 'Refunds', 'Net']}>
            {recon.rows.map((r, i) => (
              <tr key={i}>
                <td className="px-3 py-2">{String(r.store_name)}</td>
                <td className="tabular px-3 py-2">{String(r.orders)}</td>
                <td className="tabular px-3 py-2">{rupiah(Number(r.order_total))}</td>
                <td className="tabular px-3 py-2">{rupiah(Number(r.unique_codes))}</td>
                <td className="tabular px-3 py-2">{rupiah(Number(r.declared))}</td>
                <td className="tabular px-3 py-2">{rupiah(Number(r.refunds))}</td>
                <td className="tabular px-3 py-2 font-semibold">{rupiah(Number(r.net))}</td>
              </tr>
            ))}
          </Table>
        </Card>
      )}

      {items && items.length > 0 && (
        <Table head={['Order', 'Store', 'Customer', 'Due', 'Declared', 'Age', 'Proof', '']}>
          {items.map((item) => (
            <tr key={item.payment_id}>
              <td className="px-3 py-2 font-medium tracking-wide">{item.order_code}</td>
              <td className="px-3 py-2 text-muted">{item.store_name}</td>
              <td className="px-3 py-2">
                {item.customer_name}
                {item.sender_name && <span className="text-muted"> · {item.sender_name}</span>}
              </td>
              <td className="tabular px-3 py-2">{rupiah(item.amount_due)}</td>
              <td className="tabular px-3 py-2">
                {rupiah(item.declared_amount)}
                {item.mismatch !== 0 && (
                  <Badge tone="warning">
                    {item.mismatch > 0 ? '+' : ''}
                    {rupiah(item.mismatch)}
                  </Badge>
                )}
              </td>
              <td className="tabular px-3 py-2">{item.age_minutes}m</td>
              <td className="px-3 py-2">
                {item.has_proof ? (
                  <Button variant="ghost" onClick={() => openProof(item.payment_id)}>
                    View
                  </Button>
                ) : (
                  <span className="text-muted">—</span>
                )}
              </td>
              <td className="px-3 py-2">
                {item.status === 'SUBMITTED' && (
                  <div className="flex gap-2">
                    {/* Verifying is a money decision recorded immutably in
                        payment_events (D26), and for a matching amount there
                        was previously nothing between one click and the write.
                        A mismatch already prompts for a reason, which is its
                        own confirmation, so only the clean case needs this. */}
                    <AsyncButton
                      onRun={() => verify(item)}
                      busyLabel="Verifying…"
                      confirm={
                        item.mismatch === 0
                          ? `Verify ${rupiah(item.amount_due)} received for ${item.order_code}? This is recorded permanently.`
                          : undefined
                      }
                    >
                      Verify
                    </AsyncButton>
                    <AsyncButton variant="secondary" busyLabel="Rejecting…" onRun={() => reject(item)}>
                      Reject
                    </AsyncButton>
                  </div>
                )}
              </td>
            </tr>
          ))}
        </Table>
      )}
    </div>
  )
}
