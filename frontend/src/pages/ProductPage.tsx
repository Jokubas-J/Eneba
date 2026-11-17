import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Link, useParams, useSearchParams } from 'react-router'
import { ApiError, fetchProduct, type Offer, type ProductDetail, type ProductVersion } from '../api.ts'
import { Button } from '../components/Button.tsx'
import { CalendarIcon, GlobeIcon, KeyIcon, MonitorIcon, StarIcon, VerifiedIcon } from '../components/icons.tsx'
import { formatPrice } from '../money.ts'
import { bestOffer, type OfferSort, ratingLabel, sortOffers, stockState } from '../product/offers.ts'
import { isUsableInEU } from '../region.ts'
import { NotFoundPage } from './NotFoundPage.tsx'

// The cart is built in the next step; until then the buttons are shown but disabled,
// with this explanation as their tooltip.
const cartNotReady = 'The cart is added in the next build step.'

// Product page (design-reference.md §5, screenshots 11 and 12): details and info
// tiles, buy box with the best offer, the full offer comparison, other versions of
// the same game, and the description.
export function ProductPage() {
  // useParams reads the :slug part of the route /product/:slug.
  const { slug = '' } = useParams()

  const product = useQuery({
    queryKey: ['product', slug],
    queryFn: ({ signal }) => fetchProduct(slug, signal),
    // A 404 will not fix itself: do not retry it (other errors are retried 3 times).
    retry: (failureCount, error) => !(error instanceof ApiError && error.status === 404) && failureCount < 3,
  })

  if (product.error instanceof ApiError && product.error.status === 404) {
    return <NotFoundPage />
  }
  if (product.isError) {
    return (
      <div className="bg-purple-dark p-8 text-center">
        <p>The product could not be loaded.</p>
        <Button className="mt-3" onClick={() => product.refetch()}>
          Try again
        </Button>
      </div>
    )
  }
  if (!product.data) return <ProductSkeleton />

  return <ProductView product={product.data} />
}

function ProductView({ product }: { product: ProductDetail }) {
  const best = bestOffer(product.offers)
  const inStock = product.offers.filter((offer) => offer.stock > 0)
  const usable = isUsableInEU(product.region.slug)

  return (
    <>
      <nav aria-label="Breadcrumb" className="mb-4 text-xs font-semibold text-text-nav">
        <ol className="flex flex-wrap items-center gap-1">
          <li>
            <Link to="/store" className="hover:text-white">
              Store
            </Link>
          </li>
          <li aria-hidden="true">›</li>
          <li>
            <Link to={`/store?platform=${product.platform.slug}`} className="hover:text-white">
              {product.platform.name} games
            </Link>
          </li>
          <li aria-hidden="true">›</li>
          <li aria-current="page" className="truncate text-white">
            {product.name}
          </li>
        </ol>
      </nav>

      {/* A grid instead of two columns: on a phone the order is top, buy box, rest (the
          price right under the title); on a wide screen the buy box moves into a
          right-hand column next to both parts. */}
      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_310px]">
          {/* Top: cover, title, info tiles */}
          <div className="flex min-w-0 flex-col gap-4 sm:flex-row">
            <div className="aspect-[5/7] w-[180px] shrink-0 bg-purple-dark sm:w-[220px]">
              {product.coverUrl && <img src={product.coverUrl} alt="" className="size-full object-cover" />}
            </div>
            <div className="min-w-0">
              <h1 className="text-title font-bold">{product.name}</h1>
              <dl className="mt-4 grid gap-4 sm:grid-cols-2">
                <InfoTile icon={<GlobeIcon className="size-6" />} title={product.region.name}>
                  <span className={usable ? 'text-green-region' : 'text-pink-region'}>
                    {usable ? 'Can be activated in the EU' : `Can only be activated in ${product.region.name}`}
                  </span>
                </InfoTile>
                <InfoTile icon={<MonitorIcon className="size-6" />} title={product.platform.name}>
                  Activate / redeem on {product.platform.name}
                </InfoTile>
                <InfoTile icon={<KeyIcon className="size-6" />} title="Digital key">
                  A code that you activate on the platform. Shown after payment.
                </InfoTile>
                {product.releaseDate && (
                  <InfoTile icon={<CalendarIcon className="size-6" />} title="Release date">
                    {formatDate(product.releaseDate)}
                    {product.developer && ` · ${product.developer}`}
                  </InfoTile>
                )}
              </dl>
            </div>
          </div>

        <BuyBox best={best} offerCount={inStock.length} />

        <div className="min-w-0">
          <Offers product={product} />

          {product.otherVersions.length > 0 && <OtherVersions versions={product.otherVersions} />}

          <About product={product} />
        </div>
      </div>
    </>
  )
}

