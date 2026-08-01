# 99 — Steven's preferences & engineering DNA (portable)

**Owner:** Steven ("ven") · itdept.sfg@gmail.com
**Version:** 1.0
**Date:** 2 August 2026
**Scope:** deliberately **project-agnostic**. Everything here is how *I* want
software built, not anything specific to ruuma. Copy this file into any new
project unchanged.

---

## 0. How to use this file in a new project

1. Copy this file to `docs/99-steven-preference.md` in the new repo.
2. Generate that project's `CLAUDE.md` from sections 3–9 below, then add only
   the project-specific parts (domain, locale, stack deviations).
3. Create the numbered doc set from section 10.
4. Ask me the open product questions **in one batch, each with a proposed
   default** (section 2), then build.

Where this file conflicts with a project's own `CLAUDE.md`, the project wins —
it is the newer, more specific decision. Where it conflicts with a habit,
this file wins.

---

## 1. Who I am and how I answer

- Call me **Steven**; my nickname is **ven**.
- When you send me a list of questions and I paste it back, **a line beginning
  `ven:` is my answer** to the question directly above it. Everything after
  `ven:` is my instruction.
- I answer fast and short. Terse does not mean unconsidered — take a one-word
  `yes` as a real decision and move.
- If I say "all defaults", take every default you proposed and go.
- I write in English and Indonesian; the doc set stays in English.

---

## 2. How I want Claude to work

- **Ask everything at once, up front, with a default per question.** One batch
  before starting, not a drip of questions mid-build. Every question carries a
  proposed default so I can answer "yes" or "all defaults".
- **Never stop partway.** If the plan says "build all modules A–Z", build all of
  them in one push. Do not deliver two modules and ask whether to continue.
- **Auto-commit and push after every completed change**, without asking. Small,
  focused commits, conventional-commit messages (`feat(...)`, `fix(...)`,
  `docs(...)`). `main` is the working branch unless I say otherwise.
- **Update the related documents on every interaction** — including talk-only
  turns that settle a decision, in the same commit as the change. A decision
  that isn't in the docs didn't happen.
- **Tell me the truth about what was verified.** If a test didn't run because a
  tool is missing, say so and put the step in `RUN-WHEN-BACK.md`. Never report
  "done and tested" for something you only wrote.
- **Flag consequences I didn't ask about.** If my answer creates an abuse case,
  a hole in a state machine or a contradiction with an earlier decision, say so
  in a sentence or two, propose the fix, and keep going.
- **Anything needing an interactive terminal** (Docker, live servers, approval
  prompts, OAuth consent screens) goes into `docs/RUN-WHEN-BACK.md` as
  copy-paste steps — written, not guessed at.
- **`vi` is the editor** in every runbook, shell instruction and docs example.
  Never `nano`.
- **OS/server guides use full absolute paths**, never relative ones, so a
  copy-pasted command can never run in the wrong directory.
- Prefer editing existing files and reusing `platform/*` over new scaffolding.

---

## 3. Delivery workflow

The sequence I run every project through:

1. **Initial git setup** — repo, remotes, conventions, `CLAUDE.md`.
2. **Steven — preparation.** I give PRD and business-rules feedback, tuning and
   final confirmation. Nothing downstream starts until I confirm.
3. **Claude — build all documents A→Z** from the confirmed PRD/business rules.
4. **Claude — build all modules in one shot, A→Z.** Every module end to end.
5. **Claude — test, debug and security-harden, A→Z.** The whole system.
6. **Claude — production deployment handbook** (copy-paste, empty machine, full
   absolute paths), **then** the user guide, **then** the admin guide.

---

## 4. Architecture

Hexagonal / clean layering, dependencies pointing **inward only**:
`adapter → app → domain`, with `platform` available to all. The domain imports
no framework, no driver, no `net/http`, no SQL.

