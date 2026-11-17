import { ProductCardSkeleton } from '../components/ProductCard.tsx'

// Loading placeholders in the shape of each homepage block, so the page does not jump
// when the real content arrives. All of them are decorative (aria-hidden).

export function MosaicSkeleton() {
  return (
    <div className="flex h-[180px] gap-1 md:h-[310px]" aria-hidden="true">
      <div className="skeleton flex-[7]" />
      {[0, 1, 2].map((i) => (
        <div key={i} className="skeleton hidden flex-1 md:block" />
      ))}
    </div>
  )
}

export function CarouselSkeleton() {
  return (
    <div className="flex gap-4 overflow-hidden" aria-hidden="true">
      {Array.from({ length: 7 }, (_, i) => (
        <div key={i} className="w-[195px] shrink-0">
          <ProductCardSkeleton />
        </div>
      ))}
    </div>
  )
}

export function CategoryGridSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-7" aria-hidden="true">
      {Array.from({ length: 7 }, (_, i) => (
        <div key={i} className="skeleton h-[135px]" />
      ))}
    </div>
  )
}

// While the layout itself (GET /api/home) loads, the page shows the usual top of the
// homepage: banners, then a titled row of cards.
export function HomeSkeleton() {
  return (
    <div aria-hidden="true">
      <MosaicSkeleton />
      <div className="mx-auto max-w-page px-4 py-10">
        <div className="skeleton mb-6 h-[37px] w-[220px]" />
        <CarouselSkeleton />
      </div>
    </div>
  )
}
