import { Link } from 'react-router'

// Shown for every URL that has no page (yet). Header and footer already link to
// pages that are built on later days, so those links land here instead of on a
// blank screen.
export function NotFoundPage() {
  return (
    <div className="py-8 text-center">
      <h1 className="text-heading font-bold">Page not found</h1>
      <p className="mt-2 text-text-muted">This page does not exist, or is not built yet.</p>
      <Link
        to="/"
        className="mt-4 inline-flex h-7 items-center bg-yellow px-2 text-button font-extrabold text-black"
      >
        Back to Homepage
      </Link>
    </div>
  )
}
