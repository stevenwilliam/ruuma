# Domain Operations — ruuma

**Version:** 1.0
**Date:** 2 August 2026

How the product is actually run, day to day, per store. Rules referenced here are
normative in `02-business-rules.md`.

---

## 1. The operating day

| Time (store-local) | Who | What |
|---|---|---|
| Previous evening | Worker | Materialises slots for the rolling `max_advance_days` window; rolls daily stock |
| Opening − 60 min | Store manager | Checks today's board, confirms 86s and daily stock, fixes capacity if short-staffed |
| Through the day | Finance | Works the payment queue oldest-first; SLA alarm at `finance.verification_sla_minutes` |
| Per slot, −`prep_minutes` | Kitchen | Reads the production summary, cooks by longest-prep-first, marks READY |
| Per slot | Counter | Matches the order code, hands over, marks PICKED_UP |
| Close | Store manager | Reviews unpaid ageing list, cancels stale orders with a reason |
| Nightly | Finance | Reconciliation per store; exports takings including the *kode unik* total |

## 2. Runbooks

### 2.1 Emergency closure (today) — BR-2.1.7, BR-2.1.9, D27

1. Admin → Stores → *store* → Blackouts → add **today's date** with a reason.
2. The response states how many orders are already booked for that date. **They
   are not cancelled.**
3. Go to Ops → Orders → *Affected by closure*. Cancel each one with a reason
   (bulk cancel is available) — cancelling releases its slot capacity.
4. Contact those customers by hand; refund any verified payments via Finance →
   Refund with a reference and proof.
5. The blackout, every cancellation and every refund are in the audit log.

### 2.2 A dish runs out mid-service — BR-2.2.3, BR-2.2.4

- **Today only:** Ops → Menu → 86 the item with `ends_at` = end of today. It
  disappears from the remaining slots only; earlier orders are untouched.
- **Indefinitely:** 86 with no end date, then lift it when stock returns.
- **Known quantity:** set daily stock instead; the counter decrements inside the
  order transaction and the item self-disables at zero.

### 2.3 A payment lands that doesn't match — BR-2.6.7, BR-2.6.8

1. Finance opens the payment; the proof is a presigned, private object.
2. Compare the declared amount against `amount_due` (total **plus** the *kode
   unik*, which is how the transfer is matched at all).
3. Exact match → **Verify**. The order becomes PAID → ACCEPTED and reaches the
   kitchen board.
4. Over/under payment the group accepts → **Verify with mismatch**, reason
   required; the difference is recorded and shows in reconciliation.
5. Unreadable, not received, duplicate → **Reject** with the reason. The order
   returns to PENDING_PAYMENT, **keeps its slot**, and the customer can upload a
   new proof.
6. **No automated message is sent on rejection** (D28) — call the customer.

### 2.4 Slot squatting / unpaid pile-up — BR-2.3.15, D25

Phase 1 has no auto-cancel. Controls:

- `orders.max_unpaid_per_customer` (default 2) caps concurrent unpaid orders.
- Ops → Orders → *Unpaid, ageing* lists them oldest-first per store.
- Staff cancel singly or in bulk, with a reason; capacity is released.
- When QRIS lands, set `orders.auto_cancel_minutes` above zero and the worker
  takes over this job.

### 2.5 Changing a store's hours or capacity — BR-2.1.12, BR-2.3.16

1. Admin → Store → Hours (per weekday **per mode**, multiple blocks) or
   Parameters (slot length, capacity, lead, cutoff, advance days).
2. Changes apply to slots materialised **after** the change; already-booked slots
   keep their capacity.
3. Lowering capacity below what is already reserved requires explicit
   confirmation and is audited.
4. A one-off change for a single date is a **date override**, not an hours edit.

### 2.6 Onboarding a new store

1. Create the store (code, address, phone, timezone) — inactive.
2. Add fulfilment modes (phase 1: pickup).
3. Set hours for all seven weekdays per mode, marking closed days closed.
4. Add the bank account and mark it primary.
5. Set any store parameters that differ from the group defaults.
6. Add store menu overrides (price/availability) where the store differs.
7. Assign staff (`staff_store_assignments`).
8. Generate slots, verify the customer store page reads correctly, then activate.

### 2.7 Onboarding a staff member

Create the user with the narrowest role that fits, assign only the stores they
work at, and hand over credentials that must be changed on first login. Finance
is group-scoped **only** if they reconcile for the whole group. Removing someone
is a deactivation, never a delete — their audit trail must survive.

## 3. Reports and reconciliation

- **Daily takings per store:** verified payments for the business date, split
  into order totals and *kode unik* amounts (BR-2.6.3), minus refunds.
- **Slot load:** reserved vs. maximum on both axes, per slot, to retune capacity.
- **Top items** by quantity and revenue, filterable by store and date range.
- **Cancellations** by actor and reason — the honest measure of operational pain.
- **Verification ageing** — median and worst-case time from proof to decision.

## 4. Escalations

| Symptom | First check | Then |
|---|---|---|
| Customers report "no slots" | Store active? Weekday closed? Blackout today? Lead time/cutoff too aggressive? | Adjust the parameter, or add a date override |
| Kitchen overwhelmed at one slot | Slot load report; `max_kitchen_units` vs. actual | Lower capacity for future slots; lock the slot for the rest of today |
| Payment queue growing | Verification ageing report | Add a finance user, or widen scope temporarily |
| WhatsApp notifications failing | `notifications` table `last_error`; WAHA session state | Re-link the WAHA session; failures retry with backoff (BR-2.10.4) |
| Duplicate transfers | Reconciliation mismatch list | Refund the duplicate with a reference and proof |
