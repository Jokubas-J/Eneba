import { useSearchParams } from 'react-router'
import type { Offer } from '../api.ts'
import { Button } from '../components/Button.tsx'
import { CartPlusIcon, InfoIcon, StarIcon, VerifiedIcon } from '../components/icons.tsx'
import { formatPrice } from '../money.ts'
import { useNavigate } from 'react-router'
import { useCart } from '../cart/useCart.ts'
import { type OfferSort, ratingLabel, sortOffers, stockState } from './offers.ts'

const sortChoices: { value: OfferSort; label: string }[] = [
  { value: 'price', label: 'Price' },
  { value: 'rating', label: 'Seller rating' },
]

// The offer comparison: every seller's offer, sortable by price or seller rating.
// The chosen order is kept in the URL (?offers=rating), like the store's filters.
export function OfferList({ offers }: { offers: Offer[] }) {
  const [searchParams, setSearchParams] = useSearchParams()
  const sort: OfferSort = searchParams.get('offers') === 'rating' ? 'rating' : 'price'
  const sorted = sortOffers(offers, sort)

  function choose(value: OfferSort) {
    const next = new URLSearchParams(searchParams)
    if (value === 'price') next.delete('offers')
    else next.set('offers', value)
    // replace: changing the order of a list is not a new page in the browser history.
    setSearchParams(next, { replace: true, preventScrollReset: true })
  }

  return (
    <section id="offers" aria-labelledby="offers-heading" className="scroll-mt-4">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <h2 id="offers-heading" className="text-xl font-bold">
          <span className="text-teal">{offers.length}</span> {offers.length === 1 ? 'offer' : 'offers'}
        </h2>
        {offers.length > 1 && (
          <div role="group" aria-label="Sort offers by" className="flex text-sm font-extrabold">
            {sortChoices.map((choice) => (
              <button
                key={choice.value}
                type="button"
                aria-pressed={sort === choice.value}
                onClick={() => choose(choice.value)}
                className={`h-7 border border-white px-2 ${sort === choice.value ? 'bg-white text-purple-base' : ''}`}
              >
                {choice.label}
              </button>
            ))}
          </div>
        )}
      </div>

      {sorted.length === 0 ? (
        <p className="bg-overlay-row p-4 text-sm text-text-muted">No seller offers this product yet.</p>
      ) : (
        <ul className="flex flex-col gap-[8px]">
          {sorted.map((offer) => (
            <OfferRow key={offer.id} offer={offer} />
          ))}
        </ul>
      )}
    </section>
  )
}

const stockColors = { ok: 'text-green-region', low: 'text-yellow', none: 'text-pink-region' }

// One offer, as on eneba.com (screenshot 12; measured 109px high, 20px padding,
// 40px seller avatar): seller on the left, price and buttons on the right. Our
// additions in the middle: the stock state and the "Best price" mark.
function OfferRow({ offer }: { offer: Offer }) {
  const { dispatch, addWithConfirmation } = useCart()
  const navigate = useNavigate()
  const stock = stockState(offer.stock)
  const soldOut = offer.stock === 0
  const ratingHelp = `Average of ${offer.seller.ratingCount.toLocaleString('en-IE')} buyer ratings of this seller (demo data).`

  return (
    <li
      className={`flex min-h-[109px] flex-wrap items-center gap-4 border-l-4 bg-overlay-row p-4 ${
        offer.isCheapest ? 'border-yellow' : 'border-transparent'
      }`}
    >
      {/* Seller. There are no seller logos; the avatar is the first letter. */}
      <div className="flex min-w-[200px] flex-1 items-center gap-2">
        <span
          aria-hidden="true"
          className="flex size-8 shrink-0 items-center justify-center rounded-full bg-purple-light text-lg font-extrabold"
        >
          {offer.seller.name[0]}
        </span>
        <div>
          <p className="flex items-center gap-1 text-sm font-semibold">
            {offer.seller.name}
            {offer.seller.isVerified && (
              <span title="Verified seller" className="text-green-region">
                <VerifiedIcon className="size-4" />
                <span className="sr-only">(verified seller)</span>
              </span>
            )}
          </p>
          <p className="mt-1 flex items-center gap-1 text-[11px] font-extrabold text-white/85">
            <StarIcon className="size-3 text-yellow" />
            {offer.seller.rating.toFixed(1)} {ratingLabel(offer.seller.rating)}
            <span className="font-normal">rating</span>
            {/* The explanation is the button's accessible name and its tooltip. */}
            <button type="button" title={ratingHelp} aria-label={ratingHelp} className="text-text-muted hover:text-white">
              <InfoIcon className="size-4" />
            </button>
          </p>
        </div>
      </div>

      <div className="flex min-w-[110px] flex-col items-start gap-1 text-xs font-extrabold">
        {offer.isCheapest && (
          <span className="rounded-pill border border-green-cashback px-2 leading-[20px] text-green-cashback">
            Best price
          </span>
        )}
        <span className={stockColors[stock.tone]}>{stock.label}</span>
      </div>

      <div className="ml-auto flex items-center gap-3">
        <div className={`text-right ${soldOut ? 'opacity-50' : ''}`}>
          {offer.discountPercent > 0 && (
            <p className="text-xs font-extrabold">
              <s className="text-text-faded">{formatPrice(offer.originalPriceCents)}</s>{' '}
              <span className="text-green-cashback">-{offer.discountPercent}%</span>
            </p>
          )}
          <p className="text-offer font-extrabold">{formatPrice(offer.priceCents)}</p>
        </div>
        <Button
          variant="outline"
          disabled={soldOut}
          title={soldOut ? 'Sold out' : undefined}
          onClick={() => {
            dispatch({ type: 'add', offerId: offer.id, max: offer.stock })
            navigate('/cart')
          }}
        >
          Buy now
        </Button>
        <Button
          variant="outline"
          className="w-[52px]"
          disabled={soldOut}
          title={soldOut ? 'Sold out' : 'Add to cart'}
          onClick={() => addWithConfirmation(offer.id, offer.stock)}
          aria-label={`Add to cart: offer from ${offer.seller.name}`}
        >
          <CartPlusIcon className="size-5" />
        </Button>
      </div>
    </li>
  )
}
