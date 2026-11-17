import { Link, Outlet } from 'react-router'

// The frame of the sign-in pages, as on eneba.com (measured with Playwright on
// my.eneba.com/login): a standalone page without the shop header, navigation strip or
// footer; only the logo at the top left and a small row of links at the bottom. The
// page is darker than the shop (purple-deep) on wide screens; on a phone the base
// purple, with everything stacked.
export function AuthLayout() {
  return (
    <div className="flex min-h-screen flex-col bg-purple-base lg:bg-purple-deep">
      <header className="px-6 pt-6">
        <Link to="/" className="text-[40px] leading-none font-extrabold tracking-tight">
          eneba
          <span className="ml-1 align-top text-tiny font-extrabold text-yellow uppercase">assignment</span>
        </Link>
      </header>

      <main className="flex flex-1 items-center py-10">
        <Outlet />
      </main>

      <footer className="px-6 pb-6">
        <ul className="flex flex-col items-center gap-3 text-sm font-semibold sm:flex-row sm:flex-wrap sm:justify-center sm:gap-5">
          <li>
            <Link to="/" className="hover:text-yellow">
              Home
            </Link>
          </li>
          <li>
            <Link to="/store" className="hover:text-yellow">
              Store
            </Link>
          </li>
          <li>
            <a href="https://github.com/Jokubas-J/Eneba" className="hover:text-yellow">
              Source code
            </a>
          </li>
          <li className="font-normal text-text-muted">Demo shop, not affiliated with Eneba.</li>
        </ul>
      </footer>
    </div>
  )
}
