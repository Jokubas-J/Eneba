import { Outlet, useLocation } from 'react-router'
import { ErrorBoundary } from './ErrorBoundary.tsx'
import { PageSuspense } from './PageFallback.tsx'
import { Footer } from './Footer.tsx'
import { Header } from './Header.tsx'

// The frame shared by every page. <Outlet /> is where the router renders the page
// that matches the current URL (like @RenderBody() in an ASP.NET layout).
export function Layout() {
  const { pathname } = useLocation()
  return (
    // A full-height column, with the middle part growing: on a short page the
    // footer still sits at the bottom of the window instead of floating mid-screen.
    <div className="flex min-h-screen flex-col">
      {/* "Skip to content": the first thing a keyboard user reaches; invisible until
          focused. It jumps past the header and the nav strip. */}
      <a
        href="#main"
        className="sr-only bg-yellow px-4 py-2 font-extrabold text-black focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50"
      >
        Skip to content
      </a>
      <Header />
      {/* No width here: each page picks its own with <Page>, because the product
          page is wider than the rest (as on eneba.com). */}
      <main id="main" tabIndex={-1} className="flex-1 py-6 focus:outline-none">
        {/* A crashing page leaves the header and footer usable; going to another
            page (a new pathname) clears the error. */}
        <ErrorBoundary resetKey={pathname}>
          <PageSuspense>
            <Outlet />
          </PageSuspense>
        </ErrorBoundary>
      </main>
      <Footer />
    </div>
  )
}
