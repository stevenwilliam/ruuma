// Money is integer rupiah end to end (BR-1.1.1/4); the UI only formats it.

import { describe, expect, it } from 'vitest'
import { rupiah } from '../lib/format'

describe('rupiah formatting (BR-1.1.4)', () => {
  it('groups thousands the Indonesian way', () => {
    expect(rupiah(0)).toBe('Rp 0')
    expect(rupiah(8000)).toBe('Rp 8.000')
    expect(rupiah(150000)).toBe('Rp 150.000')
    expect(rupiah(214847)).toBe('Rp 214.847')
    expect(rupiah(1234567)).toBe('Rp 1.234.567')
  })

  it('keeps the kode unik visible in the amount due', () => {
    // The last three digits are how finance matches the transfer (BR-2.6.2), so
    // formatting must never round them away.
    expect(rupiah(214500 + 347)).toBe('Rp 214.847')
  })

  it('handles negatives for refunds and discounts', () => {
    expect(rupiah(-15000)).toBe('-Rp 15.000')
  })
})
