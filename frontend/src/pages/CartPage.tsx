import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { type CartItem, validateCart } from '../api.ts'
import { useCurrentUser } from '../auth/useAuth.ts'
import { useCart } from '../cart/useCart.ts'
import { maxLineQuantity } from '../cart/cartState.ts'
import { Button, ButtonLink } from '../components/Button.tsx'
import { CloseIcon, InfoIcon } from '../components/icons.tsx'
import { Page } from '../components/Page.tsx'
import { RemoteImage } from '../components/RemoteImage.tsx'
import { formatPrice } from '../money.ts'

// The cart (design-reference.md §5, screenshots 14 and 15): "My cart" with one row
// per offer on the left, the summary with the total on the right.
//
// Everything shown about a line (name, seller, price, stock) comes from
// POST /api/cart/validate; the browser only knows offer ids and quantities.
export function CartPage() {
  const { lines, dispatch } = useCart()
  const me = useCurrentUser()

  const cart = useQuery({
    // The lines are part of the key: any change (quantity, removal) asks again.
    queryKey: ['cart', lines],
    queryFn: () => validateCart(lines),
    enabled: lines.length > 0,
    // Keep the old answer on screen while a changed cart is checked (no flicker
    // when pressing + or −).
    placeholderData: keepPreviousData,
  })

  if (lines.length === 0) {
    return (
      <Page>
        <div className="flex flex-col items-center py-8 text-center">
          <InfoIcon className="size-8 text-text-nav" />
          <h1 className="mt-4 text-xl font-extrabold">Your cart is empty</h1>
          <p className="mt-1 text-xs text-text-muted">It seems you haven't chosen your products yet.</p>
          <ButtonLink to="/" className="mt-4">
            Back to Homepage
          </ButtonLink>
        </div>
      </Page>
    )
  }

  const data = cart.data
  const hasProblems = data?.items.some((item) => item.problem !== '') ?? false

  return (
    <Page>
      {/* minmax(0, 1fr): a grid column is otherwise at least as wide as its content's
          minimum, which pushed the cart wider than a phone screen. */}
      <div className="grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-[minmax(0,1fr)_380px] lg:items-start">
        <section aria-labelledby="cart-heading" className="bg-purple-dark p-4">
          <h1 id="cart-heading" className="mb-4 text-lg font-extrabold">
            My cart
          </h1>
          {cart.isError ? (
            <div className="p-4 text-center">
              <p>The cart could not be loaded.</p>
              <Button className="mt-3" onClick={() => cart.refetch()}>
                Try again
              </Button>
            </div>
          ) : !data ? (
            <div className="h-[120px] animate-pulse bg-overlay-item" aria-hidden="true" />
          ) : (
            <ul className={`flex flex-col gap-2 ${cart.isPlaceholderData ? 'opacity-60' : ''}`}>
              {data.items.map((item) => (
                <CartRow
                  key={item.offerId}
                  item={item}
                  onQuantity={(quantity) => dispatch({ type: 'setQuantity', offerId: item.offerId, quantity })}
                  onRemove={() => dispatch({ type: 'remove', offerId: item.offerId })}
                />
              ))}
            </ul>
          )}
        </section>

        <aside aria-labelledby="summary-heading" className="bg-purple-dark p-4 lg:sticky lg:top-4">
          <h2 id="summary-heading" className="text-lg font-extrabold">
            Summary
          </h2>
          {me.isPending ? (
            // Not known yet whether someone is signed in: no button rather than the
            // wrong one for a moment.
            <div className="mt-4 h-7" aria-hidden="true" />
          ) : me.data ? (
            <Button className="mt-4 w-full" disabled title="Payment is added in the next build step.">
              Proceed to checkout
            </Button>
          ) : (
            <ButtonLink to="/login?next=%2Fcart" className="mt-4 w-full">
              Log in to check out
            </ButtonLink>
          )}
          <dl className="mt-4 flex justify-between text-sm font-semibold">
            <dt>{data ? `${data.itemCount} ${data.itemCount === 1 ? 'product' : 'products'}` : '…'}</dt>
            <dd>{data ? formatPrice(data.totalCents) : '…'}</dd>
          </dl>
          <dl className="mt-4 flex items-baseline justify-between border-t border-overlay-row pt-4">
            <dt className="font-extrabold">Total:</dt>
            <dd className="text-buybox font-extrabold">{data ? formatPrice(data.totalCents) : '…'}</dd>
          </dl>
          {hasProblems && (
            <p className="mt-3 text-xs text-pink-region">
              Lines marked in pink cannot be bought as they are and are not in the total.
            </p>
          )}
          <p className="mt-3 text-xs text-text-muted">
            Prices and stock are checked again when you pay. Keys are delivered on your order page.
          </p>
        </aside>
      </div>
    </Page>
  )
}

