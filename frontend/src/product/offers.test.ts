import { describe, expect, it } from 'vitest'
import type { Offer } from '../api.ts'
import { bestOffer, ratingLabel, sortOffers, stockState } from './offers.ts'

function offer(id: number, priceCents: number, rating: number, stock: number, isCheapest = false): Offer {
  return {
    id,
    seller: { name: `Seller ${id}`, rating, ratingCount: 10, isVerified: false },
    priceCents,
    originalPriceCents: 2000,
    discountPercent: 0,
    stock,
    isCheapest,
  }
}

const offers = [
  offer(1, 900, 9.9, 0), // cheapest, but sold out
  offer(2, 1200, 9.0, 3),
  offer(3, 1000, 9.5, 1, true),
  offer(4, 1000, 8.0, 5),
  offer(5, 1500, 9.8, 2),
]
const ids = (list: Offer[]) => list.map((o) => o.id)

describe('sortOffers', () => {
  it('by price: in stock first, cheapest first, better seller on equal price', () => {
    expect(ids(sortOffers(offers, 'price'))).toEqual([3, 4, 2, 5, 1])
  })

  it('by rating: in stock first, best seller first', () => {
    expect(ids(sortOffers(offers, 'rating'))).toEqual([5, 3, 2, 4, 1])
  })

  it('does not change the original list', () => {
    sortOffers(offers, 'rating')
    expect(ids(offers)).toEqual([1, 2, 3, 4, 5])
  })
})

describe('stockState', () => {
  it.each([
    [0, 'Sold out', 'none'],
    [1, 'Only 1 left', 'low'],
    [3, 'Only 3 left', 'low'],
    [4, 'In stock', 'ok'],
  ])('%i keys -> %s', (stock, label, tone) => {
    expect(stockState(stock)).toEqual({ label, tone })
  })
})

it('labels seller ratings', () => {
  expect([9.8, 9.5, 9.2, 8.4, 7.1, 6.0].map(ratingLabel)).toEqual([
    'Superb',
    'Superb',
    'Excellent',
    'Very good',
    'Good',
    'Fair',
  ])
})

it('the buy box shows the offer the backend marked as cheapest', () => {
  expect(bestOffer(offers)?.id).toBe(3)
  expect(bestOffer([offer(1, 900, 9, 0)])).toBeUndefined()
})
