# Test Plan — ruuma

**Version:** 1.0
**Date:** 2 August 2026

Every `BR-x.y` in `02-business-rules.md` must be referenced by at least one test
name. `make test-br-coverage` greps the rule ids out of the docs and fails the
build if any rule has no referencing test.

---

## 1. Layers

| Layer | Kind | Where | Needs |
|---|---|---|---|
| `internal/domain/*` | pure unit, table-driven, exhaustive | `*_test.go` beside the code | nothing — no I/O |
| `internal/app/*` | service tests with fake ports | `*_test.go` | in-memory fakes |
| `internal/adapter/postgres` | integration | `*_integration_test.go`, build tag `integration` | `ruuma_test` DB |
| `internal/adapter/http` | handler + middleware tests | `httptest` | fakes + real router |
| End to end | the definition-of-done journey | `test/e2e` | DB + MinIO + fake notify |
| Security | negative authz, IDOR, fuzz, JWT | `test/security` | DB |
| Frontend | component + a11y | `web/src/**/*.test.tsx` (vitest) | node |

Commands: `make test` (unit), `make test-integration` (drops/recreates
`ruuma_test`), `make test-e2e`, `make test-security`, `make check`
(vet + staticcheck + gosec + govulncheck + npm audit).

## 2. Critical scenarios

### 2.1 Money (BR-1.1.x, BR-2.5.x)

- Line total with multiple options, including a negative delta, never below zero.
- Half-up rounding at exact `.5` boundaries for tax and service charge, in bps.
- Tax-exclusive vs. `pricing.tax_inclusive` produce consistent totals.
- Order totals are integer-identical to a hand-computed table of cases.
- Price snapshot: change the menu price after ordering; the order is unchanged.
- Client-sent `expected_total` mismatch → 422 `TOTAL_MISMATCH`.

### 2.2 Scheduling (BR-2.1.x, BR-2.3.x)

- Closed weekday produces **zero** slots for every mode.
- Per-date override re-opens/extends a single date; blackout closes it;
  precedence override > blackout > weekday is asserted in all six orderings.
- Blackout created for **today** blocks new orders from that instant and leaves
  booked orders untouched.
- Per-mode windows intersect correctly (delivery ends earlier than pickup).
- Lead time, cutoff, max advance and past-slot each produce their own reason code.
- Slot generation aligns to block starts and discards a short trailing interval.
- Item-level constraints (weekend-only, `min_lead_minutes`) narrow the bookable
  set for a cart containing that item.
- Store timezone: a slot at 12:00 Jakarta is stored as 05:00Z and reads back as
  12:00 regardless of server TZ (test runs with `TZ=UTC` and `TZ=America/New_York`).

### 2.3 Capacity and concurrency (BR-2.3.7–12) — the flagship test

- Two axes independently exhaust a slot (`max_orders`, `max_kitchen_units`).
- **Concurrency:** 20 goroutines check out simultaneously against a slot with
  capacity 1 → exactly one 2xx, nineteen 409 `SLOT_FULL`, and
  `reserved_orders = 1`. Repeated 50 times.
- Direct `UPDATE slots SET reserved_orders = max_orders + 1` is rejected by the
  CHECK constraint — the database refuses an oversell on its own.
- Cancellation releases capacity exactly once; a double cancel does not
  double-release.
- No auto-expiry exists: an unpaid order still holds capacity after any elapsed
  time (BR-2.3.11).
- `orders.max_unpaid_per_customer` blocks the third unpaid order with 422.

### 2.4 Order lifecycle (BR-2.4.x)

- Every legal transition succeeds; a table of all illegal pairs returns 409.
- Each transition appends exactly one `order_events` row with the right actor.
- `UPDATE`/`DELETE` on `order_events` is rejected for the app role.
- Order immutability: an attempt to change lines, slot or store after creation
  is refused.

### 2.5 Payment (BR-2.6.x)

- `AWAITING_VERIFICATION` is reachable only with an attached proof.
- *Kode unik* is unique across open orders per store; releasing on a terminal
  state allows reuse.
- Only finance may verify or reject; other roles get 403 (one test per role).
- Cross-store verification is 403 `STORE_OUT_OF_SCOPE`.
- Self-verification is 403 `SELF_VERIFICATION_FORBIDDEN`.
- Amount mismatch cannot pass without `accept_mismatch` + reason.
- Rejection requires a reason, returns to PENDING_PAYMENT, and **does not**
  release the slot.
- Verify is idempotent; a replayed `Idempotency-Key` returns the first response.
- Every verify/reject/refund writes `payment_events` and `audit_log` rows.