const problemText = {
  '': '',
  not_found: 'This offer is no longer available.',
  sold_out: 'Sold out.',
  not_enough_stock: 'Not enough keys in stock for this quantity.',
} as const

type CartRowProps = {
  item: CartItem
  onQuantity: (quantity: number) => void
  onRemove: () => void
}

function CartRow({ item, onQuantity, onRemove }: CartRowProps) {
  // An offer that no longer exists has no product data to show.
  if (item.problem === 'not_found') {
    return (
      <li className="flex items-center justify-between gap-4 bg-overlay-item p-4 text-sm">
        <span className="text-pink-region">{problemText.not_found}</span>
        <RemoveButton onRemove={onRemove} label="Remove unavailable offer" />
      </li>
    )
  }

  // The most the stepper allows: the stock, and never more than the per-line limit.
  const max = Math.min(item.stock, maxLineQuantity)

  return (
    <li className="flex gap-4 bg-overlay-item p-4">
      <Link to={`/product/${item.product.slug}`} className="h-[84px] w-[60px] shrink-0 bg-purple-deep sm:h-[106px] sm:w-[75px]">
        <RemoteImage src={item.product.coverUrl} className="size-full object-cover" />
      </Link>
      <div className="min-w-0 flex-1">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="text-xs text-text-muted">
              Sold by <span className="font-semibold text-white">{item.seller.name}</span>
            </p>
            <Link to={`/product/${item.product.slug}`} className="mt-1 block text-sm font-extrabold hover:underline">
              {item.product.name}
            </Link>
          </div>
          <RemoveButton onRemove={onRemove} label={`Remove ${item.product.name}`} />
        </div>

        <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
          <span className="text-xs text-text-muted">Digital product</span>
          <div className="flex flex-wrap items-center gap-4">
            {/* Quantity stepper: − n + */}
            <div className="flex items-center gap-2" role="group" aria-label="Quantity">
              <button
                type="button"
                onClick={() => onQuantity(item.quantity - 1)}
                aria-label="One less"
                className="size-7 text-lg font-extrabold hover:text-yellow"
              >
                −
              </button>
              <span aria-live="polite" className="w-5 text-center font-extrabold">
                {item.quantity}
              </span>
              <button
                type="button"
                onClick={() => onQuantity(item.quantity + 1)}
                disabled={item.quantity >= max}
                aria-label="One more"
                className="size-7 text-lg font-extrabold hover:text-yellow disabled:opacity-30"
              >
                +
              </button>
            </div>
            <span className="min-w-[70px] text-right font-extrabold">
              {formatPrice(item.priceCents * item.quantity)}
            </span>
          </div>
        </div>
        {item.problem !== '' && (
          <p className="mt-2 text-xs font-semibold text-pink-region">
            {problemText[item.problem]}
            {item.problem === 'not_enough_stock' && ` Only ${item.stock} left.`}
          </p>
        )}
      </div>
    </li>
  )
}

function RemoveButton({ onRemove, label }: { onRemove: () => void; label: string }) {
  return (
    <button type="button" onClick={onRemove} aria-label={label} title="Remove" className="text-text-nav hover:text-white">
      <CloseIcon className="size-5" />
    </button>
  )
}
