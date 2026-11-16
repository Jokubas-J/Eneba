import type { FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { CartIcon, MenuIcon, SearchIcon, UserIcon } from './icons.tsx'

// Navigation is part of the page frame, not homepage content, so it is defined
// here. The links are store filters; the store reads them from the URL.
const navLinks = [
  { label: 'Steam', to: '/store?platform=steam' },
  { label: 'GOG.com', to: '/store?platform=gog' },
  { label: 'Epic Games', to: '/store?platform=epic-games' },
  { label: 'Xbox Live', to: '/store?platform=xbox-live' },
  { label: 'PSN', to: '/store?platform=psn' },
  { label: 'Cheapest first', to: '/store?sort=price_asc' },
]

// Header as measured on eneba.com (design-reference.md §5): a 57px row with logo,
// search and account links, then a 50px darker navigation strip.
export function Header() {
  const navigate = useNavigate()
  // The store keeps the search text in the URL (?text=...). Showing it in the box
  // keeps the two in step, as on eneba.com.
  const [searchParams] = useSearchParams()
  const currentText = searchParams.get('text') ?? ''

  // Enter in the search box opens the store with the query in the URL, so a search
  // can be shared or reloaded. (The instant dropdown is added with the search API.)
  function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault() // stop the browser's own full-page form submit
    const text = new FormData(event.currentTarget).get('text')
    const query = typeof text === 'string' ? text.trim() : ''
    navigate(query ? `/store?text=${encodeURIComponent(query)}` : '/store')
  }

  return (
    <header>
      <div className="mx-auto flex max-w-page flex-wrap items-center gap-4 px-4 py-3">
        {/* A text wordmark, not Eneba's logo image: the logo is their trademark. */}
        <Link to="/" className="text-[32px] leading-none font-extrabold tracking-tight">
          eneba
          <span className="ml-1 align-top text-tiny font-extrabold text-yellow uppercase">assignment</span>
        </Link>

        <form onSubmit={handleSearch} role="search" className="relative order-last w-full md:order-none md:w-[550px]">
          <SearchIcon className="pointer-events-none absolute top-1/2 left-2 size-5 -translate-y-1/2" />
          <input
            // defaultValue only sets the initial text of an input; the key makes
            // React create a new input when the URL's text changes (back button,
            // removing the "Text" chip), so the box follows the URL.
            key={currentText}
            defaultValue={currentText}
            type="search"
            name="text"
            aria-label="Search"
            placeholder="Search for games, top-ups and more"
            className="h-[57px] w-full border border-white bg-transparent pr-2 pl-8 text-search font-extrabold placeholder:text-white focus:outline-2 focus:outline-yellow"
          />
        </form>

        <div className="ml-auto flex items-center gap-2 text-sm font-semibold sm:gap-4">
          <span className="hidden lg:inline">English EU | EUR</span>
          <Link to="/cart" aria-label="Cart" className="hover:text-yellow">
            <CartIcon className="size-5" />
          </Link>
          <span className="flex items-center gap-1">
            <UserIcon className="size-5" />
            <Link to="/login" className="hover:text-yellow">
              Log in
            </Link>
            <span aria-hidden="true">|</span>
            <Link to="/register" className="hover:text-yellow">
              Register
            </Link>
          </span>
        </div>
      </div>

      <nav aria-label="Main" className="bg-overlay-nav">
        <ul className="mx-auto flex h-10 max-w-page items-center gap-2 overflow-x-auto overflow-y-hidden px-2 [scrollbar-width:none] leading-4 font-semibold whitespace-nowrap text-text-nav">
          <li>
            <Link to="/store" className="flex items-center gap-2 p-2 hover:text-white">
              <MenuIcon className="size-5" />
              All games
            </Link>
          </li>
          {navLinks.map((link) => (
            <li key={link.to}>
              <Link to={link.to} className="block p-2 hover:text-white">
                {link.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </header>
  )
}
