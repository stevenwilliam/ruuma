# Business Rules — ruuma

**Status:** Normative. Where this conflicts with any other document in the set,
this document wins.
**Version:** 1.0
**Date:** 2 August 2026

Rules are identified `BR-x.y`. Engineering references these IDs in code comments
and test names — every rule below must be traceable to at least one test
(`07-test-plan.md`).

---

## 1. Foundations

### 1.1 Money

- **BR-1.1.1** All monetary values are stored as `BIGINT` in **whole rupiah
  (IDR)**. The rupiah has no circulating subunit, so there is no minor-unit
  multiplier — `150000` means Rp 150.000. _(Amended by D9; the original
  minor-unit wording no longer applies.)_
  _Prevents:_ silent 100× errors from a subunit convention nobody applies.
- **BR-1.1.2** All arithmetic on money uses integers. Floating point is
  prohibited in any code path touching money — including report aggregation and
  the frontend.
  _Prevents:_ `0.1 + 0.2` rounding drift accumulating across a day's takings.
- **BR-1.1.3** Percentage values are held in **basis points** (bps) and round to
  the nearest whole rupiah, half-up:
  `round(amount × bps / 10000) = floor((amount × bps + 5000) / 10000)`.
  _Prevents:_ two systems disagreeing by one rupiah on the same tax line.
- **BR-1.1.4** Money crossing the API is an integer field plus an explicit
  `currency` of `IDR`; it is never a formatted string, and never a decimal.
  _Prevents:_ locale-dependent parsing of `150.000` as one hundred and fifty.

### 1.2 Identifiers

- **BR-1.2.1** All primary keys are UUIDv7 — time-ordered for index locality,
  not sequential in a way that leaks volume.
- **BR-1.2.2** The human-facing **order code** is 8 characters of Crockford
  base32 drawn from a CSPRNG, uppercase, unique, non-guessable, encodes no
  customer identity and no sequence.
  _Prevents:_ order enumeration by incrementing a number.
- **BR-1.2.3** Store codes are short, human-assigned, unique and immutable once
  orders exist (e.g. `RMA-KG`).

### 1.3 Time

- **BR-1.3.1** All timestamps are stored in UTC (`timestamptz`). The operating
  timezone for all business-day logic is **Asia/Jakarta**.
- **BR-1.3.2** Every conversion between a local business date/time and an
  instant is **explicit**, using the store's timezone — never the server's local
  time, never the client's.
  _Prevents:_ a slot at 10:00 Jakarta appearing as 03:00 on a UTC server and the
  day rolling over at the wrong moment.
- **BR-1.3.3** A "business date" is a calendar date in the store's timezone.
  Opening hours, blackout dates, cutoffs and daily reports all use it.

### 1.4 Configuration

- **BR-1.4.1** Any value that can change without a code change (company phone,
  email, address, tax rate, thresholds, feature toggles, operational timings) is
  stored in `sys_parameters` and read at runtime. Hard-coding such a value is
  prohibited.
- **BR-1.4.2** `sys_parameters` has full CRUD — list (**with a search box**),
  create, read, update, delete — restricted to an admin permission. Changes are
  attributed (`updated_by`) and timestamped.
- **BR-1.4.3** Parameters flagged `is_secret` are masked in the UI and never
  written to logs.
- **BR-1.4.4** Where a parameter has a per-store equivalent, resolution is
  **store value → group default in `sys_parameters` → compiled fallback**, and
  the compiled fallback exists only so a missing row cannot take the service
  down. The resolved value and its source are visible in admin.

### 1.5 Data listing

- **BR-1.5.1** Every screen that lists or tables data provides a search box that
  filters that data. A list without search is non-conformant.
- **BR-1.5.2** List endpoints are cursor-paginated and accept a `q` search term;
  search is case-insensitive and accent-insensitive over the columns that
  identify the row.

---

## 2. Domain rules

### 2.1 Stores (master data)

