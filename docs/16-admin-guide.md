# Admin Guide — ruuma

**Version:** 1.0
**Date:** 2 August 2026
**Audience:** kitchen, counter, finance, store managers, admins and the owner.
**Where:** `https://admin.ruuma.id`

This guide is task-first: find what you need to do, do it, understand what the
system will and will not do for you.

---

## 1. Signing in and what you can see

Sign in with your work email and password. Your first sign-in forces a password
change.

**What you can see depends on your role and your stores.** Store scope is a hard
boundary: if you are assigned to Kelapa Gading, another store's orders, payments
and reports do not exist for you. That is not a display filter — the server
refuses those requests.

| Role | What you get |
|---|---|
| **Kitchen** | The orders board and production summary for your store(s); mark orders cooking and ready; 86 an item |
| **Counter** | The board; hand orders over; cancel with a reason |
| **Finance** | The payment queue, verify/reject/refund, reconciliation — for your store, or group-wide if you are flagged as such |
| **Store manager** | Everything operational for your store: hours, overrides, blackouts, capacity, availability, board, reports, audit |
| **Admin / Owner** | Every store, plus menu, prices, promotions, staff and parameters |

If a button you expect is missing, it is a permission, not a bug. Ask an admin.

---

## 2. Kitchen: the daily board

**Orders → pick your store and date.**

Orders are grouped by **time slot**, in slot order. Each card shows the order
code, the customer's first name, the lines with their options and notes, and
what it weighs in kitchen units.

**Production summary** (button on each slot) is what you actually cook from: it
aggregates every order in that slot into a single list — "18 × Nasi Goreng, 12
of them extra spicy" — ordered longest-prep-first so you start the right dish.
Print it, or print an individual ticket.

Workflow per order: **Start** → **Ready**. The counter then marks **Picked up**.

Two things worth knowing:

- **Unpaid orders never appear here.** If an order is not on your board, it has
  not been paid and verified. That is deliberate — you never cook for an order
  that was never paid.
- **A ticket carries no payment details** and only the customer's first name.

### Running out of a dish

**Menu → pick your store → 86.**

The dish disappears from the remaining slots immediately, at your store only.
Orders already placed are untouched. Use **Lift 86** when stock returns.

If you know the quantity you have, use **Set stock** instead: the counter
decrements with each order and the dish disables itself at zero.

---

## 3. Finance: the payment queue

**Finance → your store (or all, if you are group-scoped).**

The queue is **oldest first** — the customer who has waited longest is at the
top. Each row shows the order code, the customer, the amount due, the amount
they said they transferred, any difference, and how long it has been waiting.

For each payment:

1. **View** the proof. It opens through a short-lived private link — the file is
   never public.
2. Compare the declared amount against the amount due. **The amount due includes
   the customer's 3-digit *kode unik*** — that is how you match their transfer in
   the bank statement.
3. **Verify** if it matches. The order becomes accepted and appears on the
   kitchen board; the customer gets a WhatsApp confirmation.
4. **Reject** if it does not, choosing a reason: amount mismatch, unreadable
   proof, not received, duplicate, or other.

### When the amount does not match

The system will not let a mismatch through silently. You must accept it
explicitly and type a reason, which is stored on the payment and shows in
reconciliation. Over- and under-payments are both handled this way.

### What rejection does — and does not do

- The order goes back to **awaiting payment**.
- **The customer keeps their time slot.** Rejecting is not cancelling.
- The reason appears prominently on the customer's order page.
- **No automated message is sent.** You must contact the customer yourself. This
  was a deliberate decision: a rejection needs a human conversation.

### Refunds

**Finance → the payment → Refund.** Enter the amount, a bank reference and a
reason; attach the transfer-out proof if you have one. The order becomes
refunded. Refunds do **not** return the time slot — that slot was consumed.

### End of day

**Today's reconciliation** gives you, per store: verified orders, the order
total, the *kode unik* total, what customers declared, refunds, mismatches,
rejections and the net. The *kode unik* amounts are real revenue and are counted,
never written off.

