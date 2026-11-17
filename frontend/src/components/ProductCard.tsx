import { Link } from 'react-router'
import type { ProductCard as Product } from '../api.ts'
import { formatPrice } from '../money.ts'
import { isUsableInEU } from '../region.ts'
import { RemoteImage } from './RemoteImage.tsx'

// Product card as measured on eneba.com (design-reference.md §5, screenshots 02-05):
// dark body, 1px teal border, portrait cover with the platform strip over its lower
// edge, then title, region and price.
export function ProductCard({ product }: { product: Product }) {
  const usable = isUsableInEU(product.region.slug)

  return (
    // The whole card is one link, so the entire surface is clickable and the product
    // has a real URL (open in new tab, share).
    <Link
      to={`/product/${product.slug}`}
      className="group flex flex-col border border-teal bg-purple-deep focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-yellow"
    >
      {/* aspect-[5/7] reserves the image's space before it loads, so the grid does
          not jump as covers arrive. */}
      <div className="relative aspect-[5/7] overflow-hidden bg-purple-dark">
        <RemoteImage
          src={product.coverUrl}
          // Covers below the first screen are fetched only when scrolled near.
          loading="lazy"
          className="size-full object-cover transition-transform duration-200 group-hover:scale-105"
        />
        <div className="absolute inset-x-0 bottom-0 flex h-[22px] items-center justify-center bg-overlay-strip text-xs font-extrabold text-white/85">
          {product.platform.name}
        </div>
      </div>

      <div className="flex flex-1 flex-col px-3 pt-3 pb-2">
        {/* Two lines at most, then an ellipsis; min-h keeps one-line titles from
            making their card's text block shorter than the others. */}
        <h3 className="line-clamp-2 min-h-[36px] text-xs leading-[18px] font-extrabold">{product.name}</h3>

        <p className={`mt-1 text-xs leading-[12px] font-extrabold uppercase ${usable ? 'text-green-region' : 'text-pink-region'}`}>
          {product.region.name}
        </p>

        {/* mt-auto pushes the price to the bottom edge of every card in a row. */}
        <div className="mt-auto pt-2">
          {product.priceCents === null ? (
            <p className="text-price font-extrabold text-text-muted">Sold out</p>
          ) : (
            <>
              <p className="flex items-baseline gap-1 text-xs leading-[12px] font-extrabold text-text-muted">
                From
                {product.discountPercent !== null && product.discountPercent > 0 && product.originalPriceCents !== null && (
                  <>
                    <s className="text-text-faded">{formatPrice(product.originalPriceCents)}</s>
                    <span className="text-green-cashback">-{product.discountPercent}%</span>
                  </>
                )}
              </p>
              <p className="mt-1 text-price font-extrabold">{formatPrice(product.priceCents)}</p>
            </>
          )}
        </div>
      </div>
    </Link>
  )
}

// Placeholder with the card's proportions, shown while products load.
export function ProductCardSkeleton() {
  return (
    <div className="border border-purple-light bg-purple-deep" aria-hidden="true">
      <div className="skeleton aspect-[5/7]" />
      <div className="px-3 pt-3 pb-2">
        <div className="skeleton h-3 w-4/5" />
        <div className="skeleton mt-2 h-3 w-1/3" />
        <div className="skeleton mt-4 h-4 w-1/2" />
      </div>
    </div>
  )
}
