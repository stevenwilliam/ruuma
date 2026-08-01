# API Specification — ruuma

**Version:** 1.0
**Date:** 2 August 2026

REST over HTTP, `gin` router. JSON in/out. JWT auth. One error model everywhere.

---

## 1. Conventions

- **Base path:** `/api/v1`
- **Auth:** `Authorization: Bearer <access_jwt>`. Access tokens live
  `auth.access_token_minutes` (default 15); refresh tokens are rotating, hashed
  and revocable (BR-2.7.12). Refresh is `POST /api/v1/auth/refresh`.
- **IDs:** UUIDv7 in paths. Orders are also addressable by `order_code`.
- **Money:** integer whole rupiah plus `"currency": "IDR"` (BR-1.1.4). Never a
  decimal, never a formatted string.
- **Time:** RFC 3339 UTC instants (`2026-08-02T05:30:00Z`) for timestamps;
  `business_date` is `YYYY-MM-DD` in the store's timezone (BR-1.3.3).
- **Idempotency:** every mutating endpoint that creates money or reserves
  capacity **requires** `Idempotency-Key`. A replayed key with an identical body
  returns the original response; with a different body it is **409**.
- **Pagination:** cursor-based — `?limit=&cursor=`, response carries
  `{"items": [...], "next_cursor": "…"}`. Default limit 20, max 100.
- **Search:** every list endpoint accepts `?q=` (BR-1.5.1/1.5.2).
- **Store scope:** every store-scoped request resolves the store **server-side**
  from the payload or the target entity and re-checks it against the caller's
  assignments (BR-2.7.9). A `store_id` or role sent by the client is never
  trusted on its own.
- **Language:** `Accept-Language: id|en` selects `name_id`/`name_en` and message
  copy; `id` is the default.
- **Request limits:** 1 MB JSON bodies, 5 MB multipart, 10 s read / 30 s write
  timeouts (see `12-security.md`).

## 2. Error model

```json
{ "error": { "code": "SLOT_FULL", "message": "This time slot is fully booked.",
             "details": { "slot_id": "…", "remaining": 0 } } }
```

