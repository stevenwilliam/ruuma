# Build progress

Live status. Legend: ✅ done & tested · 🟡 partial · ⬜ not started.

_Last updated: 2026-08-02 (delivery workflow complete: docs → build → harden → handbooks)._

## M0 — Definition
- ✅ Repo created + `git init`, pushed to `origin`
- ✅ `CLAUDE.md` (build DNA), `README.md`, `.gitignore`, `.gitattributes`
- ✅ `initial-start-prompt.md`
- ✅ Docs scaffold `00`–`11`, `PROGRESS.md`, `RUN-WHEN-BACK.md`
- ✅ Stack decided: **Go + gin + gorm + PostgreSQL 18** (D2)
- ✅ Conventions locked: search-box-on-lists, `sys_parameters` config, full-path OS guides, doc control (D4)
- ✅ Delivery workflow agreed (D5, `CLAUDE.md §9`)
- ✅ `13a-development-server-preparation.md` — shared `claudedev` server (Part A setup once + Part B onboard-a-project); ruuma is the worked example (D6)
- ✅ WhatsApp notify via **WAHA** (self-hosted, free); Meta Cloud API is the documented official alternative (D7)
- ✅ Product-definition discussion held; **domain defined (D8)** — multi-outlet restaurant ordering, store-scoped slots
- ✅ Scope rulings recorded (D9–D23): IDR/Jakarta/ID+EN, no guest checkout, no payment hold or auto-expiry, pickup-only phase 1, PB1 10%, per-date schedule overrides, WAHA notify with provider port, no PWA, ASVS L2
- ✅ Brand locked: emerald `#277066` from the supplied logo, contrast-checked light/dark tokens in `10` (D20); logo at `web/public/brand/ruuma-logo-emerald.png`
- ✅ Auth, payment-reject, auto-cancel-phasing, same-day blackout and notification scope settled (D24–D28)
- ✅ `docs/99-steven-preference.md` — portable engineering DNA (working style, stack, DB, security, doc-set convention, bootstrap checklist) (D29)
- 🟡 Open questions in `00` — Q1–Q6 closed; Q7 (real store data), Q8 (Google/Instagram OAuth credentials), Q9 (production SMTP) remain, all non-blocking with defaults

## M1 — Documents (step 3 of the workflow)
- ✅ `01` PRD, `02` business rules (115 normative `BR-x.y`), `03` data model, `04` API spec
- ✅ `05` architecture/NFR, `06` domain operations, `07` test plan, `08` roadmap, `09` deployment, `10` design system, `11` local dev
- ✅ `12-security.md` — OWASP control map with the test that proves each control
- ✅ `99-steven-preference.md` — portable engineering DNA

## M2 — Build (step 4 of the workflow)
- ✅ Module, CI, `.env.example`, docker-compose (MinIO + mailpit only), Makefile, Dockerfile
- ✅ `platform/*` — apierror, id, config, logging, security, ratelimit, database, metrics, clock, migrate
- ✅ 15 numbered migrations, embedded, verified **up → down → up** on an empty database
- ✅ `cmd/api seed` — three deliberately different stores, a menu across all three cuisines, one staff account per role
- ✅ Pure domain: money, schedule, catalog, pricing, order, payment, identity — exhaustively tested
- ✅ App layer behind ports (the layering rule is a test), services for catalogue, orders, payments, auth, ops, admin, notifications
- ✅ Adapters: postgres (store scope enforced in the repository), MinIO storage, WAHA/Meta/log notify, SMTP mail
- ✅ HTTP: full route surface, deny-by-default authz, live scope resolution, idempotency, rate limits, security headers
- ✅ Frontend: React 18 + Vite + TS + Tailwind, customer and lazy-loaded admin, ID/EN, search box on every list

## M3 — Test & harden (step 5 of the workflow)
- ✅ Unit, integration, security and e2e suites all green (`docs/12` §6)
- ✅ **Zero oversell** proven over 10 rounds × 20 simultaneous checkouts
- ✅ `go vet`, `staticcheck`, `gosec`, `govulncheck` clean
- ✅ All 115 `BR-x.y` rules referenced by code or test (`make test-br-coverage`)
- ✅ Three real defects found by the suites and fixed (`docs/12` §6)

## M4 — Handbooks (step 6 of the workflow)
- ✅ `14-production-deployment-handbook.md` — empty Ubuntu 24.04/26.04, copy-paste, full absolute paths, `vi`
- ✅ `cmd/api create-owner` — the first-run flow, so production ships with no default credentials
- ✅ `15-user-guide.md` — customer guide, ID first with EN alongside
- ✅ `16-admin-guide.md` — kitchen, counter, finance, store manager, admin

## Known gaps
See `docs/12-security.md` §7: object-storage upload rules, OAuth providers,
the phase-2 payment webhook and load targets are not yet covered by automated
tests.
