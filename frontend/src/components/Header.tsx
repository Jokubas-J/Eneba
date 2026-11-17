import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useLocation, useSearchParams } from 'react-router'
import { fetchMenu } from '../api.ts'
import { useCurrentUser, useLogout } from '../auth/useAuth.ts'
import { useCart } from '../cart/useCart.ts'
import { SearchBox } from '../search/SearchBox.tsx'
import { AccountMenu } from './AccountMenu.tsx'
import { CategoriesDrawer } from './CategoriesDrawer.tsx'
import { CartIcon, MenuIcon, UserIcon } from './icons.tsx'

// Header as measured on eneba.com (design-reference.md §5): a 57px row with logo,
// search and account links, then a 50px darker navigation strip.
// Log in | Register for visitors; the account menu (e-mail, My orders, Log out) when
// signed in.
// The login link carries the current page in ?next=, so signing in returns here.
function AccountLinks() {
  const me = useCurrentUser()
  const logout = useLogout()
  const location = useLocation()
  const next = encodeURIComponent(location.pathname + location.search)

  // Until the server has answered, show nothing rather than "Log in", which would
  // flash for a moment for someone who is signed in.
  if (me.isPending) return <span className="skeleton block h-4 w-[120px]" aria-hidden="true" />
  if (me.data) return <AccountMenu email={me.data.email} onLogout={() => logout.mutate()} />
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
  // The menu (drawer sections and nav strip links) comes from GET /api/menu.
  const menu = useQuery({ queryKey: ['menu'], queryFn: fetchMenu })
  const [drawerOpen, setDrawerOpen] = useState(false)
  const stripLinks = menu.data?.filter((s) => s.placement === 'strip').flatMap((s) => s.links) ?? []
  const drawerSections = menu.data?.filter((s) => s.placement === 'drawer') ?? []

  return (
    <header>
      {/* One row on tablets and up (logo, search, account); on phones the search box
          moves to a row of its own below. */}
      <div className="mx-auto flex max-w-page flex-wrap items-center gap-4 px-4 py-3 md:flex-nowrap">
        {/* A text wordmark, not Eneba's logo image: the logo is their trademark. */}
        <Link to="/" className="text-[32px] leading-none font-extrabold tracking-tight">
          eneba
          <span className="ml-1 align-top text-tiny font-extrabold text-yellow uppercase">assignment</span>
        </Link>

        {/* initialText only sets the starting text; the key makes React create a
            fresh search box when the URL's text changes (back button, removing the
            "Text" chip, opening a product), so the box follows the URL. */}
        <SearchBox key={currentText} initialText={currentText} />

        <div className="ml-auto flex shrink-0 items-center gap-2 text-sm font-semibold whitespace-nowrap sm:gap-4">
          <span className="hidden xl:inline">English EU | EUR</span>
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
            {/* The button itself is page chrome; what the drawer lists is data. */}
            <button
              type="button"
              onClick={() => setDrawerOpen(true)}
              aria-haspopup="dialog"
              className="flex items-center gap-2 p-2 font-semibold hover:text-white"
            >
              <MenuIcon className="size-5" />
              Categories
            </button>
          </li>
          {/* While the menu loads, a few placeholder bars keep the strip's shape. */}
          {menu.isPending &&
            [70, 80, 90, 80, 50].map((width, i) => (
              <li key={i} aria-hidden="true" className="p-2">
                <span className="skeleton block h-3" style={{ width }} />
              </li>
            ))}
          {stripLinks.map((link) => (
            <li key={link.url} className="animate-fade-in">
              <Link to={link.url} className="block p-2 transition-colors hover:text-white">
                {link.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
      {drawerOpen && (
        <CategoriesDrawer
          sections={drawerSections}
          state={menu.isError ? 'error' : menu.data ? 'ready' : 'loading'}
          onRetry={() => menu.refetch()}
          onClose={() => setDrawerOpen(false)}
        />
      )}
    </header>
  )
}
