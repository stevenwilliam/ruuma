import { Suspense, lazy } from 'react'
import { Route, Routes } from 'react-router-dom'
import { CustomerLayout } from './components/Layout'
import { Spinner } from './components/ui'
import { t } from './i18n'

import StoresPage from './pages/customer/Stores'
import MenuPage from './pages/customer/Menu'
import ItemPage from './pages/customer/Item'
import CartPage from './pages/customer/Cart'
import CheckoutPage from './pages/customer/Checkout'
import OrderPage from './pages/customer/Order'
import OrdersPage from './pages/customer/Orders'
import AuthPage from './pages/customer/Auth'
import VerifyEmailPage from './pages/customer/VerifyEmail'
import NotFoundPage from './pages/customer/NotFound'

// Lazy: the credits page carries the whole photo manifest and almost nobody
// opens it, but the licences require it to be reachable from every page.
const CreditsPage = lazy(() => import('./pages/customer/Credits'))

// The whole admin area is a separate chunk: a customer never downloads it
// (docs/05 §1, vite.config.ts).
const AdminApp = lazy(() => import('./pages/admin/AdminApp'))

export default function App() {
  const copy = t()
  return (
    <Suspense fallback={<Spinner label={copy.common.loading} />}>
      <Routes>
        <Route element={<CustomerLayout />}>
          {/* Home is the menu, not the store picker: ruuma runs a single
              outlet (D30), so asking which store to order from was a step
              that only ever had one answer. The picker stays reachable at
              /stores for when a second outlet opens. */}
          <Route path="/" element={<MenuPage />} />
          <Route path="/menu" element={<MenuPage />} />
          <Route path="/stores" element={<StoresPage />} />
          <Route path="/menu/:id" element={<ItemPage />} />
          <Route path="/cart" element={<CartPage />} />
          <Route path="/checkout" element={<CheckoutPage />} />
          <Route path="/orders" element={<OrdersPage />} />
          <Route path="/orders/:id" element={<OrderPage />} />
          <Route path="/signin" element={<AuthPage />} />
          <Route path="/credits" element={<CreditsPage />} />
          {/* The path registration emails link to. authsvc builds
              {baseURL}/verify-email?token=…, which is this route and not the
              API's /api/v1/auth/verify-email. Without it every verification
              email landed on an empty page. */}
          <Route path="/verify-email" element={<VerifyEmailPage />} />
          {/* Catch-all. nginx serves index.html for every path, so without a
              wildcard an unknown URL rendered the chrome around nothing. */}
          <Route path="*" element={<NotFoundPage />} />
        </Route>
        <Route path="/admin/*" element={<AdminApp />} />
      </Routes>
    </Suspense>
  )
}
