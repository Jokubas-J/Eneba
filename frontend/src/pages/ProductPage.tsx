import { useQuery } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError, fetchProduct, type ProductCard as Card, type ProductDetail } from '../api.ts'
import { Button } from '../components/Button.tsx'
import { CalendarIcon, GlobeIcon, HomeIcon, KeyIcon, MonitorIcon } from '../components/icons.tsx'
import { Page } from '../components/Page.tsx'
import { PageTitle } from '../components/PageTitle.tsx'
import { ProductCard } from '../components/ProductCard.tsx'
import { RemoteImage } from '../components/RemoteImage.tsx'
import { BuyBox } from '../product/BuyBox.tsx'
import { formatDate } from '../product/formatDate.ts'
import { Gallery } from '../product/Gallery.tsx'
import { OfferList } from '../product/OfferList.tsx'
import { bestOffer } from '../product/offers.ts'
import { ImportantNotice, ProductInfo } from '../product/ProductInfo.tsx'
import { isUsableInEU } from '../region.ts'
import { NotFoundPage } from './NotFoundPage.tsx'

// Product page (design-reference.md §5, screenshots 11 and 12, and a Playwright
// measurement of a live eneba.com product page at 1920px): breadcrumb; cover, title
// and info tiles; screenshots; important notice; the offer comparison; other
// versions; product details. The buy box sits in a right-hand column.
export function ProductPage() {
  // useParams reads the :slug part of the route /product/:slug.
  const { slug = '' } = useParams()

  const product = useQuery({
    queryKey: ['product', slug],
    // No abort signal here (unlike the search dropdown): in development, StrictMode
    // mounts the page twice, the first unmount cancelled the request, and the second
    // mount stayed attached to the cancelled request, leaving the page on its
    // skeleton. Cancelling brings nothing for a whole-page load anyway.
    queryFn: () => fetchProduct(slug),
    // A 404 will not fix itself: do not retry it (other errors are retried 3 times).
    retry: (failureCount, error) => !(error instanceof ApiError && error.status === 404) && failureCount < 3,
  })

  if (product.error instanceof ApiError && product.error.status === 404) {
    return <NotFoundPage />
  }
  return (
    // Wide: Eneba's product page content is 1600px, the rest of the site 1200px.
    <Page wide>
      {product.isError ? (
        <div className="bg-purple-dark p-8 text-center">
          <p>The product could not be loaded.</p>
          <Button className="mt-3" onClick={() => product.refetch()}>
            Try again
          </Button>
        </div>
      ) : product.data ? (
        <ProductView product={product.data} />
      ) : (
        <ProductSkeleton />
      )}
    </Page>
  )
}

function ProductView({ product }: { product: ProductDetail }) {
  const best = bestOffer(product.offers)
  const others = product.offers.filter((offer) => offer.stock > 0 && offer !== best)
  const usable = isUsableInEU(product.region.slug)

  return (
    <>
      <PageTitle title={product.name} />
      <Breadcrumb product={product} />

      {/* Measured: 1220px main column + 30px gap + 350px buy box. As a grid, the buy
          box can be the second item: on a phone it then comes right after the title
          (price near the top), on a wide screen it moves into the right column. */}
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_350px]">
        <div className="flex min-w-0 flex-col gap-6 sm:flex-row">
          <div className="aspect-[224/316] w-[150px] shrink-0 bg-purple-dark sm:w-[224px]">
            <RemoteImage src={product.coverUrl} className="size-full object-cover" />
          </div>
          <div className="min-w-0 flex-1">
            <h1 className="text-title font-bold">{product.name}</h1>
            <dl className="mt-6 grid gap-x-4 gap-y-6 md:grid-cols-2">
              <InfoTile icon={<GlobeIcon className="size-8" />} title={product.region.name}>
                <span className={usable ? 'text-green-region' : 'text-pink-region'}>
                  {usable ? 'Can be activated in the EU' : `Can only be activated in ${product.region.name}`}
                </span>
              </InfoTile>
              <InfoTile icon={<MonitorIcon className="size-8" />} title={product.platform.name}>
                Activate / redeem on {product.platform.name}
              </InfoTile>
              <InfoTile icon={<KeyIcon className="size-8" />} title="Digital key">
                A digital code (CD key), shown on your order page after payment.
              </InfoTile>
              {product.releaseDate && (
                <InfoTile icon={<CalendarIcon className="size-8" />} title="Release date">
                  {formatDate(product.releaseDate)}
                  {product.developer && ` · ${product.developer}`}
                </InfoTile>
              )}
            </dl>
          </div>
        </div>

        <BuyBox best={best} others={others} />

        <div className="flex min-w-0 flex-col gap-8">
          <Gallery screenshots={product.screenshots} title={product.name} />
          {product.activation && <ImportantNotice guide={product.activation} />}
          <OfferList offers={product.offers} />
          {product.otherVersions.length > 0 && <OtherVersions versions={product.otherVersions} />}
          <ProductInfo product={product} />
        </div>
      </div>
    </>
  )
}

