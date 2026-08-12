// Item detail with option groups: required single choices and optional
// multi-select add-ons, each with its own price delta (BR-2.2.5/6).

import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type MenuItem, type OptionGroup } from '../../lib/api'
import { addLine, buildLine, loadCart, saveCart, selectedStoreId } from '../../lib/cart'
import { Badge, Button, Card, ErrorNote, Field, Spinner, TextInput } from '../../components/ui'
import { rupiah } from '../../lib/format'
import { localeDesc, localeName, t } from '../../i18n'

type ItemResponse = { item: MenuItem; option_groups: OptionGroup[] }

export default function ItemPage() {
  const copy = t()
  const { id } = useParams()
  const navigate = useNavigate()
  const storeId = selectedStoreId()

  const [data, setData] = useState<ItemResponse | null>(null)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [qty, setQty] = useState(1)
  const [notes, setNotes] = useState('')

  useEffect(() => {
    if (!storeId || !id) return
    api
      .get<ItemResponse>(`/menu/${id}?store_id=${storeId}`)
      .then((res) => {
        setData(res)
        // Pre-select the first available choice of each required group so the
        // customer never faces an invisible validation error.
        const defaults: string[] = []
        for (const g of res.option_groups) {
          if (g.is_required) {
            const first = g.choices.find((c) => c.is_available)
            if (first) defaults.push(first.id)
          }
        }
        setSelected(defaults)
      })
      .catch((err: Error) => setError(err.message))
  }, [id, storeId])

  if (error) return <ErrorNote>{error}</ErrorNote>
  if (!data) return <Spinner label={copy.common.loading} />

  const { item, option_groups: groups } = data
  const delta = groups
    .flatMap((g) => g.choices)
    .filter((c) => selected.includes(c.id))
    .reduce((sum, c) => sum + c.price_delta.value, 0)
  const lineTotal = (item.price.value + delta) * qty

  function toggle(group: OptionGroup, choiceId: string) {
    setSelected((prev) => {
      const groupChoiceIds = group.choices.map((c) => c.id)
      if (group.selection === 'single') {
        return [...prev.filter((id) => !groupChoiceIds.includes(id)), choiceId]
      }
      if (prev.includes(choiceId)) return prev.filter((id) => id !== choiceId)
      const chosenInGroup = prev.filter((id) => groupChoiceIds.includes(id))
      if (group.max_select > 0 && chosenInGroup.length >= group.max_select) return prev
      return [...prev, choiceId]
    })
  }

  function add() {
    const cart = loadCart()
    const next =
      cart.storeId && cart.storeId !== storeId
        ? { storeId, lines: [] } // switching store empties the cart (BR-2.2.1)
        : { ...cart, storeId }
    saveCart(addLine(next, buildLine(item, groups, selected, qty, notes, localeName(item))))
    navigate('/cart')
  }

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-2xl font-bold">{localeName(item)}</h1>
        <p className="text-sm text-muted">{localeDesc(item)}</p>
        <p className="tabular text-lg font-semibold">{rupiah(item.price.value)}</p>
        <div className="flex flex-wrap gap-1.5 pt-1">
          {item.tags.halal && <Badge>{copy.menu.halal}</Badge>}
          {item.tags.vegetarian && <Badge>{copy.menu.vegetarian}</Badge>}
          {item.tags.spice_level > 0 && (
            <Badge tone="warning">
              {copy.menu.spicy}{' '}
              <span aria-hidden="true">{'🌶'.repeat(item.tags.spice_level)}</span>
            </Badge>
          )}
          {item.min_lead_minutes > 0 && (
            <Badge tone="warning">
              {copy.menu.leadTime.replace('{n}', String(item.min_lead_minutes))}
            </Badge>
          )}
        </div>
      </header>

      {!item.is_available && <ErrorNote>{copy.menu.unavailable}</ErrorNote>}

      {groups.map((group) => (
        <Card key={group.id} className="flex flex-col gap-3">
          <div className="flex items-baseline justify-between">
            <h2 className="font-display text-base font-semibold">{localeName(group)}</h2>
            <span className="text-xs text-muted">
              {group.is_required ? copy.common.required : copy.common.optional}
              {group.selection === 'multi' && group.max_select > 0 && ` · max ${group.max_select}`}
            </span>
          </div>

          <fieldset className="flex flex-col gap-2">
            <legend className="sr-only">{localeName(group)}</legend>
            {group.choices.map((choice) => (
              <label
                key={choice.id}
                className={[
                  'flex min-h-[44px] items-center gap-3 rounded-xl border px-3 py-2 text-sm',
                  selected.includes(choice.id) ? 'border-primary bg-primary-subtle' : 'border-border',
                  choice.is_available ? 'cursor-pointer' : 'cursor-not-allowed opacity-60',
                ].join(' ')}
              >
                <input
                  type={group.selection === 'single' ? 'radio' : 'checkbox'}
                  name={group.id}
                  disabled={!choice.is_available}
                  checked={selected.includes(choice.id)}
                  onChange={() => toggle(group, choice.id)}
                  className="h-4 w-4 accent-[var(--primary)]"
                />
                <span className="flex-1">{localeName(choice)}</span>
                {choice.price_delta.value !== 0 && (
                  <span className="tabular text-sm text-muted">
                    {choice.price_delta.value > 0 ? '+' : ''}
                    {rupiah(choice.price_delta.value)}
                  </span>
                )}
                {!choice.is_available && <Badge tone="danger">{copy.menu.soldOut}</Badge>}
              </label>
            ))}
          </fieldset>
        </Card>
      ))}

      <Field label={copy.cart.itemNotes}>
        {(id, describedBy) => (
          <TextInput
            id={id}
            aria-describedby={describedBy}
            value={notes}
            maxLength={280}
            onChange={(e) => setNotes(e.target.value)}
          />
        )}
      </Field>

      <div className="sticky bottom-3 flex items-center gap-3 rounded-2xl border border-border bg-surface p-3">
        <div className="flex items-center gap-2">
          <Button variant="secondary" aria-label="-" onClick={() => setQty((q) => Math.max(1, q - 1))}>
            −
          </Button>
          <span className="tabular w-8 text-center text-sm font-medium">{qty}</span>
          <Button variant="secondary" aria-label="+" onClick={() => setQty((q) => Math.min(99, q + 1))}>
            +
          </Button>
        </div>
        <Button className="flex-1" disabled={!item.is_available} onClick={add}>
          {copy.common.add} · <span className="tabular">{rupiah(lineTotal)}</span>
        </Button>
      </div>
    </div>
  )
}
