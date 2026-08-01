# Security — ruuma

**Version:** 1.0
**Date:** 2 August 2026
**Target:** OWASP **ASVS v4 Level 2** and every **OWASP Top 10 (2021)** category
(D23).

Each control names **where it is implemented** and **the test that proves it**.
A control without a test is not a control.

**Status as of 2026-08-02 — the hardening pass has run.** `go vet`,
`staticcheck` and `gosec` are clean, `govulncheck` reports no vulnerability
reachable from ruuma's code, all 115 `BR-x.y` rules are referenced by code or a
test, and migrations apply up → down → up on an empty database. What is
**not** yet exercised is listed in §7.

---

## 1. Threat model in one paragraph

ruuma holds customers' names, phones and order history; **payment proofs**
(financial evidence, images of bank apps); per-store operational data; and
staff accounts with real money powers. The tenancy boundary is the **store**:
staff of one store must never see or touch another's orders, payments or
reports. The most valuable attacks are: reading someone else's order or proof,
verifying a payment you shouldn't, squatting or oversubscribing slots to deny
service, brute-forcing OTPs or promo codes, and getting a stored file to execute.

## 2. Control map — OWASP Top 10 (2021)

### A01 Broken access control

| Control | Implementation | Test |
|---|---|---|
| Deny-by-default authorization | `platform/security` middleware; every route declares a permission; a route without one fails a startup assertion | `test/security/authz_default_test.go` |
| Store scope as a tenancy boundary | `scope.Stores` threaded into **every** store-scoped repository method; queries carry `store_id = ANY($scope)` (BR-2.7.8) | `test/security/cross_store_test.go` — per role, per resource |
| No IDOR | Object reads are filtered by owner **and** store; another customer's order is **404**, not 403 | `test/security/idor_test.go` — per resource |
| Admin surface separated | Separate gin router group + separate host (`admin.ruuma.id`) | `adapter/http/router_test.go` |
| Payment privilege | finance only, in scope only, never own order (BR-2.6.5/6) | `test/security/payment_privilege_test.go` |
| CORS | Explicit allow-list from `CORS_ALLOWED_ORIGINS`; credentials only for known origins; no wildcard | `adapter/http/middleware/cors_test.go` |

### A02 Cryptographic failures

| Control | Implementation | Test |
|---|---|---|
| Password hashing | argon2id, tuned (64 MB, t=3, p=2), per-password salt | `platform/security/password_test.go` |
| Token design | 15-min access JWT; refresh token **rotating**, stored hashed, revocable; `jti` denylist on logout; new tokens on privilege change | `test/security/jwt_test.go` |
| OTP | 6 digits CSPRNG, hashed at rest, single-use, 5-min TTL, attempt-capped | `domain/identity/otp_test.go` |
| TLS | TLS 1.2+ only, HSTS (`includeSubDomains; preload`) at nginx | deployment checklist + `testssl` run |
| Secrets | env only; `.env` git-ignored; `is_secret` parameters masked in UI and logs; documented rotation (`09` §3) | `platform/config/redaction_test.go` |
| No PII in logs/URLs | Logging filter redacts phone, email, address, proof keys; ids in paths, never personal data | `platform/logging/redact_test.go` |

### A03 Injection

| Control | Implementation | Test |
|---|---|---|
| SQL | gorm parameter binding; raw SQL only with placeholders; **no string concatenation** anywhere near a query | `test/security/injection_fuzz_test.go` (fuzz over every string input) |
| Input validation | Allow-list binding + validator tags at the adapter edge; the domain assumes valid input | handler table tests |
| Output encoding | React escapes by default; **`dangerouslySetInnerHTML` is banned** by an ESLint rule | `web` lint rule + CI |
| No shell-outs | No `os/exec` in the codebase | grep assertion in `make check` |

### A04 Insecure design

| Abuse case | Control | Test |
|---|---|---|
| Slot squatting | `orders.max_unpaid_per_customer` cap, ageing list, staff bulk cancel (BR-2.3.15, D25) | `test/security/slot_squat_test.go` |
| Slot oversell (race) | `FOR UPDATE` + CHECK constraints (BR-2.3.8/9) | `test/security/slot_concurrency_test.go` |
| OTP flooding | 3/10 min per phone, 10/hour per IP, attempt cap, generic responses | `test/security/ratelimit_test.go` |
| Promo brute force | 10/min per identifier, generic failure reasons, usage caps enforced transactionally | `test/security/promo_bruteforce_test.go` |
| Order enumeration | 8-char Crockford code from CSPRNG, tracking requires an authenticated owner, rate-limited (BR-2.7.11) | `test/security/order_enum_test.go` |
| Menu scraping | Rate limit + cursor pagination caps; menu is public by design | `ratelimit_test.go` |
| Fake payment verification | finance-only, scope-checked, no self-verify, all events immutable | `payment_privilege_test.go` |

