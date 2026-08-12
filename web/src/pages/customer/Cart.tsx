import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { cartCount, clearCart, estimateTotal, loadCart, saveCart, type Cart } from '../../lib/cart'
import { Button, Card, EmptyState } from '../../components/ui'
import { rupiah } from '../../lib/format'
import { t } from '../../i18n'

export default function CartPage() {
  const copy = t()
  const navigate = useNavigate()
  const [cart, setCart] = useState<Cart>(() => loadCart())

  useEffect(() => {
    const update = () => setCart(loadCart())
    window.addEventListener('ruuma:cart', update)
    return () => window.removeEventListener('ruuma:cart', update)
  }, [])

  function setQty(key: string, qty: number) {
    const next = {
      ...cart,
      lines: cart.lines
        .map((l) => (l.key === key ? { ...l, qty } : l))
        .filter((l) => l.qty > 0),
    }
    saveCart(next)
    setCart(next)
  }

  if (cartCount(cart) === 0) {
    return (
      <div className="flex flex-col gap-4">
        <h1 className="font-display text-2xl font-bold">{copy.cart.title}</h1>
        <EmptyState>{copy.cart.empty}</EmptyState>
        <Link to="/menu" className="text-sm font-medium text-primary-ink underline underline-offset-4">
          {copy.nav.menu}
        </Link>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-display text-2xl font-bold">{copy.cart.title}</h1>

      <ul className="flex flex-col gap-3">
        {cart.lines.map((line) => (
          <li key={line.key}>
            <Card className="flex items-start gap-3">
              <div className="flex-1">
                <p className="font-medium">{line.name}</p>
                {line.optionLabels.length > 0 && (
                  <p className="text-sm text-muted">{line.optionLabels.join(' · ')}</p>
                )}
                {line.notes && <p className="text-sm italic text-muted">“{line.notes}”</p>}
                <p className="tabular pt-1 text-sm">
                  {rupiah((line.unitPrice + line.optionsDelta) * line.qty)}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button variant="secondary" aria-label="-" onClick={() => setQty(line.key, line.qty - 1)}>
                  −
                </Button>
                <span className="tabular w-6 text-center text-sm">{line.qty}</span>
                <Button variant="secondary" aria-label="+" onClick={() => setQty(line.key, line.qty + 1)}>
                  +
                </Button>
              </div>
            </Card>
          </li>
        ))}
      </ul>

      <Card className="flex items-center justify-between">
        <span className="text-sm text-muted">{copy.common.subtotal}</span>
        {/* An estimate only: the server prices the order (BR-2.5.13). */}
        <span className="tabular text-lg font-semibold">{rupiah(estimateTotal(cart))}</span>
      </Card>

      <div className="flex gap-2">
        <Button variant="secondary" onClick={() => { clearCart(); setCart(loadCart()) }}>
          {copy.common.remove}
        </Button>
        <Button className="flex-1" onClick={() => navigate('/checkout')}>
          {copy.cart.checkout}
        </Button>
      </div>
    </div>
  )
}
