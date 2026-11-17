// The logic of the search dropdown that does not need React, kept here so it can be
// unit tested: which entries the list contains, where each one leads, and how the
// combobox state reacts to the visitor's actions.

import { type UseComboboxState, type UseComboboxStateChangeOptions, useCombobox } from 'downshift'
import type { ProductCard, SuggestResult } from '../api.ts'

// One selectable entry of the dropdown. A "discriminated union": the kind field
// says which of the three shapes it is, and TypeScript narrows the type after a
// check of kind (like a C# pattern match on subclasses).
export type SearchItem =
  | { kind: 'query'; query: string }
  | { kind: 'product'; product: ProductCard }
  | { kind: 'all'; query: string; total: number }

// The dropdown opens from this many characters, as the backend's suggestions do.
export const minQueryLength = 2

export function isSearchable(text: string): boolean {
  return text.trim().length >= minQueryLength
}

// The entries in display order: query suggestions, product rows, "Show all".
export function buildItems(result: SuggestResult | undefined, typed: string): SearchItem[] {
  if (!result) return []
  const items: SearchItem[] = [
    ...result.suggestions.map((query): SearchItem => ({ kind: 'query', query })),
    ...result.items.map((product): SearchItem => ({ kind: 'product', product })),
  ]
  if (result.total > 0) items.push({ kind: 'all', query: typed.trim(), total: result.total })
  return items
}

export function storeSearchUrl(text: string): string {
  const query = text.trim()
  return query ? `/store?text=${encodeURIComponent(query)}` : '/store'
}

// Where choosing an entry leads.
export function itemUrl(item: SearchItem): string {
  switch (item.kind) {
    case 'query':
      return storeSearchUrl(item.query)
    case 'product':
      return `/product/${item.product.slug}`
    case 'all':
      return storeSearchUrl(item.query)
  }
}

// A stable key for React's list rendering.
export function itemKey(item: SearchItem): string {
  return item.kind === 'product' ? `product-${item.product.id}` : `${item.kind}-${item.query}`
}

// Downshift calls this to turn an item into text, e.g. for the screen-reader
// announcement of the highlighted entry.
export function itemToString(item: SearchItem | null): string {
  if (!item) return ''
  switch (item.kind) {
    case 'query':
      return item.query
    case 'product':
      return item.product.name
    case 'all':
      return `Show all ${item.total} results`
  }
}

const types = useCombobox.stateChangeTypes

// stateReducer adjusts what Downshift would do by default. Downshift is a generic
// combobox; its defaults are made for "pick a value from a list", where the input
// shows the picked value and Escape clears it. A search box behaves differently:
//
//   - Escape closes the list but keeps the typed text.
//   - Leaving the box (blur) closes the list. By default Downshift would also select
//     the highlighted entry, which here would mean navigating away by accident.
//   - Choosing a query suggestion puts it in the box; choosing a product or "Show
//     all" keeps the typed text.
export function searchStateReducer(
  state: UseComboboxState<SearchItem>,
  // Only the two fields that are used, so a test can call it without a full
  // Downshift context.
  { type, changes }: Pick<UseComboboxStateChangeOptions<SearchItem>, 'type' | 'changes'>,
): Partial<UseComboboxState<SearchItem>> {
  switch (type) {
    case types.InputKeyDownEscape:
      return { ...changes, inputValue: state.inputValue, isOpen: false }
    case types.InputBlur:
      return { ...changes, inputValue: state.inputValue, selectedItem: state.selectedItem }
    case types.InputKeyDownEnter:
    case types.ItemClick: {
      const item = changes.selectedItem
      return {
        ...changes,
        inputValue: item?.kind === 'query' ? item.query : state.inputValue,
        isOpen: false,
      }
    }
    default:
      return changes
  }
}
