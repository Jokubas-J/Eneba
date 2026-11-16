import { Link } from 'react-router'
import { pageWindow } from './pageWindow.ts'

type Props = {
  page: number
  pageCount: number
  // Builds the URL of a given page (the store keeps the page number in the URL).
  hrefFor: (page: number) => string
}

// Page links under the grid. They are real links, not buttons: each page of results
// has its own URL, so it can be opened in a new tab and the back button returns to
// the previous page.
export function Pagination({ page, pageCount, hrefFor }: Props) {
  if (pageCount <= 1) return null

  const cell = 'flex h-7 min-w-7 items-center justify-center px-2 text-button font-extrabold'

  return (
    <nav aria-label="Pages" className="mt-8 flex flex-wrap items-center justify-center gap-1">
      {page > 1 && (
        <Link to={hrefFor(page - 1)} className={`${cell} border border-white hover:bg-white hover:text-purple-base`}>
          Previous
        </Link>
      )}

      {pageWindow(page, pageCount).map((p, index) =>
        p === 0 ? (
          // The index is a safe key here: gaps have no identity of their own.
          <span key={`gap-${index}`} className={cell} aria-hidden="true">
            …
          </span>
        ) : p === page ? (
          <span key={p} aria-current="page" className={`${cell} bg-yellow text-black`}>
            {p}
          </span>
        ) : (
          <Link key={p} to={hrefFor(p)} className={`${cell} border border-white hover:bg-white hover:text-purple-base`}>
            {p}
          </Link>
        ),
      )}

      {page < pageCount && (
        <Link to={hrefFor(page + 1)} className={`${cell} border border-white hover:bg-white hover:text-purple-base`}>
          Next
        </Link>
      )}
    </nav>
  )
}
