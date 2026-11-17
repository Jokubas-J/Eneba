import { Route, Routes } from 'react-router'
import { AuthLayout } from './components/AuthLayout.tsx'
import { CheckoutLayout } from './components/CheckoutLayout.tsx'
import { Layout } from './components/Layout.tsx'
import { PageTitle } from './components/PageTitle.tsx'
import { AuthPage } from './pages/AuthPage.tsx'
import { CartPage } from './pages/CartPage.tsx'
import { HomePage } from './pages/HomePage.tsx'
import { NotFoundPage } from './pages/NotFoundPage.tsx'
import { OrderPage } from './pages/OrderPage.tsx'
import { OrdersPage } from './pages/OrdersPage.tsx'
import { ProductPage } from './pages/ProductPage.tsx'
import { SimulatedPaymentPage } from './pages/SimulatedPaymentPage.tsx'
import { StorePage } from './pages/StorePage.tsx'

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
