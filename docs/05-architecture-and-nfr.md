# Technical Architecture & Non-Functional Requirements — ruuma

**Version:** 1.0
**Date:** 2 August 2026

---

## 1. Stack

| Layer | Choice | Notes |
|---|---|---|
| Language | Go (latest, 1.26) | Transactional API + background workers |
| HTTP | `gin` | Router + middleware |
| Database | PostgreSQL 18 | Native install on the dev server (D10) |
| DB access | `gorm` + `gorm.io/driver/postgres` | ORM. **Money and capacity paths use raw SQL** with integer math (BR-1.1.x, BR-2.3.8) |
| Migrations | numbered SQL, `go:embed` | Forward-only in production |
| Auth | `golang-jwt/jwt/v5` + `golang.org/x/crypto/argon2` | Short access + rotating refresh (BR-2.7.12) |
| Object storage | MinIO (`minio-go/v7`) | Private buckets, presigned URLs only |
| Notifications | WAHA (v1) / Meta Cloud (stub) behind `notify.Provider` | D11, D28 |
| Email | SMTP (mailpit in dev) | Verification + password reset |
| Observability | Prometheus + structured slog | `/metrics` bound to localhost only |
| IDs | `google/uuid` v7 | BR-1.2.1 |
| Frontend | React 18 + Vite + TypeScript + Tailwind | Pinned React 18; **no PWA** (D22) |

## 2. Layering

Hexagonal: `adapter → app → domain`, `platform` shared. `domain` is pure — no
framework, no driver, no `net/http`, no SQL. See `../CLAUDE.md §2`; it governs.

```
cmd/api/main.go                 serve | migrate | seed | worker
internal/
  domain/
    money/         integer rupiah arithmetic, bps rounding      (BR-1.1.x)
    schedule/      weekday/override/blackout resolution, slot generation (BR-2.1, BR-2.3)
    catalog/       item availability resolution, option validation (BR-2.2)
    pricing/       line/order totals, promo evaluation          (BR-2.5)
    order/         state machine, transitions, invariants        (BR-2.4)
    payment/       verification rules, kode unik allocation      (BR-2.6)
    identity/      roles, permissions, store-scope decisions     (BR-2.7)
  app/
    catalogsvc/ availabilitysvc/ ordersvc/ paymentsvc/
    opssvc/ adminsvc/ authsvc/ reportsvc/ notifysvc/
  adapter/
    http/          gin handlers, DTOs, middleware
    postgres/      repositories (store-scope filter is mandatory)
    storage/       MinIO presigned upload/download
    notify/        waha, meta_cloud, log
    mail/          SMTP
  platform/
    config/ logging/ metrics/ apierror/ id/ security/ ratelimit/ database/ idempotency/
db/migrations + db/embed.go
web/ (Vite app: customer + lazy-loaded admin)
```

**The dependency rule is enforced by a test** (`internal/architecture_test.go`)
that walks imports and fails if `domain` imports `gin`, `gorm`, `net/http`,
`database/sql` or any adapter package.

## 3. Key mechanisms

### 3.1 Capacity reservation (BR-2.3.8–10)

One transaction: `SELECT … FOR UPDATE` on the slot row → verify both axes →
verify item availability, 86 and daily stock → `UPDATE slots SET reserved_… ` →
insert order. The `slots_no_oversell_*` CHECK constraints are the database's own
refusal; a constraint violation surfaces as **409 `SLOT_FULL`**, never a 500.
Proven by a concurrency test that fires N simultaneous checkouts at a slot with
capacity 1 and asserts exactly one 2xx.

### 3.2 Store scope (BR-2.7.8)

Every store-scoped repository method takes a `scope.Stores` value derived from
the authenticated principal, and every generated query includes
`store_id = ANY($scope)`. There is no repository method that reads a scoped
entity without it — enforced by review and by a negative test per role. Handlers
add a second check, but the repository is the boundary.

### 3.3 Time (BR-1.3.2)

`schedule` takes an explicit `*time.Location` from the store. Slot generation
converts store-local opening blocks to UTC instants once, at materialisation.
Nothing calls `time.Now()` inside the domain — a `Clock` port is injected, so
cutoff and lead-time rules are testable to the minute.

### 3.4 Idempotency

`platform/idempotency` stores `(key, subject, endpoint, request_hash)` with the
first response. Same key + same body replays the stored response; same key +
different body is 409 `IDEMPOTENCY_MISMATCH`.

### 3.5 Background worker

`cmd/api worker` runs: slot materialisation (rolling `max_advance_days` window
per store), notification dispatch with exponential backoff, daily-stock rollover,
and expired-token cleanup. **It never cancels an order** (BR-2.3.11, D25).

## 4. Performance targets

| Path | Target |
|---|---|
| `GET /menu` (store-resolved, cached) | p95 < 200 ms at 100 concurrent |
| `GET /availability/slots` | p95 < 250 ms |
| `POST /orders` (full transaction) | p95 < 400 ms |
| Admin lists | p95 < 300 ms at 20 concurrent staff |
| Oversell under concurrency | **0**, always |

Indexes are stated in `03-data-model.md`. Menu and store reads are cached in
memory with a short TTL keyed by store and language; every write to menu, price,
override or 86 busts the cache for that store.

## 5. Observability

- `/health` (liveness) and `/health/ready` (DB + storage reachable).
- `/metrics` Prometheus, **bound to 127.0.0.1** and never exposed publicly.
- Structured `slog` JSON logs with a request id propagated through context;
  PII (phone, email, address, proof keys) is redacted by a logging filter.
- Counters/histograms: request duration by route and status, order creations by
  outcome, slot rejections by reason, payment verifications and rejections,
  notification sends by result, DB transaction retries.
- Alerts: auth-failure spikes, 5xx rate, payment verification ageing beyond
  `finance.verification_sla_minutes`, notification failure rate, worker lag.

## 6. Security

Full control map in `12-security.md`. Summary: argon2id passwords, short access
tokens plus rotating hashed refresh tokens, deny-by-default authorization with
repository-level store scope, magic-byte-checked private uploads, CSP and the
rest of the header set, per-identifier and per-IP rate limiting, append-only
audit log, no secrets outside env, `govulncheck`/`gosec`/`staticcheck` in CI.

## 7. Reliability

- Graceful shutdown drains in-flight requests (15 s) before exit.
- DB pool bounded; statement timeout set; long report queries run with a
  separate, longer timeout.
- Migrations run before the new binary serves; every migration has a tested
  `.down.sql`.
- Nightly `pg_dump` plus MinIO bucket sync; documented restore drill with a
  30-minute recovery target (`09-deployment.md`).

## 8. Deployment

See `09-deployment.md`. Single Ubuntu node, nginx + certbot, native PostgreSQL,
MinIO under systemd, multi-stage Docker build to a small static binary
(`ruuma.id`, D21).
