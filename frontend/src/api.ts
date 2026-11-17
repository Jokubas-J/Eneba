// All calls to the backend go through this file, so there is one place that knows
// how requests are made and how failures are reported.
//
// The types below mirror the JSON written by the Go structs in
// backend/internal/catalog/model.go. Nothing checks at runtime that they agree; the
// backend's JSON-shape test is what keeps that side of the contract fixed.

export type Health = {
  status: string
}

export type Named = {
  slug: string
  name: string
}

export type ProductCard = {
  id: number
  slug: string
  name: string
  type: string
  coverUrl: string
  platform: Named
  region: Named
  // The three price fields are null when no offer is in stock (sold out).
  priceCents: number | null
  originalPriceCents: number | null
  discountPercent: number | null
  offerCount: number
}

export type FacetOption = {
  slug: string
  name: string
  count: number
}

export type Facets = {
  platforms: FacetOption[]
  regions: FacetOption[]
  genres: FacetOption[]
  types: FacetOption[]
}

export type ProductList = {
  items: ProductCard[]
  total: number
  page: number
  pageSize: number
  facets: Facets
}

// Fetches a JSON resource from our API. The path is always relative ("/api/..."):
// the dev server and Vercel both forward it to the Go server.
//
// signal lets the caller cancel the request (AbortController). TanStack Query passes
// one in and aborts it when the result is no longer wanted, e.g. because the visitor
// typed another letter before the answer arrived.
async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { signal })
  // fetch() only rejects on network failure. A 404 or 500 is a "successful" fetch,
  // so the status has to be checked by hand or errors would be parsed as data.
  if (!response.ok) {
    throw new Error(`GET ${path} failed with status ${response.status}`)
  }
  // The cast is a promise to the compiler, not a runtime check: TypeScript types do
  // not exist at runtime, so the shape is only as true as the backend makes it.
  return (await response.json()) as T
}

export function fetchHealth(): Promise<Health> {
  return getJSON<Health>('/api/healthz')
}

// query is an already encoded query string without the "?", e.g. "platform=steam&page=2".
export function fetchProducts(query: string, signal?: AbortSignal): Promise<ProductList> {
  return getJSON<ProductList>(query ? `/api/products?${query}` : '/api/products', signal)
}

// The content of the search dropdown (GET /api/search/suggest).
export type SuggestResult = {
  suggestions: string[] // search queries, shown on top
  items: ProductCard[] // best few product matches
  total: number // number of all matches, for "Show all N results"
}

export function fetchSuggestions(text: string, signal?: AbortSignal): Promise<SuggestResult> {
  return getJSON<SuggestResult>(`/api/search/suggest?q=${encodeURIComponent(text)}`, signal)
}