- **BR-2.1.1** Every order, slot, menu override, capacity counter, promotion
  scope, staff assignment, bank account and report row belongs to **exactly one
  store**. There is no store-less order.
  _Prevents:_ an order that no kitchen owns.
- **BR-2.1.2** A store declares its supported fulfilment modes (`pickup`,
  `delivery`, or both). Selecting a mode the store does not support is rejected
  with **422**, and the UI never offers it.
  _Prevents:_ a delivery order landing at a pickup-only counter.
- **BR-2.1.3** **Phase 1 is pickup only.** The `fulfilment.delivery_enabled`
  parameter is `false`; while false, `delivery` is rejected with 422 for every
  store regardless of that store's declared modes (D16).
- **BR-2.1.4** A store declares, per weekday, whether it is OPEN or CLOSED and
  its opening blocks (one or more, e.g. lunch and dinner). A closed weekday
  generates **no slots at all** for that date, for any mode.
  _Prevents:_ Saturday orders at a store that closes on Saturday.
- **BR-2.1.5** Opening hours may differ **per fulfilment mode** within the same
  store. The bookable window for a mode is the **intersection** of the store's
  weekday block and that mode's window.
  _Prevents:_ accepting a 20:30 delivery when delivery stops at 20:00.
- **BR-2.1.6** A **per-date schedule override** (`store_date_overrides`) replaces
  the weekday schedule for one specific date at one store — either marking it
  closed or supplying different blocks (e.g. this Sunday closes at 22:00).
- **BR-2.1.7** A **blackout date** closes a store for a whole date. Blackouts may
  target **the current date** (emergency closure) and take effect immediately:
  from that moment, no new order may be created for any remaining slot of that
  date.
  _Prevents:_ new orders arriving after the kitchen has physically shut.
- **BR-2.1.8** Precedence, highest first: **per-date override → blackout →
  weekday schedule**. A per-date override may therefore re-open a blacked-out
  date deliberately; nothing else may.
- **BR-2.1.9** A blackout or closure **never auto-cancels orders already
  booked**. Affected orders are listed for staff to cancel and refund by hand
  (D27).
  _Prevents:_ a system silently cancelling a customer's booking.
- **BR-2.1.10** Every blackout, per-date override and schedule change requires an
  actor and is written to the audit log; blackouts also require a reason.
- **BR-2.1.11** Deactivating a store hides it from customers immediately but
  never deletes, reassigns or renumbers its historical orders.
- **BR-2.1.12** All schedule values (opening blocks, slot length, capacity, lead
  time, cutoff, max advance days, cancel cutoff) are **store-level master data**,
  with a group default in `sys_parameters` used when a store leaves a value unset
  (BR-1.4.4).
- **BR-2.1.13** A store holds one or more bank accounts for transfers; exactly
  one is `is_primary` and it is the account shown at checkout for that store.

### 2.2 Menu, options and availability

- **BR-2.2.1** The menu (categories, items, option groups) is **group-level**
  master data; each store may override an item's **availability** and **price**
  via `store_menu_overrides`. Resolution is store override → group default.
  _Prevents:_ maintaining three divergent menus by hand.
- **BR-2.2.2** An item is orderable at a store only if: the item is active, the
  category is active, the store override does not mark it unavailable, no active
  "86" record covers the requested date/slot, and any item-level availability
  rule admits that date and slot.
- **BR-2.2.3** A scheduled **"86"** (out of stock) marks an item unavailable at
  one store from now until a stated date/time, or for a stated set of slots.
  A sold-out item disappears from the affected slots only, not from the menu.
  _Prevents:_ hiding a dish from tomorrow because it ran out today.
- **BR-2.2.4** An item may carry a **daily stock countdown** per store per
  business date. When it reaches zero the item is unavailable for the rest of
  that date; the counter decrements inside the same transaction that reserves
  slot capacity.
- **BR-2.2.5** An item may have any number of **option groups**. A group is
  either `single` (exactly one choice; `is_required` decides whether it may be
  omitted) or `multi` (between `min_select` and `max_select` choices).
  Violating those bounds is **422**.
