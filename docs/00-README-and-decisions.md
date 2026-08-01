# ruuma — Document Set

**Product codename:** ruuma
**Version:** 0.2 (domain defined)
**Date:** 2 August 2026
**Status:** domain defined (D8) — docs being written A–Z, build not started

---

## 1. What this document set is

The engineering & product spec for ruuma, built in the house style. It starts as
a structured scaffold: the shape and conventions are real; the domain-specific
content is `TODO(domain)` until the first product-definition discussion (see
`../initial-start-prompt.md`).

`02-business-rules.md` is **normative** — where it conflicts with any other
document, it wins. Build/working conventions live in `../CLAUDE.md`.

| # | Document | Purpose |
|---|---|---|
| 00 | This file | Index, decision log, open questions |
| 01 | `01-PRD.md` | Product requirements: problem, personas, scope, requirements, metrics |
| 02 | `02-business-rules.md` | Normative business logic — the product/engineering contract |
| 03 | `03-data-model.md` | PostgreSQL schema, ERD, DDL, migration notes |
| 04 | `04-api-specification.md` | REST contract, error model, idempotency, auth |
| 05 | `05-architecture-and-nfr.md` | Go service architecture, security, performance, deployment |
| 06 | `06-domain-operations.md` | Domain-specific operational logic & runbooks |
| 07 | `07-test-plan.md` | Test strategy, critical scenarios, QA checklist |
| 08 | `08-roadmap.md` | Phasing, release plan, sequencing rationale |
| 09 | `09-deployment.md` | Production deployment (Docker/systemd, TLS, backups) |
| 10 | `10-design-system.md` | Palette, typography, components, a11y |
| 11 | `11-local-dev-setup.md` | Local dev environment and everyday commands |
| 12 | `12-security.md` | OWASP ASVS L2 / Top-10 control map, abuse cases, security test suite |
| 13a | `13a-development-server-preparation.md` | Shared `claudedev` dev server — Part A (setup once) + Part B (onboard a project); copy-paste, full paths |

Plus `PROGRESS.md` (live build status) and `RUN-WHEN-BACK.md` (interactive steps).

---

## 2. Decision log

Record every decision that changes behaviour here, with a date, and reflect it
in the affected docs the same day.

