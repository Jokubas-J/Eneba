import { Outlet } from 'react-router'
import { Footer } from './Footer.tsx'
import { Header } from './Header.tsx'

// The frame shared by every page. <Outlet /> is where the router renders the page
// that matches the current URL (like @RenderBody() in an ASP.NET layout).
export function Layout() {
  return (
    // A full-height column, with the middle part growing: on a short page the
    // footer still sits at the bottom of the window instead of floating mid-screen.
    <div className="flex min-h-screen flex-col">
      <Header />
      {/* No width here: each page picks its own with <Page>, because the product
          page is wider than the rest (as on eneba.com). */}
      <main className="flex-1 py-6">
        <Outlet />
      </main>
      <Footer />
    </div>
  )
}
