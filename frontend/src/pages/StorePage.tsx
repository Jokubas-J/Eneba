import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { useSearchParams } from 'react-router'
import { type Facets, fetchProducts } from '../api.ts'
import { Button } from '../components/Button.tsx'
import { Page } from '../components/Page.tsx'
import { PageTitle } from '../components/PageTitle.tsx'
import { ProductCard, ProductCardSkeleton } from '../components/ProductCard.tsx'
import { centsToEuros } from '../money.ts'
import { FilterSidebar } from '../store/FilterSidebar.tsx'
import { FiltersDrawer } from '../store/FiltersDrawer.tsx'
import { Pagination } from '../store/Pagination.tsx'
import {
  clearAll,
  clearText,
  type GroupKey,
  groupKeys,
  hasActiveFilters,
  parseStoreParams,
  setPage,
  setPriceRange,
  setSort,
  type SortValue,
  sortOptions,
  type StoreFilters,
  toApiQuery,
  toggleGroupValue,
  toSearchParams,
} from '../store/storeParams.ts'

// The store: filter sidebar, sort order, product grid and pages (screenshots 08, 10).
//
// The single source of truth is the URL. Every render reads the filters from it;
// every change (a checkbox, the sort box, a page link) writes a new URL, which makes
// React Router render the page again with the new filters. There is no second copy
// of the filters in component state that could drift out of step with the address
// bar.
export function StorePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const filters = parseStoreParams(searchParams)
  const apiQuery = toApiQuery(filters)

  const products = useQuery({
    // The query string is part of the cache key: each distinct set of filters is
    // cached separately, and going back to a previous one shows it instantly.
    queryKey: ['products', apiQuery],
    // No abort signal here (unlike the search dropdown): in development, StrictMode
    // mounts the page twice, the first unmount cancelled the request, and the second
    // mount stayed attached to the cancelled request, leaving the page on its
    // skeleton. Cancelling brings nothing for a whole-page load anyway.
    queryFn: () => fetchProducts(apiQuery),
    // While the next set of filters loads, keep showing the previous result (dimmed)
    // instead of flashing an empty page.
    placeholderData: keepPreviousData,
  })

  // navigate to the URL that represents the given filters.
  const apply = (next: StoreFilters) => setSearchParams(toSearchParams(next))
  const hrefFor = (page: number) => {
    const query = toSearchParams(setPage(filters, page)).toString()
    return query ? `/store?${query}` : '/store'
  }

  const data = products.data
  const pageCount = data ? Math.ceil(data.total / data.pageSize) : 0
  const [filtersOpen, setFiltersOpen] = useState(false)
  // How many filters are on, for the phone's "Filters (3)" button.
  const activeCount =
    groupKeys.reduce((sum, key) => sum + filters.groups[key].length, 0) +
    (filters.priceMin !== null || filters.priceMax !== null ? 1 : 0)
  const sidebar = (
    <FilterSidebar
      filters={filters}
      facets={data?.facets}
      onToggle={(key, slug) => apply(toggleGroupValue(filters, key, slug))}
      onPriceChange={(min, max) => apply(setPriceRange(filters, min, max))}
    />
  )

  return (
    <Page>
      <PageTitle title={filters.text ? `Results for “${filters.text}”` : 'Store'} />
      <h1 className="text-heading font-bold">{filters.text ? `Results for “${filters.text}”` : 'Store'}</h1>

      <div className="mt-4 flex flex-col gap-4 md:flex-row md:items-start">
        {/* Wide screens: the sidebar. Phones: a button that opens the same filters
            in a drawer, so the results are not pushed far down the page. */}
        <aside className="hidden md:block md:w-[220px] md:shrink-0" aria-label="Filters">
          {sidebar}
        </aside>
        <Button variant="outline" className="w-full md:hidden" aria-haspopup="dialog" onClick={() => setFiltersOpen(true)}>
          {activeCount > 0 ? `Filters (${activeCount})` : 'Filters'}
        </Button>
        {filtersOpen && (
          <FiltersDrawer resultCount={data ? data.total : null} onClose={() => setFiltersOpen(false)}>
            {sidebar}
          </FiltersDrawer>
        )}

        <section className="min-w-0 flex-1" aria-label="Products">
          {hasActiveFilters(filters) && (
            <ActiveFilters filters={filters} facets={data?.facets} onChange={apply} />
          )}

          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            {/* aria-live: a screen reader announces the new count after filtering. */}
            <p className="text-sm" aria-live="polite">
              Results found: <strong className="font-extrabold">{data ? data.total : '…'}</strong>
            </p>

            <label className="flex items-center gap-2 text-sm font-extrabold">
              <span className="sr-only">Sort by</span>
              <select
                // "" stands for "not chosen": the backend's default order.
                value={filters.sort ?? ''}
                onChange={(event) => apply(setSort(filters, (event.target.value || null) as SortValue | null))}
                className="h-8 bg-overlay-input px-2 font-extrabold inset-shadow-input focus:outline-2 focus:outline-yellow"
              >
                <option value="" className="bg-purple-dark">
                  {filters.text ? 'Best match' : 'Popularity: Most popular'}
                </option>
                {sortOptions
                  // The two defaults are already offered by the first option.
                  .filter((option) => option.value !== 'relevance' && option.value !== 'popular')
                  .map((option) => (
                    <option key={option.value} value={option.value} className="bg-purple-dark">
                      {option.label}
                    </option>
                  ))}
              </select>
            </label>
          </div>

          {products.isError ? (
            <div className="bg-purple-dark p-4 text-center">
              <p>The products could not be loaded.</p>
              <Button className="mt-3" onClick={() => products.refetch()}>
                Try again
              </Button>
            </div>
          ) : !data ? (
            <Grid>
              {Array.from({ length: 8 }, (_, index) => (
                <ProductCardSkeleton key={index} />
              ))}
            </Grid>
          ) : data.items.length === 0 ? (
            <div className="bg-purple-dark p-8 text-center">
              <p className="text-lg font-extrabold">No products found</p>
              <p className="mt-1 text-sm text-text-muted">Try a different spelling or remove some filters.</p>
              {hasActiveFilters(filters) && (
                <Button className="mt-3" onClick={() => apply(clearAll(filters))}>
                  Clear all filters
                </Button>
              )}
            </div>
          ) : (
            // isPlaceholderData: the grid still shows the previous filters' result
            // while the new one is on its way.
            <div className={products.isPlaceholderData ? 'opacity-50 transition-opacity' : 'transition-opacity'}>
              <Grid>
                {data.items.map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </Grid>
              <Pagination page={data.page} pageCount={pageCount} hrefFor={hrefFor} />
            </div>
          )}
        </section>
      </div>
    </Page>
  )
}

