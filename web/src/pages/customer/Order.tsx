// Order page: state machine, the exact transfer amount with its kode unik, the
// proof upload, and — because no automated message is sent on rejection (D28) —
// the rejection reason shown prominently.

import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, newIdempotencyKey, type Order } from '../../lib/api'
import { Badge, Button, Card, ErrorNote, Field, Spinner, TextInput } from '../../components/ui'
import { longDate, rupiah, slotLabel } from '../../lib/format'
import { t } from '../../i18n'
import { useSeo, useNoIndex } from '../../lib/seo'

export default function OrderPage() {
  const copy = t()
  const { id } = useParams()
  const [order, setOrder] = useState<Order | null>(null)

  useSeo(order ? `${copy.order.code} ${order.order_code}` : '')
  useNoIndex()
  const [error, setError] = useState('')
  const [declared, setDeclared] = useState('')
  const [sender, setSender] = useState('')
  const [busy, setBusy] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const load = useCallback(() => {
    if (!id) return
    api
      .get<Order>(`/orders/${id}`)
      .then((o) => {
        setOrder(o)
        setDeclared((d) => d || String(o.amount_due))
      })
      .catch((err: Error) => setError(err.message))
  }, [id])

  useEffect(load, [load])

  async function upload() {
    if (!id || !fileRef.current?.files?.[0]) return
    setBusy(true)
    setError('')
    try {
      const form = new FormData()
      form.append('proof', fileRef.current.files[0])
      form.append('declared_amount', declared)
      form.append('sender_name', sender)
      await api.upload(`/orders/${id}/payment-proof`, form, newIdempotencyKey())
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  if (error && !order) return <ErrorNote>{error}</ErrorNote>
  if (!order) return <Spinner label={copy.common.loading} />

  const payment = order.payment
  const awaitingPayment = order.status === 'PENDING_PAYMENT'
  const rejected = payment?.status === 'REJECTED'

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-col gap-1">
        <p className="text-xs uppercase tracking-wide text-muted">{copy.order.code}</p>
        <h1 className="font-display text-3xl font-bold tracking-wide">{order.order_code}</h1>
        <div className="flex flex-wrap items-center gap-2 pt-1">
          <Badge tone={statusTone(order.status)}>
            {copy.order.status[order.status] ?? order.status}
          </Badge>
          <span className="text-sm text-muted">
            {copy.order.pickupAt} {order.store.name}
          </span>
        </div>
      </header>

      <Card className="flex flex-col gap-1">
        <p className="text-sm text-muted">{longDate(order.slot.starts_at)}</p>
        <p className="tabular text-lg font-semibold">
          {slotLabel(order.slot.starts_at, order.slot.ends_at)}
        </p>
      </Card>

      {rejected && (
        <ErrorNote>
          <strong>{copy.payment.rejected}</strong>
          {payment?.rejection_reason && (
            <>
              {' — '}
              {copy.payment.rejectedReasons[payment.rejection_reason] ?? payment.rejection_reason}
            </>
          )}
          {payment?.rejection_note && <p className="mt-1">{payment.rejection_note}</p>}
        </ErrorNote>
      )}

      {(awaitingPayment || rejected) && order.bank_account && (
        <Card className="flex flex-col gap-3">
          <h2 className="font-display text-lg font-semibold">{copy.payment.title}</h2>
          <p className="text-sm text-muted">{copy.payment.instructions}</p>

          <div className="rounded-xl bg-primary-subtle p-3">
            <p className="text-xs uppercase tracking-wide text-muted">{copy.payment.exactAmount}</p>
            <p className="tabular text-2xl font-bold text-primary-ink">{rupiah(order.amount_due)}</p>
            <p className="pt-1 text-xs text-muted">
              {copy.payment.uniqueCodeNote.replace('{code}', String(order.unique_code))}
            </p>
          </div>

          <dl className="grid grid-cols-[auto,1fr] gap-x-4 gap-y-1 text-sm">
            <dt className="text-muted">{copy.payment.bank}</dt>
            <dd>{order.bank_account.bank_name}</dd>
            <dt className="text-muted">{copy.payment.accountName}</dt>
            <dd>{order.bank_account.account_name}</dd>
            <dt className="text-muted">{copy.payment.accountNumber}</dt>
            <dd className="tabular">{order.bank_account.account_number}</dd>
          </dl>

          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={copy.payment.declaredAmount}>
              {(id) => (
                <TextInput
                  id={id}
                  inputMode="numeric"
                  value={declared}
                  onChange={(e) => setDeclared(e.target.value.replace(/\D/g, ''))}
                />
              )}
            </Field>
            <Field label={copy.payment.senderName}>
              {(id) => <TextInput id={id} value={sender} onChange={(e) => setSender(e.target.value)} />}
            </Field>
          </div>

          <Field label={copy.payment.uploadProof} hint="JPEG, PNG, WebP or PDF · max 5 MB">
            {(id) => (
              <input
                id={id}
                ref={fileRef}
                type="file"
                accept="image/jpeg,image/png,image/webp,application/pdf"
                className="text-sm"
              />
            )}
          </Field>

          <Button onClick={upload} disabled={busy}>
            {busy ? copy.common.loading : rejected ? copy.payment.reupload : copy.payment.uploadProof}
          </Button>
        </Card>
      )}

      {payment?.status === 'SUBMITTED' && (
        <Card>
          <p className="text-sm">{copy.payment.awaiting}</p>
        </Card>
      )}
      {payment?.status === 'VERIFIED' && (
        <Card>
          <p className="text-sm font-medium text-primary-ink">{copy.payment.verified}</p>
        </Card>
      )}

      <section className="flex flex-col gap-2">
        <h2 className="font-display text-lg font-semibold">{copy.order.title}</h2>
        <ul className="flex flex-col gap-2">
          {order.lines.map((line) => (
            <li key={line.id}>
              <Card className="flex items-start justify-between gap-3">
                <div>
                  <p className="font-medium">
                    {line.qty} × {line.name_id}
                  </p>
                  {line.options.length > 0 && (
                    <p className="text-sm text-muted">
                      {line.options.map((o) => o.choice_id).join(' · ')}
                    </p>
                  )}
                  {line.notes && <p className="text-sm italic text-muted">“{line.notes}”</p>}
                </div>
                <span className="tabular text-sm">{rupiah(line.line_total.value)}</span>
              </Card>
            </li>
          ))}
        </ul>
      </section>

      <Card className="flex flex-col gap-1 text-sm">
        <div className="flex justify-between">
          <span className="text-muted">{copy.common.subtotal}</span>
          <span className="tabular">{rupiah(order.subtotal)}</span>
        </div>
        {order.discount > 0 && (
          <div className="flex justify-between">
            <span className="text-muted">{copy.common.discount}</span>
            <span className="tabular">-{rupiah(order.discount)}</span>
          </div>
        )}
        <div className="flex justify-between">
          <span className="text-muted">{copy.common.tax}</span>
          <span className="tabular">{rupiah(order.tax)}</span>
        </div>
        <div className="flex justify-between border-t border-border pt-2 text-base font-semibold">
          <span>{copy.common.total}</span>
          <span className="tabular">{rupiah(order.total)}</span>
        </div>
      </Card>

      {order.history && order.history.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="font-display text-lg font-semibold">{copy.order.track}</h2>
          <ol className="flex flex-col gap-1 text-sm">
            {order.history.map((e, i) => (
              <li key={i} className="flex gap-3">
                <span className="tabular text-muted">
                  {new Date(e.at).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta' })}
                </span>
                <span>{copy.order.status[e.to] ?? e.to}</span>
              </li>
            ))}
          </ol>
        </section>
      )}
    </div>
  )
}

function statusTone(status: string): 'neutral' | 'primary' | 'warning' | 'danger' {
  if (status === 'CANCELLED' || status === 'REFUNDED') return 'danger'
  if (status === 'PENDING_PAYMENT' || status === 'AWAITING_VERIFICATION') return 'warning'
  if (status === 'READY' || status === 'COMPLETED' || status === 'PICKED_UP') return 'primary'
  return 'neutral'
}
