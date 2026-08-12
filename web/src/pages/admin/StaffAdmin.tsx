// Staff accounts and their store assignments. A staff member is deactivated,
// never deleted — their audit trail has to outlive them (docs/06 §2.7).

import { useCallback, useEffect, useState } from 'react'
import { api, type ListResponse, type Staff } from '../../lib/api'
import { SearchBox } from '../../components/SearchBox'
import { AsyncButton, Badge, Card, ErrorNote, Field, Spinner, TextInput } from '../../components/ui'
import { Table, useStores } from './common'
import { t } from '../../i18n'

type StaffRow = Staff & { is_active: boolean; group_scope: boolean }

const ROLES = ['kitchen', 'counter', 'finance', 'store_manager', 'admin', 'owner']

export default function StaffAdmin() {
  const copy = t()
  const stores = useStores()
  const [rows, setRows] = useState<StaffRow[] | null>(null)
  const [query, setQuery] = useState('')
  const [error, setError] = useState('')

  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [role, setRole] = useState('kitchen')
  const [storeIds, setStoreIds] = useState<string[]>([])
  const [password, setPassword] = useState('')

  const load = useCallback(() => {
    api
      .get<ListResponse<StaffRow>>(`/admin/users${query ? `?q=${encodeURIComponent(query)}` : ''}`)
      .then((res) => setRows(res.items))
      .catch((err: Error) => setError(err.message))
  }, [query])

  useEffect(load, [load])

  async function create() {
    setError('')
    try {
      await api.post('/admin/users', {
        email,
        full_name: name,
        role,
        stores: storeIds,
        group_scope: role === 'finance' && storeIds.length === 0,
        is_active: true,
        password,
      })
      setEmail('')
      setName('')
      setPassword('')
      setStoreIds([])
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function deactivate(id: string) {
    setError('')
    try {
      await api.del(`/admin/users/${id}`)
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-display text-2xl font-bold">Staff</h1>
      <SearchBox value={query} onChange={setQuery} placeholder="Name, email or role" />
      {error && <ErrorNote>{error}</ErrorNote>}
      {!rows && <Spinner label={copy.common.loading} />}

      {rows && (
        <Table head={['Name', 'Email', 'Role', 'Scope', 'Active', '']}>
          {rows.map((u) => (
            <tr key={u.id}>
              <td className="px-3 py-2 font-medium">{u.full_name}</td>
              <td className="px-3 py-2 text-muted">{u.email}</td>
              <td className="px-3 py-2">
                <Badge>{u.role}</Badge>
              </td>
              <td className="px-3 py-2 text-muted">
                {u.group_scope
                  ? 'group-wide'
                  : (u.stores ?? [])
                      .map((id) => stores.find((s) => s.id === id)?.code ?? '—')
                      .join(', ') || '—'}
              </td>
              <td className="px-3 py-2">
                {u.is_active ? <Badge tone="primary">active</Badge> : <Badge tone="warning">inactive</Badge>}
              </td>
              <td className="px-3 py-2">
                {u.is_active && (
                  <AsyncButton
                    variant="secondary"
                    busyLabel="Deactivating…"
                    confirm={`Deactivate ${u.full_name}? They lose admin access immediately.`}
                    onRun={() => deactivate(u.id)}
                  >
                    Deactivate
                  </AsyncButton>
                )}
              </td>
            </tr>
          ))}
        </Table>
      )}

      <Card className="flex flex-col gap-3">
        <h2 className="font-display text-lg font-semibold">Add staff</h2>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Full name">
            {(id) => <TextInput id={id} value={name} onChange={(e) => setName(e.target.value)} />}
          </Field>
          <Field label="Email">
            {(id) => (
              <TextInput id={id} type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
            )}
          </Field>
          <Field label="Role">
            {(id) => (
              <select
                id={id}
                value={role}
                onChange={(e) => setRole(e.target.value)}
                className="min-h-[44px] rounded-xl border border-border bg-surface px-3 text-sm"
              >
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
            )}
          </Field>
          <Field label="Temporary password" hint="minimum 12 characters; must be changed on first login">
            {(id) => (
              <TextInput id={id} type="text" value={password} onChange={(e) => setPassword(e.target.value)} />
            )}
          </Field>
        </div>

        <fieldset className="flex flex-wrap gap-2">
          <legend className="pb-1 text-sm font-medium">Stores</legend>
          {stores.map((s) => (
            <label key={s.id} className="flex min-h-[40px] items-center gap-2 rounded-xl border border-border px-3 text-sm">
              <input
                type="checkbox"
                checked={storeIds.includes(s.id)}
                onChange={(e) =>
                  setStoreIds((prev) =>
                    e.target.checked ? [...prev, s.id] : prev.filter((id) => id !== s.id),
                  )
                }
                className="h-4 w-4 accent-[var(--primary)]"
              />
              {s.code}
            </label>
          ))}
        </fieldset>

        <AsyncButton
          className="self-start"
          busyLabel="Creating…"
          onRun={create}
          disabled={!email || !name || !password}
        >
          Create staff account
        </AsyncButton>
      </Card>
    </div>
  )
}
