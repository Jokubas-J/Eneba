import { lazy } from 'react'
import { Route, Routes } from 'react-router'
import { AuthLayout } from './components/AuthLayout.tsx'
import { CheckoutLayout } from './components/CheckoutLayout.tsx'
import { Layout } from './components/Layout.tsx'
import { PageTitle } from './components/PageTitle.tsx'
import { HomePage } from './pages/HomePage.tsx'

// Every page except the homepage is a separate file the browser downloads only when
// that page is first opened (code splitting), so the first visit loads less.
// React.lazy expects a default export; our pages are named exports, hence the .then.
const AuthPage = lazy(() => import('./pages/AuthPage.tsx').then((m) => ({ default: m.AuthPage })))
const CartPage = lazy(() => import('./pages/CartPage.tsx').then((m) => ({ default: m.CartPage })))
const NotFoundPage = lazy(() => import('./pages/NotFoundPage.tsx').then((m) => ({ default: m.NotFoundPage })))
const OrderPage = lazy(() => import('./pages/OrderPage.tsx').then((m) => ({ default: m.OrderPage })))
const OrdersPage = lazy(() => import('./pages/OrdersPage.tsx').then((m) => ({ default: m.OrdersPage })))
const ProductPage = lazy(() => import('./pages/ProductPage.tsx').then((m) => ({ default: m.ProductPage })))
const SimulatedPaymentPage = lazy(() =>
  import('./pages/SimulatedPaymentPage.tsx').then((m) => ({ default: m.SimulatedPaymentPage })),
)
const StorePage = lazy(() => import('./pages/StorePage.tsx').then((m) => ({ default: m.StorePage })))

// The route table: which page component belongs to which URL. The shop pages share
// Layout (header and footer); the sign-in pages AuthLayout (logo and a link row);
// the checkout pages CheckoutLayout (logo and progress bar), as on eneba.com.
function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<><PageTitle /><HomePage /></>} />
        <Route path="/store" element={<StorePage />} />
        <Route path="/product/:slug" element={<ProductPage />} />
        <Route path="/orders" element={<><PageTitle title="My orders" /><OrdersPage /></>} />
        <Route path="*" element={<><PageTitle title="Page not found" /><NotFoundPage /></>} />
      </Route>
      <Route element={<AuthLayout />}>
        <Route path="/login" element={<><PageTitle title="Log in" /><AuthPage mode="login" /></>} />
        <Route path="/register" element={<><PageTitle title="Register" /><AuthPage mode="register" /></>} />
      </Route>
      <Route element={<CheckoutLayout step={1} />}>
        <Route path="/cart" element={<><PageTitle title="Cart" /><CartPage /></>} />
      </Route>
      <Route element={<CheckoutLayout step={2} />}>
        <Route path="/checkout/simulated/:id" element={<><PageTitle title="Payment" /><SimulatedPaymentPage /></>} />
      </Route>
      <Route element={<CheckoutLayout step={3} />}>
        <Route path="/orders/:id" element={<OrderPage />} />
      </Route>
    </Routes>
  )
}

export default App
