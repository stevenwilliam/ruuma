# Product Requirements Document — ruuma

**Status:** Confirmed (D8–D29)
**Owner:** stevenwilliam ("ven")
**Version:** 1.0
**Date:** 2 August 2026

---

## 1. Problem statement

ruuma is a **multi-outlet restaurant group** — several stores in different
locations, serving Indonesian, Chinese and Western food. Ordering today happens
over WhatsApp and phone calls, and every failure comes from the same root cause:
**nobody knows what to cook, for when, at which store.**

Concretely, the current-state failure modes are:

- **The kitchen cooks blind.** Orders arrive as chat messages with no agreed
  pickup time, so the kitchen either cooks early (cold food) or cooks late
  (queues at the counter). There is no view of "what is due at 12:30".
- **The store oversells itself.** Nothing counts how many orders one 30-minute
  window can physically produce, so a busy Friday takes 40 orders for 12:00 and
  disappoints most of them.
- **Every store is different and nothing knows it.** One store closes Saturday
  and Sunday, another closes Saturday only, another opens later. Staff answer
  "are you open?" by hand, dozens of times a day, and get it wrong on holidays.
- **Payment is unverifiable.** Customers transfer to a bank account and send a
  screenshot; finance matches transfers to orders by eye, and two identical
  amounts from two customers are indistinguishable.
- **The menu is a moving target.** A dish sells out at one store but stays on the
  shared price list; a price rises and old quotes get honoured by accident.
- **No history worth the name.** No reorder, no per-store revenue, no view of
  which dish sells at which hour.

ruuma removes all six by making the **store + fulfilment slot** the centre of the
product: a customer picks a store, sees only what that store can actually make,
and books a specific time window the store has capacity for. The kitchen gets a
slot-by-slot production board; finance gets a queue of payments it can match to
the rupiah; the owner gets numbers per store.

### 1.1 Why now / why this shape

The group is opening more outlets. Per-store scheduling and capacity is exactly
the thing that does not scale by adding another WhatsApp number — it needs master
data, rules and a board. Payment stays **manual bank transfer** in phase 1 (D13)
because that is what the group has today; the payment layer sits behind a
provider port so QRIS and a full gateway drop in later without reshaping the
order flow (D25).

---

## 2. Personas

| Persona | Goal | Primary actions |
|---------|------|-----------------|
| **Customer** (registered — no guest checkout, D12) | Get the food they want, at a time they choose, from the nearest open store, without a phone call | Pick store · browse menu · configure item options · cart · pick date + slot · pay by transfer + upload proof · track order · reorder from history |
| **Kitchen staff** | Know exactly what to cook and by when | Open the slot board for their store · read the aggregated production summary · print the kitchen ticket · mark items and orders ready |
| **Counter / driver** | Hand the right bag to the right person on time | See ready orders for the current and next slot · verify the order code · mark picked up (phase 2: dispatch delivery) |
| **Finance staff** | Turn a pile of transfer screenshots into verified revenue | Work the pending-payment queue oldest-first · compare the declared amount + *kode unik* against the order total · verify or reject with a reason · record refunds · reconcile daily takings per store |
| **Store manager** | Run one store's operating reality | Weekday hours and opening blocks · per-date overrides · blackout dates (including today) · slot length and capacity · lead time and cutoffs · local menu availability and price overrides · their store's orders board |
| **Admin / owner** | Run the group | All stores · menu, options and photos · promos · delivery zones (phase 2) · staff accounts and store assignments · `sys_parameters` · reports across and per store · audit log |

Every staff role except admin/owner is **scoped to one or more stores**. Finance
may be scoped to a store or to the whole group. Cross-store access is a 403
(BR-2.7.x).

---

## 3. Scope

### 3.1 v1 — the thin slice, never cut

**Fulfilment is PICKUP ONLY in phase 1** (D16). Delivery is modelled in the
schema and master data but disabled behind a feature flag.

**Customer web**

1. **Store selection.** Cards with address, phone, today's open/closed state,
   supported modes and the honest schedule ("closed Sat & Sun", "pickup until
   21:00", next open date). The choice is remembered. Changing store
   **revalidates the cart** against that store's menu, prices and availability
   and warns before dropping anything unavailable.
2. **Home = the selected store's menu.** Grid by category (Indonesian / Chinese /
   Western), cuisine and dietary filters, sort, and a **debounced search box**
   (BR-1.5.1). Cards show photo, name, short description, price, tags (spice
   level, halal, vegetarian, contains pork / alcohol / nuts) and availability.
3. **Item detail with option groups.** Required single-choice (rice type, spice
   level) and optional multi-choice add-ons, each with its own price delta and
   its own availability.
4. **Cart** with quantity editing, per-item notes, and a total recomputed
   **server-side** before checkout.
5. **Checkout, in this order:** store (preselected, changeable) → fulfilment type
   (only modes the store supports; phase 1 shows pickup) → **date** (only open
   dates) → **time slot** (remaining capacity per slot; every disabled slot
   states its reason) → contact details → promo code → payment (that store's bank
   account, the exact amount **including the *kode unik***, proof upload) →
   confirmation with the order code.