- **BR-2.2.6** Each option choice carries its own **price delta** (may be zero or
  negative, never taking the line below zero) and its own availability; an
  unavailable choice cannot be selected.
- **BR-2.2.7** Item constraints and slot availability **intersect**: an item
  restricted to weekends, or to a minimum lead time of its own, narrows the
  bookable slot set for any cart containing it. The cart's bookable set is the
  intersection across all its items.
  _Prevents:_ booking an 11:00 slot for a dish that needs four hours' notice.
- **BR-2.2.8** Photos are stored in object storage under generated keys, served
  by presigned URL, re-encoded on upload, and never trusted by file extension
  (see `12-security.md`).

### 2.3 Scheduling and capacity

- **BR-2.3.1** An order carries **exactly one store** and **exactly one
  fulfilment slot**: a business date, a start time, an end time and a type
  (`pickup` | `delivery`).
- **BR-2.3.2** A **date** is bookable for a store and mode only if: the store is
  active, the store supports that mode, the mode is enabled group-wide, the
  effective schedule for that date (BR-2.1.8) is open, and the date lies between
  today and `today + max_advance_days` inclusive, in the store's timezone.
- **BR-2.3.3** Slots are generated from the effective opening blocks for that
  date and mode, cut into `slot_length_minutes` intervals aligned to the start of
  each block. A trailing interval shorter than the slot length is discarded.
- **BR-2.3.4** Slots are **materialised** per store, date, mode and start time,
  with counters. `(store_id, business_date, fulfilment_type, starts_at)` is
  unique.
  _Prevents:_ two rows counting the same physical window.
- **BR-2.3.5** A slot is **bookable** only if all hold: the store is active; the
  store supports and the group enables that mode; the date is bookable
  (BR-2.3.2); the date is not blacked out (BR-2.1.7); `now + lead_time_minutes ≤
  slot.starts_at`; `now ≤ slot.starts_at − cutoff_minutes`; and remaining
  capacity on **both** axes is greater than zero.
- **BR-2.3.6** Every slot returned to a client that is **not** bookable carries a
  machine-readable reason: `PAST`, `CLOSED`, `BLACKOUT`, `LEAD_TIME`, `CUTOFF`,
  `FULL`, `MODE_UNSUPPORTED`, `MODE_DISABLED`, `ITEM_CONSTRAINT`.
  _Prevents:_ a greyed-out slot the customer cannot explain.
- **BR-2.3.7** Capacity is enforced per store per slot on **two axes**:
  `max_orders` and `max_kitchen_units`. Each menu item carries a
  `kitchen_units` weight (default 1); a line contributes `kitchen_units × qty`.
  Both axes are checked; either being exhausted makes the slot `FULL`.
  _Prevents:_ twelve tiny orders and twelve banquet orders counting the same.
- **BR-2.3.8** Capacity is **reserved at order creation**, inside a single
  transaction, taking `SELECT … FOR UPDATE` on the slot row before incrementing
  its counters.
- **BR-2.3.9** The database itself refuses an oversell: `CHECK (reserved_orders
  <= max_orders)` and `CHECK (reserved_kitchen_units <= max_kitchen_units)` on
  the slot row. Application logic is the first line of defence, not the only one.
  _Prevents:_ a race, a bug or a manual `UPDATE` overselling a slot.
- **BR-2.3.10** Two concurrent checkouts for the last place must produce exactly
  one success and one **409**; this is proven by a concurrency test.
- **BR-2.3.11** **There is no payment hold and no auto-expiry in phase 1**
  (D13/D25). An unpaid order holds its capacity until a human cancels it, or
  until `orders.auto_cancel_minutes` becomes non-zero in the QRIS phase.
- **BR-2.3.12** Capacity is **released** exactly when an order reaches
  `CANCELLED`, and never otherwise. Release decrements the same counters inside
  one transaction and is idempotent per order.
  _Prevents:_ double-release inflating a slot beyond its true capacity.