### A05 Security misconfiguration

| Control | Implementation | Test |
|---|---|---|
| Security headers | CSP without `unsafe-inline` (hashed/nonce'd), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` + `frame-ancestors 'none'`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy` minimal | `middleware/headers_test.go` |
| Debug surfaces off | no pprof in production; gin release mode; `/metrics` bound to `127.0.0.1` | `router_test.go` + deployment check |
| Least-privilege DB | `ruuma_app` has no `CREATE`; append-only tables are `INSERT`-only (BR-2.10.2) | `migrations` + `test/security/append_only_test.go` |
| Private object storage | MinIO bucket private; presigned URLs only; generated object names | `adapter/storage/presign_test.go` |
| No default admin | First-run setup flow creates the first owner; no seeded production credential | `cmd/api` setup test |
| Errors leak nothing | Single error model; driver errors mapped in `platform/apierror`; stack traces never serialised | `apierror/mapping_test.go` |

### A06 Vulnerable and outdated components

`govulncheck`, `gosec`, `staticcheck` and `npm audit` run in `make check` and in
CI on every push. Go and npm dependencies are pinned (`go.sum`,
`package-lock.json`). Update cadence: dependency review monthly, security
advisories acted on within 7 days, Go minor upgrades within 30 days of release.

### A07 Identification and authentication failures

| Control | Implementation | Test |
|---|---|---|
| Rate limiting | Per identifier **and** per IP on login, staff login, OTP request/verify, tracking, promo (`04` §9) | `ratelimit_test.go` |
| Progressive lockout | `failed_attempts` + `locked_until` with exponential backoff; documented unlock path (admin action, audited) | `authsvc/lockout_test.go` |
| No session fixation | New access + refresh on any privilege change; old `jti` revoked | `jwt_test.go` |
| Generic auth errors | Login, OTP and reset responses never reveal account existence | `authsvc/enumeration_test.go` |
| Password policy | Minimum length 12, breach-list (k-anonymity local list) check, no composition theatre | `password_policy_test.go` |
| Identity linking | Links only on a **verified** matching email or phone (BR-2.7.3) | `identity/link_test.go` |

### A08 Software and data integrity failures

- CI dependencies pinned by digest; `go.sum` and `package-lock.json` committed.
- Migrations are reviewed, numbered and forward-only; `.down.sql` tested in CI.
- The phase-2 payment webhook verifies an **HMAC signature** and a timestamp
  inside a 5-minute window, rejects replays by event id, and is idempotent —
  built and tested now so phase 3 needs no new plumbing (`04` §10).
- No third-party script tags; fonts and assets are self-hosted (so CSP needs no
  external origins).

### A09 Logging and monitoring failures

- Structured `slog` JSON with a request id propagated through context.
- **Append-only `audit_log`** for every privileged action with actor, action,
  entity, store, before/after, IP, user agent (BR-2.10.1).
- Alerts on auth-failure spikes, 5xx rate, payment verification ageing, and
  notification failures.
- Retention: application logs 30 days, `audit_log` 7 years (financial evidence),
  `notifications` 1 year. PII redacted at write time.

### A10 Server-side request forgery

No user-supplied URL is fetched server-side. The only outbound calls are to
WAHA, the SMTP host, the OAuth providers and MinIO — all from configuration, all
constant. If that ever changes, the call goes through an allow-list with
private-range and redirect blocking. Tested by an assertion that no HTTP client
in the codebase takes a URL from request data.

## 3. File uploads (payment proofs and menu photos)

| Rule | Detail |
|---|---|
| Type | **Magic-byte sniffed**, not extension: JPEG, PNG, WebP, PDF (proofs only) |
| Size | Proofs ≤ 5 MB, photos ≤ 8 MB; multipart limit enforced before parsing |
| Dimensions | Images capped at 4000×4000 before re-encode; decompression-bomb guard |
| Re-encode | Every image is re-encoded server-side (strips EXIF, kills polyglots) |
| Naming | Generated object keys; the client filename is never used on disk |
| Storage | Private bucket, presigned GET with a short TTL, never the app origin |
| Serving | `Content-Disposition: attachment` + `nosniff` for proofs |

Tests: oversize, wrong magic bytes, SVG and HTML disguised as `.png`, a PDF with
JavaScript, a zip bomb, and a presigned-URL expiry check.

## 4. Privacy / PDPA

Personal data is minimised to what an order needs: name, phone, optional email,
optional address (phase 2). No date of birth, no ID numbers, no card data ever.
Customers may request deletion: the account is anonymised (name/phone/email
replaced by tombstones) while orders keep their financial record — order events,
payments and audit rows are retained as required evidence. The procedure is in
the admin guide; every deletion is audited.

## 5. Security test suite (required artefacts)

| Suite | Proves |
|---|---|
| `authz_default_test` | Every route declares a permission; unknown routes 404 |
| `cross_store_test` | Per staff role × resource: store A cannot touch store B |
| `idor_test` | Per resource: customer A cannot touch customer B (404) |
| `payment_privilege_test` | finance-only, in-scope-only, no self-verification |
| `slot_concurrency_test` | 20 parallel checkouts, capacity 1 → exactly one success, 50 repeats |
| `slot_squat_test` | Unpaid cap enforced; no auto-cancel exists |
| `ratelimit_test` | Login, OTP, tracking, promo limits with `Retry-After` |
| `injection_fuzz_test` | Fuzz over every string input; no 500s, no SQL errors |
| `jwt_test` | Tampered signature, `alg=none`, expiry, revoked refresh, rotation replay |
| `upload_test` | Magic bytes, size, disguised types, re-encode, presign expiry |
| `headers_test` | CSP and the full header set on every response |
| `append_only_test` | `UPDATE`/`DELETE` on event and audit tables rejected |
| `enumeration_test` | Auth/OTP/tracking responses reveal no account existence |

## 6. What the tests actually run — 2026-08-02

| Suite | Command | Result |
|---|---|---|
| Domain unit | `make test` | pass — money, schedule, catalogue, pricing, order state machine (all 169 transition pairs), payment, identity, layering rule, permissions matrix |
| Integration | `make test-integration` | pass — **10 rounds × 20 simultaneous checkouts on a slot with room for one: exactly one success every round**, both capacity axes, a direct `UPDATE` refused by the CHECK constraint, release-exactly-once, no auto-expiry after 30 days |
| Security | `make test-security` | pass — cross-store per role and resource, permission matrix denials, anonymous denial, IDOR (404 not 403), payment privilege, JWT tampering/forged role/`alg=none`/foreign key/expiry, live scope and active-flag re-resolution, headers and CSP, CORS allow-list, no internals in errors, rate limits with `Retry-After`, unpaid cap, injection fuzz, append-only enforcement |
| End to end | `make test-e2e` | pass — the definition-of-done journey |
| Frontend | `npm run test` | pass — 14 tests including the search-box-on-every-list check |
| Quality gate | `make check` | `go vet`, `staticcheck`, `gosec`, `govulncheck`, no-shell-out all clean |

Three defects the suites found, now fixed and each covered by the test that
found them: an idempotent replay that silently re-executed (the stored response
was written as `bytea` into a `jsonb` column and the error was ignored); no
notification queued at all (order-received was never wired and the verified
path swallowed its lookup error); and a verify response reporting
`verified_at: null` instead of the timestamp it had just written.

## 7. Not yet exercised

Honest gaps, so nobody reads a green suite as more than it is:

- **TLS, HSTS and the CDN caveat** are deployment-time; they are asserted by the
  handbook checklist, not by a test.
- **MinIO upload rules** (magic bytes, size, re-encode, presigned expiry) are
  implemented in `adapter/storage` and exercised by hand; the suites use an
  in-memory stand-in, so the object-storage path itself has no automated test.
- **Google and Instagram OAuth** flows are built but disabled without
  credentials (docs/00 Q8), so only the refusal path is covered.
- **The phase-2 payment webhook** (HMAC, timestamp window, replay) is written
  but unused; it needs its own suite before QRIS goes live.
- **Load and p95 targets** in `05-architecture-and-nfr.md` §4 have not been
  measured.

## 8. ASVS L2 coverage notes

Chapters and how they are met: V1 architecture (`05`), V2 authentication (A07 +
BR-2.7), V3 session management (rotating refresh, revocation), V4 access control
(A01 + the permissions matrix in `02` §3), V5 validation and encoding (A03),
V7 error handling and logging (A09), V8 data protection (§4 + redaction),
V9 communications (TLS/HSTS), V10 malicious code (no eval, no dynamic import of
user data), V11 business logic (BR-2.3, BR-2.5, BR-2.6 with their tests),
V12 files and resources (§3), V13 API (`04` conventions), V14 configuration
(A05 + `09` §3, §5).

Not applicable at L2 for this system: SOAP/XML parsing, WebSockets, and native
mobile chapters.
