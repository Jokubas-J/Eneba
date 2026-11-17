// Rules of the offer comparison that do not need React (unit tested).

import type { Offer } from '../api.ts'

export type OfferSort = 'price' | 'rating'

// Returns a new, sorted array (the input is not changed). In both orders the offers
// that can be bought come before the sold-out ones: a sold-out offer at the top of
// the list would be a dead end.
//   price:  cheapest first; equal prices: better seller first
//   rating: best seller first; equal ratings: cheaper first
export function sortOffers(offers: Offer[], by: OfferSort): Offer[] {
  // toSorted (ES2023) is sort() without modifying the original array.
  return offers.toSorted((a, b) => {
    const soldOut = Number(a.stock === 0) - Number(b.stock === 0)
    if (soldOut !== 0) return soldOut
    const price = a.priceCents - b.priceCents
    const rating = b.seller.rating - a.seller.rating
    if (by === 'price') return price || rating || a.id - b.id
    return rating || price || a.id - b.id
  })
}

// From this many keys down, the row says "Only N left".
export const lowStockLimit = 3

export type StockState = { label: string; tone: 'ok' | 'low' | 'none' }

export function stockState(stock: number): StockState {
  if (stock <= 0) return { label: 'Sold out', tone: 'none' }
  if (stock <= lowStockLimit) return { label: `Only ${stock} left`, tone: 'low' }
  return { label: 'In stock', tone: 'ok' }
}

// The word Eneba shows next to a seller's rating ("9.80 Superb rating").
export function ratingLabel(rating: number): string {
  if (rating >= 9.5) return 'Superb'
  if (rating >= 9) return 'Excellent'
  if (rating >= 8) return 'Very good'
  if (rating >= 7) return 'Good'
  return 'Fair'
}

// The offer shown in the buy box: the cheapest one in stock (the backend marks it).
export function bestOffer(offers: Offer[]): Offer | undefined {
  return offers.find((offer) => offer.isCheapest)
}
