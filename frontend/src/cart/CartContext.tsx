import { type ReactNode, useEffect, useReducer, useState } from 'react'
import { AddedToCartDialog } from './AddedToCartDialog.tsx'
import { CartContext, type CartContextValue } from './useCart.ts'
import { cartReducer, countItems, parseStoredCart, storageKey } from './cartState.ts'

// CartProvider holds the cart for the whole app (see useCart.ts) and saves it in
// localStorage, so it survives reloads and works before signing in.
export function CartProvider({ children }: { children: ReactNode }) {
  // The third argument is a lazy initialiser: localStorage is read once, on the
  // first render, not on every render.
  const [lines, dispatch] = useReducer(cartReducer, null, () => parseStoredCart(localStorage.getItem(storageKey)))
  const [dialogOpen, setDialogOpen] = useState(false)

  // Save every change. An effect, because writing to localStorage is a side effect
  // and a reducer must stay a pure function.
  useEffect(() => {
    localStorage.setItem(storageKey, JSON.stringify(lines))
  }, [lines])

  // The "storage" event fires in OTHER tabs of the same site when one tab writes to
  // localStorage, so a cart changed in one tab is updated in the others.
  useEffect(() => {
    function handleStorage(event: StorageEvent) {
      if (event.key === storageKey) dispatch({ type: 'replace', lines: parseStoredCart(event.newValue) })
    }
    window.addEventListener('storage', handleStorage)
    return () => window.removeEventListener('storage', handleStorage)
  }, [])

  const value: CartContextValue = {
    lines,
    itemCount: countItems(lines),
    dispatch,
    addWithConfirmation: (offerId, max) => {
      dispatch({ type: 'add', offerId, max })
      setDialogOpen(true)
    },
  }

  return (
    <CartContext.Provider value={value}>
      {children}
      {dialogOpen && <AddedToCartDialog itemCount={value.itemCount} onClose={() => setDialogOpen(false)} />}
    </CartContext.Provider>
  )
}