| HTTP | When |
|------|------|
| 400 | validation / malformed |
| 401 | missing/invalid/expired auth |
| 403 | authenticated but not permitted (includes cross-store, BR-2.7.10) |
| 404 | not found **or** not visible to the caller (another customer's order) |
| 409 | conflict / illegal state transition / idempotency mismatch |
| 422 | semantically invalid — a business rule refused it |
| 429 | rate limited (`Retry-After` set) |
| 500 | unexpected (never leaks driver text or stack traces) |

Stable codes include: `VALIDATION_FAILED`, `UNAUTHENTICATED`, `FORBIDDEN`,
`STORE_OUT_OF_SCOPE`, `NOT_FOUND`, `ILLEGAL_TRANSITION`, `IDEMPOTENCY_MISMATCH`,
`STORE_INACTIVE`, `MODE_UNSUPPORTED`, `MODE_DISABLED`, `DATE_NOT_BOOKABLE`,
`SLOT_NOT_BOOKABLE`, `SLOT_FULL`, `SLOT_PAST`, `SLOT_LEAD_TIME`, `SLOT_CUTOFF`,
`BLACKOUT`, `ITEM_UNAVAILABLE`, `OPTION_INVALID`, `TOTAL_MISMATCH`,
`PROMO_INVALID`, `PROMO_EXHAUSTED`, `PHONE_VERIFICATION_REQUIRED`,
`UNPAID_LIMIT_REACHED`, `PAYMENT_ALREADY_VERIFIED`, `SELF_VERIFICATION_FORBIDDEN`,
`REJECTION_REASON_REQUIRED`, `RATE_LIMITED`.

---

## 3. Public endpoints (no auth)

| Method | Path | Purpose | Rules |
|---|---|---|---|
| GET | `/health` | liveness | — |
| GET | `/api/v1/stores?q=` | active stores: address, phone, modes, today's open state, next open date | BR-2.1.1/2/11 |
| GET | `/api/v1/stores/:id` | one store: hours per weekday per mode, per-date overrides, blackouts, modes | BR-2.1.4–8 |
| GET | `/api/v1/menu?store_id=&q=&category=&cuisine=&diet=&sort=&limit=&cursor=` | store-resolved menu with effective price and availability | BR-2.2.1/2 |
| GET | `/api/v1/menu/:id?store_id=` | item detail with option groups, choices, deltas, availability | BR-2.2.5/6 |
| GET | `/api/v1/availability/dates?store_id=&type=&month=` | bookable dates + a reason per unbookable date | BR-2.3.2 |
| GET | `/api/v1/availability/slots?store_id=&date=&type=&items=` | slots with `remaining_orders`, `remaining_units`, `is_bookable`, `reason` | BR-2.3.5/6 |
| POST | `/api/v1/cart/quote` | server-side pricing of a cart (lines, options, promo) | BR-2.5.x |

`GET /api/v1/availability/slots` response element:

```json
{ "slot_id": "0192…", "starts_at": "2026-08-03T05:00:00Z", "ends_at": "…",
  "label": "12:00–12:30", "is_bookable": false, "reason": "FULL",
  "remaining_orders": 0, "remaining_units": 14, "almost_full": false }
```

`POST /api/v1/cart/quote` request/response:

```json
{ "store_id": "…", "fulfilment_type": "pickup", "business_date": "2026-08-03",
  "slot_id": "…", "promo_code": "RUUMA10",
  "lines": [{ "menu_item_id": "…", "qty": 2, "notes": "no cucumber",
              "option_choice_ids": ["…","…"] }] }
```

```json
{ "currency": "IDR", "subtotal": 150000, "discount": 15000,
  "service_charge": 0, "tax": 13500, "delivery_fee": 0, "total": 148500,
  "tax_bps": 1000, "service_charge_bps": 0,
  "lines": [{ "menu_item_id": "…", "unit_price": 65000, "options_delta": 10000,
              "qty": 2, "line_total": 150000, "kitchen_units": 2 }],
  "quote_id": "…", "expires_at": "2026-08-02T06:00:00Z",
  "warnings": [{ "code": "ITEM_UNAVAILABLE", "menu_item_id": "…" }] }
```

## 4. Authentication (D24, BR-2.7.1–5)

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/v1/auth/register` | email + password; sends a verification link |
| GET | `/api/v1/auth/verify-email?token=` | consume the emailed token |
| POST | `/api/v1/auth/login` | email + password → access + refresh |
| POST | `/api/v1/auth/oauth/:provider/start` | `google` \| `instagram`; returns the authorize URL + state |
| GET | `/api/v1/auth/oauth/:provider/callback` | exchanges the code, links or creates the customer |
| POST | `/api/v1/otp/request` | phone OTP (`signup` \| `login` \| `verify_phone`), rate-limited |
| POST | `/api/v1/otp/verify` | consumes the OTP; verifies the phone or issues tokens |
| POST | `/api/v1/auth/refresh` | rotates the refresh token, denylists the old `jti` |
| POST | `/api/v1/auth/logout` | revokes the refresh token |
| POST | `/api/v1/auth/password/forgot` / `/reset` | reset by emailed token |
| POST | `/api/v1/staff/login` | staff email + password (separate router group) |

Auth responses never reveal whether an account exists; OTP responses are
identical for wrong, expired, used and over-attempted codes (BR-2.7.5).

## 5. Customer endpoints (customer auth)

| Method | Path | Purpose | Rules |
|---|---|---|---|
| GET | `/api/v1/me` | profile, verification state, language | BR-2.7.4 |
| PATCH | `/api/v1/me` | name, language, marketing opt-in | BR-2.10.4 |
| POST | `/api/v1/orders` | **create order** — `Idempotency-Key` required | BR-2.3.5–15, BR-2.5.x, BR-2.6.2 |
| GET | `/api/v1/orders?q=&limit=&cursor=` | own order history | BR-2.7.10 |
| GET | `/api/v1/orders/:id` | own order with events, payment state, slot | BR-2.4.4 |
| GET | `/api/v1/orders/track?code=` | own order by code (authenticated, rate-limited) | BR-2.7.11 |
| POST | `/api/v1/orders/:id/cancel` | cancel inside the customer window | BR-2.3.13 |
| POST | `/api/v1/orders/:id/reorder` | build a revalidated cart from a past order | BR-2.2.2 |
| POST | `/api/v1/orders/:id/payment-proof` | presigned upload + declared amount + sender name | BR-2.6.4/11 |
| GET/POST/PATCH/DELETE | `/api/v1/addresses` | saved addresses (phase 2 use) | — |
| GET/POST/DELETE | `/api/v1/favourites` | favourite items | — |

`POST /api/v1/orders` — the whole checkout in one transaction:

```json
{ "store_id": "…", "fulfilment_type": "pickup", "slot_id": "…",
  "contact_name": "Budi", "contact_phone": "+628123456789",
  "promo_code": "RUUMA10", "expected_total": 148500,
  "lines": [{ "menu_item_id": "…", "qty": 2, "notes": "…",
              "option_choice_ids": ["…"] }] }
```

Server behaviour, in one transaction: re-price from master data → compare with
`expected_total`, 422 `TOTAL_MISMATCH` on disagreement (BR-2.5.13) →
`SELECT … FOR UPDATE` the slot → check both capacity axes → check item
availability, 86s and daily stock → check the unpaid-order cap (BR-2.3.15) →
reserve capacity and decrement stock → allocate the `unique_code` (BR-2.6.2) →
record the promo redemption → insert order, lines, options, payment row and the
first `order_events` row → commit → queue the "order received" notification.

Response:

```json
{ "id": "…", "order_code": "K7M2X9QA", "status": "PENDING_PAYMENT",
  "store": { "id": "…", "name": "Ruuma Kelapa Gading", "address_line": "…" },
  "slot": { "business_date": "2026-08-03", "starts_at": "…", "label": "12:00–12:30" },
  "total": 148500, "unique_code": 347, "amount_due": 148847, "currency": "IDR",
  "bank_account": { "bank_name": "BCA", "account_name": "PT Ruuma", "account_number": "…" } }
```

`POST /api/v1/orders/:id/payment-proof` returns a **presigned PUT URL** for a
private object plus the declared amount echo; the order moves to
`AWAITING_VERIFICATION` only after the upload is confirmed (BR-2.6.4).

## 6. Finance endpoints (`finance` permission, store-scoped)

| Method | Path | Purpose | Rules |
|---|---|---|---|
| GET | `/api/v1/finance/payments?status=&store_id=&q=&sort=oldest&limit=&cursor=` | searchable queue with ageing | BR-2.6.5 |
| GET | `/api/v1/finance/payments/:id` | payment, order, customer, presigned proof URL | BR-2.6.11 |
| POST | `/api/v1/finance/payments/:id/verify` | verify; mismatch requires `accept_mismatch` + `reason` | BR-2.6.6/7/13 |
| POST | `/api/v1/finance/payments/:id/reject` | **reason required** from the closed set | BR-2.6.8 |
| POST | `/api/v1/finance/payments/:id/refund` | amount, reference, optional proof, reason | BR-2.6.12 |
| GET | `/api/v1/finance/reconciliation?date=&store_id=` | daily takings, mismatches, rejections, *kode unik* total | BR-2.6.3 |
| GET | `/api/v1/finance/reconciliation/export?date=&store_id=` | CSV export | — |

Verify is idempotent (BR-2.6.13); self-verification is 403
`SELF_VERIFICATION_FORBIDDEN` (BR-2.6.6); out-of-scope stores are 403
`STORE_OUT_OF_SCOPE`.

## 7. Operations endpoints (kitchen / counter / store manager)

| Method | Path | Purpose | Rules |
|---|---|---|---|
| GET | `/api/v1/ops/orders?store_id=&date=&status=&q=` | orders board, grouped by slot | BR-2.8.1 |
| GET | `/api/v1/ops/slots/:id/production` | aggregated item + option quantities | BR-2.8.2/3 |
| GET | `/api/v1/ops/slots/:id/ticket` | printable ticket payload | BR-2.8.4 |
| POST | `/api/v1/ops/orders/:id/status` | `IN_KITCHEN` \| `READY` \| `PICKED_UP` | BR-2.4.2/3 |
| POST | `/api/v1/ops/orders/:id/cancel` | staff cancel with reason (single) | BR-2.3.14 |
| POST | `/api/v1/ops/orders/cancel-bulk` | staff bulk cancel with reason | BR-2.3.14 |
| GET | `/api/v1/ops/orders/unpaid?store_id=` | ageing unpaid orders (D25 interim control) | BR-2.3.15 |
| GET | `/api/v1/ops/orders/affected?store_id=&date=` | orders hit by a closure/blackout | BR-2.1.9 |

## 8. Admin endpoints (`admin`/`owner`, or `store_manager` within scope)

All lists accept `?q=` (BR-1.5.1) and are cursor-paginated.

| Area | Endpoints |
|---|---|
| Stores | `GET/POST /admin/stores`, `GET/PATCH /admin/stores/:id`, `POST /admin/stores/:id/activate|deactivate` |
| Modes | `GET/PUT /admin/stores/:id/fulfilment-modes` |
| Hours | `GET/PUT /admin/stores/:id/hours` (weekday × mode × block) |
| Date overrides | `GET/POST/DELETE /admin/stores/:id/date-overrides` |
| Blackouts | `GET/POST/DELETE /admin/stores/:id/blackouts` (today allowed, reason required) |
| Bank accounts | `GET/POST/PATCH/DELETE /admin/stores/:id/bank-accounts` |
| Store parameters | `GET/PUT /admin/stores/:id/parameters` |
| Slots | `GET /admin/stores/:id/slots?date=`, `POST /admin/stores/:id/slots/generate`, `PATCH /admin/slots/:id` (capacity, lock) |
| Staff | `GET/POST/PATCH/DELETE /admin/users`, `PUT /admin/users/:id/stores` |
| Categories | `GET/POST/PATCH/DELETE /admin/categories` |
| Menu items | `GET/POST/PATCH/DELETE /admin/menu-items`, `POST /admin/menu-items/:id/photo` (presigned) |
| Options | `GET/POST/PATCH/DELETE /admin/menu-items/:id/option-groups`, `…/choices` |
| Availability | `PUT /admin/stores/:id/menu-overrides`, `POST/DELETE /admin/stores/:id/86`, `PUT /admin/stores/:id/daily-stock` |
| Promotions | `GET/POST/PATCH/DELETE /admin/promotions` (+ store list, categories) |
| Delivery zones | `GET/POST/PATCH/DELETE /admin/stores/:id/delivery-zones` (phase 2) |
| Parameters | `GET/POST/PATCH/DELETE /admin/sys-parameters` |
| Reports | `GET /admin/reports/sales?from=&to=&store_id=&group_by=day|slot|item`, `/admin/reports/cancellations`, `/admin/reports/top-items` |
| Dashboard | `GET /admin/dashboard?store_id=&date=` — today's slots, load, revenue, top items, cancellations |
| Audit | `GET /admin/audit-log?q=&entity=&actor=&store_id=&from=&to=` |

`POST /admin/stores/:id/blackouts` accepts today's date, returns the count of
already-booked orders it affects, and never cancels them (BR-2.1.9):

```json
{ "business_date": "2026-08-02", "reason": "Kitchen flood" }
→ { "id": "…", "affected_orders": 7,
    "note": "Existing orders are untouched. Review them under ops/orders/affected." }
```

## 9. Idempotent + rate-limited surfaces

**Idempotency-Key required:** `POST /orders`, `/orders/:id/payment-proof`,
`/finance/payments/:id/{verify,reject,refund}`, `/ops/orders/:id/status`,
`/ops/orders/cancel-bulk`, `/admin/stores/:id/slots/generate`.

**Rate limits** (per identifier **and** per IP, `sys_parameters`-tunable):
login 5/min, staff login 5/min, OTP request 3/10 min per phone and 10/hour per
IP, OTP verify 5/10 min, order tracking 20/min, promo validation 10/min,
order creation 10/min, menu reads 120/min.

## 10. Webhooks (phase 2)

`POST /api/v1/webhooks/payments/:provider` — verifies an HMAC signature and a
timestamp inside a 5-minute window, rejects replays by event id, and is
idempotent. Unused in phase 1; the route and its verification are built and
tested so QRIS can be enabled without new plumbing (D25).
