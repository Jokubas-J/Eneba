import { type UseComboboxState, useCombobox } from 'downshift'
import { describe, expect, it } from 'vitest'
import type { ProductCard, SuggestResult } from '../api.ts'
import {
  buildItems,
  isSearchable,
  itemToString,
  itemUrl,
  type SearchItem,
  searchStateReducer,
  storeSearchUrl,
} from './searchItems.ts'

const product: ProductCard = {
  id: 7,
  slug: 'the-witcher-3-pc-steam-key-global',
  name: 'The Witcher 3 (PC) Steam Key GLOBAL',
  type: 'game',
  coverUrl: '',
  platform: { slug: 'steam', name: 'Steam' },
  region: { slug: 'global', name: 'Global' },
  priceCents: 1000,
  originalPriceCents: 2000,
  discountPercent: 50,
  offerCount: 2,
}

const result: SuggestResult = { suggestions: ['witcher', 'witcher 3'], items: [product], total: 12 }

describe('buildItems', () => {
  it('lists suggestions, then products, then "show all"', () => {
    expect(buildItems(result, ' witc ')).toEqual([
      { kind: 'query', query: 'witcher' },
      { kind: 'query', query: 'witcher 3' },
      { kind: 'product', product },
      { kind: 'all', query: 'witc', total: 12 },
    ])
  })

  it('has no "show all" when nothing matches, and nothing before the first answer', () => {
    expect(buildItems({ suggestions: [], items: [], total: 0 }, 'zzz')).toEqual([])
    expect(buildItems(undefined, 'witc')).toEqual([])
  })
})

describe('where an entry leads', () => {
  it.each<[SearchItem, string]>([
    [{ kind: 'query', query: 'gta 5' }, '/store?text=gta%205'],
    [{ kind: 'product', product }, '/product/the-witcher-3-pc-steam-key-global'],
    [{ kind: 'all', query: 'witc', total: 3 }, '/store?text=witc'],
  ])('%o -> %s', (item, url) => {
    expect(itemUrl(item)).toBe(url)
  })

  it('an empty search opens the whole store', () => {
    expect(storeSearchUrl('   ')).toBe('/store')
  })
})

it('opens from two characters', () => {
  expect(isSearchable('g')).toBe(false)
  expect(isSearchable(' g ')).toBe(false)
  expect(isSearchable('gt')).toBe(true)
})

it('describes entries for screen readers', () => {
  expect(itemToString({ kind: 'all', query: 'x', total: 5 })).toBe('Show all 5 results')
  expect(itemToString({ kind: 'product', product })).toBe(product.name)
  expect(itemToString(null)).toBe('')
})

describe('searchStateReducer', () => {
  const types = useCombobox.stateChangeTypes
  const state: UseComboboxState<SearchItem> = {
    inputValue: 'witc',
    isOpen: true,
    highlightedIndex: 2,
    selectedItem: null,
  }
  // What Downshift would do by default for the action, before our reducer.
  const reduce = (type: (typeof types)[keyof typeof types], changes: Partial<UseComboboxState<SearchItem>>) =>
    searchStateReducer(state, { type, changes: { ...state, ...changes } })

  it('keeps the typed text on Escape', () => {
    expect(reduce(types.InputKeyDownEscape, { inputValue: '', isOpen: false })).toMatchObject({
      inputValue: 'witc',
      isOpen: false,
    })
  })

  it('does not choose the highlighted entry when the box loses focus', () => {
    const changes = reduce(types.InputBlur, { selectedItem: { kind: 'product', product }, isOpen: false })
    expect(changes.selectedItem).toBeNull()
    expect(changes.inputValue).toBe('witc')
  })

  it('puts a chosen suggestion into the box', () => {
    const chosen: SearchItem = { kind: 'query', query: 'witcher 3' }
    expect(reduce(types.ItemClick, { selectedItem: chosen, inputValue: 'witcher 3' })).toMatchObject({
      inputValue: 'witcher 3',
      isOpen: false,
    })
  })

  it('keeps the typed text when a product is chosen', () => {
    const chosen: SearchItem = { kind: 'product', product }
    expect(reduce(types.InputKeyDownEnter, { selectedItem: chosen, inputValue: product.name })).toMatchObject({
      inputValue: 'witc',
      selectedItem: chosen,
    })
  })
})
