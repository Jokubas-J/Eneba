// The store page keeps ALL of its state in the URL: filters, search text, sort
// order and page. Nothing is held in component state. That is what makes a store
// link shareable, the back button meaningful, and a reload harmless.
//
// This file is the translation layer, with no React in it so it can be unit tested:
//
//   address bar  --parseStoreParams-->  StoreFilters  --toApiQuery-->  /api/products?...
//                <--toSearchParams---

import { centsToEuros, eurosToCents } from '../money.ts'

export const sortOptions = [
  { value: 'relevance', label: 'Best match' },
  { value: 'popular', label: 'Popularity: Most popular' },
  { value: 'price_asc', label: 'Price: Low to high' },
  { value: 'price_desc', label: 'Price: High to low' },
  { value: 'newest', label: 'Date: Newest' },
  { value: 'discount', label: 'Biggest discount' },
  { value: 'oldest', label: 'Date: Oldest' },
  { value: 'name', label: 'Alphabet: A-Z' },
] as const

// The union of the option values: 'relevance' | 'popular' | ...
export type SortValue = (typeof sortOptions)[number]['value']

// The four checkbox groups. The key is used both in the page URL and in the API.
export const groupKeys = ['platform', 'region', 'genre', 'type'] as const
export type GroupKey = (typeof groupKeys)[number]

export type StoreFilters = {
  text: string
  // Selected slugs per checkbox group.
  groups: Record<GroupKey, string[]>
  // Price bounds in cents; null = no bound.
  priceMin: number | null
  priceMax: number | null
  // null = not chosen: the backend then picks best match or most popular.
  sort: SortValue | null
  page: number
}

// Reads the filters out of the address bar. Anything malformed (a hand-edited URL)
// is dropped instead of causing an error: a broken link should still show a store.
export function parseStoreParams(params: URLSearchParams): StoreFilters {
  const sort = params.get('sort')
  const page = Number(params.get('page'))

  return {
    text: (params.get('text') ?? '').trim(),
    groups: {
      // getAll returns every value of a repeated parameter (?platform=a&platform=b).
      platform: unique(params.getAll('platform')),
      region: unique(params.getAll('region')),
      genre: unique(params.getAll('genre')),
      type: unique(params.getAll('type')),
    },
    // In the page URL prices are in euros (price_max=20), because a person reads it.
    priceMin: eurosToCents(params.get('price_min') ?? ''),
    priceMax: eurosToCents(params.get('price_max') ?? ''),
    sort: sortOptions.some((option) => option.value === sort) ? (sort as SortValue) : null,
    page: Number.isInteger(page) && page >= 1 ? page : 1,
  }
}

function unique(values: string[]): string[] {
  return [...new Set(values.filter((value) => value !== ''))]
}

// Writes filters back as the query string of the page URL. Defaults are left out,
// so the plain store is "/store" and not "/store?page=1&sort=...".
export function toSearchParams(filters: StoreFilters): URLSearchParams {
  const params = new URLSearchParams()
  if (filters.text) params.set('text', filters.text)
  for (const key of groupKeys) {
    for (const slug of filters.groups[key]) params.append(key, slug)
  }
  if (filters.priceMin !== null) params.set('price_min', centsToEuros(filters.priceMin))
  if (filters.priceMax !== null) params.set('price_max', centsToEuros(filters.priceMax))
  if (filters.sort) params.set('sort', filters.sort)
  if (filters.page > 1) params.set('page', String(filters.page))
  return params
}

// Builds the query string for GET /api/products. Same names as the page URL, except
// that the API takes prices in cents ("money is integer cents").
export function toApiQuery(filters: StoreFilters): string {
  const params = toSearchParams(filters)
  if (filters.priceMin !== null) params.set('price_min', String(filters.priceMin))
  if (filters.priceMax !== null) params.set('price_max', String(filters.priceMax))
  return params.toString()
}

// The functions below each return a NEW filters object (the input is not changed)
// and reset the page: after any change of filter, page 3 of the old result means
// nothing.

export function toggleGroupValue(filters: StoreFilters, key: GroupKey, slug: string): StoreFilters {
  const current = filters.groups[key]
  const next = current.includes(slug) ? current.filter((value) => value !== slug) : [...current, slug]
  return { ...filters, groups: { ...filters.groups, [key]: next }, page: 1 }
}

export function setPriceRange(filters: StoreFilters, priceMin: number | null, priceMax: number | null): StoreFilters {
  return { ...filters, priceMin, priceMax, page: 1 }
}

export function setSort(filters: StoreFilters, sort: SortValue | null): StoreFilters {
  return { ...filters, sort, page: 1 }
}

export function setPage(filters: StoreFilters, page: number): StoreFilters {
  return { ...filters, page }
}

export function clearText(filters: StoreFilters): StoreFilters {
  // "Best match" has no meaning without a search text.
  return { ...filters, text: '', sort: filters.sort === 'relevance' ? null : filters.sort, page: 1 }
}

// Removes every filter and the search, but keeps the chosen sort order.
export function clearAll(filters: StoreFilters): StoreFilters {
  return {
    text: '',
    groups: { platform: [], region: [], genre: [], type: [] },
    priceMin: null,
    priceMax: null,
    sort: filters.sort === 'relevance' ? null : filters.sort,
    page: 1,
  }
}

export function hasActiveFilters(filters: StoreFilters): boolean {
  return (
    filters.text !== '' ||
    groupKeys.some((key) => filters.groups[key].length > 0) ||
    filters.priceMin !== null ||
    filters.priceMax !== null
  )
}
