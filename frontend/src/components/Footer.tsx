import { Link } from 'react-router'

// Footer columns. Eneba's footer links to company pages that this project does not
// have, so the columns list what actually exists here.
const columns = [
  {
    title: 'Buy',
    links: [
      { label: 'Store', to: '/store' },
      { label: 'Cart', to: '/cart' },
    ],
  },
  {
    title: 'Account',
    links: [
      { label: 'Log in', to: '/login' },
      { label: 'Register', to: '/register' },
      { label: 'My orders', to: '/orders' },
    ],
  },
]

// Footer as on eneba.com (design-reference.md §5): a white strip about payments,
// then link columns and a legal line on a near-black purple.
export function Footer() {
  return (
    <footer className="mt-8">
      <div className="bg-white text-black">
        <p className="mx-auto max-w-page px-4 py-2 text-xs font-semibold">
          Demo shop: payments run in Stripe test mode. No real money is charged and the keys are not real.
        </p>
      </div>

      <div className="bg-overlay-footer">
        <div className="mx-auto max-w-page px-4 py-8">
          <div className="flex flex-wrap gap-8">
            <section>
              <h2 className="text-sm leading-[21px] font-extrabold">About</h2>
              <ul className="mt-2 text-xs leading-[18px] text-text-nav">
                <li className="py-0.5">
                  {/* An external site: a plain <a>, not the router's <Link>. */}
                  <a href="https://github.com/Jokubas-J/Eneba" className="hover:text-white">
                    Source code on GitHub
                  </a>
                </li>
                <li className="py-0.5">
                  <a href="https://store.steampowered.com/" className="hover:text-white">
                    Game data: Steam store
                  </a>
                </li>
              </ul>
            </section>

            {columns.map((column) => (
              <section key={column.title}>
                <h2 className="text-sm leading-[21px] font-extrabold">{column.title}</h2>
                <ul className="mt-2 text-xs leading-[18px] text-text-nav">
                  {column.links.map((link) => (
                    <li key={link.to} className="py-0.5">
                      <Link to={link.to} className="hover:text-white">
                        {link.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>

          <p className="mt-8 text-xs leading-[18px] font-semibold text-text-muted">
            Internship assignment project, not affiliated with or endorsed by Eneba. Game titles, descriptions and
            cover images belong to their owners and come from the Steam store; sellers, prices, stock and keys are
            invented demo data.
          </p>
        </div>
      </div>
    </footer>
  )
}
