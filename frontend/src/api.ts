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

// ApiError is a failed request with its HTTP status and, when the backend sent one,
// its error code and message ({"error": {"code", "message"}}). A page can then tell
// "not found" (404) from "wrong password" (401, invalid_credentials) from "something
// broke" (show a retry button).
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(path: string, status: number, code = '', message = '') {
    super(message || `${path} failed with status ${status}`)
    this.status = status
    this.code = code
  }
}

// Builds an ApiError from a failed response, reading the backend's error body if
// there is one (a proxy error page, for example, is not JSON).
async function toApiError(path: string, response: Response): Promise<ApiError> {
  try {
    const body = (await response.json()) as { error?: { code?: string; message?: string } }
    return new ApiError(path, response.status, body.error?.code ?? '', body.error?.message ?? '')
  } catch {
    return new ApiError(path, response.status)
  }
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
    throw await toApiError(path, response)
  }
  // The cast is a promise to the compiler, not a runtime check: TypeScript types do
  // not exist at runtime, so the shape is only as true as the backend makes it.
  return (await response.json()) as T
}

// Sends a JSON body with POST. The browser adds the Origin header and, because the
// URL is on our own domain, the session cookie; nothing has to be set by hand.
async function postJSON<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: 'POST',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!response.ok) {
    throw await toApiError(path, response)
  }
  // 204 No Content (logout) has no body to parse.
  return (response.status === 204 ? undefined : await response.json()) as T
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

// GET /api/products/{slug} — mirrors catalog.ProductDetail in the backend.
export type Seller = {
  name: string
  rating: number // 0-10
  ratingCount: number
  isVerified: boolean
}

export type Offer = {
  id: number
  seller: Seller
  priceCents: number
  originalPriceCents: number
  discountPercent: number
  stock: number // keys available now; 0 = sold out
  isCheapest: boolean // the cheapest offer that is in stock
}

export type Screenshot = {
  thumbnailUrl: string // 600x338
  fullUrl: string // 1920x1080
}

export type ActivationGuide = {
  notice: string
  steps: string[]
  redeemUrl: string | null
}

export type ProductDetail = {
  id: number
  slug: string
  name: string
  description: string
  type: string
  coverUrl: string
  platform: Named
  region: Named
  genres: Named[]
  releaseDate: string | null // YYYY-MM-DD
  developer: string
  publisher: string
  steamAppId: number | null
  screenshots: Screenshot[]
  activation: ActivationGuide | null // null when there is no guide for the platform
  offers: Offer[]
  otherVersions: ProductCard[] // same game, other platforms/regions (card shape)
}

export function fetchProduct(slug: string, signal?: AbortSignal): Promise<ProductDetail> {
  return getJSON<ProductDetail>(`/api/products/${encodeURIComponent(slug)}`, signal)
}

// --- Accounts (GET/POST /api/auth/...) ---

export type User = {
  id: number
  email: string
}

type UserResponse = { user: User | null }

export async function fetchMe(signal?: AbortSignal): Promise<User | null> {
  return (await getJSON<UserResponse>('/api/auth/me', signal)).user
}

export async function register(email: string, password: string): Promise<User> {
  return (await postJSON<{ user: User }>('/api/auth/register', { email, password })).user
}

export async function login(email: string, password: string): Promise<User> {
  return (await postJSON<{ user: User }>('/api/auth/login', { email, password })).user
}

export function logout(): Promise<void> {
  return postJSON<void>('/api/auth/logout')
}

// --- Cart (POST /api/cart/validate) — mirrors catalog.ValidatedCart ---

export type CartLine = {
  offerId: number
  quantity: number
}

export type CartProblem = '' | 'not_found' | 'sold_out' | 'not_enough_stock'

export type CartItem = {
  offerId: number
  quantity: number
  product: { slug: string; name: string; coverUrl: string; platform: Named; region: Named }
  seller: Seller
  priceCents: number
  originalPriceCents: number
  discountPercent: number
  stock: number
  problem: CartProblem
}

export type ValidatedCart = {
  items: CartItem[]
  totalCents: number // lines without a problem only
  itemCount: number
}

export function validateCart(lines: CartLine[]): Promise<ValidatedCart> {
  return postJSON<ValidatedCart>('/api/cart/validate', { items: lines })
}