```
cmd/api/main.go            # thin entrypoint: wire + run subcommands (serve, migrate, seed)
internal/
  domain/                  # pure business logic + types; exhaustively unit-tested; no I/O
  app/                     # use-cases / services; orchestrates domain + ports
  adapter/
    http/                  #   handlers, request/response mapping
    postgres/              #   repositories (raw SQL on money paths)
    storage/               #   S3 / MinIO
    notify/                #   email / WhatsApp / outbound
  platform/                # cross-cutting infra, business-agnostic, reusable across projects
    config/ logging/ metrics/ apierror/ id/ security/ ratelimit/ database/
db/
  migrations/NNNN_name.up.sql + NNNN_name.down.sql
  embed.go                 # go:embed migrations
web/                       # SPA, if the project has a UI
```

`internal/platform/*` is meant to be **portable** — carry it between projects
and adapt rather than reinvent.

---

## 5. Language & stack preferences

**Backend: Go (latest).** `gin` for HTTP, `gorm` + `gorm.io/driver/postgres` for
persistence, `golang-jwt/jwt/v5`, `google/uuid` (v7), `minio-go/v7`,
`prometheus/client_golang`, `golang.org/x/crypto`. Standard library first;
a dependency has to earn its place.

**Database: PostgreSQL (latest major).** See section 6.

**Frontend (when there is a UI): React 18 + Vite + TypeScript + Tailwind.**
Pin React to 18 — not 19. Structure `web/src/{components,lib,pages}`. Node 20.
No PWA unless I ask for one.

**Not my defaults, don't reach for them unprompted:** an ORM's automigrate as
the source of truth, GraphQL, microservices, Kubernetes, a NoSQL primary store,
server-side rendering frameworks, CSS-in-JS.

---

## 6. Database conventions

- **Money is integers.** Store the appropriate whole unit as `BIGINT` and do all
  arithmetic in integers. Floating point is **prohibited** in any code path
  touching money. Percentages round half-up:
  `floor((amount * bps + 5000) / 10000)`. Rates are held in **basis points**.
- **Money paths use explicit raw SQL** (`gorm.Exec` / `Raw` with placeholders),
  never ORM arithmetic — even in a project where the ORM handles everything else.
- **Primary keys are UUIDv7** — time-ordered for index locality, not sequential
  in a way that leaks volume. Human-facing codes use CSPRNG + Crockford base32.
- **Migrations are numbered SQL**, `NNNN_name.up.sql` with a matching
  `.down.sql`, embedded via `go:embed`, **forward-only in production**. The
  migrations are the source of truth; ORM models map onto them.
- **The database enforces the invariant, not just the application.** Foreign
  keys, `NOT NULL`, `CHECK` constraints, partial and unique indexes. If a
  counter must never exceed a maximum, a `CHECK` says so, so the database itself
  refuses the bad write even under a race.
- **Concurrency is tested, not assumed.** Anything that reserves a limited
  resource takes `SELECT ... FOR UPDATE` (or a constraint-backed counter) inside
  one transaction, and ships with a concurrency test that proves it can't
  oversell.
- **Timestamps are `timestamptz` in UTC.** Business-day logic converts to the
  operating timezone **explicitly** — never rely on the server's local time.
- **Append-only tables for history** — events, audit log, payment events. No
  updates, no deletes; the table's migration spells that out.
- **Multi-tenant / multi-site scoping is a column plus an index plus a
  repository-layer filter**, e.g. `store_id NOT NULL`. Uniqueness constraints
  are per tenant, not global. Scope is enforced in the repository, not only in
  handlers.
- **Seed data lives in its own numbered migration** and is realistic enough to
  demo the product.

---

## 7. Security posture

Target **OWASP ASVS v4 Level 2** and cover every **OWASP Top 10 (2021)**
category explicitly, in a `docs/12-security.md` that maps each control to where
it is implemented **and to the test that proves it**. Non-negotiables:

- **Deny-by-default authorization.** Every handler declares its required
  permission. Every object read is scoped by owner **and** by tenant. Admin
  routes live in a separate router group. Negative authz and IDOR tests exist
  per role and per resource.
- **Passwords: argon2id** with tuned parameters. Never bcrypt-by-default, never
  plaintext, never a homegrown hash.
