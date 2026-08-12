# ruuma — Document Set

**Product codename:** ruuma
**Version:** 0.2 (domain defined)
**Date:** 2 August 2026
**Status:** built. Docs, service, frontend, tests and handbooks complete (see `PROGRESS.md`); phase 1 is pickup-only with manual transfer.

---

## 1. What this document set is

The engineering & product spec for ruuma, built in the house style. The domain
is defined (D8), the scope rulings are recorded (D9–D29), and documents `01`–`16`
are written against the built system.

House style itself — how Steven works, and his stack, database and security
preferences — lives in `99-steven-preference.md` and is portable to other
projects.

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
| 14 | `14-production-deployment-handbook.md` | Empty-machine production deployment; copy-paste, full absolute paths |
| 15 | `15-user-guide.md` | Customer guide (ID/EN) |
| 16 | `16-admin-guide.md` | Staff guide: kitchen, counter, finance, store manager, admin |
| 99 | `99-steven-preference.md` | Steven's portable engineering DNA — how he works, stack/DB/security preferences; **project-agnostic, copy into any new repo** |

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
| D13 | 2026-08-02 | **No payment hold and no auto-expiry** — an unpaid order stays **UNPAID** indefinitely until a human acts. Capacity is reserved at order creation and released only by an explicit **cancellation** (customer within the cutoff, or staff at any time); no background job ever cancels a customer's slot in phase 1. **Amended same day (see D26): finance keeps an explicit reject action**, so the payment states are UNPAID → (VERIFIED \| REJECTED → UNPAID), with rejection requiring a reason and never touching the slot. | Steven: manual operations; bank transfer in phase 1 makes automation unsafe. | 02, 03, 04, 06 |
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
| D24 | 2026-08-02 | **Customer accounts support four sign-in methods: email + password (email must be verified), Google, Instagram, and phone + OTP.** One `customers` row may own several `customer_identities` rows (provider, provider_user_id, verified_at); identities are auto-linked only on a **verified** matching email or phone, never on an unverified claim. Google/Instagram credentials come from env and each provider can be toggled off in `sys_parameters`. **A verified phone is required before an order can be placed**, whatever the sign-in method, because the counter must be able to reach the customer. Supersedes the "phone + password" proposal in D12; D12's "no guest checkout" still stands. | Steven's directive — lower signup friction. | 01, 02, 03, 04, 12 |
| D25 | 2026-08-02 | **Auto-cancel of unpaid orders is deferred to the QR/QRIS payment phase.** Phase 1 (bank transfer) has no timer at all; when QRIS lands, an `orders.auto_cancel_minutes` parameter (0 = off in phase 1) starts releasing unpaid capacity. The payment provider port is shaped for `manual_transfer` (v1), `qris`, and a full gateway, so adding QRIS is an adapter, not a reshape of the order flow. Interim phase-1 controls: staff single + bulk cancel from the orders board, an "unpaid, ageing" panel oldest-first, and a configurable cap on concurrent unpaid orders per customer (default 2, 0 = unlimited). | Steven: bank transfer is unavoidable in phase 1; automation arrives with QR. | 02, 03, 04, 06, 08 |
| D26 | 2026-08-02 | **Finance keeps an explicit REJECT action** with a mandatory reason (amount mismatch, unreadable proof, not received, duplicate). Rejection returns the order to UNPAID, is recorded immutably in `payment_events` + audit log, and **never releases the slot**. The customer may upload a new proof. Amends D13. | Steven's directive. | 02, 03, 04, 06 |
| D27 | 2026-08-02 | **Blackout dates may target the current day** (emergency closure). A blackout takes effect immediately: all remaining slots for that store/date stop accepting new orders. **Already-booked orders are never auto-cancelled** — they surface in an "affected by closure" panel for staff to cancel and refund by hand. Blackout requires a reason and is audited. Precedence stays: per-date schedule override > blackout > weekday schedule (D18). | Steven's directive; consistent with "no machine cancels a customer's slot" (D13). | 02, 03, 04, 06 |
| D28 | 2026-08-02 | **Notification scope trimmed.** Payment rejection and payment queries are handled **manually by finance/operations** — no automated message. Automated WhatsApp is limited to: order received (with transfer instructions + *kode unik* amount), payment verified, order ready for pickup, and a pre-slot reminder. Templates live in `sys_parameters`, each event individually switchable. | Steven: manual verification, no rejection notification. | 02, 04, 06 |
| D29 | 2026-08-02 | **Add `docs/99-steven-preference.md`** — Steven's portable engineering DNA (working style, delivery workflow, architecture, Go/React stack, database conventions, security posture, product/UI conventions, infra defaults, doc-set convention, anti-patterns, bootstrap checklist). Deliberately **project-agnostic** so it can be copied verbatim into any new repo; a project's own `CLAUDE.md` wins where they conflict. | Carry the same DNA across projects without re-deriving it. | 00, 99 |
| D30 | 2026-08-03 | **ruuma trades from one outlet: `RMA-MM` — RUUMA Menara Matahari**, Jl. Boulevard Palem Raya No. 7, Lippo Karawaci Central, Tangerang, Banten 15811. Pickup only. The three invented seed outlets (Kelapa Gading, Senayan, BSD) are **deactivated, not deleted** — orders, slots and staff assignments reference them and history must survive. Consequences: the **home page is the menu, not the store picker** (a choice with one answer is not a choice); the store is resolved silently and `/stores` only reappears when a second outlet exists; nearest-store ordering is **deferred** — `stores.latitude/longitude` are populated so distance sorting is a query, not a migration, when it is needed. **Store scope in the schema and repository layer is unchanged** — it is a tenancy boundary that must hold before a second store exists, not after. | Steven's directive. | 00, 01, 02, 10, 11 |
| D32 | 2026-08-12 | **Claude Code plugin baseline = `security-guidance` + `frontend-design` + `gopls-lsp`**, all from `claude-plugins-official`, all installed at **user scope** so they carry to the next project rather than being re-chosen per repo. `security-guidance` is the important one: it is four *harness hooks*, not a skill, so it runs on every edit and every stop regardless of Claude's judgement — the only kind of guard that holds up next to "auto-commit and push without asking" (CLAUDE.md §6). Rejected, with reasons recorded in `99` §13: cloud-vendor DB plugins (managed Postgres, we run native), generic Postgres skills (they teach `NUMERIC` for money, violating §4), commercial SaaS scanners (paid accounts), `superdesign` (ships codebase context off-box), `graphify` (knowledge-graph indexer — pays off past ~150k LOC or where architecture is undocumented; ruuma is ~32k LOC with the dependency rule already written down, and `gopls-lsp` is more accurate for Go), and `samber/cc-skills-golang` (a third of it evangelises libraries and DI frameworks that conflict with the pinned stack). | Skills are model-triggered and optional; only hooks are guaranteed. Tooling choices belong in the portable DNA file, not in one repo's head. | 00, 99 |
| D33 | 2026-08-12 | **`ui-ux-pro-max` replaces `frontend-design`** as the design skill (amends D32). From `nextlevelbuilder/ui-ux-pro-max-skill` — MIT, 115.7k stars, last pushed 2026-08-06, **no network calls and no account**; its ~1.2 MB of CSVs is queried on demand by local Python and never enters context. Seven skills, ~741 tok always-on. Bought for the **UX rules and accessibility checks** (loading/empty states, breakpoints, touch targets, focus, ARIA) rather than its palette and typography pickers, which are largely moot here — D20 already fixed emerald `#277066` and a full light/dark token set at ≥ WCAG AA in `10`. **Where the skill's palette or font suggestions conflict with `docs/10`, `docs/10` wins.** Only one design skill runs at a time, so `frontend-design` was uninstalled; swapping back is one command each way. | Steven's directive. | 00, 10, 99 |
| D31 | 2026-08-03 | **Dish photography comes from Wikimedia Commons**, fetched by `tools/dishphotos` and rendered to 4:3 cards at `web/public/dish/<SKU>.jpg`. Only licences permitting **commercial** use are accepted — the tool refuses NC and ND — and every image's photographer, licence and source are written to `web/src/credits.json` and published at **`/credits`**, linked from the footer of every page, because CC BY and CC BY-SA oblige it. Each image was **reviewed by eye**: titles lie, and a keyword search returned chicken-fried steak for grilled chicken and a bao bun photographed over a rival restaurant's menu. `tools/genassets` still generates the square app icons and the reversed-out wordmark, and its placeholder cards remain the fallback when a fetch fails. **Real photography of ruuma's own kitchen replaces all of this before launch**; the admin upload path is unchanged and still does not reach the customer menu. | No licensed food photography exists yet, and unlicensed stock on a site taking real orders is a liability. | 10, 11 |

---

## 3. Open questions

Q1–Q6 are **closed** by D8–D28 (2026-08-02). Remaining, none of them blocking —
each has a working default that ships unless Steven says otherwise:

- **Q7.** Real store master data (addresses, phones, bank accounts). _Default: `SEED — replace` placeholders._
- **Q8.** Google + Instagram OAuth app credentials (client id/secret, redirect URIs). _Default: both flows implemented, read from env, and disabled at runtime until credentials exist — email and phone sign-in cover launch._ Note: Instagram Login returns **no email and no phone**, so an Instagram-first customer must add and verify a phone before their first order (D24).
- **Q9.** Production SMTP sender for email verification (host, user, from-address). _Default: env-configured SMTP with mailpit in dev and a placeholder in `.env.example`._
