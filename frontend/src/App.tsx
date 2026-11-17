import { Route, Routes } from 'react-router'
import { AuthLayout } from './components/AuthLayout.tsx'
import { CheckoutLayout } from './components/CheckoutLayout.tsx'
import { Layout } from './components/Layout.tsx'
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
        <Route path="/" element={<HomePage />} />
        <Route path="/store" element={<StorePage />} />
        <Route path="/product/:slug" element={<ProductPage />} />
        <Route path="/orders" element={<OrdersPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
      <Route element={<AuthLayout />}>
        <Route path="/login" element={<AuthPage mode="login" />} />
        <Route path="/register" element={<AuthPage mode="register" />} />
      </Route>
      <Route element={<CheckoutLayout step={1} />}>
        <Route path="/cart" element={<CartPage />} />
      </Route>
      <Route element={<CheckoutLayout step={2} />}>
        <Route path="/checkout/simulated/:id" element={<SimulatedPaymentPage />} />
      </Route>
      <Route element={<CheckoutLayout step={3} />}>
        <Route path="/orders/:id" element={<OrderPage />} />
      </Route>
    </Routes>
  )
}

export default App
