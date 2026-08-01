// The cart lives in localStorage so a refresh does not lose it, but it is only
// ever a request: every price and every total is recomputed server-side
// (BR-2.5.13).

import type { MenuItem, OptionGroup } from './api'

export type CartLine = {
  key: string
  menuItemId: string
  name: string
  unitPrice: number
  optionsDelta: number
  qty: number
  notes: string
  optionChoiceIds: string[]
  optionLabels: string[]
}

export type Cart = { storeId: string; lines: CartLine[] }

const KEY = 'ruuma.cart'

export function loadCart(): Cart {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { storeId: '', lines: [] }
    return JSON.parse(raw) as Cart
  } catch {
    return { storeId: '', lines: [] }
  }
}

export function saveCart(cart: Cart) {
  localStorage.setItem(KEY, JSON.stringify(cart))
  window.dispatchEvent(new CustomEvent('ruuma:cart'))
}

export function clearCart() {
  localStorage.removeItem(KEY)
  window.dispatchEvent(new CustomEvent('ruuma:cart'))
}

export function cartCount(cart: Cart): number {
  return cart.lines.reduce((n, l) => n + l.qty, 0)
}

// estimateTotal is for display only while the customer browses. The server's
// quote is the authority and is fetched before checkout (BR-2.5.13).
export function estimateTotal(cart: Cart): number {
  return cart.lines.reduce((sum, l) => sum + (l.unitPrice + l.optionsDelta) * l.qty, 0)
}

export function lineKey(menuItemId: string, choiceIds: string[], notes: string): string {
  return [menuItemId, [...choiceIds].sort().join('+'), notes].join('|')
}

export function addLine(cart: Cart, line: CartLine): Cart {
  const existing = cart.lines.find((l) => l.key === line.key)
  if (existing) {
    existing.qty += line.qty
    return { ...cart }
  }
  return { ...cart, lines: [...cart.lines, line] }
}

export function buildLine(
  item: MenuItem,
  groups: OptionGroup[],
  choiceIds: string[],
  qty: number,
  notes: string,
  displayName: string,
): CartLine {
  let delta = 0
  const labels: string[] = []
  for (const g of groups) {
    for (const c of g.choices) {
      if (choiceIds.includes(c.id)) {
        delta += c.price_delta.value
        labels.push(c.name_id)
      }
    }
  }
  return {
    key: lineKey(item.id, choiceIds, notes),
    menuItemId: item.id,
    name: displayName,
    unitPrice: item.price.value,
    optionsDelta: delta,
    qty,
    notes,
    optionChoiceIds: choiceIds,
    optionLabels: labels,
  }
}

// selectedStore is remembered so the customer is not asked twice
// (docs/01 §3.1).
const STORE_KEY = 'ruuma.store'

export function selectedStoreId(): string {
  return localStorage.getItem(STORE_KEY) ?? ''
}

export function selectStore(id: string) {
  localStorage.setItem(STORE_KEY, id)
  window.dispatchEvent(new CustomEvent('ruuma:store'))
}
