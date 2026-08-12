// The admin area is a separate lazy chunk and a separate router group, matching
// the server's separate admin route group (docs/12, A01).

import { useEffect, useState } from 'react'
import { NavLink, Route, Routes, useNavigate } from 'react-router-dom'
import { api, tokens, type Staff } from '../../lib/api'
import { Button, Card, ErrorNote, Field, Spinner, TextInput } from '../../components/ui'
import { LanguagePicker } from '../../components/LanguagePicker'
import { t } from '../../i18n'

import Dashboard from './Dashboard'
import OrdersBoard from './OrdersBoard'
import FinanceQueue from './FinanceQueue'
import StoresAdmin from './StoresAdmin'
import ScheduleAdmin from './ScheduleAdmin'
import MenuAdmin from './MenuAdmin'
import ParametersAdmin from './ParametersAdmin'
import StaffAdmin from './StaffAdmin'
import AuditAdmin from './AuditAdmin'

export default function AdminApp() {
  const [staff, setStaff] = useState<Staff | null>(null)
  const [loading, setLoading] = useState(true)
  const copy = t()

  useEffect(() => {
    if (!tokens.access()) {
      setLoading(false)
      return
    }
    api
      .get<Staff>('/me')
      .then(setStaff)
      .catch(() => tokens.clear())
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <Spinner label={copy.common.loading} />
  if (!staff) return <AdminLogin onSignedIn={setStaff} />

  const can = (perm: string) => staff.permissions?.includes(perm) ?? false

  return (
    <div className="min-h-dvh">
      <header className="border-b border-border bg-surface">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-2 px-4 py-3">
          <img src="/brand/ruuma-logo-emerald.png" alt="" width={96} height={50} className="h-7 w-auto" />
          <span className="font-display text-sm font-semibold">Admin</span>

          <nav className="flex flex-wrap gap-1 text-sm">
            <Tab to="/admin">Dashboard</Tab>
            {can('order.view.store') && <Tab to="/admin/orders">Orders</Tab>}
            {can('payment.queue.view') && <Tab to="/admin/finance">Finance</Tab>}
            {can('store.schedule.manage') && <Tab to="/admin/schedule">Schedule</Tab>}
            {can('store.manage') && <Tab to="/admin/stores">Stores</Tab>}
            {can('menu.availability.manage') && <Tab to="/admin/menu">Menu</Tab>}
            {can('parameters.manage') && <Tab to="/admin/parameters">Parameters</Tab>}
            {can('staff.manage') && <Tab to="/admin/staff">Staff</Tab>}
            {can('audit.view') && <Tab to="/admin/audit">Audit</Tab>}
          </nav>

          <div className="ml-auto flex items-center gap-2">
            <span className="text-sm text-muted">
              {staff.full_name} · {staff.role}
            </span>
            <LanguagePicker />
            <Button
              variant="secondary"
              onClick={() => {
                void api.post('/auth/logout').catch(() => undefined)
                tokens.clear()
                window.location.href = '/admin'
              }}
            >
              {copy.common.signOut}
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-6">
        <Routes>
          <Route path="/" element={<Dashboard staff={staff} />} />
          <Route path="/orders" element={<OrdersBoard staff={staff} />} />
          <Route path="/finance" element={<FinanceQueue />} />
          <Route path="/stores" element={<StoresAdmin />} />
          <Route path="/schedule" element={<ScheduleAdmin />} />
          <Route path="/menu" element={<MenuAdmin />} />
          <Route path="/parameters" element={<ParametersAdmin />} />
          <Route path="/staff" element={<StaffAdmin />} />
          <Route path="/audit" element={<AuditAdmin />} />
        </Routes>
      </main>
    </div>
  )
}

function Tab({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      end={to === '/admin'}
      className={({ isActive }) =>
        [
          'inline-flex min-h-[40px] items-center rounded-lg px-3',
          isActive ? 'bg-primary-subtle font-medium text-primary' : 'text-body hover:bg-primary-subtle',
        ].join(' ')
      }
    >
      {children}
    </NavLink>
  )
}

function AdminLogin({ onSignedIn }: { onSignedIn: (staff: Staff) => void }) {
  const copy = t()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const session = await api.post<{ access_token: string; refresh_token: string; staff: Staff }>(
        '/staff/login',
        { email, password },
      )
      tokens.set(session.access_token, session.refresh_token)
      onSignedIn(session.staff)
      navigate('/admin')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="mx-auto flex min-h-dvh max-w-sm flex-col justify-center gap-4 px-4">
      {/* self-start: a flex item in a column container is stretched to the
          cross-axis width by default, which overrides w-auto and distorts the
          wordmark. Same fix as the customer footer. */}
      <img
        src="/brand/ruuma-logo-emerald.png"
        alt="Ruuma"
        width={140}
        height={72}
        className="h-10 w-auto self-start"
      />
      <h1 className="font-display text-xl font-bold">Admin</h1>
      {error && <ErrorNote>{error}</ErrorNote>}
      <Card>
        <form className="flex flex-col gap-3" onSubmit={submit}>
          <Field label={copy.auth.email}>
            {(id) => (
              <TextInput
                id={id}
                type="email"
                autoComplete="username"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            )}
          </Field>
          <Field label={copy.auth.password}>
            {(id) => (
              <TextInput
                id={id}
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            )}
          </Field>
          <Button type="submit" disabled={busy}>
            {copy.auth.signIn}
          </Button>
        </form>
      </Card>
    </div>
  )
}
