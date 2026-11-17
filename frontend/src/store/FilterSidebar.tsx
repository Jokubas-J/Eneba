import { type FormEvent, type ReactNode, useState } from 'react'
import type { FacetOption, Facets } from '../api.ts'
import { centsToEuros, eurosToCents } from '../money.ts'
import type { GroupKey, StoreFilters } from './storeParams.ts'

// Which facet list of the API response feeds which checkbox group, and its title.
const groups: { key: GroupKey; title: string; facet: keyof Facets }[] = [
  { key: 'type', title: 'Product type', facet: 'types' },
  { key: 'platform', title: 'Platforms', facet: 'platforms' },
  { key: 'genre', title: 'Genres', facet: 'genres' },
  { key: 'region', title: 'Regions', facet: 'regions' },
]

type Props = {
  filters: StoreFilters
  // undefined while the very first response is still loading.
  facets: Facets | undefined
  onToggle: (key: GroupKey, slug: string) => void
  onPriceChange: (priceMin: number | null, priceMax: number | null) => void
}

// The filter column of the store (design-reference.md §5, screenshots 08 and 10).
// It owns no filter state: what is ticked comes from the URL through "filters", and
// a click only reports upwards. The counts come from the API with every response.
export function FilterSidebar({ filters, facets, onToggle, onPriceChange }: Props) {
  return (
    <div className="flex flex-col gap-2">
      <Panel title="Price range (EUR)">
        {/* The key makes React build a fresh PriceRange when the bounds in the URL
            change (back button, "Clear all"), which resets what is typed in the
            boxes to the new values. */}
        <PriceRange
          key={`${filters.priceMin}-${filters.priceMax}`}
          priceMin={filters.priceMin}
          priceMax={filters.priceMax}
          onChange={onPriceChange}
        />
      </Panel>

      {groups.map((group) => (
        <Panel key={group.key} title={group.title}>
          <CheckboxList
            name={group.key}
            options={facets?.[group.facet]}
            selected={filters.groups[group.key]}
            onToggle={(slug) => onToggle(group.key, slug)}
          />
        </Panel>
      ))}
    </div>
  )
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    // <fieldset>/<legend> is the HTML for "a group of controls with a caption":
    // a screen reader announces "Platforms, group" before the checkboxes.
    <fieldset className="min-w-0 bg-purple-light p-3">
      <legend className="float-left mb-2 w-full text-base leading-[22.4px] font-extrabold">{title}</legend>
      <div className="clear-both">{children}</div>
    </fieldset>
  )
}

type CheckboxListProps = {
  name: string
  options: FacetOption[] | undefined
  selected: string[]
  onToggle: (slug: string) => void
}

function CheckboxList({ name, options, selected, onToggle }: CheckboxListProps) {
  if (!options) {
    return <div className="skeleton h-4" aria-hidden="true" />
  }
  return (
    <ul className="flex flex-col gap-2">
      {options.map((option) => {
        const checked = selected.includes(option.slug)
        // An option that would give no results cannot be ticked, but a ticked one
        // can always be unticked.
        const disabled = option.count === 0 && !checked
        return (
          <li key={option.slug}>
            <label className={`flex items-center gap-2 text-sm ${disabled ? 'opacity-50' : 'cursor-pointer'}`}>
              <input
                type="checkbox"
                name={name}
                value={option.slug}
                checked={checked}
                disabled={disabled}
                onChange={() => onToggle(option.slug)}
                className="checkbox"
              />
              <span className="min-w-0 flex-1 truncate">{option.name}</span>
              <span className="text-text-faded">{option.count}</span>
            </label>
          </li>
        )
      })}
    </ul>
  )
}

type PriceRangeProps = {
  priceMin: number | null
  priceMax: number | null
  onChange: (priceMin: number | null, priceMax: number | null) => void
}

// Two boxes "min - max" in euros. Unlike the checkboxes, typing does not filter on
// every keystroke (typing "20" would first filter by 2): the range is applied when
// the visitor presses Enter or leaves a box.
function PriceRange({ priceMin, priceMax, onChange }: PriceRangeProps) {
  // The text being typed is the one piece of local state in the sidebar; it becomes
  // a filter (and part of the URL) only when applied.
  const [minText, setMinText] = useState(priceMin === null ? '' : centsToEuros(priceMin))
  const [maxText, setMaxText] = useState(priceMax === null ? '' : centsToEuros(priceMax))

  function apply() {
    let nextMin = eurosToCents(minText)
    let nextMax = eurosToCents(maxText)
    // "20 - 10" is read as "10 - 20" rather than rejected.
    if (nextMin !== null && nextMax !== null && nextMin > nextMax) {
      ;[nextMin, nextMax] = [nextMax, nextMin]
    }
    if (nextMin !== priceMin || nextMax !== priceMax) onChange(nextMin, nextMax)
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    apply()
  }

  const inputClass =
    'h-8 w-full min-w-0 bg-overlay-input px-2 text-sm inset-shadow-input placeholder:text-text-muted focus:outline-2 focus:outline-yellow'

  return (
    <form onSubmit={handleSubmit} className="flex items-center gap-2">
      <input
        inputMode="decimal"
        aria-label="Minimum price in euros"
        placeholder="min"
        value={minText}
        onChange={(event) => setMinText(event.target.value)}
        onBlur={apply}
        className={inputClass}
      />
      <span aria-hidden="true">-</span>
      <input
        inputMode="decimal"
        aria-label="Maximum price in euros"
        placeholder="max"
        value={maxText}
        onChange={(event) => setMaxText(event.target.value)}
        onBlur={apply}
        className={inputClass}
      />
      {/* A form submits on Enter only if it has a submit button; this one is
          visually hidden but still there for the keyboard. */}
      <button type="submit" className="sr-only">
        Apply price range
      </button>
    </form>
  )
}