- **BR-2.3.13** A customer may cancel until `cancel_cutoff_minutes` before slot
  start. After that, only staff may cancel.
- **BR-2.3.14** Staff may cancel any order of a store in their scope at any time,
  with a reason; single and bulk cancellation are both audited per order.
- **BR-2.3.15** A customer may hold at most `orders.max_unpaid_per_customer`
  (default 2, `0` = unlimited) orders in `PENDING_PAYMENT` across all stores.
  Exceeding it is **422**.
  _Prevents:_ slot squatting while there is no auto-cancel (D25).
- **BR-2.3.16** Changing a store's slot length, capacity or hours **never
  retroactively invalidates already-booked slots**; it applies to slots
  materialised after the change. Reducing capacity below what is already
  reserved is rejected with 422 unless the manager explicitly confirms an
  over-reserved state, which is recorded in the audit log.

### 2.4 Order lifecycle

- **BR-2.4.1** An order is always in exactly one state:
  `DRAFT`, `PENDING_PAYMENT`, `AWAITING_VERIFICATION`, `PAID`, `ACCEPTED`,
  `IN_KITCHEN`, `READY`, `PICKED_UP`, `OUT_FOR_DELIVERY` (phase 2), `DELIVERED`
  (phase 2), `COMPLETED`, `CANCELLED`, `REFUNDED`.
- **BR-2.4.2** Only these transitions are legal:

```
DRAFT               → PENDING_PAYMENT | CANCELLED
PENDING_PAYMENT     → AWAITING_VERIFICATION | CANCELLED
AWAITING_VERIFICATION → PAID | PENDING_PAYMENT (finance rejected) | CANCELLED
PAID                → ACCEPTED | CANCELLED | REFUNDED
ACCEPTED            → IN_KITCHEN | CANCELLED | REFUNDED
IN_KITCHEN          → READY | CANCELLED | REFUNDED
READY               → PICKED_UP | OUT_FOR_DELIVERY | CANCELLED | REFUNDED
PICKED_UP           → COMPLETED | REFUNDED
OUT_FOR_DELIVERY    → DELIVERED | CANCELLED | REFUNDED         (phase 2)
DELIVERED           → COMPLETED | REFUNDED                      (phase 2)
COMPLETED           → REFUNDED
CANCELLED           → (terminal)
REFUNDED            → (terminal)
```

- **BR-2.4.3** Any transition not listed is **409 Conflict**, with the current
  state and the attempted target in the error details.
- **BR-2.4.4** Every transition is appended to `order_events` with actor
  (user or `system`), from-state, to-state, reason and timestamp.
  `order_events` is append-only: no updates, no deletes.
  _Prevents:_ an order whose history cannot be reconstructed in a dispute.
- **BR-2.4.5** `PENDING_PAYMENT` is entered when the order is created and
  capacity is reserved. `AWAITING_VERIFICATION` is entered **only** when a proof
  file is attached (BR-2.6.2).
- **BR-2.4.6** `ACCEPTED` is entered automatically on payment verification; the
  kitchen moves `ACCEPTED → IN_KITCHEN → READY`; the counter moves
  `READY → PICKED_UP → COMPLETED`.
- **BR-2.4.7** `CANCELLED` releases capacity exactly once (BR-2.3.12).
  `REFUNDED` does not release capacity — the slot was consumed.
- **BR-2.4.8** An order's store, slot, lines and prices are **immutable after
  creation**. A change means cancelling and creating a new order.
  _Prevents:_ an edited order diverging from what the kitchen already cooked.

### 2.5 Pricing, totals and promotions

- **BR-2.5.1** The **price charged is a snapshot** copied onto the order line at
  creation: item name, unit price and each option's name and delta. Later menu
  edits never change historical orders.
  _Prevents:_ yesterday's revenue moving when someone edits a price today.
- **BR-2.5.2** `line_total = (item_unit_price + Σ option_price_delta) × qty`,
  in whole rupiah, integer arithmetic only.