---

## 4. Store manager: running your store

### Opening hours

**Schedule → your store.** Hours are set **per weekday and per fulfilment mode**,
so pickup can run until 21:00 while delivery stops at 20:00. Mark a closed
weekday **Closed** — a closed weekday generates no slots at all, for any mode.

Changes apply to slots generated from then on. **Slots customers have already
booked keep their capacity** — changing your hours never cancels a booking.

### One-off changes to a single date

Use a **date override** rather than editing your weekly hours: it replaces the
schedule for that one date only, in either direction (open a normally-closed
Sunday, or close early on a public holiday).

### Closing today — an emergency

**Schedule → Blackout dates → today's date + a reason.**

This takes effect immediately: no new orders can be placed for any remaining
slot that day. The screen tells you **how many orders are already booked**.

**Those orders are not cancelled.** The system will never cancel a customer's
booking for you. Go to **Orders → Affected by closure**, cancel each one with a
reason, contact the customers, and ask finance to refund anyone who has paid.

### Capacity

Each slot has two limits and both are enforced:

- **Orders per slot** — how many separate orders you can hand over.
- **Kitchen units per slot** — how much cooking you can do. Every dish carries a
  weight; a whole duck weighs eight, a drink weighs nothing much.

If lunch is chaos but the order count looks fine, the kitchen-unit limit is the
one to lower. Both live in **Parameters**, group-wide or per store.

### Lead time and cutoff

- **Lead time** — the earliest a customer may book from now (default 90 min).
- **Cutoff** — how long before a slot starts that ordering closes (default 60 min).

Raise the lead time when the kitchen is behind; lower it when you are quiet.
Both take effect immediately for new orders.

---

## 5. Admin: menu, prices and stores

### The menu is group-level; availability and price are per store

One menu across the group. Each store then overrides **availability** and
**price** where it differs — everything else falls back to the group default.
Only admins and the owner can change a price; store managers can change
availability.

Editing a price **never changes a past order**: every order line keeps the price
it was sold at.

### Adding a store

1. Create it (code, name, address, phone) — it starts **inactive**.
2. Add its fulfilment modes.
3. Set hours for all seven weekdays, marking closed days closed.
4. Add its bank account and mark it primary. **Checkout cannot complete without
   one.**
5. Set any parameters that differ from the group.
6. Add per-store menu overrides.
7. Assign staff.
8. Check the customer page, then **activate**.

Deactivating a store hides it from customers immediately and never touches its
historical orders.

### Promotions

A promo code applies to an **explicit list of stores** — there is no "all
stores" shortcut, because that is how a code meant for one launch ends up
discounting the whole group. Set the window, minimum spend, total and
per-customer caps, and any category restriction.

### Staff

**Staff → Add.** Give the narrowest role that fits and assign only the stores
they work at. Finance is group-wide only if they reconcile for the whole group.

Staff are **deactivated, never deleted** — their audit trail has to outlive
them.

---

## 6. Parameters: changing the business without a deploy

**Parameters.** Everything here takes effect immediately, is attributed to you,
and is recorded with its before and after values.

| What you want to change | Parameter |
|---|---|
| Length of each time slot | `scheduling.slot_length_minutes` |
| Orders per slot | `scheduling.max_orders_per_slot` |
| Cooking capacity per slot | `scheduling.max_kitchen_units_per_slot` |
| Earliest booking from now | `scheduling.lead_time_minutes` |
| When ordering closes before a slot | `scheduling.cutoff_minutes` |
| How far ahead customers may book | `scheduling.max_advance_days` |
| Customer self-cancel window | `scheduling.cancel_cutoff_minutes` |
| Unpaid orders one customer may hold | `orders.max_unpaid_per_customer` |
| Restaurant tax (PB1) | `pricing.tax_bps` (1000 = 10%) |
| Service charge | `pricing.service_charge_bps` |
| Prices already include tax | `pricing.tax_inclusive` |
| WhatsApp message wording | `notify.template.*` |
| Turn a message on or off | `notify.event.*_enabled` |
| Which WhatsApp provider | `notify.provider` |
| Finance ageing alarm | `finance.verification_sla_minutes` |
| WhatsApp button on the website | `company.whatsapp_enabled` |
| The number that button dials | `company.whatsapp_number` |
| What the customer's message says | `company.whatsapp_message_id` / `_en` |