function Breadcrumb({ product }: { product: ProductDetail }) {
  return (
    <nav aria-label="Breadcrumb" className="mb-6 text-sm text-text-nav">
      <ol className="flex flex-wrap items-center gap-2">
        <li>
          <Link to="/" aria-label="Home" className="block hover:text-white">
            <HomeIcon className="size-5" />
          </Link>
        </li>
        <Separator />
        <li>
          <Link to="/store" className="hover:text-white">
            Store
          </Link>
        </li>
        <Separator />
        <li>
          <Link to={`/store?platform=${product.platform.slug}`} className="hover:text-white">
            {product.platform.name} games
          </Link>
        </li>
        <Separator />
        {/* Cut to 150px as on eneba.com; the full name is in the title right below
            and in the tooltip. */}
        <li aria-current="page" title={product.name} className="max-w-[150px] truncate">
          {product.name}
        </li>
      </ol>
    </nav>
  )
}

function Separator() {
  return (
    <li aria-hidden="true" className="text-text-muted">
      ›
    </li>
  )
}

function InfoTile({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <div className="flex gap-3">
      <span className="shrink-0 text-white">{icon}</span>
      <div>
        <dt className="font-extrabold">{title}</dt>
        <dd className="text-xs leading-[17px] text-text-muted">{children}</dd>
      </div>
    </div>
  )
}

// How many versions are shown at first and added per "Load more".
const versionsPage = 5

// The same game on other platforms or for other regions, as the store's product
// cards. Region is part of the product (a Europe key is a different product), so
// this is where regions are compared.
function OtherVersions({ versions }: { versions: Card[] }) {
  const [shown, setShown] = useState(versionsPage)

  return (
    <section aria-labelledby="versions-heading">
      <h2 id="versions-heading" className="mb-4 text-xl font-bold">
        Other versions
      </h2>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 2xl:grid-cols-5">
        {versions.slice(0, shown).map((version) => (
          <ProductCard key={version.id} product={version} />
        ))}
      </div>
      {shown < versions.length && (
        <div className="mt-6 text-center">
          <Button variant="outline" onClick={() => setShown(shown + versionsPage)}>
            Load more
          </Button>
        </div>
      )}
    </section>
  )
}

function ProductSkeleton() {
  return (
    <div aria-hidden="true">
      <div className="skeleton mb-6 h-3 w-1/3" />
      <div className="flex flex-col gap-6 md:flex-row">
        <div className="skeleton aspect-[224/316] w-[224px]" />
        <div className="flex-1">
          <div className="skeleton h-6 w-2/3" />
          <div className="skeleton mt-3 h-4 w-1/3" />
          <div className="skeleton mt-6 h-24" />
          <div className="skeleton mt-4 h-[109px]" />
        </div>
      </div>
    </div>
  )
}