- **BR-2.5.3** `subtotal = Σ line_total`.
- **BR-2.5.4** `discount` is computed from the applied promotion (BR-2.5.8+) and
  can never exceed `subtotal`.
- **BR-2.5.5** `service_charge = round_half_up((subtotal − discount) ×
  service_charge_bps)`, `tax = round_half_up((subtotal − discount +
  service_charge) × tax_bps)`, using BR-1.1.3. Default `tax_bps = 1000` (PB1
  10%), `service_charge_bps = 0`, both overridable per store (D17).
- **BR-2.5.6** `delivery_fee` is `0` in phase 1 and comes from the store's
  delivery zone in phase 2.
- **BR-2.5.7** `total = subtotal − discount + service_charge + tax +
  delivery_fee`, and `total ≥ 0` always. If
  `pricing.tax_inclusive` is true, prices are treated as tax-inclusive and the
  tax line is derived, not added (D17).
- **BR-2.5.8** A promotion carries: code, type (`percent` | `fixed`), value,
  validity window, minimum spend, total usage cap, per-customer usage cap, an
  explicit **list of stores** it applies to (D15), and an optional category
  restriction.
- **BR-2.5.9** A promotion applies only if: the code matches
  case-insensitively, `now` is inside the window, the order's store is in the
  promotion's store list, the subtotal meets the minimum spend, neither cap is
  exhausted, and (if restricted) the order contains a qualifying category.
  Otherwise **422** with the specific reason.
- **BR-2.5.10** A percent promotion computes
  `round_half_up(eligible_subtotal × value_bps)` and is capped by
  `max_discount` when set; a fixed promotion is capped at `eligible_subtotal`.
  A discount never produces a negative total (BR-2.5.7).
- **BR-2.5.11** Redemption is recorded in `promotion_redemptions` at order
  creation, inside the same transaction as capacity reservation, with a unique
  constraint enforcing the per-customer cap.
  _Prevents:_ parallel checkouts each spending the last use of a code.
- **BR-2.5.12** Cancelling or refunding an order **releases its redemption**, so
  the usage cap reflects live reality.
- **BR-2.5.13** The **client-sent total is never trusted**. The server recomputes
  every amount from master data at quote time and again at order creation; a
  mismatch between the client's expected total and the server's is **422** with
  both figures in the details.
- **BR-2.5.14** A price quote is valid for `pricing.quote_ttl_minutes`; an order
  created against a stale or altered quote is re-priced and re-checked against
  BR-2.5.13.

### 2.6 Payment — manual bank transfer

- **BR-2.6.1** Phase 1 supports exactly one payment method: `manual_transfer`,
  behind a provider port that also declares `qris` and `gateway` for later
  phases (D25). The order flow does not change when a provider is added.
- **BR-2.6.2** The **amount due** is `total + unique_code`, where `unique_code`
  is an integer in 1–999, unique among all **open** orders sharing the same store
  bank account, assigned at order creation and released when the order reaches a
  terminal state.
  _Prevents:_ two customers transferring identical amounts on the same day.
- **BR-2.6.3** `unique_code_amount` is recorded on the order and reconciled as
  received revenue; it is never treated as a discount or written off.
- **BR-2.6.4** An order moves to `AWAITING_VERIFICATION` **only** when a proof
  file is attached together with a declared amount and sender name. A proof
  upload that does not resolve to an order owned by the uploading customer is
  discarded and audited.
- **BR-2.6.5** Only a user holding the **finance** permission may verify or
  reject a payment, and only for a store in their scope. Cross-store attempts are
  **403**.
- **BR-2.6.6** A finance user may **never verify a payment for an order they
  created themselves**; the attempt is 403 and audited.
  _Prevents:_ a single actor manufacturing paid orders.
- **BR-2.6.7** Verification compares the **declared amount** against
  `amount_due`. An exact match may be verified directly. A mismatch (over or
  under) must be **explicitly accepted with a reason**, which is stored; it can
  never pass silently.
