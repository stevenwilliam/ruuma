import { beforeEach, describe, expect, it } from 'vitest'
import { addLine, cartCount, estimateTotal, lineKey, loadCart, saveCart } from '../lib/cart'

describe('cart', () => {
  beforeEach(() => localStorage.clear())

  const line = (qty: number, notes = '') => ({
    key: lineKey('item-1', ['c1'], notes),
    menuItemId: 'item-1',
    name: 'Nasi Goreng',
    unitPrice: 48000,
    optionsDelta: 8000,
    qty,
    notes,
    optionChoiceIds: ['c1'],
    optionLabels: ['Telur'],
  })

  it('merges identical lines instead of duplicating them', () => {
    let cart = loadCart()
    cart = addLine(cart, line(1))
    cart = addLine(cart, line(2))
    expect(cart.lines).toHaveLength(1)
    expect(cartCount(cart)).toBe(3)
  })

  it('keeps lines with different notes separate', () => {
    let cart = loadCart()
    cart = addLine(cart, line(1, 'no cucumber'))
    cart = addLine(cart, line(1, 'extra spicy'))
    expect(cart.lines).toHaveLength(2)
  })

  it('estimates with integer arithmetic only', () => {
    let cart = loadCart()
    cart = addLine(cart, line(2))
    // (48000 + 8000) * 2 — the server still prices the order (BR-2.5.13).
    expect(estimateTotal(cart)).toBe(112000)
    expect(Number.isInteger(estimateTotal(cart))).toBe(true)
  })

  it('survives a reload', () => {
    saveCart(addLine(loadCart(), line(1)))
    expect(cartCount(loadCart())).toBe(1)
  })
})
