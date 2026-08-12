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
- 🟡 Open questions in `00` — Q1–Q6 closed; Q7 (real store data) **closed by D30**; Q8 (Google/Instagram OAuth credentials), Q9 (production SMTP) remain, all non-blocking with defaults

## M1 — Documents (step 3 of the workflow)
- ✅ `01` PRD, `02` business rules (115 normative `BR-x.y`), `03` data model, `04` API spec
- ✅ `05` architecture/NFR, `06` domain operations, `07` test plan, `08` roadmap, `09` deployment, `10` design system, `11` local dev
- ✅ `12-security.md` — OWASP control map with the test that proves each control
- ✅ `99-steven-preference.md` — portable engineering DNA

## M2 — Build (step 4 of the workflow)
- ✅ Module, CI, `.env.example`, docker-compose (MinIO + mailpit only), Makefile, Dockerfile
- ✅ `platform/*` — apierror, id, config, logging, security, ratelimit, database, metrics, clock, migrate
- ✅ 15 numbered migrations, embedded, verified **up → down → up** on an empty database
- ✅ `cmd/api seed` — the single outlet `RMA-MM` (D30), a menu across all three cuisines, one staff account per role
- ✅ Pure domain: money, schedule, catalog, pricing, order, payment, identity — exhaustively tested
- ✅ App layer behind ports (the layering rule is a test), services for catalogue, orders, payments, auth, ops, admin, notifications
- ✅ Adapters: postgres (store scope enforced in the repository), MinIO storage, WAHA/Meta/log notify, SMTP mail
- ✅ HTTP: full route surface, deny-by-default authz, live scope resolution, idempotency, rate limits, security headers
- ✅ Frontend: React 18 + Vite + TS + Tailwind, customer and lazy-loaded admin, ID/EN, search box on every list
- ✅ Home is the photo-led menu grid; store resolved silently against the single outlet (D30)
- ✅ Emerald header/footer, square app icons, reversed-out wordmark (D31)
- ✅ Dish photography: 21 real Commons photos, all commercially licensed, each reviewed by eye; credits at `/credits` (D31)
- 🟡 Commons photos are a stopgap — they show the right dish, not ruuma's dish; the kitchen's own photography is outstanding
- ⬜ Admin-uploaded menu photos do not reach the customer menu: the API returns `photo_key` and no route serves the object

## M3 — Test & harden (step 5 of the workflow)
- ✅ Unit, integration, security and e2e suites all green (`docs/12` §6)
- ✅ **Zero oversell** proven over 10 rounds × 20 simultaneous checkouts
- ✅ `go vet`, `staticcheck`, `gosec`, `govulncheck` clean — `make check` green
      end to end, including the `web-audit` dependency gate
- ✅ All 115 `BR-x.y` rules referenced by code or test (`make test-br-coverage`)
- ✅ Three real defects found by the suites and fixed (`docs/12` §6)
- ✅ **UI write-path hardening (D34)** — every mutating control is an
      `AsyncButton`: one click one write, and a confirmation on anything
      irreversible. Six new tests in `web/src/__tests__/async-button.test.tsx`
- ✅ **Operator-changeable backdrop (D43)** — validated as a security control,
      8 injection tests (CSS breakout, traversal, absolute URL, `.svg`)
- ✅ **Elegance pass (D44)** — Playfair Display self-hosted, editorial header,
      quieter chips; every AA pair still passing, verified by screenshot
- ✅ **Public config endpoint (D35)** — `GET /api/v1/public-config` on a
      compiled allowlist; 4 tests in `test/security/public_config_test.go`
      including a planted-secret leak probe
- ✅ **SEO baseline (D40)** — per-route titles, `noindex` on the transactional
      surface, robots + sitemap, JSON-LD, generated 1200×630 share card; 5 tests
      in `web/src/__tests__/seo.test.tsx`
- ⬜ **Per-page share previews need prerendering/SSR** — static OG tags give the
      site one correct card; a shared dish link previews as the site, not the
      dish (`docs/08`)
- ✅ **Contrast budget in the gate (D36/D37/D39)** — `make contrast` recomputes
      every AA pair against the ambient wash, not against a flat `--bg`, and
      also fails the wash for being *imperceptible*: the first version passed
      every ratio and shipped looking like a flat page
- ⚠️ **This section overstated itself between D31 and D34.** `gosec` had been
      failing on 5 findings in `tools/genassets` and `tools/dishphotos` since
      the dish-photography work landed, and `web-audit` was failing on the
      `nanoid` advisory, while this file still read ✅. Both are fixed and the
      gate is genuinely green — the lesson is that a ✅ here has to be re-earned
      by running the gate, not inherited from the last time it passed

## M4 — Handbooks (step 6 of the workflow)
- ✅ `14-production-deployment-handbook.md` — empty Ubuntu 24.04/26.04, copy-paste, full absolute paths, `vi`
- ✅ `cmd/api create-owner` — the first-run flow, so production ships with no default credentials
- ✅ `15-user-guide.md` — customer guide, ID first with EN alongside
- ✅ `16-admin-guide.md` — kitchen, counter, finance, store manager, admin

## Known gaps
See `docs/12-security.md` §7: object-storage upload rules, OAuth providers,
the phase-2 payment webhook and load targets are not yet covered by automated
tests.

- ⬜ **Admin copy is inline English, not message catalogues** (`docs/10` §5.1).
  The customer app reads every string from `web/src/i18n`; the admin app does
  not, across roughly ten files. This contradicts CLAUDE.md §10 and must close
  before `16-admin-guide.md` can claim bilingual support.