- **BR-2.6.8** **Rejection requires a reason** from a closed set
  (`AMOUNT_MISMATCH`, `PROOF_UNREADABLE`, `NOT_RECEIVED`, `DUPLICATE`, `OTHER`
  plus free text) and returns the order to `PENDING_PAYMENT`. The customer may
  upload a new proof. Rejection **never releases the slot** (D26).
- **BR-2.6.9** No automated message is sent on rejection; finance and operations
  contact the customer by hand (D28). The rejection reason is shown prominently
  on the customer's order page.
- **BR-2.6.10** Verify, reject, accept-with-mismatch and refund each append an
  immutable row to `payment_events` recording actor, timestamp, amounts and
  reason, and each lands in the audit log.
- **BR-2.6.11** Uploaded proofs are **private objects**: presigned access only,
  size- and type-checked by magic bytes, re-encoded, stored under generated keys,
  never in a public bucket, never served from the application origin.
- **BR-2.6.12** A refund records the refunded amount, a reference, an optional
  finance-uploaded proof and a reason, and moves the order to `REFUNDED`. Phase 1
  supports full refunds; the amount column allows partials later.
- **BR-2.6.13** Verification is **idempotent**: verifying an already-verified
  payment returns the original result, not a second `PAID` transition.

### 2.7 Identity, access and store scope

- **BR-2.7.1** **There is no guest checkout** (D12). Placing an order requires an
  authenticated customer account.
- **BR-2.7.2** A customer may sign in with **email + password** (email verified
  by link), **Google**, **Instagram**, or **phone + OTP** (D24). One customer may
  own several `customer_identities` rows.
- **BR-2.7.3** Identities are linked to an existing customer **only on a verified
  matching email or verified matching phone**. An unverified claim never links
  and never grants access to an existing account.
  _Prevents:_ account takeover by registering an unverified matching email.
- **BR-2.7.4** A **verified phone number is required before an order may be
  created**, whatever the sign-in method — the counter must be able to reach the
  customer. Ordering without one is **422** with a `PHONE_VERIFICATION_REQUIRED`
  code.
- **BR-2.7.5** OTP codes are 6 digits from a CSPRNG, stored hashed, single-use,
  expire in `auth.otp_ttl_minutes` (default 5), and are attempt-capped and
  rate-limited per phone and per IP. A used, expired or over-attempted code is
  indistinguishable in the response from a wrong one.
- **BR-2.7.6** Roles are: `customer`, `kitchen`, `counter`, `finance`,
  `store_manager`, `admin`, `owner`. Authorization is **deny-by-default**: a
  handler without a declared permission serves nobody.
- **BR-2.7.7** Every staff role except `admin` and `owner` is **scoped to one or
  more stores** through `staff_store_assignments`. `finance` may additionally be
  scoped group-wide by an explicit flag.
- **BR-2.7.8** Store scope is a **tenancy boundary enforced in the repository
  layer**: every query for a store-scoped entity carries the caller's permitted
  store set. Enforcing it only in handlers is non-conformant.
  _Prevents:_ one forgotten handler exposing every store's orders.
- **BR-2.7.9** A request's store is resolved **server-side** from the payload or
  the target entity and re-checked against the caller's assignments. A store id
  or role claim sent by the client is never trusted.
- **BR-2.7.10** Cross-store access is **403** and is covered by a negative test
  per role and per resource. A customer reading another customer's order is
  **404** (not 403 — existence is not disclosed).
- **BR-2.7.11** Order tracking requires an authenticated session owning that
  order; the order code alone is never sufficient, and lookups are rate-limited.
- **BR-2.7.12** Staff authenticate with email + password (argon2id) and a
  short-lived access token plus a rotating, hashed, revocable refresh token. Any
  privilege change issues new tokens and revokes the old ones.

### 2.8 Kitchen and fulfilment operations

- **BR-2.8.1** The orders board is **always scoped** to the staff member's
  assigned stores and grouped by slot, in slot-start order.
