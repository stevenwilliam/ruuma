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

// The whole admin area is a separate chunk: a customer never downloads it
// (docs/05 §1, vite.config.ts).
const AdminApp = lazy(() => import('./pages/admin/AdminApp'))

export default function App() {
  const copy = t()
  return (
    <Suspense fallback={<Spinner label={copy.common.loading} />}>
      <Routes>
        <Route element={<CustomerLayout />}>
          <Route path="/" element={<StoresPage />} />
          <Route path="/menu" element={<MenuPage />} />
          <Route path="/menu/:id" element={<ItemPage />} />
          <Route path="/cart" element={<CartPage />} />
          <Route path="/checkout" element={<CheckoutPage />} />
          <Route path="/orders" element={<OrdersPage />} />
          <Route path="/orders/:id" element={<OrderPage />} />
          <Route path="/signin" element={<AuthPage />} />
        </Route>
        <Route path="/admin/*" element={<AdminApp />} />
      </Routes>
    </Suspense>
  )
}
