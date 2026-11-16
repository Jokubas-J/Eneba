// All calls to the backend go through this file, so there is one place that knows
// how requests are made and how failures are reported.

export type Health = {
  status: string
}

// Fetches a JSON resource from our API. The path is always relative ("/api/..."):
// the dev server and Vercel both forward it to the Go server.
async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path)
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