function Grid({ children }: { children: ReactNode }) {
  // 4 cards per row beside the sidebar at desktop width, as on eneba.com.
  return <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">{children}</div>
}

const groupLabels: Record<GroupKey, string> = {
  platform: 'Platform',
  region: 'Region',
  genre: 'Genre',
  type: 'Type',
}

// Maps a group key to its list in the API's facets, to look up display names.
const facetOf: Record<GroupKey, keyof Facets> = {
  platform: 'platforms',
  region: 'regions',
  genre: 'genres',
  type: 'types',
}

type ActiveFiltersProps = {
  filters: StoreFilters
  facets: Facets | undefined
  onChange: (next: StoreFilters) => void
}

// The row of removable chips above the results ("Text: witchr 3 ✕") with "Clear all".
function ActiveFilters({ filters, facets, onChange }: ActiveFiltersProps) {
  const chips: { key: string; label: string; remove: () => void }[] = []

  if (filters.text) {
    chips.push({ key: 'text', label: `Text: ${filters.text}`, remove: () => onChange(clearText(filters)) })
  }
  for (const group of groupKeys) {
    for (const slug of filters.groups[group]) {
      // The URL holds slugs ("xbox-live"); the display name comes from the facets.
      const name = facets?.[facetOf[group]].find((option) => option.slug === slug)?.name ?? slug
      chips.push({
        key: `${group}-${slug}`,
        label: `${groupLabels[group]}: ${name}`,
        remove: () => onChange(toggleGroupValue(filters, group, slug)),
      })
    }
  }
  if (filters.priceMin !== null || filters.priceMax !== null) {
    const min = filters.priceMin === null ? '0' : centsToEuros(filters.priceMin)
    const max = filters.priceMax === null ? '' : centsToEuros(filters.priceMax)
    chips.push({
      key: 'price',
      label: max ? `Price: €${min} - €${max}` : `Price: from €${min}`,
      remove: () => onChange(setPriceRange(filters, null, null)),
    })
  }

  return (
    <ul className="mb-3 flex flex-wrap items-center gap-2" aria-label="Active filters">
      {chips.map((chip) => (
        <li key={chip.key}>
          <button
            type="button"
            onClick={chip.remove}
            aria-label={`Remove filter ${chip.label}`}
            className="flex items-center gap-1 rounded-pill bg-purple-deep px-2 py-1 text-xs font-semibold hover:bg-purple-dark focus-visible:outline-2 focus-visible:outline-yellow"
          >
            {chip.label}
            <span aria-hidden="true">✕</span>
          </button>
        </li>
      ))}
      <li>
        <button
          type="button"
          onClick={() => onChange(clearAll(filters))}
          className="text-xs font-semibold text-yellow hover:underline"
        >
          Clear all
        </button>
      </li>
    </ul>
  )
}