Rates are in **basis points**: 1000 = 10%, 1150 = 11.5%.

### Changing the WhatsApp number

The green button at the bottom right of the customer site is
`company.whatsapp_number`. Edit it under **Parameters** and it changes on the
website within about half a minute — no deploy, no developer.

Type the number however is natural: `+62 817-6315-568`, `0817 6315 568` and
`628176315568` all work, because spaces, dashes, brackets and the `+` are
stripped and a leading `0` is swapped for `62` before the link is built.

Two things worth knowing:

- **Clearing the number hides the button**, even if `company.whatsapp_enabled`
  is still `true`. That is deliberate — a button that opens a chat with nobody
  is worse than no button, because the customer thinks they have asked and then
  waits for a reply that is not coming. To take the button down temporarily,
  either works; to change numbers, just edit the number.
- The greeting is the text already typed into the customer's WhatsApp when the
  chat opens. Keep it short and let them add their own detail.

Parameters marked **per store** can be overridden for one store under
**Schedule → that store → Parameters**; anything unset falls back to the group
value.

Changing a parameter never rewrites history: past orders keep their prices and
booked slots keep their capacity.

---

## 7. Unpaid orders and slot squatting

Phase 1 has **no automatic cancellation**. An unpaid order holds its slot until a
human cancels it. That was a deliberate decision — no machine takes a customer's
booking away — and it means somebody has to watch the list.

**Orders → Unpaid, ageing** shows them oldest first per store. Cancel with a
reason, singly or in bulk. A customer can hold at most
`orders.max_unpaid_per_customer` unpaid orders at once (default 2).

When QR payment arrives, `orders.auto_cancel_minutes` starts releasing unpaid
capacity automatically and this becomes a fallback rather than a routine.

---

## 8. Reports and the audit log

- **Dashboard** — today's slots, load per slot, revenue, cancellations, filtered
  by store or across all of them.
- **Reconciliation** — daily takings per store, exportable.
- **Audit log** — every privileged action with who did it, when, and what
  changed. Searchable, scoped to your stores.

The audit log, order history and payment history are **append-only**: nothing
can edit or delete them, including an administrator. A correction is a new
entry. That is what makes them worth having in a dispute.

---

## 9. When something goes wrong

| Symptom | Look at | Then |
|---|---|---|
| "There are no slots" | Store active? Weekday closed? Blackout today? Lead time/cutoff too aggressive? | Fix the schedule, or add a date override |
| Kitchen overwhelmed at one slot | Slot load vs. kitchen units | Lower capacity for future slots; lock the slot for today |
| Payment queue growing | Verification ageing | Add a finance user, or widen scope temporarily |
| Customer says they paid but it is unverified | The queue, filtered to their store | Check the *kode unik*; contact them if the amount is wrong |
| WhatsApp messages not arriving | Ask an admin to check the notification log | The gateway session may need re-linking; orders are unaffected |
| A customer cannot order at all | Their account | A **verified phone** is required before a first order |

---

## 10. Rules that will not bend

These are enforced by the system, and knowing them saves arguments:

1. A time slot cannot be oversold. Two customers taking the last place produces
   exactly one order.
2. Unpaid orders never reach the kitchen.
3. Only finance verifies payments, only for their stores, and never for an order
   they placed themselves.
4. An amount mismatch cannot be verified silently — it needs a reason.
5. A rejection keeps the customer's slot.
6. Nothing automatic cancels a customer's booking.
7. Editing a price or a parameter never changes a past order.
8. Staff of one store cannot see another store's orders, payments or reports.
9. Order history, payment history and the audit log cannot be edited or deleted.
10. A customer must have a verified phone before their first order.
