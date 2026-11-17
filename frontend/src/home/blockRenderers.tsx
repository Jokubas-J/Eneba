import type { ReactNode } from 'react'
import type { Banner, Category, ProductCard } from '../api.ts'
import { BannerMosaic } from './BannerMosaic.tsx'
import { CategoryGrid } from './CategoryGrid.tsx'
import { ProductCarousel } from './ProductCarousel.tsx'

// The only thing the frontend knows about the homepage: how to draw each block TYPE.
// Which blocks exist, their order, titles and data all come from the API.
// `render` receives the block's items as parsed JSON; each renderer states which
// shape its block type delivers (the API contract in docs/homepage.md). `height` is
// the skeleton's height while loading; `framed` = drawn in a titled band.
type Renderer = { render: (items: unknown[]) => ReactNode; height: number; framed: boolean }

export const blockRenderers: Record<string, Renderer> = {
  banner_mosaic: {
    render: (items) => <BannerMosaic items={items as Banner[]} />,
    height: 310,
    framed: false,
  },
  product_carousel: {
    render: (items) => <ProductCarousel items={items as ProductCard[]} />,
    height: 400,
    framed: true,
  },
  category_grid: {
    render: (items) => <CategoryGrid items={items as Category[]} />,
    height: 135,
    framed: true,
  },
}
