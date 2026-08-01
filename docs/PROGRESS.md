# Build progress

Live status. Legend: ✅ done & tested · 🟡 partial · ⬜ not started.

_Last updated: 2026-07-31 (scaffold + stack decision + dev-server handbook)._

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
- ✅ `visual_initial_design/` — merged design brief + source snapshots, ready to drop into a design tool
- ⬜ Product-definition discussion → fill 01/02/03 (step 2 of workflow)
- ⬜ Resolve open questions in `00` (Q3 "UI or API-only?" blocks all visual design)

## M1 — Foundation
- ⬜ Module, CI, `.env.example`, docker-compose, Makefile, Dockerfile
- ⬜ `platform/*` copied & adapted from SCHOOLCATERING
- ⬜ Schema + migrations + seed
- ⬜ Auth + permission matrix
- ⬜ Pure domain packages + tests

## Notes
- Nothing under `cmd/`, `internal/`, `db/`, `web/` exists yet — docs-only repo.
