// @vitest-environment jsdom
// (A browser-like DOM for this file only; the other tests are plain functions.)
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { HomePage } from './HomePage.tsx'

const card = (id: number, name: string) => ({
  id,
  slug: `game-${id}`,
  name,
  type: 'game',
  coverUrl: '',
  platform: { slug: 'steam', name: 'Steam' },
  region: { slug: 'global', name: 'Global' },
  priceCents: 999,
  originalPriceCents: 1999,
  discountPercent: 50,
  offerCount: 2,
})

// The homepage the "API" returns: one block type the frontend has never heard of,
// two carousels (the second one's endpoint is broken) and the category grid.
const layout = [
  { id: 1, type: 'hologram_wall', title: 'From the future', source: '/api/home/hologram', showAllUrl: null },
  { id: 2, type: 'product_carousel', title: 'Best sellers', source: '/api/home/best-sellers', showAllUrl: '/store?sort=popular' },
  { id: 3, type: 'product_carousel', title: 'Biggest discounts', source: '/api/home/deals', showAllUrl: '/store?sort=discount' },
  { id: 4, type: 'category_grid', title: 'Categories', source: '/api/home/categories', showAllUrl: '/store' },
]

function stubApi(failing: Set<string>) {
  const calls: string[] = []
  vi.stubGlobal('fetch', async (input: string) => {
    calls.push(input)
    const json = (status: number, body: unknown) =>
      new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
    if (failing.has(input)) return json(500, { error: { code: 'internal', message: 'Something went wrong.' } })
    switch (input) {
      case '/api/home':
        return json(200, { items: layout })
      case '/api/home/best-sellers':
        return json(200, { items: [card(1, 'Elden Ring'), card(2, 'Hades')] })
      case '/api/home/deals':
        return json(200, { items: [card(3, 'Celeste')] })
      case '/api/home/categories':
        return json(200, { items: [{ slug: 'action', name: 'Action', count: 321 }] })
    }
    return json(404, { error: { code: 'not_found', message: 'nope' } })
  })
  return calls
}

function renderHome() {
  // retry: false so a failing block shows its error at once instead of retrying.
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('HomePage', () => {
  it('renders the blocks the API lists, in order, and skips an unknown block type', async () => {
    const calls = stubApi(new Set())
    renderHome()

    expect(await screen.findByText('Elden Ring')).toBeTruthy()
    expect(await screen.findByText('Celeste')).toBeTruthy()
    expect(await screen.findByText('Action')).toBeTruthy()

    const headings = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(headings).toEqual(['Best sellers', 'Biggest discounts', 'Categories'])
    expect(screen.queryByText('From the future')).toBeNull()
    // The unknown block's data is not even requested.
    expect(calls).not.toContain('/api/home/hologram')
  })

  it('shows an error in a failing block only, and its retry reloads just that block', async () => {
    const failing = new Set(['/api/home/deals'])
    const calls = stubApi(failing)
    renderHome()

    expect(await screen.findByText('This section could not be loaded.')).toBeTruthy()
    // The blocks before and after the broken one are fine.
    expect(await screen.findByText('Elden Ring')).toBeTruthy()
    expect(await screen.findByText('Action')).toBeTruthy()
    // A block in its error state offers no "Show all" for content it could not show.
    expect(screen.getAllByText('Show all')).toHaveLength(2)

    failing.clear()
    const before = calls.length
    fireEvent.click(screen.getByText('Try again'))
    expect(await screen.findByText('Celeste')).toBeTruthy()
    expect(calls.slice(before)).toEqual(['/api/home/deals'])
  })

  it('refuses a block source outside the homepage API', async () => {
    layout.push({ id: 5, type: 'product_carousel', title: 'Elsewhere', source: 'https://evil.example/x', showAllUrl: null })
    try {
      const calls = stubApi(new Set())
      renderHome()
      expect(await screen.findByText('Elden Ring')).toBeTruthy()
      expect(calls).not.toContain('https://evil.example/x')
    } finally {
      layout.pop()
    }
  })
})
