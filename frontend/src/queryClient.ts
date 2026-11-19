import { QueryClient } from '@tanstack/react-query'
import { ApiError } from './api.ts'

// How many times a failed read is tried again before its error is shown. With
// TanStack Query's growing delays (1, 2, 4, 8, 16 s) that covers about half a minute:
// the free Render server sleeps when idle and needs up to a minute to wake, and
// during that time requests fail with 502/504 from the proxy. Without these extra
// tries a visitor's first page would often end in an error message.
export const maxRetries = 5

// shouldRetry decides whether a failed read is worth trying again. A 4xx answer is
// the server's considered reply ("not found", "not signed in"): asking again gives
// the same answer, so it is shown at once. Network errors and 5xx are retried.
// (Mutations such as checkout or payment are never retried automatically.)
export function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false
  return failureCount < maxRetries
}

// One QueryClient for the whole app: it owns the cache of server responses, so two
// components asking for the same data share one request.
export const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: shouldRetry } },
})
