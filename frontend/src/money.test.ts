import { describe, expect, it } from 'vitest'
import { centsToEuros, eurosToCents, formatPrice } from './money.ts'

describe('formatPrice', () => {
  it('shows cents as euros', () => {
    expect(formatPrice(1557)).toBe('€15.57')
    expect(formatPrice(1000)).toBe('€10.00')
    expect(formatPrice(5)).toBe('€0.05')
  })
})

describe('eurosToCents', () => {
  it.each([
    ['10', 1000],
    ['9.99', 999], // 9.99 * 100 is 998.99... in floating point; must still be 999
    ['9,99', 999],
    ['0', 0],
    [' 19.9 ', 1990],
  ])('reads %s', (text, cents) => {
    expect(eurosToCents(text)).toBe(cents)
  })

  it.each(['', 'abc', '-5', '12abc', '1.234', '1e3', '.5'])('rejects "%s"', (text) => {
    expect(eurosToCents(text)).toBeNull()
  })
})

describe('centsToEuros', () => {
  it('writes whole euros without decimals', () => {
    expect(centsToEuros(1000)).toBe('10')
    expect(centsToEuros(999)).toBe('9.99')
    expect(centsToEuros(1990)).toBe('19.90')
  })

  it('round-trips with eurosToCents', () => {
    for (const cents of [0, 5, 99, 100, 999, 1990, 123456]) {
      expect(eurosToCents(centsToEuros(cents))).toBe(cents)
    }
  })
})
