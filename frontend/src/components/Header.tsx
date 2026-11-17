import { Link, useLocation, useSearchParams } from 'react-router'
import { useCurrentUser, useLogout } from '../auth/useAuth.ts'
import { useCart } from '../cart/useCart.ts'
import { SearchBox } from '../search/SearchBox.tsx'
import { CartIcon, MenuIcon, UserIcon } from './icons.tsx'

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
// Log in | Register for visitors; the e-mail address and "Log out" when signed in.
// The login link carries the current page in ?next=, so signing in returns here.
function AccountLinks() {
  const me = useCurrentUser()
  const logout = useLogout()
  const location = useLocation()
  const next = encodeURIComponent(location.pathname + location.search)

  // Until the server has answered, show nothing rather than "Log in", which would
  // flash for a moment for someone who is signed in.
  if (me.isPending) return <span className="w-[120px]" aria-hidden="true" />
  if (me.data) {
    return (
      <span className="flex items-center gap-2">
        <UserIcon className="size-5 shrink-0" />
        <span className="hidden max-w-[160px] truncate md:inline" title={me.data.email}>
          {me.data.email}
        </span>
        <span aria-hidden="true" className="hidden md:inline">
          |
        </span>
        <button type="button" onClick={() => logout.mutate()} className="hover:text-yellow">
          Log out
        </button>
      </span>
    )
  }
  return (
    <span className="flex items-center gap-1">
      <UserIcon className="size-5" />
      <Link to={`/login?next=${next}`} className="hover:text-yellow">
        Log in
      </Link>
      <span aria-hidden="true">|</span>
      <Link to={`/register?next=${next}`} className="hover:text-yellow">
        Register
      </Link>
    </span>
  )
}

export function Header() {
  const { itemCount } = useCart()
  // The store keeps the search text in the URL (?text=...). Showing it in the box
  // keeps the two in step, as on eneba.com.
  const [searchParams] = useSearchParams()
  const currentText = searchParams.get('text') ?? ''

  return (
    <header>
      <div className="mx-auto flex max-w-page flex-wrap items-center gap-4 px-4 py-3">
        {/* A text wordmark, not Eneba's logo image: the logo is their trademark. */}
        <Link to="/" className="text-[32px] leading-none font-extrabold tracking-tight">
          eneba
          <span className="ml-1 align-top text-tiny font-extrabold text-yellow uppercase">assignment</span>
        </Link>

        {/* initialText only sets the starting text; the key makes React create a
            fresh search box when the URL's text changes (back button, removing the
            "Text" chip, opening a product), so the box follows the URL. */}
        <SearchBox key={currentText} initialText={currentText} />

        <div className="ml-auto flex items-center gap-2 text-sm font-semibold sm:gap-4">
          <span className="hidden lg:inline">English EU | EUR</span>
          <Link
            to="/cart"
            aria-label={itemCount > 0 ? `Cart, ${itemCount} items` : 'Cart'}
            className="relative hover:text-yellow"
          >
            <CartIcon className="size-5" />
            {/* The small count bubble on the cart icon. Red is not one of the design
                tokens; the pink accent is the closest colour in the palette. */}
            {itemCount > 0 && (
              <span className="absolute -top-2 -right-2 flex size-4 items-center justify-center rounded-full bg-pink-region text-[10px] leading-none font-extrabold text-white">
                {itemCount}
              </span>
            )}
          </Link>
          <AccountLinks />
        </div>
      </div>

      <nav aria-label="Main" className="bg-overlay-nav">
        <ul className="mx-auto flex h-10 max-w-page items-center gap-2 overflow-x-auto overflow-y-hidden px-2 [scrollbar-width:none] leading-[16px] font-semibold whitespace-nowrap text-text-nav">
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
