import { describe, expect, it } from 'vitest'
import {
  clearAll,
  clearText,
  hasActiveFilters,
  parseStoreParams,
  setPage,
  setPriceRange,
  setSort,
  toApiQuery,
  toggleGroupValue,
  toSearchParams,
} from './storeParams.ts'

const parse = (query: string) => parseStoreParams(new URLSearchParams(query))

describe('parseStoreParams', () => {
  it('gives defaults for an empty URL', () => {
    expect(parse('')).toEqual({
      text: '',
      groups: { platform: [], region: [], genre: [], type: [] },
      priceMin: null,
      priceMax: null,
      sort: null,
      page: 1,
    })
  })

  it('reads every kind of parameter', () => {
    const filters = parse(
      'text=witchr+3&platform=steam&platform=gog&region=europe&genre=rpg&type=game&price_min=9.99&price_max=20&sort=price_asc&page=3',
    )
    expect(filters).toEqual({
      text: 'witchr 3',
      groups: { platform: ['steam', 'gog'], region: ['europe'], genre: ['rpg'], type: ['game'] },
      priceMin: 999, // euros in the URL, cents in the data
      priceMax: 2000,
      sort: 'price_asc',
      page: 3,
    })
  })

  it('drops malformed values instead of failing', () => {
    const filters = parse('sort=cheapest&page=-2&price_min=abc&price_max=-5&platform=&platform=steam&platform=steam')
    expect(filters.sort).toBeNull()
    expect(filters.page).toBe(1)
    expect(filters.priceMin).toBeNull()
    expect(filters.priceMax).toBeNull()
    expect(filters.groups.platform).toEqual(['steam']) // empty and repeated values removed
  })

  it.each(['page=1.5', 'page=abc', 'page=0'])('treats %s as page 1', (query) => {
    expect(parse(query).page).toBe(1)
  })
})

describe('URL round trip', () => {
  // The property that makes links shareable: writing the filters to the URL and
  // reading them back gives the same filters.
  it.each([
    '',
    'text=witchr+3',
    'platform=steam&platform=gog&region=europe',
    'genre=rpg&type=game&price_min=9.99&price_max=20&sort=newest&page=4',
    'text=dark+souls&sort=relevance',
  ])('"%s" survives parse -> write -> parse', (query) => {
    const filters = parse(query)
    expect(parseStoreParams(toSearchParams(filters))).toEqual(filters)
    expect(toSearchParams(filters).toString()).toBe(query)
  })

  it('leaves defaults out of the URL', () => {
    expect(toSearchParams(parse('page=1')).toString()).toBe('')
  })
})

describe('toApiQuery', () => {
  it('sends prices in cents and keeps the rest', () => {
    const query = new URLSearchParams(toApiQuery(parse('text=gta&platform=steam&price_min=9.99&price_max=20&page=2')))
    expect(query.get('price_min')).toBe('999')
    expect(query.get('price_max')).toBe('2000')
    expect(query.get('text')).toBe('gta')
    expect(query.getAll('platform')).toEqual(['steam'])
    expect(query.get('page')).toBe('2')
  })

  it('is empty for the plain store', () => {
    expect(toApiQuery(parse(''))).toBe('')
  })
})

describe('changing filters', () => {
  const start = parse('platform=steam&page=3&sort=price_asc')

  it('adds and removes a checkbox value and goes back to page 1', () => {
    const added = toggleGroupValue(start, 'platform', 'gog')
    expect(added.groups.platform).toEqual(['steam', 'gog'])
    expect(added.page).toBe(1)

    const removed = toggleGroupValue(added, 'platform', 'steam')
    expect(removed.groups.platform).toEqual(['gog'])
  })

  it('does not modify the object it was given', () => {
    toggleGroupValue(start, 'platform', 'gog')
    expect(start.groups.platform).toEqual(['steam'])
    expect(start.page).toBe(3)
  })

  it('resets the page for price and sort changes, but not for paging itself', () => {
    expect(setPriceRange(start, 500, null)).toMatchObject({ priceMin: 500, priceMax: null, page: 1 })
    expect(setSort(start, 'newest')).toMatchObject({ sort: 'newest', page: 1 })
    expect(setPage(start, 5).page).toBe(5)
  })

  it('clears everything but keeps the sort order', () => {
    const cleared = clearAll(parse('text=gta&platform=steam&price_max=20&sort=price_asc&page=2'))
    expect(hasActiveFilters(cleared)).toBe(false)
    expect(cleared.sort).toBe('price_asc')
    expect(cleared.page).toBe(1)
  })

  it('drops "best match" when the search text is removed', () => {
    expect(clearText(parse('text=gta&sort=relevance')).sort).toBeNull()
    expect(clearAll(parse('text=gta&sort=relevance')).sort).toBeNull()
    expect(clearText(parse('text=gta&sort=newest')).sort).toBe('newest')
  })

  it('knows whether anything is filtered', () => {
    expect(hasActiveFilters(parse(''))).toBe(false)
    expect(hasActiveFilters(parse('sort=newest&page=2'))).toBe(false) // sorting is not filtering
    expect(hasActiveFilters(parse('text=a'))).toBe(true)
    expect(hasActiveFilters(parse('genre=rpg'))).toBe(true)
    expect(hasActiveFilters(parse('price_min=1'))).toBe(true)
  })
})