6. **Order tracking** — live state machine, the store and slot booked, payment
   state including "waiting for verification" and, if rejected, the reason and a
   re-upload button.
7. **Registered customers:** order history, one-tap **reorder** (revalidated
   against current menu, price and availability), saved addresses (phase 2 use),
   favourites.
8. **Sign-in:** email + password (email verified), Google, Instagram, or phone +
   OTP (D24). A **verified phone is required before the first order**, whatever
   the sign-in method.

**Admin web** — every list has a search box (BR-1.5.1):

- **Stores** CRUD: name, code, address, geo, phone, timezone, active flag,
  supported fulfilment modes, bank accounts, ticket/printer settings; delivery
  zones and fees exist but are phase-2-flagged.
- **Store schedule:** opening hours per weekday **per fulfilment mode**, multiple
  opening blocks per day (lunch/dinner), closed-weekday flag, **per-date
  overrides** (D18), **blackout dates including the current day** (D27), slot
  length, lead time, cutoff, capacity (orders + kitchen units), max advance days.
- **Menu:** categories, items, option groups and choices, photos to MinIO,
  availability toggles, scheduled **"86"** (out of stock until a date/slot), and
  **per-store availability + price overrides** falling back to the group default.
- **Orders board** scoped to the staff member's stores, grouped by slot, with the
  **kitchen production summary** (aggregated item + option quantities per slot)
  and a printable ticket.
- **Finance queue:** pending payments with proof, amount, store, order and
  customer; **verify** or **reject with a reason** (D26); refunds; daily
  reconciliation per store; ageing view oldest-first; export.
- **Staff accounts** and their store assignments.
- **Promotions** (scoped to an explicit list of stores, D15), reports (sales per
  day / slot / item, cancellations, top items), **`sys_parameters` CRUD**, audit
  log viewer.

**Notifications** (D28) — WhatsApp via the notify port: order received (with
transfer instructions and the exact *kode unik* amount), payment verified, order
ready for pickup, and a pre-slot reminder. **No automated message on rejection** —
finance and operations handle that by hand.

### 3.2 Explicitly out of v1

Table reservations · dine-in QR ordering · loyalty points · marketplace
integrations (GoFood/GrabFood) · driver map tracking · subscriptions ·
inter-store stock transfer · online payment gateways (the port exists; only
manual transfer is implemented) · **delivery** (phase 2, D16) · **auto-cancel of
unpaid orders** (arrives with QRIS, D25) · **PWA / offline** (D22).

---

## 4. Functional requirements

Logic lives in `02-business-rules.md`; this section names the capability and
points at the normative rule.

| ID | Requirement | Rules |
|----|-------------|-------|
| FR-1 | Customers browse stores and see each store's true open state, modes and next open date | BR-2.1.x |
| FR-2 | Menu, prices and availability resolve **per store**, falling back to group defaults | BR-2.2.x |
| FR-3 | Items carry option groups with price deltas and their own availability | BR-2.2.5–7 |
| FR-4 | The bookable date set derives from weekday schedule, per-date override, blackout and max advance | BR-2.3.1–4 |
| FR-5 | The bookable slot set derives from the mode window, lead time, cutoff and remaining capacity | BR-2.3.5–9 |
| FR-6 | Slot capacity is reserved atomically and can never oversell | BR-2.3.10–12 |
| FR-7 | Orders are priced server-side from snapshots; the client total is never trusted | BR-2.5.x |
| FR-8 | Payment is manual transfer with a *kode unik*; finance verifies or rejects with a reason | BR-2.6.x |
| FR-9 | The order lifecycle is a closed state machine with an append-only event log | BR-2.4.x |
| FR-10 | Customers sign in by email / Google / Instagram / phone; a verified phone gates ordering | BR-2.7.1–5 |
| FR-11 | Staff access is deny-by-default and scoped to assigned stores | BR-2.7.6–10 |
| FR-12 | Kitchen sees an aggregated production summary per slot and can print a ticket | BR-2.8.x |
| FR-13 | Promotions validate window, caps, minimum spend and store scope, and never go negative | BR-2.5.8–12 |
| FR-14 | Every configurable value is a `sys_parameter` with admin CRUD, per-store override where relevant | BR-1.4.x, BR-2.9.x |
| FR-15 | Every privileged action is audited; every list is searchable | BR-1.5.1, BR-2.10.x |

## 5. Non-functional requirements

See `05-architecture-and-nfr.md` and `12-security.md`. Targets: p95 < 300 ms on
menu and availability reads at 100 concurrent customers per store; **zero**
oversell under concurrent checkout, proven by test; ASVS L2; WCAG AA; restore
from a nightly backup inside 30 minutes.

## 6. Success metrics

1. **Slot adherence** — ≥ 95% of orders handed over inside their booked slot.
2. **Oversell incidents** — exactly **0**.
3. **Payment verification lag** — median < 30 minutes from proof upload to
   finance decision during opening hours.
4. **Order completion rate** — ≥ 85% of created orders reach COMPLETED (the rest
   cancelled, not abandoned unpaid).
5. **Reorder share** — ≥ 20% of orders from registered customers start as a
   one-tap reorder by month three.
