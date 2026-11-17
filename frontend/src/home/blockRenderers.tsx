import type { ReactNode } from 'react'
import type { Banner, Category, ProductCard } from '../api.ts'
import { BannerMosaic } from './BannerMosaic.tsx'
import { CategoryGrid } from './CategoryGrid.tsx'
import { ProductCarousel } from './ProductCarousel.tsx'
import { CarouselSkeleton, CategoryGridSkeleton, MosaicSkeleton } from './skeletons.tsx'

// The only thing the frontend knows about the homepage: how to draw each block TYPE.
// Which blocks exist, their order, titles and data all come from the API.
// `render` receives the block's items as parsed JSON; each renderer states which
// shape its block type delivers (the API contract in docs/homepage.md). `skeleton` is
// drawn while the data loads, in the block's shape; `framed` = drawn in a titled band.
type Renderer = { render: (items: unknown[]) => ReactNode; skeleton: () => ReactNode; framed: boolean }

export const blockRenderers: Record<string, Renderer> = {
  banner_mosaic: {
    render: (items) => <BannerMosaic items={items as Banner[]} />,
    skeleton: () => <MosaicSkeleton />,
    framed: false,
  },
  product_carousel: {
    render: (items) => <ProductCarousel items={items as ProductCard[]} />,
    skeleton: () => <CarouselSkeleton />,
    framed: true,
  },
  category_grid: {
    render: (items) => <CategoryGrid items={items as Category[]} />,
    skeleton: () => <CategoryGridSkeleton />,
    framed: true,
  },
}
