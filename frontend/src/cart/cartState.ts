// The cart's state and the rules for changing it, without React (unit tested).
//
// The cart holds only offer ids and quantities. Names, prices and stock are never
// stored here: the cart page asks the backend for them every time, so a price shown
// in the cart is always the database's current price.

import type { CartLine } from '../api.ts'

// The same limit as the backend (catalog.MaxLineQuantity).
export const maxLineQuantity = 10
const maxLines = 50

export type CartAction =
  // max: the most that may be in the cart for this offer (its stock), if known.
  | { type: 'add'; offerId: number; max?: number }
  | { type: 'setQuantity'; offerId: number; quantity: number }
  | { type: 'remove'; offerId: number }
  | { type: 'replace'; lines: CartLine[] } // another browser tab changed the cart

// cartReducer returns the next state for an action. A reducer never changes the old
// state: it returns a new array, which is how React notices that something changed.
export function cartReducer(lines: CartLine[], action: CartAction): CartLine[] {
  switch (action.type) {
    case 'add': {
      const limit = Math.min(maxLineQuantity, action.max ?? maxLineQuantity)
      const existing = lines.find((line) => line.offerId === action.offerId)
      if (existing) {
        return lines.map((line) =>
          line.offerId === action.offerId ? { ...line, quantity: Math.min(line.quantity + 1, limit) } : line,
        )
      }
      if (lines.length >= maxLines || limit < 1) return lines
      return [...lines, { offerId: action.offerId, quantity: 1 }]
    }
    case 'setQuantity': {
      const quantity = Math.trunc(action.quantity)
      if (quantity < 1) return lines.filter((line) => line.offerId !== action.offerId)
      return lines.map((line) =>
        line.offerId === action.offerId ? { ...line, quantity: Math.min(quantity, maxLineQuantity) } : line,
      )
    }
    case 'remove':
      return lines.filter((line) => line.offerId !== action.offerId)
    case 'replace':
      return action.lines
  }
}

export function countItems(lines: CartLine[]): number {
  return lines.reduce((sum, line) => sum + line.quantity, 0)
}

export const storageKey = 'eneba-cart'

// parseStoredCart reads the cart from localStorage text. Anything in localStorage can
// have been edited by hand or written by an older version of the app, so every line
// is checked and bad ones are dropped instead of breaking the page.
export function parseStoredCart(text: string | null): CartLine[] {
  if (!text) return []
  let data: unknown
  try {
    data = JSON.parse(text)
  } catch {
    return []
  }
  if (!Array.isArray(data)) return []

  const lines: CartLine[] = []
  const seen = new Set<number>()
  for (const item of data) {
    const offerId = (item as CartLine)?.offerId
    const quantity = (item as CartLine)?.quantity
    if (!Number.isInteger(offerId) || offerId <= 0 || seen.has(offerId)) continue
    if (!Number.isInteger(quantity) || quantity < 1) continue
    seen.add(offerId)
    lines.push({ offerId, quantity: Math.min(quantity, maxLineQuantity) })
    if (lines.length === maxLines) break
  }
  return lines
}