### 2.6 Identity and access (BR-2.7.x)

- Ordering without a verified phone → 422 `PHONE_VERIFICATION_REQUIRED`, for all
  four sign-in methods.
- Identity linking: an **unverified** matching email does not link to an existing
  customer; a verified one does.
- OTP: hashed at rest, single-use, expiry, attempt cap; wrong/expired/used/
  over-attempted responses are byte-identical.
- Deny-by-default: a handler registered without a permission declaration fails a
  routing test at startup.
- Negative authz matrix: for every role × every protected action, the denied
  cells in `02` §3 return 403 (or 404 where existence must not leak).
- IDOR: customer A cannot read, cancel or upload proof for customer B's order —
  404, not 403.
- Cross-store: staff of store A cannot read, board, cancel, verify or report on
  store B — 403, one test per role and per resource.
- JWT: tampered signature, `alg=none`, expired token, revoked refresh `jti`, and
  a refresh reuse (rotation replay) are all rejected.

### 2.7 Catalogue (BR-2.2.x)

- Store override beats group default for price and availability; absence falls
  back.
- An active 86 hides the item for the covered window only.
- Daily stock decrements inside the order transaction and blocks at zero.
- Required single-choice groups must be satisfied; multi-select respects
  min/max; an unavailable choice is refused.

### 2.8 Promotions (BR-2.5.8–12)

- Window, minimum spend, category restriction and **store scope** each reject
  with their own reason.
- Percent with `max_discount` cap; fixed capped at subtotal; never negative.
- Total and per-customer caps hold under concurrent redemption.
- Cancel/refund releases the redemption and restores the cap.

### 2.9 Operations (BR-2.8.x)

- Production summary aggregates by item **and** option across a slot.
- Board is scoped to assigned stores and ordered by slot then prep time.
- Unpaid orders never appear on the kitchen board.
- Kitchen ticket contains no payment data and no full contact details.

### 2.10 Configuration (BR-1.4.x, BR-2.9.x)

- Every parameter in the BR-2.9.1 table resolves store → group → fallback.
- Changing a parameter changes behaviour with no restart, and is audited with
  before/after.
- No hard-coded scheduling or pricing constant: a test greps the codebase for
  numeric literals in the forbidden set outside `sys_parameters` defaults.

### 2.11 Search and lists (BR-1.5.1)

- Every list endpoint accepts `q` and filters.
- A frontend test asserts every list/table screen renders a search input — the
  test enumerates routes, so a new list without search fails CI.

## 3. Security test suite

Detailed in `12-security.md` §5; summarised here as required artefacts:
negative authz per role, IDOR per resource, rate-limit tests on login/OTP/
tracking/promo, injection fuzz over every string input, the slot concurrency
test, cross-store tests per staff role, payment privilege tests, JWT
tamper/expiry tests, upload tests (magic bytes, oversize, SVG/HTML disguised as
image), and a header/CSP assertion test.

## 4. Data and fixtures

- `ruuma_test` is dropped and recreated per integration run; migrations are
  applied **up then down then up** to prove both directions.
- **The suites share one database and are serialised by a Postgres advisory
  lock.** Each environment truncates and re-seeds, so two running at once would
  pull the fixtures from under each other — and `go test` parallelises packages
  by default, which makes that easy to trip into. A suite that fails only when
  something else is running is worse than no suite, so the harness takes
  `pg_advisory_lock` for its lifetime. Verified by running all three suites
  concurrently.
- Seed fixtures mirror the demo seed: three stores with different closed days,
  modes and hours; a menu spanning all three cuisines; staff for every role,
  each scoped to a different store — which is what makes the cross-store tests
  meaningful.
- A frozen `Clock` is injected everywhere time matters; no test sleeps.

## 5. QA checklist before release

- [ ] `make check` clean (vet, staticcheck, gosec, govulncheck, npm audit)
- [ ] `make test`, `make test-integration`, `make test-e2e`, `make test-security` green
- [ ] Migrations up → down → up on a fresh database
- [ ] BR coverage check passes (every rule referenced)
- [ ] Concurrency test run 50×, zero oversell
- [ ] Definition of done walked by hand: choose store → order across three
      cuisines → pick date + slot honouring closed days → upload proof → finance
      verifies → WhatsApp received → kitchen sees only its store's board →
      admin changes a parameter without a deploy
- [ ] a11y pass: keyboard-only checkout, contrast, announced errors
- [ ] `docs/12-security.md` fully green with test references
