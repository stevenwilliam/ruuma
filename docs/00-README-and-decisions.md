# ruuma — Document Set

**Product codename:** ruuma
**Version:** 0.1 (initial scaffold)
**Date:** 31 July 2026
**Status:** greenfield — product not yet defined

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
| 13a | `13a-development-server-preparation.md` | Fresh Ubuntu 26.04 dev-server setup (copy-paste, full paths) |

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
| D6 | — | _TODO: what ruuma actually is (domain)._ | | |

---

## 3. Open questions

- **Q1.** What is ruuma's domain / problem? (blocks 01, 02, 03, 06)
- **Q2.** Does ruuma handle money? Currency, timezone, language?
- **Q3.** Is there a user-facing UI, or API-only for v1?
- **Q4.** Who are the personas and what is the thin v1 slice?