- **BR-2.8.2** The **production summary** for a slot aggregates quantities per
  item **and per selected option** across all non-cancelled orders in that slot,
  so the kitchen reads "18 × Nasi Goreng, 12 of them extra spicy" rather than 18
  separate tickets.
- **BR-2.8.3** Slots are ordered on the board by start time; items within a slot
  are ordered by the item's `prep_minutes` descending, so the longest-cooking
  dish is started first.
- **BR-2.8.4** A kitchen ticket may be printed per order and per slot; the ticket
  shows the order code, slot, customer first name, lines, options and notes, and
  never shows payment details or the customer's full contact.
- **BR-2.8.5** Only orders in `ACCEPTED` or later appear on the kitchen board.
  Unpaid orders never reach the kitchen.
  _Prevents:_ cooking for an order that was never paid.
- **BR-2.8.6** Handover requires matching the **order code**; the counter marks
  `PICKED_UP`, which is audited with the acting user.

### 2.9 Configurable parameters

- **BR-2.9.1** The following are `sys_parameters` with per-store overrides where
  marked (†), and none of them may be hard-coded:

| Key | Default | Meaning |
|---|---|---|
| `scheduling.slot_length_minutes` † | 30 | Slot size |
| `scheduling.max_orders_per_slot` † | 12 | Capacity axis 1 |
| `scheduling.max_kitchen_units_per_slot` † | 60 | Capacity axis 2 |
| `scheduling.lead_time_minutes` † | 90 | Earliest bookable distance from now |
| `scheduling.cutoff_minutes` † | 60 | Slot closes this long before start |
| `scheduling.max_advance_days` † | 14 | Furthest bookable date |
| `scheduling.cancel_cutoff_minutes` † | 120 | Customer self-cancel limit |
| `orders.auto_cancel_minutes` | 0 | 0 = off in phase 1 (D25) |
| `orders.max_unpaid_per_customer` | 2 | 0 = unlimited (BR-2.3.15) |
| `pricing.tax_bps` † | 1000 | PB1 10% |
| `pricing.service_charge_bps` † | 0 | Service charge |
| `pricing.tax_inclusive` | false | Inclusive vs. exclusive pricing |
| `pricing.quote_ttl_minutes` | 15 | Quote validity |
| `fulfilment.delivery_enabled` | false | Phase-2 switch (D16) |
| `auth.otp_ttl_minutes` | 5 | OTP lifetime |
| `auth.otp_max_attempts` | 5 | Attempts before lockout |
| `auth.access_token_minutes` | 15 | Access token lifetime |
| `auth.refresh_token_days` | 30 | Refresh token lifetime |
| `auth.provider_google_enabled` | false | Until credentials exist (Q8) |
| `auth.provider_instagram_enabled` | false | Until credentials exist (Q8) |
| `notify.provider` | `waha` | `waha` \| `meta_cloud` \| `log` |
| `notify.event.*_enabled` | true | Per-event switches (D28) |
| `notify.template.*` | (text) | Message templates, ID + EN |
| `company.name/phone/email/address` | — | Group identity |
| `finance.verification_sla_minutes` | 60 | Ageing alarm only |

- **BR-2.9.2** Changing a parameter takes effect without a deploy and is audited
  with before/after values.
- **BR-2.9.3** A parameter change never rewrites historical data — snapshots
  (BR-2.5.1) and already-booked slots (BR-2.3.16) are unaffected.

### 2.10 Audit, notifications and data handling

- **BR-2.10.1** Every privileged action writes to the append-only `audit_log`:
  actor, action, entity type and id, store id, before/after values, IP and user
  agent, timestamp. This covers price changes, parameter changes, schedule and
  blackout changes, verifications, rejections, refunds, cancellations, staff and
  role changes, and store activation.
- **BR-2.10.2** `audit_log`, `order_events` and `payment_events` are
  **append-only**: no `UPDATE`, no `DELETE`, enforced by permissions and stated
  in the migration.
