import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useCombobox } from 'downshift'
import { type FormEvent, type KeyboardEvent, useState } from 'react'
import { useNavigate } from 'react-router'
import { fetchSuggestions, type ProductCard } from '../api.ts'
import { SearchIcon } from '../components/icons.tsx'
import { RemoteImage } from '../components/RemoteImage.tsx'
import { formatPrice } from '../money.ts'
import {
  buildItems,
  isSearchable,
  itemKey,
  itemToString,
  itemUrl,
  type SearchItem,
  searchStateReducer,
  storeSearchUrl,
} from './searchItems.ts'
import { useDebouncedValue } from './useDebouncedValue.ts'

// Wait this long after the last keystroke before asking the server.
const debounceMs = 200

// The header search box with its dropdown (design-reference.md §5, screenshot 09):
// query suggestions on top, product rows below, "Show all N results" at the end.
//
// The keyboard and screen-reader behaviour of a combobox (arrow keys move through
// the entries, Enter chooses, Escape closes, the highlighted entry is announced) is
// provided by Downshift's useCombobox. It returns "prop getters": functions that
// produce the attributes and event handlers for each element (getInputProps for the
// input, getItemProps for each entry...). Downshift renders nothing itself; the
// markup and styling are ours.
export function SearchBox({ initialText }: { initialText: string }) {
  const navigate = useNavigate()
  // A copy of the box's text, only used to decide what to ask the server. The text
  // in the box itself is owned by Downshift (see inputValue below).
  const [text, setText] = useState(initialText)

  const debouncedText = useDebouncedValue(text.trim(), debounceMs)
  const searchable = isSearchable(debouncedText)

  const suggestions = useQuery({
    // Each distinct text is cached separately: deleting a letter shows the earlier
    // answer at once, without a request.
    queryKey: ['suggest', debouncedText],
    // TanStack Query hands over an AbortSignal. When the key changes while this
    // request is still running, the old query has no one waiting for it and Query
    // aborts it, so a slow answer for "witc" can never replace the one for "witch".
    queryFn: ({ signal }) => fetchSuggestions(debouncedText, signal),
    enabled: searchable,
    // Keep the previous list on screen while the next one loads (no flicker).
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  })

  const items = searchable ? buildItems(suggestions.data, debouncedText) : []

  const { isOpen, highlightedIndex, inputValue, getInputProps, getItemProps, getLabelProps, getMenuProps, closeMenu } =
    useCombobox<SearchItem>({
      items,
      itemToString,
      // Downshift keeps the input's text in its own state. A first version passed
      // it in from our state instead ("controlled"), and fast typing lost letters:
      // Downshift reports the change after its render, so for one render the input
      // was given the old text and the browser's newer text was overwritten.
      initialInputValue: initialText,
      onInputValueChange: ({ inputValue }) => setText(inputValue),
      // The box never keeps a "selected" entry: choosing one navigates away. With
      // selectedItem fixed at null, every choice is reported as a change below,
      // even choosing the same entry twice.
      selectedItem: null,
      onSelectedItemChange: ({ selectedItem }) => {
        if (selectedItem) navigate(itemUrl(selectedItem))
      },
      stateReducer: searchStateReducer,
    })

  // Enter with no entry highlighted submits the form: a full search in the store.
  // (With an entry highlighted, Downshift handles Enter and the form is not submitted.)
  //
  // Downshift (9.5) cancels the form submit on Enter whenever the list is OPEN, even
  // with nothing highlighted, so the first Enter only closed the list. Found by the
  // end-to-end test on day 6. preventDownshiftDefault is Downshift's documented way
  // to skip its own handling of one event; the browser then submits the form.
  function handleInputKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter' && highlightedIndex === -1) {
      // A flag Downshift reads on the browser's event object (not React's wrapper).
      Object.assign(event.nativeEvent, { preventDownshiftDefault: true })
    }
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    closeMenu()
    navigate(storeSearchUrl(inputValue))
  }

  const showMenu = isOpen && searchable && suggestions.data !== undefined
  const nothingFound = showMenu && items.length === 0

  return (
    <>
      {/* The page behind the open dropdown is dimmed, as on eneba.com. It is only
          a picture: a click on it is a click outside the box, which closes the list. */}
      {showMenu && <div className="fixed inset-0 z-10 bg-black/50" aria-hidden="true" />}

      <form onSubmit={handleSubmit} role="search" className="relative z-20 order-last w-full md:order-none md:w-[550px]">
        <label {...getLabelProps()} className="sr-only">
          Search
        </label>
        <SearchIcon className="pointer-events-none absolute top-[28px] left-2 size-5 -translate-y-1/2" />
        <input
          {...getInputProps({
            type: 'search',
            name: 'text',
            placeholder: 'Search for games, top-ups and more',
            onKeyDown: handleInputKeyDown,
          })}
          className={`h-[57px] w-full border border-white pr-2 pl-8 text-search font-extrabold placeholder:text-white focus:outline-2 focus:outline-yellow ${
            showMenu ? 'bg-purple-dark' : 'bg-transparent'
          }`}
        />

        {/* The list element must always be in the page (Downshift points the input's
            aria-controls at it); it is only hidden when closed. */}
        <ul
          {...getMenuProps()}
          className={`absolute inset-x-0 top-full max-h-[70vh] overflow-y-auto bg-purple-light shadow-panel ${
            showMenu && !nothingFound ? '' : 'hidden'
          }`}
        >
          {showMenu &&
            items.map((item, index) => (
              <li
                key={itemKey(item)}
                {...getItemProps({ item, index })}
                className={`cursor-pointer ${highlightedIndex === index ? 'bg-purple-base' : ''} ${
                  item.kind === 'product' && items[index - 1]?.kind === 'query' ? 'mt-2' : ''
                }`}
              >
                <ItemContent item={item} />
              </li>
            ))}
        </ul>
        {nothingFound && (
          <p className="absolute inset-x-0 top-full bg-purple-light p-4 text-sm shadow-panel" role="status">
            No results for “{debouncedText}”. Try a different spelling.
          </p>
        )}
      </form>
    </>
  )
}