| ID | Date | Decision | Rationale | Docs touched |
|----|------|----------|-----------|--------------|
| D1 | 2026-07-31 | Adopt the SCHOOLCATERING house style (hexagonal Go, numbered docs, money-as-integers, UUIDv7). | Proven; reusable `platform/*`. | CLAUDE.md, all |
| D2 | 2026-07-31 | **Backend stack = Go (latest) + gin + gorm + PostgreSQL 18** (supersedes chi/pgx/no-ORM for ruuma). Money paths still use raw SQL + integer math. | Steven's directive for ruuma. | CLAUDE.md, 05, README |
| D3 | 2026-07-31 | Add development-server preparation handbook (`13a`), copy-paste, fresh Ubuntu 26.04. | Develop on a dedicated server, not VS local. | 13a |
| D4 | 2026-07-31 | Conventions: search box on every list; configurable values in `sys_parameters` + CRUD; docs updated with every change; OS guides use full absolute paths. | Steven's standing preferences. | CLAUDE.md, 02, 03, 05, 10 |
| D5 | 2026-07-31 | Delivery workflow = prep→confirm → docs A–Z → build all modules one shot → test/harden one shot → deployment handbook + user/admin guides. | Steven's process. | CLAUDE.md §9, 08 |
| D6 | 2026-07-31 | Dev server is **shared across many projects**, hostname **`claudedev`**. `13a` restructured into Part A (server once) + Part B (onboard a project); ruuma is the worked example. Shared config under `/etc/claudedev/`, per-project under `/etc/<project>/`, projects at `/home/dev/projects/<project>`, nginx reverse proxy fronts project ports. | Steven's directive. | 13a |
| D7 | 2026-07-31 | WhatsApp notify/ChatOps uses **WAHA (Core, self-hosted, free)** on `claudedev` (own number via QR), not Meta Cloud API. `dev-notify` POSTs to WAHA on localhost:3000. Meta Cloud API kept as the documented official alternative for customer-facing messaging. | Free + stable for dev; official API reserved for production. | 13a |
| D8 | 2026-08-02 | **Domain defined: ruuma is the online ordering site for a multi-outlet restaurant group** (Indonesian / Chinese / Western). Customers pick a **store**, browse that store's menu, and check out against a **fulfilment date + time slot** at that store; each kitchen works a slot-by-slot production board. Every order, slot, price override, promo and staff assignment is store-scoped. | Steven's product definition, 2026-08-02. | 01, 02, 03, 04, 06, 07, 08 |
| D9 | 2026-08-02 | **Locale: IDR stored as whole rupiah in `BIGINT`** (no subunit in practice) — **BR-1.1.1 amended**. Business timezone **Asia/Jakarta**, all timestamps UTC in the DB. UI languages **ID (default) + EN** via message catalogues; the doc set stays in English. | Indonesian market; rupiah has no circulating subunit. | 02, 03, 05, 10 |
| D10 | 2026-08-02 | **Dev environment on `claudedev`:** install Node 20 + the `docker compose` plugin; the project compose file carries **MinIO + mailpit only**. The app uses the **native PostgreSQL 18** (`ruuma` DB) and the **shared WAHA** container, not private copies. Integration/concurrency tests run against a `ruuma_test` database, dropped and recreated by `make test-integration`. | Avoids a second Postgres on a nonstandard port and a duplicate WhatsApp session on a shared server. | 11, 13a, 09 |
| D11 | 2026-08-02 | **Notifications go through a `notify.Provider` port** with two implementations: `waha` (v1) and `meta_cloud` (stub for production). Provider selected by `sys_parameters`. Steven has **explicitly accepted the ban risk** of WAHA being an unofficial WhatsApp-Web gateway for customer-facing messages. | Free and already running; official API remains a drop-in swap. | 02, 05, 09, 12 |
| D12 | 2026-08-02 | **No guest checkout.** Ordering requires a registered customer account; the phone number is verified by OTP (delivered over the same notify provider) at registration. | Traceable customers, less order-tracking abuse surface. | 01, 02, 04 |
| D13 | 2026-08-02 | **No payment hold, no auto-expiry, no payment-rejection state.** An unpaid order simply stays **UNPAID** until operations/finance verify the transfer manually. Capacity is reserved at order creation and released only by an explicit **cancellation** (customer within the cutoff, or staff at any time). `AWAITING_VERIFICATION`, `EXPIRED`-by-hold and the rejection loop are removed from the state machine. | Steven: manual operations, no machine-driven cancellation of a customer's slot. | 02, 03, 04, 06 |
| D14 | 2026-08-02 | **Unique payment code (*kode unik*):** `amount_due = order_total + N`, `N` ∈ 1–999, unique across open orders per store bank account, recorded as `unique_code_amount` and reconciled (never discounted). | Lets finance match an incoming transfer to one order without a gateway. | 02, 03, 04 |
| D15 | 2026-08-02 | **Promotions are scoped to an explicit list of stores** (`promotion_stores`) — one promo may cover several selected stores. There is no implicit "all stores" scope. | Steven's directive; explicit beats a NULL-means-everything convention. | 02, 03, 04 |
| D16 | 2026-08-02 | **Phase 1 is PICKUP ONLY.** Delivery — zones, fees, minimum order, free-delivery threshold, `OUT_FOR_DELIVERY`/`DELIVERED` — is **phase 2**: modelled in the schema and the fulfilment-mode master data, but disabled by feature flag and not exposed in the v1 UI. | Steven's directive; ships the slot engine without geocoding work. | 01, 02, 03, 04, 08 |
| D17 | 2026-08-02 | **Tax:** menu prices are **tax-exclusive**; **PB1 10%** is added as its own checkout line; **service charge 0%** for online orders. A `pricing.tax_inclusive` flag flips the group behaviour, and both rates are per-store overridable. | Indonesian restaurant tax is regional PB1, not PPN. | 02, 03, 04 |
| D18 | 2026-08-02 | **Store schedules support per-date overrides** (`store_date_overrides`) on top of the weekday schedule and blackout dates — e.g. a store that normally closes at 21:00 may close at 22:00 on one specific Sunday. Precedence: per-date override > blackout date > weekday schedule. | Steven: real stores vary by date, not only by weekday. | 02, 03, 04, 06 |
| D19 | 2026-08-02 | **Every scheduling value is backend-configurable** — slot length, capacity (orders + kitchen units), lead time, cutoff, max advance days, cancel cutoff — as `sys_parameters` group defaults with per-store overrides and admin CRUD. No scheduling constant is hard-coded. | Steven must retune operations without a deploy. | 02, 03, 04 |
| D20 | 2026-08-02 | **Brand: emerald `#277066`**, sampled from the supplied logo, now stored at `web/public/brand/ruuma-logo-emerald.png`. Full light/dark token set in `10`, every pair ≥ WCAG AA. | Steven supplied the logo. | 10 |
| D21 | 2026-08-02 | **Production domain `ruuma.id`** (+ `admin.ruuma.id`), single Ubuntu node, nginx + certbot TLS, native PostgreSQL, MinIO under systemd. | Steven's domain. | 09 |
| D22 | 2026-08-02 | **No PWA.** No installability, no service worker, no offline menu cache — responsive, fast web only. | Steven's directive. | 01, 05, 10 |
| D23 | 2026-08-02 | **Security target: OWASP ASVS v4 Level 2 + all OWASP Top 10 (2021)**, documented in a new **`docs/12-security.md`** mapping every control to its implementation and the test that proves it. | Payment proofs, PII and multi-store tenancy. | 12, 07, 05 |

---

## 3. Open questions

Q1–Q4 are **closed** by D8–D23 (2026-08-02). Remaining:

- **Q5.** Customer login mechanism — phone + OTP each time (passwordless) vs. phone + password with OTP only at registration/reset. _Proposed: password, OTP at registration and password reset._
- **Q6.** With no payment hold (D13), an unpaid order occupies slot capacity indefinitely until someone cancels it. Confirm the operational control: staff bulk-cancel from the orders board + an "unpaid, ageing" alert, and a per-customer cap on concurrent unpaid orders.
- **Q7.** Real store master data (addresses, phones, bank accounts) — seeded as `SEED — replace` placeholders until Steven supplies them.
