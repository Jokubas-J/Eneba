import { createContext, useContext } from 'react'
import type { CartLine } from '../api.ts'
import type { CartAction } from './cartState.ts'

export type CartContextValue = {
  lines: CartLine[]
  itemCount: number
  dispatch: (action: CartAction) => void
  // Adds one key of an offer and shows the "Added to cart" dialog.
  addWithConfirmation: (offerId: number, max: number) => void
}

// A React context makes the cart available to every component below the provider
// (header count, product page buttons, cart page) without passing it through each
// level as props. Roughly a scoped singleton service in C# dependency injection.
export const CartContext = createContext<CartContextValue | null>(null)

export function useCart(): CartContextValue {
  const value = useContext(CartContext)
  if (!value) throw new Error('useCart must be used inside <CartProvider>')
  return value
}
