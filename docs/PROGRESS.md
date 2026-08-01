# Build progress

Live status. Legend: ✅ done & tested · 🟡 partial · ⬜ not started.

_Last updated: 2026-08-02 (domain defined, D8–D29 recorded, brand locked, preferences doc added)._

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
- ⬜ Rewrite docs `01`–`10` + new `12-security.md` from the confirmed scope (step 2 of workflow)

## M1 — Foundation
- ⬜ Module, CI, `.env.example`, docker-compose, Makefile, Dockerfile
- ⬜ `platform/*` copied & adapted from SCHOOLCATERING
- ⬜ Schema + migrations + seed
- ⬜ Auth + permission matrix
- ⬜ Pure domain packages + tests

## Notes
- Nothing under `cmd/`, `internal/`, `db/`, `web/` exists yet — docs-only repo.
