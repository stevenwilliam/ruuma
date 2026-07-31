# ruuma — engineering & product DNA

This file is the contract for how ruuma is built. Read it first, every session,
before touching code or docs. Where it conflicts with a habit, this file wins.
Where it conflicts with `docs/02-business-rules.md` on *product* logic, that
document wins — this file governs *how* we build, not *what* the product does.

---

## 1. What ruuma is

**Codename:** ruuma
**Owner:** stevenwilliam (itdept.sfg@gmail.com)
**Repo:** https://github.com/stevenwilliam/ruuma
**Status:** greenfield — product not yet defined.

> TODO(domain): one-paragraph problem statement goes here once decided. Until
> then, the domain docs (`01`, `02`, `03`, `06`) are structured placeholders.
> See `initial-start-prompt.md` for how to run that first discussion.

---

## 2. Architecture — non-negotiable

Hexagonal / clean layering. Dependencies point **inward only**:
`adapter → app → domain`, with `platform` available to all. `domain` imports
no framework, no driver, no `net/http`, no SQL.

```
cmd/api/main.go            # thin entrypoint: wire + run subcommands (serve, migrate, seed)
internal/
  domain/                  # pure business logic + types; exhaustively unit-tested; no I/O
  app/                     # use-cases / services; orchestrates domain + ports
  adapter/                 # driven & driving adapters
    http/                  #   gin handlers, request/response mapping
    postgres/              #   gorm repositories (raw SQL on money paths)
    storage/               #   S3 / MinIO
    notify/                #   email / outbound
  platform/                # cross-cutting infra, business-agnostic, reusable across projects
    config/ logging/ metrics/ apierror/ id/ security/ ratelimit/ database/
db/
  migrations/NNNN_name.up.sql + NNNN_name.down.sql
  embed.go                 # go:embed migrations
```

The `internal/platform/*` packages are meant to be **portable** — carried over
from the SCHOOLCATERING project's proven shapes. Prefer copying and adapting
those over reinventing.

---

## 3. Stack

Backend: **Go (latest)** · **`gin`** (HTTP) · **`gorm`** (ORM) +
`gorm.io/driver/postgres` · **PostgreSQL 18** · `golang-jwt/jwt/v5` ·
`google/uuid` · S3/MinIO (`minio-go/v7`) · Prometheus (`client_golang`) ·
`golang.org/x/crypto`.

Frontend (if/when there is a UI): **React 18** + **Vite 5** + **TypeScript 5** +
**Tailwind 3**, `web/src/{components,lib,pages}`. Pin React to 18.

ORM is `gorm`. **Exception:** any code path touching money uses explicit
`gorm.Exec`/raw SQL with integer arithmetic — never rely on the ORM for money
math (see §4).

---

## 4. Hard rules

- **Money is integers.** If the domain touches money, store the minor unit as
  `BIGINT` in whole currency units and do all arithmetic in integers. Floating
  point is prohibited in any code path touching money. Percentage values round
  to the nearest whole unit, half-up: `floor((amount * bps + 5000) / 10000)`.
- **IDs are UUIDv7** — time-ordered for index locality, non-sequential enough
  not to leak volume. Human-facing codes use CSPRNG + Crockford base32.
- **The domain layer is pure and exhaustively unit-tested.** No mocks needed
  there because there's no I/O. Adapters get integration tests.
- **Migrations are forward-only in production**, versioned, numbered, with a
  matching `.down.sql`, embedded via `go:embed`.
- **Errors are typed** through `platform/apierror`; handlers map to a single
  consistent JSON error model. Never leak driver errors to clients.
- **Secrets only via config/env.** Nothing secret in git. `.env.example` is the
  documented surface; real `.env` is git-ignored.

---

## 5. Docs discipline

- Docs live in `docs/`, numbered. `docs/02-business-rules.md` is **normative** —
  business rules carry `BR-x.y` IDs and code/tests reference those IDs.
- **Keep all docs in sync on every decision.** When a decision changes behaviour,
  update every affected doc in the same change — PRD, business rules, data model,
  API spec. A decision that isn't in the docs didn't happen.
- `docs/PROGRESS.md` is the live build status (✅ done & tested · 🟡 partial ·
  ⬜ not started). Update it as work lands.
- `docs/RUN-WHEN-BACK.md` holds steps that need an interactive terminal (Docker,
  live server, integration tests, approval prompts) — written but not run.

---

## 6. Working conventions (how I want Claude to operate)

- **Editor is `vi`** in any runbook, shell instruction, or docs example — never
  `nano`.
- **Auto-commit + push after every completed change**, without asking. Small,
  focused commits. Conventional-commit style messages
  (`feat(...)`, `fix(...)`, `docs(...)`).
- Never work directly on a throwaway branch when `main` is the working branch
  here — commit to `main` and push to `origin`.
- When something needs an interactive terminal I can't drive, write it and add
  it to `RUN-WHEN-BACK.md` rather than guessing.
- Prefer editing existing files and reusing `platform/*` over new scaffolding.
- Keep changes verifiable: if the change has a runtime surface, exercise it.

---

## 7. Product & UI conventions

- **Search box on every list.** Every screen that renders a list/table of data
  must have a search box that filters that data. No exceptions — a list without
  search is incomplete.
- **Configurable values live in `sys_parameters`.** Anything that could change
  without a code change — company phone number, email, address, tax rate,
  feature toggles, business thresholds — is stored as a row in the
  `sys_parameters` table, **not** hard-coded. Every such value ships with full
  CRUD on `sys_parameters` (list + search, create, read, update, delete) behind
  the appropriate admin permission. Read config through this table at runtime.

## 8. Document control

- **Always update related documents after each change**, in the same commit as
  the change. PRD, business rules, data model, API spec, deployment/user/admin
  guides — whatever the change touches. A change whose docs are stale is not done.
- **OS / server guides use full absolute paths, never relative paths**, so a
  copy-pasted command can never run in the wrong directory. This applies to the
  production deployment handbook, `13a-development-server-preparation.md`, and
  every runbook.

## 9. Delivery workflow (how a ruuma build runs)

The agreed sequence for taking ruuma from empty repo to shipped:

1. **Initial git setup** — repo, remotes, conventions, this `CLAUDE.md`.
2. **Steven — preparation.** Steven gives PRD & business-rules feedback, tuning,
   and final confirmation. Nothing downstream starts until this is confirmed.
3. **Claude — build all documents A→Z.** Complete the full doc set from the
   confirmed PRD/business rules.
4. **Claude — build all modules in one shot, A→Z.** Implement every module end
   to end. **Do not stop** in the middle or after a few modules — go the whole way.
5. **Claude — test, debug, and security-harden, A→Z.** Again **do not stop**
   partway; carry it through the entire system.
6. **Claude — production deployment handbook** (copy-paste, assuming an empty
   machine, full absolute paths), **then** the user guide and the admin guide.

## 10. Locale / environment

> TODO(domain): timezone, currency, and language are product decisions. Fill in
> once the domain is set (SCHOOLCATERING was Asia/Jakarta, IDR, EN/ID).
