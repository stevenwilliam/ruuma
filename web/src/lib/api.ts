// Typed client for the ruuma API (docs/04).
//
// Every response goes through the one error envelope, so the UI can show the
// server's message and switch on a stable code rather than parsing prose.

export type ApiErrorBody = {
  error: { code: string; message: string; details?: Record<string, unknown> }
}

export class ApiError extends Error {
  code: string
  status: number
  details?: Record<string, unknown>

  constructor(status: number, body: ApiErrorBody) {
    super(body?.error?.message ?? 'Something went wrong.')
    this.status = status
    this.code = body?.error?.code ?? 'INTERNAL'
    this.details = body?.error?.details
  }
}

const ACCESS_KEY = 'ruuma.access_token'
const REFRESH_KEY = 'ruuma.refresh_token'

export const tokens = {
  access: () => localStorage.getItem(ACCESS_KEY),
  refresh: () => localStorage.getItem(REFRESH_KEY),
  set(access: string, refresh: string) {
    localStorage.setItem(ACCESS_KEY, access)
    localStorage.setItem(REFRESH_KEY, refresh)
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY)
    localStorage.removeItem(REFRESH_KEY)
  },
}

type RequestOptions = {
  method?: string
  body?: unknown
  formData?: FormData
  idempotencyKey?: string
  language?: string
  retryOnUnauthorized?: boolean
}

async function raw<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Accept-Language': opts.language ?? localStorage.getItem('ruuma.lang') ?? 'id',
  }
  const access = tokens.access()
  if (access) headers.Authorization = `Bearer ${access}`
  if (opts.idempotencyKey) headers['Idempotency-Key'] = opts.idempotencyKey

  let body: BodyInit | undefined
  if (opts.formData) {
    body = opts.formData
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(`/api/v1${path}`, {
    method: opts.method ?? (body ? 'POST' : 'GET'),
    headers,
    body,
  })

  if (res.status === 204) return undefined as T

  const text = await res.text()
  const parsed = text ? JSON.parse(text) : {}

  if (!res.ok) {
    // One silent refresh attempt, then the caller decides what to do.
    if (res.status === 401 && opts.retryOnUnauthorized !== false && tokens.refresh()) {
      const refreshed = await tryRefresh()
      if (refreshed) return raw<T>(path, { ...opts, retryOnUnauthorized: false })
    }
    throw new ApiError(res.status, parsed as ApiErrorBody)
  }
  return parsed as T
}

async function tryRefresh(): Promise<boolean> {
  const refresh_token = tokens.refresh()
  if (!refresh_token) return false
  try {
    const res = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token }),
    })
    if (!res.ok) {
      tokens.clear()
      return false
    }
    const session = await res.json()
    tokens.set(session.access_token, session.refresh_token)
    return true
  } catch {
    tokens.clear()
    return false
  }
}

export const api = {
  get: <T>(path: string) => raw<T>(path),
  post: <T>(path: string, body?: unknown, idempotencyKey?: string) =>
    raw<T>(path, { method: 'POST', body, idempotencyKey }),
  patch: <T>(path: string, body?: unknown) => raw<T>(path, { method: 'PATCH', body }),
  put: <T>(path: string, body?: unknown) => raw<T>(path, { method: 'PUT', body }),
  del: <T>(path: string) => raw<T>(path, { method: 'DELETE' }),
  upload: <T>(path: string, form: FormData, idempotencyKey?: string) =>
    raw<T>(path, { method: 'POST', formData: form, idempotencyKey }),
}

// newIdempotencyKey generates the key required by every money- or
// capacity-creating endpoint (docs/04 §9).
export function newIdempotencyKey(): string {
  return crypto.randomUUID()
}

export type ListResponse<T> = { items: T[]; next_cursor?: string }

export type Money = { value: number; currency: string }

export type Store = {
  id: string
  code: string
  name: string
  slug: string
  address_line: string
  city: string
  phone: string
  timezone: string
  fulfilment_modes: string[]
  // Admin lists carry the active flag; the customer list only ever shows
  // active stores, so it is optional here.
  is_active?: boolean
  open_today: boolean
  today_reason?: string
  today_hours?: string[]
  next_open_date?: string
}

export type MenuItem = {
  id: string
  category_id: string
  category_name_id: string
  category_name_en: string
  cuisine: string
  // The SKU also names the dish photo under /dish/<sku>.jpg.
  sku: string
  name_id: string
  name_en: string
  description_id: string
  description_en: string
  price: Money
  photo_key: string
  prep_minutes: number
  min_lead_minutes: number
  tags: {
    spice_level: number
    halal: boolean
    vegetarian: boolean
    contains_pork: boolean
    contains_alcohol: boolean
    contains_nuts: boolean
  }
  is_available: boolean
  availability: string
  stock_left: number | null
}

export type OptionChoice = {
  id: string
  name_id: string
  name_en: string
  price_delta: Money
  is_available: boolean
}

export type OptionGroup = {
  id: string
  name_id: string
  name_en: string
  selection: 'single' | 'multi'
  is_required: boolean
  min_select: number
  max_select: number
  choices: OptionChoice[]
}

export type Slot = {
  slot_id: string
  starts_at: string
  ends_at: string
  label: string
  is_bookable: boolean
  reason?: string
  remaining_orders: number
  remaining_units: number
  almost_full: boolean
}

export type DateAvailability = { date: string; is_bookable: boolean; reason?: string }

export type Quote = {
  currency: string
  subtotal: number
  discount: number
  service_charge: number
  tax: number
  delivery_fee: number
  total: number
  tax_bps: number
  service_charge_bps: number
  kitchen_units: number
  lines: Array<{
    menu_item_id: string
    name_id: string
    qty: number
    line_total: Money
  }>
  expires_at: string
}

export type OrderPayment = {
  status: string
  method: string
  amount_due: number
  declared_amount: number
  has_proof: boolean
  proof_uploaded_at?: string
  rejection_reason?: string
  rejection_note?: string
  verified_at?: string
}

export type Order = {
  id: string
  order_code: string
  status: string
  store: { id: string; name: string }
  slot: { id: string; business_date: string; starts_at: string; ends_at: string }
  fulfilment_type: string
  contact: { name: string; phone: string }
  notes: string
  subtotal: number
  discount: number
  service_charge: number
  tax: number
  total: number
  unique_code: number
  amount_due: number
  promo_code: string
  created_at: string
  lines: Array<{
    id: string
    menu_item_id: string
    name_id: string
    name_en: string
    unit_price: Money
    qty: number
    line_total: Money
    notes: string
    options: Array<{ group: string; choice_id: string; price_delta: Money }>
  }>
  payment?: OrderPayment
  bank_account?: { bank_name: string; account_name: string; account_number: string }
  history?: Array<{ from: string; to: string; actor: string; reason: string; at: string }>
}

export type Session = {
  access_token: string
  refresh_token: string
  expires_in: number
  customer?: Customer
  staff?: Staff
}

export type Customer = {
  id: string
  full_name: string
  email: string
  email_verified: boolean
  phone: string
  phone_verified: boolean
  preferred_language: 'id' | 'en'
  marketing_opt_in: boolean
  can_order: boolean
}

export type Staff = {
  id: string
  email: string
  full_name: string
  role: string
  stores: string[]
  permissions: string[]
}