function InfoTile({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <div className="flex gap-2">
      <span className="shrink-0 text-white">{icon}</span>
      <div>
        <dt className="font-extrabold">{title}</dt>
        <dd className="text-xs leading-[18px] text-text-muted">{children}</dd>
      </div>
    </div>
  )
}

// The panel on the right: the best offer, as on Eneba's "featured offer" box. Ours
// is the cheapest offer in stock (Eneba's is chosen by them and not always the cheapest).
function BuyBox({ best, offerCount }: { best: Offer | undefined; offerCount: number }) {
  return (
    <aside
      aria-label="Best offer"
      // lg: second column, spanning both rows; self-start keeps it its own height
      // so "sticky" can keep it in view while the offers scroll past.
      className="bg-purple-light p-4 shadow-panel lg:sticky lg:top-4 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:self-start"
    >
      {best ? (
        <>
          <p className="text-tiny font-extrabold text-yellow uppercase">Best price</p>
          {best.discountPercent > 0 && (
            <p className="mt-2 text-xs font-extrabold">
              <s className="text-text-faded">{formatPrice(best.originalPriceCents)}</s>{' '}
              <span className="text-green-cashback">-{best.discountPercent}%</span>
            </p>
          )}
          <p className="mt-1 text-buybox font-extrabold">{formatPrice(best.priceCents)}</p>
          <p className="mt-2 flex items-center gap-1 text-xs text-text-muted">
            Sold by <span className="font-semibold text-white">{best.seller.name}</span>
            <StarIcon className="size-3 text-yellow" />
            {best.seller.rating.toFixed(1)}
          </p>
          <Button className="mt-4 w-full" disabled title={cartNotReady}>
            Add to cart
          </Button>
          {offerCount > 1 && (
            <a href="#offers" className="mt-3 block text-center text-sm font-semibold text-yellow hover:underline">
              +{offerCount - 1} more {offerCount - 1 === 1 ? 'offer' : 'offers'}
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

const sortChoices: { value: OfferSort; label: string }[] = [
  { value: 'price', label: 'Price' },
  { value: 'rating', label: 'Seller rating' },
]

// The offer comparison: every seller's offer, sortable by price or seller rating.
// The chosen order is kept in the URL (?offers=rating), like the store's filters.
function Offers({ product }: { product: ProductDetail }) {
  const [searchParams, setSearchParams] = useSearchParams()
  const sort: OfferSort = searchParams.get('offers') === 'rating' ? 'rating' : 'price'
  const sorted = sortOffers(product.offers, sort)

  function choose(value: OfferSort) {
    const next = new URLSearchParams(searchParams)
    if (value === 'price') next.delete('offers')
    else next.set('offers', value)
    // replace: changing the order of a list is not a new page in the browser history.
    setSearchParams(next, { replace: true, preventScrollReset: true })
  }

  return (
    <section id="offers" aria-labelledby="offers-heading" className="scroll-mt-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 id="offers-heading" className="text-xl font-bold">
          <span className="text-teal">{product.offers.length}</span> {product.offers.length === 1 ? 'offer' : 'offers'}
        </h2>
        {product.offers.length > 1 && (
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
        <ul className="flex flex-col gap-1">
          {sorted.map((offer) => (
            <OfferRow key={offer.id} offer={offer} />
          ))}
        </ul>
      )}
    </section>
  )
}

const stockColors = { ok: 'text-green-region', low: 'text-yellow', none: 'text-pink-region' }

function OfferRow({ offer }: { offer: Offer }) {
  const stock = stockState(offer.stock)
  const soldOut = offer.stock === 0

  return (
    <li
      className={`flex flex-wrap items-center gap-4 border-l-4 bg-overlay-row p-4 ${
        offer.isCheapest ? 'border-yellow' : 'border-transparent'
      }`}
    >
      {/* Seller */}
      <div className="flex min-w-[180px] flex-1 items-center gap-3">
        <span
          aria-hidden="true"
          className="flex size-8 shrink-0 items-center justify-center rounded-full bg-purple-light font-extrabold"
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
          <p className="flex items-center gap-1 text-[11px] font-extrabold text-white/85">
            <StarIcon className="size-3 text-yellow" />
            {offer.seller.rating.toFixed(1)} {ratingLabel(offer.seller.rating)} rating
            <span className="font-normal text-text-muted">({offer.seller.ratingCount.toLocaleString('en-IE')})</span>
          </p>
        </div>
      </div>

      {/* Stock and the best-price mark */}
      <div className="flex min-w-[110px] flex-col items-start gap-1 text-xs font-extrabold">
        {offer.isCheapest && (
          <span className="rounded-pill border border-green-cashback px-2 leading-[20px] text-green-cashback">Best price</span>
        )}
        <span className={stockColors[stock.tone]}>{stock.label}</span>
      </div>

      {/* Price and action */}
      <div className="ml-auto flex items-center gap-4">
        <div className={`text-right ${soldOut ? 'opacity-50' : ''}`}>
          {offer.discountPercent > 0 && (
            <p className="text-xs font-extrabold">
              <s className="text-text-faded">{formatPrice(offer.originalPriceCents)}</s>{' '}
              <span className="text-green-cashback">-{offer.discountPercent}%</span>
            </p>
          )}
          <p className="text-offer font-extrabold">{formatPrice(offer.priceCents)}</p>
        </div>
        <Button variant="outline" disabled title={soldOut ? 'Sold out' : cartNotReady}>
          Add to cart
        </Button>
      </div>
    </li>
  )
}

// The same game on other platforms or for other regions, each with its own price.
// Region is part of the product (a Europe key is a different product), so this strip
// is where regions are compared.
function OtherVersions({ versions }: { versions: ProductVersion[] }) {
  return (
    <section aria-labelledby="versions-heading" className="mt-8">
      <h2 id="versions-heading" className="mb-3 text-xl font-bold">
        Other versions
      </h2>
      <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
        {versions.map((version) => (
          <li key={version.slug}>
            <Link
              to={`/product/${version.slug}`}
              className="flex h-full items-center justify-between gap-2 border border-purple-light bg-purple-dark p-3 hover:border-teal"
            >
              <span>
                <span className="block text-sm font-extrabold">{version.platform.name}</span>
                <span
                  className={`text-xs font-extrabold uppercase ${
                    isUsableInEU(version.region.slug) ? 'text-green-region' : 'text-pink-region'
                  }`}
                >
                  {version.region.name}
                </span>
              </span>
              <span className="text-right">
                {version.priceCents === null ? (
                  <span className="text-sm font-extrabold text-text-muted">Sold out</span>
                ) : (
                  <>
                    <span className="block text-xs font-extrabold text-text-muted">From</span>
                    <span className="font-extrabold">{formatPrice(version.priceCents)}</span>
                  </>
                )}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

function About({ product }: { product: ProductDetail }) {
  return (
    <section aria-labelledby="about-heading" className="mt-8">
      <h2 id="about-heading" className="mb-3 text-xl font-bold">
        Product details
      </h2>
      {/* Plain text from Steam's short description; React escapes it, so no HTML
          from the data can run in the page. */}
      {product.description && <p className="text-sm leading-[24px]">{product.description}</p>}

      {product.genres.length > 0 && (
        <ul className="mt-4 flex flex-wrap gap-2" aria-label="Genres">
          {product.genres.map((genre) => (
            <li key={genre.slug}>
              <Link
                to={`/store?genre=${genre.slug}`}
                className="block rounded-pill border border-white px-3 text-xs leading-[24px] font-extrabold hover:bg-white hover:text-purple-base"
              >
                {genre.name}
              </Link>
            </li>
          ))}
        </ul>
      )}

      <dl className="mt-4 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
        <Detail label="Platform">{product.platform.name}</Detail>
        <Detail label="Region">{product.region.name}</Detail>
        {product.releaseDate && <Detail label="Release date">{formatDate(product.releaseDate)}</Detail>}
        {product.developer && <Detail label="Developer">{product.developer}</Detail>}
        {product.publisher && <Detail label="Publisher">{product.publisher}</Detail>}
      </dl>
      <p className="mt-4 text-xs text-text-muted">
        Title, description, genres, dates and cover from the Steam store. Sellers, prices and stock are demo data.
      </p>
    </section>
  )
}

function Detail({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-text-muted">{label}</dt>
      <dd>{children}</dd>
    </>
  )
}

// "2015-05-18" -> "18 May 2015". The date is read as UTC so the day cannot shift by
// the visitor's time zone.
function formatDate(isoDate: string): string {
  return new Date(`${isoDate}T00:00:00Z`).toLocaleDateString('en-IE', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  })
}

function ProductSkeleton() {
  return (
    <div className="animate-pulse" aria-hidden="true">
      <div className="mb-4 h-3 w-1/3 bg-purple-dark" />
      <div className="flex gap-4">
        <div className="aspect-[5/7] w-[220px] bg-purple-dark" />
        <div className="flex-1">
          <div className="h-6 w-2/3 bg-purple-dark" />
          <div className="mt-4 h-16 bg-purple-dark" />
        </div>
      </div>
    </div>
  )
}
