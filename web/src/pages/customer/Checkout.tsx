// Checkout in the order the customer thinks: store → method → date → slot →
// contact → promo → payment (docs/01 §3.1). Every amount comes from the
// server's quote; the client's figure is only ever compared (BR-2.5.13).

import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  api,
  ApiError,
  newIdempotencyKey,
  type Customer,
  type DateAvailability,
  type ListResponse,
  type Order,
  type Quote,
  type Slot,
  type Store,
} from '../../lib/api'
import { clearCart, loadCart, selectedStoreId } from '../../lib/cart'
import { DatePicker } from '../../components/DatePicker'
import { SlotPicker } from '../../components/SlotPicker'
import { Button, Card, ErrorNote, Field, Spinner, TextInput } from '../../components/ui'
import { rupiah } from '../../lib/format'
import { t } from '../../i18n'

export default function CheckoutPage() {
  const copy = t()
  const navigate = useNavigate()
  const storeId = selectedStoreId()
  const cart = loadCart()

  const [store, setStore] = useState<Store | null>(null)
  const [me, setMe] = useState<Customer | null>(null)
  const [dates, setDates] = useState<DateAvailability[]>([])
  const [date, setDate] = useState('')
  const [slots, setSlots] = useState<Slot[] | null>(null)
  const [slot, setSlot] = useState<Slot | null>(null)
  const [quote, setQuote] = useState<Quote | null>(null)
  const [promo, setPromo] = useState('')
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!storeId || cart.lines.length === 0) navigate('/menu')
  }, [storeId, cart.lines.length, navigate])

  useEffect(() => {
    api.get<ListResponse<Store>>('/stores').then((res) => {
      setStore(res.items.find((s) => s.id === storeId) ?? null)
    })
    api
      .get<Customer>('/me')
      .then((c) => {
        setMe(c)
        setName((n) => n || c.full_name)
        setPhone((p) => p || c.phone)
      })
      .catch(() => navigate('/signin?next=/checkout'))
  }, [storeId, navigate])

  useEffect(() => {
    if (!storeId) return
    api
      .get<ListResponse<DateAvailability>>(
        `/availability/dates?store_id=${storeId}&type=pickup&days=14`,
      )
      .then((res) => {
        setDates(res.items)
        const first = res.items.find((d) => d.is_bookable)
        if (first) setDate((current) => current || first.date)
      })
      .catch((err: Error) => setError(err.message))
  }, [storeId])

  useEffect(() => {
    if (!storeId || !date) return
    setSlots(null)
    setSlot(null)
    const items = cart.lines.map((l) => `items=${encodeURIComponent(l.menuItemId)}`).join('&')
    const units = cart.lines.reduce((n, l) => n + l.qty, 0)
    api
      .get<ListResponse<Slot>>(
        `/availability/slots?store_id=${storeId}&date=${date}&type=pickup&units=${units}${items ? `&${items}` : ''}`,
      )
      .then((res) => setSlots(res.items))
      .catch((err: ApiError) => {
        setSlots([])
        setError(err.message)
      })
    // cart is read fresh on each render; the dependency list is deliberate.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storeId, date])

  // The quote is refreshed whenever anything that can change a price changes.
  useEffect(() => {
    if (!storeId || cart.lines.length === 0) return
    api
      .post<Quote>('/cart/quote', {
        store_id: storeId,
        fulfilment_type: 'pickup',
        slot_id: slot?.slot_id,
        promo_code: promo || undefined,
        lines: cart.lines.map((l) => ({
          menu_item_id: l.menuItemId,
          qty: l.qty,
          notes: l.notes,
          option_choice_ids: l.optionChoiceIds,
        })),
      })
      .then(setQuote)
      .catch((err: ApiError) => {
        setQuote(null)
        setError(err.message)
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storeId, slot?.slot_id, promo])

  async function placeOrder() {
    if (!slot || !quote) return
    setBusy(true)
    setError('')
    try {
      const order = await api.post<Order>(
        '/orders',
        {
          store_id: storeId,
          slot_id: slot.slot_id,
          fulfilment_type: 'pickup',
          contact_name: name,
          contact_phone: phone,
          promo_code: promo || undefined,
          // The server recomputes and refuses a mismatch (BR-2.5.13).
          expected_total: quote.total,
          lines: cart.lines.map((l) => ({
            menu_item_id: l.menuItemId,
            qty: l.qty,
            notes: l.notes,
            option_choice_ids: l.optionChoiceIds,
          })),
        },
        newIdempotencyKey(),
      )
      clearCart()
      navigate(`/orders/${order.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const canOrder = me?.can_order ?? false

  return (
    <div className="flex flex-col gap-5">
      <h1 className="font-display text-2xl font-bold">{copy.checkout.title}</h1>

      {store && (
        <Card className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-wide text-muted">{copy.checkout.steps.store}</p>
            <p className="font-medium">{store.name}</p>
            <p className="text-sm text-muted">{store.address_line}</p>
          </div>
          <Button variant="ghost" onClick={() => navigate('/')}>
            {copy.stores.change}
          </Button>
        </Card>
      )}

      <section className="flex flex-col gap-2">
        <h2 className="font-display text-lg font-semibold">{copy.checkout.pickDate}</h2>
        <DatePicker dates={dates} selected={date} onSelect={setDate} />
      </section>

      <section className="flex flex-col gap-2">
        <h2 className="font-display text-lg font-semibold">{copy.checkout.pickSlot}</h2>
        {!slots && <Spinner label={copy.common.loading} />}
        {slots && (
          <SlotPicker
            slots={slots}
            selected={slot?.slot_id}
            onSelect={setSlot}
            timeZone={store?.timezone}
          />
        )}
      </section>

      <section className="grid gap-3 sm:grid-cols-2">
        <Field label={copy.checkout.contactName}>
          {(id) => <TextInput id={id} value={name} onChange={(e) => setName(e.target.value)} />}
        </Field>
        <Field label={copy.checkout.contactPhone}>
          {(id) => (
            <TextInput id={id} value={phone} inputMode="tel" onChange={(e) => setPhone(e.target.value)} />
          )}
        </Field>
      </section>

      <section className="flex items-end gap-2">
        <div className="flex-1">
          <Field label={copy.checkout.promoCode}>
            {(id) => (
              <TextInput
                id={id}
                value={promo}
                onChange={(e) => setPromo(e.target.value.toUpperCase())}
              />
            )}
          </Field>
        </div>
      </section>

      {quote && (
        <Card className="flex flex-col gap-1 text-sm">
          <Row label={copy.common.subtotal} value={quote.subtotal} />
          {quote.discount > 0 && <Row label={copy.common.discount} value={-quote.discount} />}
          {quote.service_charge > 0 && (
            <Row label={copy.common.serviceCharge} value={quote.service_charge} />
          )}
          <Row label={`${copy.common.tax} ${quote.tax_bps / 100}%`} value={quote.tax} />
          <div className="mt-1 flex items-center justify-between border-t border-border pt-2 text-base font-semibold">
            <span>{copy.common.total}</span>
            <span className="tabular">{rupiah(quote.total)}</span>
          </div>
        </Card>
      )}

      {error && <ErrorNote>{error}</ErrorNote>}
      {me && !canOrder && <ErrorNote>{copy.checkout.verifyPhoneFirst}</ErrorNote>}

      <Button disabled={!slot || !quote || busy || !canOrder} onClick={placeOrder}>
        {busy ? copy.common.loading : copy.checkout.placeOrder}
      </Button>
    </div>
  )
}

function Row({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-muted">{label}</span>
      <span className="tabular">{rupiah(value)}</span>
    </div>
  )
}