- **BR-2.10.3** Automated customer notifications are limited to: order received
  (with transfer instructions and the exact amount due), payment verified, order
  ready, and a pre-slot reminder (D28). Each is individually switchable.
- **BR-2.10.4** Every notification send is logged with its provider, template,
  target, result and attempt count. Failures retry with exponential backoff up to
  a bounded number of attempts; a customer opt-out is respected on all
  non-transactional messages.
- **BR-2.10.5** Notification templates and their ID/EN copy live in
  `sys_parameters`, never inline in code.
- **BR-2.10.6** Personal data is minimised to what an order needs (name, phone,
  optional email, optional address for phase 2). PII never appears in logs or
  URLs, and a documented deletion path exists (see `12-security.md`).

---

## 3. Permissions matrix

Legend: **✓** allowed · **✓ (scope)** allowed only for assigned stores ·
**—** denied (403; 404 where existence must not be disclosed).

| Action | customer | kitchen | counter | finance | store_manager | admin | owner |
|---|---|---|---|---|---|---|---|
| Browse stores / menu / availability | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Create order | ✓ (own) | — | — | — | — | — | — |
| View order | ✓ (own) | ✓ (scope) | ✓ (scope) | ✓ (scope) | ✓ (scope) | ✓ | ✓ |
| Cancel order — customer window | ✓ (own) | — | — | — | — | — | — |
| Cancel order — any time, with reason | — | — | ✓ (scope) | — | ✓ (scope) | ✓ | ✓ |
| Upload payment proof | ✓ (own) | — | — | — | — | — | — |
| View payment queue | — | — | — | ✓ (scope/group) | ✓ (scope) | ✓ | ✓ |
| Verify / reject payment | — | — | — | ✓ (scope, not own order) | — | ✓ | ✓ |
| Record refund | — | — | — | ✓ (scope) | — | ✓ | ✓ |
| Reconciliation + export | — | — | — | ✓ (scope/group) | ✓ (scope) | ✓ | ✓ |
| Kitchen board + production summary | — | ✓ (scope) | ✓ (scope) | — | ✓ (scope) | ✓ | ✓ |
| Transition IN_KITCHEN / READY | — | ✓ (scope) | — | — | ✓ (scope) | ✓ | ✓ |
| Mark PICKED_UP | — | — | ✓ (scope) | — | ✓ (scope) | ✓ | ✓ |
| Print kitchen ticket | — | ✓ (scope) | ✓ (scope) | — | ✓ (scope) | ✓ | ✓ |
| Store hours / overrides / blackouts | — | — | — | — | ✓ (scope) | ✓ | ✓ |
| Store capacity / slot settings | — | — | — | — | ✓ (scope) | ✓ | ✓ |
| Store CRUD (create/deactivate) | — | — | — | — | — | ✓ | ✓ |
| Store bank accounts | — | — | — | ✓ (scope, read) | — | ✓ | ✓ |
| Menu CRUD (group level) | — | — | — | — | — | ✓ | ✓ |
| Per-store availability / "86" | — | ✓ (scope, 86 only) | — | — | ✓ (scope) | ✓ | ✓ |
| Per-store price override | — | — | — | — | — | ✓ | ✓ |
| Promotions CRUD | — | — | — | — | — | ✓ | ✓ |
| Staff accounts + store assignment | — | — | — | — | — | ✓ | ✓ |
| `sys_parameters` CRUD | — | — | — | — | — | ✓ | ✓ |
| Reports — one store | — | — | — | ✓ (scope) | ✓ (scope) | ✓ | ✓ |
| Reports — all stores | — | — | — | ✓ (group only) | — | ✓ | ✓ |
| Audit log viewer | — | — | — | — | ✓ (scope) | ✓ | ✓ |

Notes:

- `owner` is `admin` plus the ability to change owner-level parameters and
  deactivate stores; both are group-wide and unscoped.
- A `finance` user flagged group-wide sees every store's payments; otherwise only
  assigned stores (BR-2.7.7).
- Every **—** in a scoped row is a required negative test (`07-test-plan.md`).
