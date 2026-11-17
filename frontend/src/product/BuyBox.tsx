import { useNavigate } from 'react-router'
import type { Offer } from '../api.ts'
import { useCart } from '../cart/useCart.ts'
import { Button } from '../components/Button.tsx'
import { CartPlusIcon, StarIcon } from '../components/icons.tsx'
import { formatPrice } from '../money.ts'

type Props = {
  best: Offer | undefined
  // The other offers that can be bought, for "+N offers from €X".
  others: Offer[]
}

// The buy box on the right (design-reference.md §5, screenshot 11; measured 350px
// wide with 20px padding on eneba.com). Ours shows the cheapest offer in stock,
// where Eneba shows an offer it chooses ("Featured offer").
export function BuyBox({ best, others }: Props) {
  const { dispatch, addWithConfirmation } = useCart()
  const navigate = useNavigate()
  const cheapestOther = others.length > 0 ? Math.min(...others.map((offer) => offer.priceCents)) : null

  return (
    <aside
      aria-label="Best offer"
      // lg: second column of the page grid, spanning both rows; self-start keeps its
      // own height so "sticky" can keep it in view while the offers scroll past.
      className="bg-purple-light p-4 shadow-panel lg:sticky lg:top-4 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:self-start"
    >
      {best ? (
        <>
          <p className="text-tiny font-extrabold text-yellow uppercase">Best price</p>
          {best.discountPercent > 0 && (
            <p className="mt-4 text-xs font-extrabold">
              <s className="text-text-faded">{formatPrice(best.originalPriceCents)}</s>{' '}
              <span className="text-green-cashback">-{best.discountPercent}%</span>
            </p>
          )}
          <p className="text-buybox font-extrabold">{formatPrice(best.priceCents)}</p>
          <p className="mt-2 flex items-center gap-1 text-xs text-text-muted">
            Sold by <span className="font-semibold text-white">{best.seller.name}</span>
            <StarIcon className="size-3 text-yellow" />
            {best.seller.rating.toFixed(1)}
          </p>

          {/* Measured on eneba.com: 243px "Buy now" + 15px gap + 52px cart button. */}
          <div className="mt-4 flex gap-3">
            {/* Buy now = put the offer in the cart and go straight to it (the cart
                is the first checkout step). */}
            <Button
              className="flex-1"
              onClick={() => {
                dispatch({ type: 'add', offerId: best.id, max: best.stock })
                navigate('/cart')
              }}
            >
              Buy now
            </Button>
            <Button
              variant="outline"
              className="w-[52px] shrink-0"
              onClick={() => addWithConfirmation(best.id, best.stock)}
              aria-label="Add to cart"
              title="Add to cart"
            >
              <CartPlusIcon className="size-5" />
            </Button>
          </div>

          {cheapestOther !== null && (
            <a
              href="#offers"
              onClick={(event) => {
                // Smooth scrolling instead of the jump of a plain anchor link. The
                // href stays, so the link still works with JavaScript off and can be
                // opened in a new tab.
                event.preventDefault()
                document.getElementById('offers')?.scrollIntoView({ behavior: 'smooth' })
              }}
              className="mt-6 block text-center text-sm font-semibold text-yellow hover:underline"
            >
              +{others.length} {others.length === 1 ? 'offer' : 'offers'} from {formatPrice(cheapestOther)}
            </a>
          )}
        </>
      ) : (
        <>
          <p className="text-lg font-extrabold">Sold out</p>
          <p className="mt-1 text-sm text-text-muted">No seller has a key for this product at the moment.</p>
        </>
      )}
    </aside>
  )
}