function ItemContent({ item }: { item: SearchItem }) {
  switch (item.kind) {
    case 'query':
      return (
        <span className="flex items-center gap-2 border-b border-overlay-row px-4 py-2 text-base">
          <SearchIcon className="size-4 shrink-0" />
          {item.query}
        </span>
      )
    case 'product':
      return <ProductRow product={item.product} />
    case 'all':
      return (
        <span className="block px-4 py-3 text-center text-sm font-extrabold text-yellow">
          Show all {item.total} results
        </span>
      )
  }
}

// A product hit as on eneba.com: cover, platform pill, name, "From" with the old price
// crossed out when discounted, and the price.
function ProductRow({ product }: { product: ProductCard }) {
  const discounted =
    product.discountPercent !== null && product.discountPercent > 0 && product.originalPriceCents !== null

  return (
    <span className="flex items-center gap-3 px-4 py-2">
      <span className="h-[84px] w-[60px] shrink-0 bg-purple-dark">
        <RemoteImage src={product.coverUrl} className="size-full object-cover" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="inline-block rounded-pill border border-teal px-2 text-xs leading-[20px] font-extrabold text-teal">
          {product.platform.name}
        </span>
        <span className="mt-1 line-clamp-2 text-sm leading-[18px] font-extrabold">{product.name}</span>
      </span>
      <span className="shrink-0 text-right">
        {product.priceCents === null ? (
          <span className="text-sm font-extrabold text-text-muted">Sold out</span>
        ) : (
          <>
            <span className="block text-xs font-extrabold text-text-muted">
              From {discounted && <s className="text-text-faded">{formatPrice(product.originalPriceCents!)}</s>}
            </span>
            <span className="block text-lg leading-[24px] font-extrabold">{formatPrice(product.priceCents)}</span>
          </>
        )}
      </span>
    </span>
  )
}
