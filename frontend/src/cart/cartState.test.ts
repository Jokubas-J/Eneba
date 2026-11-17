import { describe, expect, it } from 'vitest'
import { cartReducer, countItems, maxLineQuantity, parseStoredCart } from './cartState.ts'

describe('cartReducer', () => {
  it('adds a new offer with quantity 1, and adding again raises it', () => {
    let lines = cartReducer([], { type: 'add', offerId: 7 })
    expect(lines).toEqual([{ offerId: 7, quantity: 1 }])
    lines = cartReducer(lines, { type: 'add', offerId: 7 })
    expect(lines).toEqual([{ offerId: 7, quantity: 2 }])
  })

  it('never goes above the stock or the per-line limit', () => {
    let lines = [{ offerId: 7, quantity: 2 }]
    lines = cartReducer(lines, { type: 'add', offerId: 7, max: 2 })
    expect(lines[0].quantity).toBe(2)

    lines = [{ offerId: 7, quantity: maxLineQuantity }]
    expect(cartReducer(lines, { type: 'add', offerId: 7, max: 99 })[0].quantity).toBe(maxLineQuantity)
  })

  it('does not add a sold-out offer', () => {
    expect(cartReducer([], { type: 'add', offerId: 7, max: 0 })).toEqual([])
  })

  it('sets a quantity, and 0 removes the line', () => {
    const lines = [
      { offerId: 1, quantity: 1 },
      { offerId: 2, quantity: 3 },
    ]
    expect(cartReducer(lines, { type: 'setQuantity', offerId: 2, quantity: 5 })[1].quantity).toBe(5)
    expect(cartReducer(lines, { type: 'setQuantity', offerId: 2, quantity: 50 })[1].quantity).toBe(maxLineQuantity)
    expect(cartReducer(lines, { type: 'setQuantity', offerId: 2, quantity: 0 })).toEqual([{ offerId: 1, quantity: 1 }])
  })

  it('removes a line and leaves the input unchanged', () => {
    const lines = [{ offerId: 1, quantity: 1 }]
    expect(cartReducer(lines, { type: 'remove', offerId: 1 })).toEqual([])
    expect(lines).toEqual([{ offerId: 1, quantity: 1 }])
  })

  it('counts items', () => {
    expect(
      countItems([
        { offerId: 1, quantity: 2 },
        { offerId: 2, quantity: 3 },
      ]),
    ).toBe(5)
  })
})

describe('parseStoredCart', () => {
  it('reads a valid cart', () => {
    expect(parseStoredCart('[{"offerId":3,"quantity":2}]')).toEqual([{ offerId: 3, quantity: 2 }])
  })

  it.each([null, '', 'not json', '{"offerId":3}', '42'])('treats %j as an empty cart', (text) => {
    expect(parseStoredCart(text)).toEqual([])
  })

  it('drops bad lines and repairs quantities', () => {
    const text = JSON.stringify([
      { offerId: 1, quantity: 1 },
      { offerId: 1, quantity: 5 }, // duplicate
      { offerId: -2, quantity: 1 },
      { offerId: 3.5, quantity: 1 },
      { offerId: 4, quantity: 0 },
      { offerId: 5, quantity: 'two' },
      { offerId: 6, quantity: 99 }, // capped
      null,
    ])
    expect(parseStoredCart(text)).toEqual([
      { offerId: 1, quantity: 1 },
      { offerId: 6, quantity: maxLineQuantity },
    ])
  })
})