- **JWT: short-lived access (~15 min) + rotating refresh tokens** stored hashed
  and revocable, with a `jti` denylist on logout. A new token on any privilege
  change. OTP codes are hashed, 6 digits, single-use, short TTL, attempt-capped.
- **Injection:** parameter binding everywhere, raw SQL only with placeholders,
  never string concatenation. Allow-list validation at the adapter edge; the
  domain assumes valid input. No `dangerouslySetInnerHTML`. No shell-outs.
- **Rate limiting** per identifier and per IP on login, OTP, lookup and any
  brute-forceable endpoint, with progressive lockout and a documented unlock path.
- **File uploads** are type-checked by **magic bytes** (not extension), size- and
  dimension-limited, re-encoded, stored under generated names in a **private**
  bucket, served only by presigned URL.
- **Security headers** via middleware: CSP without `unsafe-inline`,
  `X-Content-Type-Options`, `X-Frame-Options: DENY` / `frame-ancestors 'none'`,
  `Referrer-Policy`, `Permissions-Policy`, HSTS. TLS 1.2+ only.
- **Secrets only via env/config.** Nothing secret in git; `.env.example` is the
  documented surface and the real `.env` is ignored. Rotation is documented.
  No default admin credentials — first-run setup flow instead.
- **Errors are typed** through `platform/apierror` and map to one consistent JSON
  error model. Driver errors, stack traces and internal messages never reach a
  client. Auth errors are generic and never reveal account existence.
- **Logging:** structured, with a request id propagated through context; an
  append-only audit log for every privileged action, price change, refund and
  parameter change (actor, before/after, IP, user agent). No PII in logs or URLs.
- **Idempotency** on every mutating endpoint that creates money or reserves
  capacity (`Idempotency-Key`).
- **Supply chain:** `govulncheck`, `gosec`, `staticcheck`, `npm audit` wired into
  both the Makefile and CI. Dependencies pinned.
- **Abuse cases are written down** with the control for each — resource
  squatting, OTP flooding, code brute-force, scraping, enumeration.
- **A security test suite ships with the product**: negative authz per role,
  IDOR per resource, rate-limit tests, injection fuzz on every input, a
  concurrency test, cross-tenant access tests, and JWT tampering/expiry tests.

---

## 8. Product & UI conventions

- **Search box on every list.** Every screen rendering a list or table has a
  debounced search box that filters that data. No exceptions — a list without
  search is incomplete.
- **Configurable values live in a `sys_parameters` table**, never hard-coded:
  company phone/email/address, tax rates, thresholds, feature toggles,
  operational timings. Every one ships with full CRUD (list + search, create,
  read, update, delete) behind an admin permission, is attributed
  (`updated_by`), and secret-flagged parameters are masked in UI and logs.
  If I might want to change it without a deploy, it is a parameter.
- **Operational timings are parameters too** — lead times, cutoffs, capacities,
  hold windows. I will retune these in production; never bake them into code.
- **Nothing automated cancels a customer's booking** unless I explicitly ask for
  it. Humans cancel; the system surfaces the queue for them.
- **Accessibility to WCAG AA**: measured contrast (state the ratios), visible
  focus rings, real labels, keyboard-operable pickers, announced errors,
  respects `prefers-reduced-motion` and `prefers-color-scheme`. Colour is never
  the only signal.
- **Mobile-first**, designed at 360px, light **and** dark themes as tokens.
- **Multi-language via message catalogues**, never inline strings.
- **Disabled states explain themselves** — show the reason, not a grey box.

---

## 9. Infrastructure defaults

- Development happens on a **shared dev server** (`claudedev`), not a laptop.
  Projects live at `/home/dev/projects/<project>`, per-project config at
  `/etc/<project>/<project>.env`, shared config at `/etc/claudedev/`.
- **nginx reverse-proxies each project's local port**; only 80/443 are open.
- **PostgreSQL runs natively** on the dev server and is shared across projects
  (one database per project, plus a `<project>_test` database for integration
  and concurrency tests). Don't stand up a second Postgres in Docker.
- **Docker is for the satellites** — MinIO, mailpit, WAHA — not for the database.
- **WhatsApp notifications go through WAHA** (self-hosted, one shared container)
  behind a `notify.Provider` port, with the official Meta Cloud API as the
  documented swap-in for production.
- **Every outbound integration sits behind a port/interface** with at least two
  implementations planned, so swapping a provider is an adapter change and never
  a reshape of the core flow.
- Production: single Ubuntu node, nginx + certbot TLS, native PostgreSQL, MinIO
  under systemd, multi-stage Dockerfile → small static Go binary, migrations run
  before the new binary serves, documented backups and rollback.

---

## 10. Doc set convention

Numbered, in `docs/`, kept in sync on every change:

| # | File | Purpose |
|---|---|---|
| 00 | `00-README-and-decisions.md` | Index, **decision log** (`D1…`, dated, with docs touched), open questions |
| 01 | `01-PRD.md` | Problem, personas, scope, requirements, metrics |
| 02 | `02-business-rules.md` | **Normative** — rules carry `BR-x.y` IDs; code comments and test names reference them |
| 03 | `03-data-model.md` | Schema, mermaid ERD, DDL, constraints, indexes, migration notes |
| 04 | `04-api-specification.md` | REST contract, error model, idempotency, auth, pagination |
| 05 | `05-architecture-and-nfr.md` | Architecture, security, performance, observability |
| 06 | `06-domain-operations.md` | Domain-specific operational logic & runbooks |
| 07 | `07-test-plan.md` | Strategy, critical scenarios, QA checklist |
| 08 | `08-roadmap.md` | Phasing and sequencing rationale |
| 09 | `09-deployment.md` | Production deployment, TLS, backups, rollback |
| 10 | `10-design-system.md` | Palette (with measured contrast), typography, components, a11y |
| 11 | `11-local-dev-setup.md` | Local/dev environment and everyday commands |
| 12 | `12-security.md` | ASVS L2 / Top-10 control map, abuse cases, security test suite |
| 13a | `13a-development-server-preparation.md` | Dev-server handbook — Part A (server once) + Part B (onboard a project) |
| 99 | `99-steven-preference.md` | This file — portable preferences |
| — | `PROGRESS.md` | Live build status: ✅ done & tested · 🟡 partial · ⬜ not started |
| — | `RUN-WHEN-BACK.md` | Copy-paste steps that need an interactive terminal |

Rules: `02` is normative and wins over the other docs on product logic;
every behaviour-changing decision gets a **dated row in the `00` decision log**
naming the docs it touched; `PROGRESS.md` is updated as work lands.

---

## 11. Things I don't want to see

- `nano` in a runbook, or relative paths in an OS guide.
- Floating point anywhere near money.
- Secrets in git, or a default admin password.
- A list screen without a search box.
- A configurable value hard-coded in a handler.
- Stopping halfway through an agreed A–Z step to ask whether to continue.
- "Done and tested" for something that was never run.
- Silent scope changes — narrowing, widening or reinterpreting what I asked for.
- Business logic in a handler, or a domain package importing a driver.
- An ORM's automigrate treated as the schema's source of truth.

---

## 12. New-project bootstrap checklist

- [ ] `git init`, remote, `main` as working branch, `.gitignore`, `.gitattributes`
- [ ] `CLAUDE.md` generated from this file + the project's domain and locale
- [ ] `docs/` set from section 10, with `00` decision log started at D1
- [ ] Ask the open product questions **in one batch, defaults proposed**
- [ ] `.env.example`, Makefile, Dockerfile, docker-compose (satellites only), CI
- [ ] `internal/platform/*` carried over and adapted
- [ ] Migrations `0001…` + realistic seed
- [ ] Domain packages with exhaustive unit tests referencing `BR-x.y`
- [ ] `sys_parameters` table + admin CRUD before any configurable value is used
- [ ] Auth, roles and the permissions matrix, with negative tests per role
- [ ] `docs/12-security.md` green, with the tests that prove each control
- [ ] Deployment handbook → user guide → admin guide
